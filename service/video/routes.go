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

type Handler struct {
	store     types.VideoStore
	userStore types.UserStore
}

func NewHandler(store types.VideoStore, userStore types.UserStore) *Handler {
	return &Handler{
		store:     store,
		userStore: userStore,
	}
}

func (h *Handler) RegisterRoutes(router *mux.Router) {
	// API 路由
	router.HandleFunc("/videos", auth.WithJWTAuth(h.handleGetVideos, h.userStore)).Methods(http.MethodGet)
	router.HandleFunc("/videos", auth.WithJWTAuth(h.handleUpload, h.userStore)).Methods(http.MethodPost)
	router.HandleFunc("/videos/{id}", auth.WithJWTAuth(h.handleGetVideo, h.userStore)).Methods(http.MethodGet)
	router.HandleFunc("/videos/{id}", auth.WithJWTAuth(h.handleDeleteVideo, h.userStore)).Methods(http.MethodDelete)

	// 静态文件服务 - 提供上传的视频文件访问
	router.PathPrefix("/uploads/").Handler(http.StripPrefix("/uploads/", http.FileServer(http.Dir(config.Envs.UploadDir))))
}

func (h *Handler) handleGetVideos(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	if userID == -1 {
		utils.WriteError(w, http.StatusUnauthorized, fmt.Errorf("unauthorized"))
		return
	}

	videos, err := h.store.GetVideos(userID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	utils.WriteJSON(w, http.StatusOK, videos)
}

func (h *Handler) handleGetVideo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("invalid video id"))
		return
	}

	video, err := h.store.GetVideoByID(id)
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, fmt.Errorf("video not found"))
		return
	}

	// 验证用户权限
	userID := auth.GetUserIDFromContext(r.Context())
	if video.UserID != userID {
		utils.WriteError(w, http.StatusForbidden, fmt.Errorf("permission denied"))
		return
	}

	utils.WriteJSON(w, http.StatusOK, video)
}

func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	if userID == -1 {
		utils.WriteError(w, http.StatusUnauthorized, fmt.Errorf("unauthorized"))
		return
	}

	// 限制文件大小
	r.Body = http.MaxBytesReader(w, r.Body, config.Envs.MaxUploadSize)
	if err := r.ParseMultipartForm(config.Envs.MaxUploadSize); err != nil {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("file too large"))
		return
	}

	// 获取表单数据
	title := r.FormValue("title")
	description := r.FormValue("description")

	if title == "" {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("title is required"))
		return
	}

	// 获取文件 - 支持 "video" 和 "file" 两个字段名
	file, header, err := r.FormFile("video")
	if err != nil {
		// 尝试使用 "file" 字段名（用于录制上传）
		file, header, err = r.FormFile("file")
		if err != nil {
			utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("failed to get file: %v", err))
			return
		}
	}
	defer file.Close()

	// 验证文件类型
	contentType := header.Header.Get("Content-Type")
	if !isValidVideoType(contentType) {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("invalid file type: %s", contentType))
		return
	}

	// 生成唯一文件名
	ext := filepath.Ext(header.Filename)
	fileName := fmt.Sprintf("%d_%s%s", userID, time.Now().Format("20060102_150405"), ext)
	filePath := filepath.Join(config.Envs.UploadDir, fileName)

	// 确保上传目录存在
	if err := os.MkdirAll(config.Envs.UploadDir, os.ModePerm); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	// 保存文件
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

	// 创建视频记录
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

	utils.WriteJSON(w, http.StatusCreated, video)
}

func (h *Handler) handleDeleteVideo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("invalid video id"))
		return
	}

	video, err := h.store.GetVideoByID(id)
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, fmt.Errorf("video not found"))
		return
	}

	// 验证用户权限
	userID := auth.GetUserIDFromContext(r.Context())
	if video.UserID != userID {
		utils.WriteError(w, http.StatusForbidden, fmt.Errorf("permission denied"))
		return
	}

	// 删除数据库记录
	if err := h.store.DeleteVideo(id); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	// 删除文件
	if err := os.Remove(video.FilePath); err != nil {
		// 记录错误但不返回失败（文件可能已被删除）
		fmt.Printf("warning: failed to delete file %s: %v\n", video.FilePath, err)
	}

	utils.WriteJSON(w, http.StatusOK, map[string]string{"message": "video deleted successfully"})
}

func isValidVideoType(contentType string) bool {
	validTypes := []string{
		"video/mp4",
		"video/mpeg",
		"video/quicktime",
		"video/x-msvideo",
		"video/x-ms-wmv",
		"video/webm",       // 添加 webm 支持
		"video/x-matroska", // 添加 matroska 支持
	}

	for _, t := range validTypes {
		if t == contentType {
			return true
		}
	}
	return false
}
