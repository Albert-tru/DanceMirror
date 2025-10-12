package user

import (
	"fmt"
	"net/http"

	"github.com/Albert-tru/DanceMirror/config"
	"github.com/Albert-tru/DanceMirror/service/auth"
	"github.com/Albert-tru/DanceMirror/types"
	"github.com/Albert-tru/DanceMirror/utils"
	"github.com/gorilla/mux"
)

type Handler struct {
	store types.UserStore
}

func NewHandler(store types.UserStore) *Handler {
	return &Handler{store: store}
}

func (h *Handler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/register", h.handleRegister).Methods(http.MethodPost)
	router.HandleFunc("/login", h.handleLogin).Methods(http.MethodPost)
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
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

	utils.WriteJSON(w, http.StatusOK, map[string]string{"token": token})
}
