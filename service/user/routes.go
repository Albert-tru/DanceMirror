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
	return &Handler{store: store, limiter: ratelimit.NewLimiter()}
}

func (h *Handler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/register", h.handleRegister).Methods(http.MethodPost)
	router.HandleFunc("/login", h.handleLogin).Methods(http.MethodPost)
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	clientIP := ratelimit.ClientIP(r)
	if ok, resetAt := h.limiter.Allow("register:ip:"+clientIP, 3, time.Hour); !ok {
		writeRateLimited(w, resetAt, "Too many register attempts from this IP, please try again later")
		return
	}

	var payload types.RegisterUserPayload
	if err := utils.ParseJSON(r, &payload); err != nil {
		utils.WriteError(w, http.StatusBadRequest, err)
		return
	}

	if err := utils.Validate.Struct(payload); err != nil {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("invalid payload: %v", err))
		return
	}

	if _, err := h.store.GetUserByPhone(payload.Phone); err == nil {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("手机号 %s 已被注册", payload.Phone))
		return
	}

	if payload.Email != "" {
		if _, err := h.store.GetUserByEmail(payload.Email); err == nil {
			utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("邮箱 %s 已被注册", payload.Email))
			return
		}
	}

	hashedPassword, err := auth.HashPassword(payload.Password)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	err = h.store.CreateUser(types.User{
		Phone:         payload.Phone,
		Password:      hashedPassword,
		Email:         payload.Email,
		FirstName:     payload.FirstName,
		LastName:      payload.LastName,
		AccountStatus: "active",
		Role:          "user",
	})
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	_ = utils.WriteJSON(w, http.StatusCreated, map[string]string{"message": "user created successfully"})
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var payload types.LoginUserPayload
	if err := utils.ParseJSON(r, &payload); err != nil {
		utils.WriteError(w, http.StatusBadRequest, err)
		return
	}

	clientIP := ratelimit.ClientIP(r)
	if ok, resetAt := h.limiter.Allow("login:ip:"+clientIP, 30, time.Minute); !ok {
		writeRateLimited(w, resetAt, "Too many login attempts from this IP")
		return
	}
	if payload.Phone != "" {
		phoneKey := fmt.Sprintf("login:phone:%s", payload.Phone)
		if ok, resetAt := h.limiter.Allow(phoneKey, 5, time.Minute); !ok {
			writeRateLimited(w, resetAt, "Too many login attempts for this account")
			return
		}
	}

	if err := utils.Validate.Struct(payload); err != nil {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("invalid payload: %v", err))
		return
	}

	u, err := h.store.GetUserByPhone(payload.Phone)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("手机号或密码错误"))
		return
	}

	if !auth.ComparePasswords(u.Password, []byte(payload.Password)) {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("手机号或密码错误"))
		return
	}

	if u.AccountStatus == "" {
		u.AccountStatus = "active"
	}
	if u.Role == "" {
		u.Role = "user"
	}

	if s, ok := h.store.(*Store); ok {
		_ = s.UpdateLoginMeta(u.ID, time.Now())
	}

	secret := []byte(config.Envs.JWTSecret)
	token, err := auth.CreateJWT(secret, u.ID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	_ = utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
		"user": map[string]interface{}{
			"id":            u.ID,
			"phone":         u.Phone,
			"email":         u.Email,
			"firstName":     u.FirstName,
			"lastName":      u.LastName,
			"nickname":      u.Nickname,
			"accountStatus": u.AccountStatus,
			"role":          u.Role,
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
