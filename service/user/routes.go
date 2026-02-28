package user

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Albert-tru/DanceMirror/config"
	"github.com/Albert-tru/DanceMirror/pkg/ratelimit"
	"github.com/Albert-tru/DanceMirror/service/auth"
	"github.com/Albert-tru/DanceMirror/types"
	"github.com/Albert-tru/DanceMirror/utils"
	"github.com/gorilla/mux"
)

type Handler struct {
	store   types.UserStore
	limiter *ratelimit.Limiter
}

func NewHandler(store types.UserStore) *Handler {
	return &Handler{
		store:   store,
		limiter: ratelimit.NewLimiter(),
	}
}

func (h *Handler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/register", h.handleRegister).Methods(http.MethodPost)
	router.HandleFunc("/login", h.handleLogin).Methods(http.MethodPost)
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {

	// 限流
	clientIP := ratelimit.ClientIP(r)
	if ok, resetAt := h.limiter.Allow("register:ip:"+clientIP, 3, time.Hour); !ok {
		writeRateLimited(w, resetAt, "Too many register attempts from this IP, please try again later")
		return
	}

	// 解析请求
	var payload types.RegisterUserPayload
	if err := utils.ParseJSON(r, &payload); err != nil {
		utils.WriteError(w, http.StatusBadRequest, err)
		return
	}

	// 验证请求
	if err := utils.Validate.Struct(payload); err != nil {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("invalid payload: %v", err))
		return
	}

	// 检查手机号是否已存在
	_, err := h.store.GetUserByPhone(payload.Phone)
	if err == nil {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("手机号 %s 已被注册", payload.Phone))
		return
	}

	// 检查邮箱是否已存在（如果提供了邮箱）
	if payload.Email != "" {
		_, err = h.store.GetUserByEmail(payload.Email)
		if err == nil {
			utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("邮箱 %s 已被注册", payload.Email))
			return
		}
	}

	// 加密密码
	hashedPassword, err := auth.HashPassword(payload.Password)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	// 创建用户
	err = h.store.CreateUser(types.User{
		Phone:     payload.Phone,
		Password:  hashedPassword,
		Email:     payload.Email,
		FirstName: payload.FirstName,
		LastName:  payload.LastName,
	})
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	utils.WriteJSON(w, http.StatusCreated, map[string]string{"message": "user created successfully"})
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	// 解析请求
	var payload types.LoginUserPayload
	if err := utils.ParseJSON(r, &payload); err != nil {
		utils.WriteError(w, http.StatusBadRequest, err)
		return
	}

	clientIP := ratelimit.ClientIP(r)

	// 限流1: 每个IP每分钟最多30次登录尝试 (防爆破兜底)
	if ok, resetAt := h.limiter.Allow("login:ip:"+clientIP, 30, time.Minute); !ok {
		writeRateLimited(w, resetAt, "Too many login attempts from this IP")
		return
	}

	// 限流2: 每个手机号每分钟最多5次登录尝试 (防特定账号爆破)
	if payload.Phone != "" {
		phoneKey := fmt.Sprintf("login:phone:%s", payload.Phone)
		if ok, resetAt := h.limiter.Allow(phoneKey, 5, time.Minute); !ok {
			writeRateLimited(w, resetAt, "Too many login attempts for this account")
			return
		}
	}

	// 验证请求
	if err := utils.Validate.Struct(payload); err != nil {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("invalid payload: %v", err))
		return
	}

	// 通过手机号查找用户
	u, err := h.store.GetUserByPhone(payload.Phone)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("手机号或密码错误"))
		return
	}

	// 验证密码
	if !auth.ComparePasswords(u.Password, []byte(payload.Password)) {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("手机号或密码错误"))
		return
	}

	// 生成 JWT
	secret := []byte(config.Envs.JWTSecret)
	token, err := auth.CreateJWT(secret, u.ID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
		"user": map[string]interface{}{
			"id":        u.ID,
			"phone":     u.Phone,
			"email":     u.Email,
			"firstName": u.FirstName,
			"lastName":  u.LastName,
		},
	})
}

func writeRateLimited(w http.ResponseWriter, resetAt time.Time, msg string) {
	retryAfter := int(time.Until(resetAt).Seconds())
	if retryAfter < 1 {
		retryAfter = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	utils.WriteError(w, http.StatusTooManyRequests, fmt.Errorf("%s", msg))
}
