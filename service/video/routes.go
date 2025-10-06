package video

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Albert-tru/DanceMirror/config"
	"github.com/Albert-tru/DanceMirror/service/auth"
	"github.com/Albert-tru/DanceMirror/types"
	"github.com/Albert-tru/DanceMirror/utils"
	"github.com/gorilla/mux"
)

// Handler 处理视频相关的 HTTP 请求
type Handler struct {
	store     types.VideoStore // 视频数据库操作
	userStore types.UserStore  // 用户数据库操作（用于验证权限）
}

// NewHandler 创建一个新的视频处理器
func NewHandler(store types.VideoStore, userStore types.UserStore) *Handler {
	return &Handler{
		store:     store,
		userStore: userStore,
	}
}

// RegisterRoutes 注册所有视频相关的路由
func (h *Handler) RegisterRoutes(router *mux.Router) {
	// 所有这些路由都需要登录（通过 JWT 验证）
	router.HandleFunc("/videos", auth.WithJWTAuth(h.handleGetVideos, h.userStore)).Methods(http.MethodGet)
	router.HandleFunc("/videos/upload", auth.WithJWTAuth(h.handleUpload, h.userStore)).Methods(http.MethodPost)
	router.HandleFunc("/videos/{id}", auth.WithJWTAuth(h.handleGetVideo, h.userStore)).Methods(http.MethodGet)
	router.HandleFunc("/videos/{id}", auth.WithJWTAuth(h.handleDeleteVideo, h.userStore)).Methods(http.MethodDelete)
}

// handleGetVideos 获取当前用户的所有视频
func (h *Handler) handleGetVideos(w http.ResponseWriter, r *http.Request) {
	// 1. 从 JWT Token 中获取用户 ID
	userID := auth.GetUserIDFromContext(r.Context())
	if userID == -1 {
		utils.WriteError(w, http.StatusUnauthorized, fmt.Errorf("未授权"))
		return
	}

	// 2. 查询数据库，获取该用户的所有视频
	videos, err := h.store.GetVideosByUserID(userID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	// 3. 返回 JSON 格式的视频列表
	utils.WriteJSON(w, http.StatusOK, videos)
}

// handleGetVideo 获取某个视频的详细信息
func (h *Handler) handleGetVideo(w http.ResponseWriter, r *http.Request) {
	// 1. 从 URL 中获取视频 ID（例如 /videos/123）
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("无效的视频 ID"))
		return
	}

	// 2. 从数据库查询视频信息
	video, err := h.store.GetVideoByID(id)
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, fmt.Errorf("视频不存在"))
		return
	}

	// 3. 权限检查：只能查看自己的视频
	userID := auth.GetUserIDFromContext(r.Context())
	if video.UserID != userID {
		utils.WriteError(w, http.StatusForbidden, fmt.Errorf("没有权限"))
		return
	}

	// 4. 返回视频信息
	utils.WriteJSON(w, http.StatusOK, video)
}

// handleUpload 处理视频上传
func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	// 1. 获取当前用户 ID
	userID := auth.GetUserIDFromContext(r.Context())
	if userID == -1 {
		utils.WriteError(w, http.StatusUnauthorized, fmt.Errorf("未授权"))
		return
	}

	// 2. 限制上传文件大小（防止超大文件）
	r.Body = http.MaxBytesReader(w, r.Body, config.Envs.MaxUploadSize)
	if err := r.ParseMultipartForm(config.Envs.MaxUploadSize); err != nil {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("文件太大"))
		return
	}

	// 3. 获取表单数据（标题、描述）
	title := r.FormValue("title")
	description := r.FormValue("description")

	if title == "" {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("标题不能为空"))
		return
	}

	// 4. 获取上传的文件
	file, header, err := r.FormFile("video")
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("获取文件失败: %v", err))
		return
	}
	defer file.Close()

	// 5. 验证文件类型（只允许视频格式）
	contentType := header.Header.Get("Content-Type")
	if !isValidVideoType(contentType) {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("不支持的文件类型: %s", contentType))
		return
	}

	// 6. 生成唯一的文件名（用户ID_时间戳.扩展名）
	ext := filepath.Ext(header.Filename)
	fileName := fmt.Sprintf("%d_%s%s", userID, time.Now().Format("20060102_150405"), ext)
	filePath := filepath.Join(config.Envs.UploadDir, fileName)

	// 7. 确保上传目录存在
	if err := os.MkdirAll(config.Envs.UploadDir, os.ModePerm); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	// 8. 保存文件到磁盘
	dst, err := os.Create(filePath)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	// 9. 创建视频记录到数据库
	video := &types.Video{
		UserID:      userID,
		Title:       title,
		Description: description,
		FilePath:    filePath,
		FileName:    fileName,
		FileSize:    header.Size,
	}

	if err := h.store.CreateVideo(video); err != nil {
		// 如果数据库保存失败，删除已上传的文件
		os.Remove(filePath)
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	// 10. 返回上传成功的视频信息
	utils.WriteJSON(w, http.StatusCreated, video)
}

// handleDeleteVideo 删除视频
func (h *Handler) handleDeleteVideo(w http.ResponseWriter, r *http.Request) {
	// 1. 获取视频 ID
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("无效的视频 ID"))
		return
	}

	// 2. 查询视频是否存在
	video, err := h.store.GetVideoByID(id)
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, fmt.Errorf("视频不存在"))
		return
	}

	// 3. 权限检查：只能删除自己的视频
	userID := auth.GetUserIDFromContext(r.Context())
	if video.UserID != userID {
		utils.WriteError(w, http.StatusForbidden, fmt.Errorf("没有权限"))
		return
	}

	// 4. 从数据库删除记录
	if err := h.store.DeleteVideo(id); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	// 5. 删除文件（如果失败也不影响）
	if err := os.Remove(video.FilePath); err != nil {
		fmt.Printf("警告: 删除文件失败 %s: %v\n", video.FilePath, err)
	}

	// 6. 返回成功消息
	utils.WriteJSON(w, http.StatusOK, map[string]string{"message": "视频已删除"})
}

// isValidVideoType 检查是否是有效的视频格式
func isValidVideoType(contentType string) bool {
	validTypes := []string{
		"video/mp4",
		"video/mpeg",
		"video/quicktime",
		"video/x-msvideo",
		"video/x-ms-wmv",
	}

	for _, t := range validTypes {
		if t == contentType {
			return true
		}
	}
	return false
}
