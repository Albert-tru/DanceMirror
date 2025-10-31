package video

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

	// 视频裁剪相关路由
	router.HandleFunc("/crop-video", auth.WithJWTAuth(h.handleCropVideo, h.userStore)).Methods(http.MethodPost)
	router.HandleFunc("/crop-available", auth.WithJWTAuth(h.handleCheckCropAvailable, h.userStore)).Methods(http.MethodGet)

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

// checkVideoOwnership 验证用户是否是视频的所有者
// 返回视频对象和错误信息（如果有）
func (h *Handler) checkVideoOwnership(w http.ResponseWriter, r *http.Request, videoID int) (*types.Video, error) {
	// 获取当前用户 ID
	userID := auth.GetUserIDFromContext(r.Context())
	if userID == -1 {
		return nil, fmt.Errorf("unauthorized")
	}

	// 查询视频
	video, err := h.store.GetVideoByID(videoID)
	if err != nil {
		return nil, fmt.Errorf("video not found")
	}

	// 权限校验：只有视频所有者才能访问
	if video.UserID != userID {
		return nil, fmt.Errorf("permission denied: you are not the owner of this video")
	}

	return video, nil
}

func (h *Handler) handleGetVideo(w http.ResponseWriter, r *http.Request) {
	// 解析视频 ID
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("invalid video id"))
		return
	}

	// 权限验证
	video, err := h.checkVideoOwnership(w, r, id)
	if err != nil {
		if err.Error() == "unauthorized" {
			utils.WriteError(w, http.StatusUnauthorized, err)
		} else if err.Error() == "video not found" {
			utils.WriteError(w, http.StatusNotFound, err)
		} else {
			utils.WriteError(w, http.StatusForbidden, err)
		}
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
	tempDir := filepath.Join(config.Envs.UploadDir, "temp")
	finalDir := config.Envs.UploadDir
	tempPath := filepath.Join(tempDir, fileName)
	finalPath := filepath.Join(finalDir, fileName)

	// 确保上传目录存在
	if err := os.MkdirAll(tempDir, os.ModePerm); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, fmt.Errorf("failed to create temp directory: %v", err))
		return
	}
	if err := os.MkdirAll(finalDir, os.ModePerm); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, fmt.Errorf("failed to create upload directory: %v", err))
		return
	}

	// 使用事务保护：文件保存 + 数据库插入的原子性
	var video *types.Video
	uploadErr := func() error {
		// 步骤 1: 保存文件到临时目录
		dst, err := os.Create(tempPath)
		if err != nil {
			return fmt.Errorf("failed to create temp file: %v", err)
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			os.Remove(tempPath) // 清理临时文件
			return fmt.Errorf("failed to save file: %v", err)
		}

		// 步骤 2: 创建数据库记录（事务保护）
		video = &types.Video{
			UserID:      userID,
			Title:       title,
			Description: description,
			FilePath:    "/uploads/" + fileName, // 存储Web可访问路径
			FileName:    fileName,
			FileSize:    header.Size,
		}
		if err := h.store.CreateVideo(video); err != nil {
			os.Remove(tempPath) // 清理临时文件
			return fmt.Errorf("failed to create video record: %v", err)
		}

		// 步骤 3: 移动文件到正式目录（原子操作）
		if err := os.Rename(tempPath, finalPath); err != nil {
			// 数据库插入成功但文件移动失败，回滚数据库记录
			h.store.DeleteVideo(video.ID)
			os.Remove(tempPath) // 清理临时文件
			return fmt.Errorf("failed to move file to final location: %v", err)
		}

		return nil
	}()

	if uploadErr != nil {
		utils.WriteError(w, http.StatusInternalServerError, uploadErr)
		return
	}

	utils.WriteJSON(w, http.StatusCreated, video)
}

func (h *Handler) handleDeleteVideo(w http.ResponseWriter, r *http.Request) {
	// 解析视频 ID
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("invalid video id"))
		return
	}

	// 权限验证
	video, err := h.checkVideoOwnership(w, r, id)
	if err != nil {
		if err.Error() == "unauthorized" {
			utils.WriteError(w, http.StatusUnauthorized, err)
		} else if err.Error() == "video not found" {
			utils.WriteError(w, http.StatusNotFound, err)
		} else {
			utils.WriteError(w, http.StatusForbidden, err)
		}
		return
	}

	// 删除数据库记录（软删除）
	if err := h.store.DeleteVideo(id); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, fmt.Errorf("failed to delete video record: %v", err))
		return
	}

	// 删除物理文件 - 将Web路径转换为文件系统路径
	filePath := video.FilePath
	if strings.HasPrefix(filePath, "/uploads/") {
		// 转换为完整文件系统路径
		filePath = filepath.Join(config.Envs.UploadDir, strings.TrimPrefix(filePath, "/uploads/"))
	}
	if err := os.Remove(filePath); err != nil {
		// 记录错误但不返回失败（文件可能已被删除或移动）
		fmt.Printf("warning: failed to delete file %s: %v\n", filePath, err)
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
		"video/webm",       // 支持 webm 格式（浏览器录制）
		"video/x-matroska", // 支持 mkv 格式
	}

	for _, t := range validTypes {
		if t == contentType {
			return true
		}
	}
	return false
}

// handleCropVideo 处理视频裁剪请求
func (h *Handler) handleCropVideo(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	if userID == -1 {
		utils.WriteError(w, http.StatusUnauthorized, fmt.Errorf("unauthorized"))
		return
	}

	// 限制请求大小（最大 500MB）
	r.Body = http.MaxBytesReader(w, r.Body, config.Envs.MaxUploadSize)
	if err := r.ParseMultipartForm(config.Envs.MaxUploadSize); err != nil {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("file too large"))
		return
	}

	// 获取裁剪参数
	cropXStr := r.FormValue("cropX")
	cropYStr := r.FormValue("cropY")
	cropWStr := r.FormValue("cropW")
	cropHStr := r.FormValue("cropH")

	if cropXStr == "" || cropYStr == "" || cropWStr == "" || cropHStr == "" {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("missing crop parameters"))
		return
	}

	// 解析裁剪参数
	cropParams, err := ParseCropParams(cropXStr, cropYStr, cropWStr, cropHStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, err)
		return
	}

	// 获取上传的视频文件
	file, header, err := r.FormFile("video")
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("failed to get video file: %v", err))
		return
	}
	defer file.Close()

	// 验证文件类型
	contentType := header.Header.Get("Content-Type")
	if !isValidVideoType(contentType) {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("invalid video type: %s", contentType))
		return
	}

	// 创建临时目录和输出目录
	tempDir := GetTempCropDir()
	outputDir := GetCroppedOutputDir()
	if err := os.MkdirAll(tempDir, os.ModePerm); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, fmt.Errorf("failed to create temp directory: %v", err))
		return
	}
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, fmt.Errorf("failed to create output directory: %v", err))
		return
	}

	// 保存上传的文件到临时目录
	ext := filepath.Ext(header.Filename)
	tempFilename := fmt.Sprintf("%d_temp_%s%s", userID, time.Now().Format("20060102_150405"), ext)
	tempPath := filepath.Join(tempDir, tempFilename)

	tempFile, err := os.Create(tempPath)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, fmt.Errorf("failed to create temp file: %v", err))
		return
	}

	_, err = io.Copy(tempFile, file)
	tempFile.Close()
	if err != nil {
		CleanupTempFile(tempPath)
		utils.WriteError(w, http.StatusInternalServerError, fmt.Errorf("failed to save temp file: %v", err))
		return
	}

	// 生成输出文件名
	outputFilename := GenerateCroppedFilename(header.Filename, userID)
	outputPath := filepath.Join(outputDir, outputFilename)

	// 执行视频裁剪
	if err := CropVideo(tempPath, outputPath, cropParams); err != nil {
		CleanupTempFile(tempPath)
		utils.WriteError(w, http.StatusInternalServerError, fmt.Errorf("video crop failed: %v", err))
		return
	}

	// 清理临时文件
	CleanupTempFile(tempPath)

	// 返回裁剪后的视频文件
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", outputFilename))

	http.ServeFile(w, r, outputPath)

	// 异步清理裁剪后的文件（延迟删除，确保下载完成）
	go func() {
		time.Sleep(5 * time.Minute)
		CleanupTempFile(outputPath)
	}()
}

// handleCheckCropAvailable 检查服务器端裁剪是否可用
func (h *Handler) handleCheckCropAvailable(w http.ResponseWriter, r *http.Request) {
	available := CheckFFmpegAvailable()
	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"available": available,
		"message":   getFFmpegStatusMessage(available),
	})
}

// getFFmpegStatusMessage 获取 FFmpeg 状态消息
func getFFmpegStatusMessage(available bool) string {
	if available {
		return "Server-side FFmpeg cropping is available"
	}
	return "FFmpeg is not installed on the server, please use client-side cropping"
}
