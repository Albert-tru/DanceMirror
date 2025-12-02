package video

//处理视频相关的HTTP路由和请求

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Albert-tru/DanceMirror/config"
	"github.com/Albert-tru/DanceMirror/service/auth"
	"github.com/Albert-tru/DanceMirror/service/cache"
	"github.com/Albert-tru/DanceMirror/service/storage"
	"github.com/Albert-tru/DanceMirror/types"
	"github.com/Albert-tru/DanceMirror/utils"
	"github.com/gorilla/mux"
)

type Handler struct {
	store     types.VideoStore
	userStore types.UserStore
	cache     *cache.RedisClient
	storage   storage.VideoStorage
}

func NewHandler(store types.VideoStore, userStore types.UserStore, cache *cache.RedisClient, storage storage.VideoStorage) *Handler {
	return &Handler{
		store:     store,
		userStore: userStore,
		cache:     cache,
		storage:   storage,
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

	// 静态文件服务 - 只在本地存储模式下启用
	if config.Envs.StorageDriver != "minio" {
		router.PathPrefix("/uploads/").Handler(http.StripPrefix("/uploads/", http.FileServer(http.Dir(config.Envs.UploadDir))))
	}
}

func (h *Handler) handleGetVideos(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())

	// 1. 先尝试从缓存获取视频列表
	videos, err := h.cache.GetUserVideos(userID)
	if err == nil {
		//为缓存中的视频生成预签名URL
		for i := range videos {
			// 生成预签名 URL，有效期为 15 分钟
			signedURL, err := h.storage.GetPresignedURL(videos[i].FilePath, time.Minute*15)
			if err != nil {
				log.Printf("警告：为视频 ID %d 生成预签名 URL 失败: %v", videos[i].ID, err)
				continue
			}
			videos[i].FilePath = signedURL
		}

		// 缓存命中，直接返回
		log.Printf("✅ 缓存命中 - 用户 %d 的视频列表", userID)
		utils.WriteJSON(w, http.StatusOK, videos)
		return
	}

	// 2. 缓存未命中，查询数据库
	log.Printf("⚠️  缓存未命中 - 查询数据库 (原因: %v)", err)
	videos, err = h.store.GetVideos(userID) // 直接复用 videos 变量
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	// 为数据库查询到的视频生成预签名URL
	for i := range videos {
		// 生成预签名 URL，有效期为 15 分钟
		signedURL, err := h.storage.GetPresignedURL(videos[i].ObjectKey, time.Minute*15)
		if err != nil {
			log.Printf("警告：为视频 ID %d 生成预签名 URL 失败: %v", videos[i].ID, err)
			continue
		}
		videos[i].FilePath = signedURL
	}

	// 3. 将从数据库查到的结果存入缓存
	if err := h.cache.CacheUserVideos(userID, videos); err != nil {
		// 缓存失败不应阻塞主流程，记录日志即可
		log.Printf("警告：缓存用户 %d 的视频列表失败: %v", userID, err)
	}

	// 4. 返回从数据库查到的结果
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

	//尝试从缓存获取视频信息
	cacheVideo, err := h.cache.GetVideoByID(id)
	if err == nil {
		userID := auth.GetUserIDFromContext(r.Context()) // 权限验证
		if cacheVideo.UserID == userID {

			//为缓存中的视频生成预签名URL
			if cacheVideo.ObjectKey != "" {
				signedURL, err := h.storage.GetPresignedURL(cacheVideo.ObjectKey, time.Minute*15)
				if err != nil {
					log.Printf("警告：为视频 ID %d 生成预签名 URL 失败: %v", cacheVideo.ID, err)
				}
				cacheVideo.FilePath = signedURL
			}

			// 缓存命中且权限验证通过
			log.Printf("✅ 缓存命中 - 视频 ID %d", id)
			utils.WriteJSON(w, http.StatusOK, cacheVideo)
			return
		}
	}

	log.Printf("⚠️  缓存未命中 - 查询数据库 视频 ID %d (原因: %v)", id, err)

	// 从数据库获取视频信息
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

	//为数据库查询的视频生成预签名URL
	if video.ObjectKey != "" {
		signedURL, err := h.storage.GetPresignedURL(video.ObjectKey, time.Minute*15)
		if err != nil {
			log.Printf("警告：为视频 ID %d 生成预签名 URL 失败: %v", video.ID, err)
		} else {
			video.FilePath = signedURL
		}
	}

	// 将视频信息缓存起来
	if err := h.cache.CacheVideoByID(video); err != nil {
		log.Printf("警告：缓存视频 ID %d 失败: %v", id, err)
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

	// 验证文件类型（修改这部分）
	contentType := header.Header.Get("Content-Type")

	// 优先检查 Content-Type
	if !isValidVideoType(contentType) {
		// 如果 Content-Type 是 application/octet-stream，则检查文件扩展名
		if contentType == "application/octet-stream" {
			if !isValidVideoExtension(header.Filename) {
				utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("invalid file type: %s (filename: %s)", contentType, header.Filename))
				return
			}
			// 扩展名有效，继续处理
		} else {
			utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("invalid file type: %s", contentType))
			return
		}
	}

	// 生成对象键（MinIO中的唯一标识）
	objectKey := fmt.Sprintf("videos/%d/%d_%s", userID, time.Now().UnixNano(), header.Filename)

	// 读取文件内容到内存
	buf := new(bytes.Buffer)
	fileSize, err := io.Copy(buf, file)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, fmt.Errorf("failed to read file: %v", err))
		return
	}
	// 上传到 MinIO
	if err := h.storage.Upload(context.Background(), objectKey, bytes.NewReader(buf.Bytes()), fileSize, contentType); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, fmt.Errorf("failed to upload file to storage: %v", err))
		return
	}

	// 创建视频记录
	video := &types.Video{
		UserID:      userID,
		Title:       title,
		Description: description,
		ObjectKey:   objectKey,
		FilePath:    "", // MinIO 存储不使用本地文件路径
		FileName:    header.Filename,
		FileSize:    fileSize,
	}

	if err := h.store.CreateVideo(video); err != nil {
		// 上传失败，删除已上传的文件
		h.storage.Delete(context.Background(), objectKey)
		utils.WriteError(w, http.StatusInternalServerError, fmt.Errorf("failed to create video record: %v", err))
		return
	}

	//上传成功后，清除该用户的视频列表缓存
	if err := h.cache.ClearUserVideosCache(userID); err != nil {
		log.Printf("警告：清除用户 %d 视频列表缓存失败: %v", userID, err)
	} else {
		log.Printf("✅ 成功清除用户 %d 视频列表缓存", userID)
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

	// 从存储中删除文件（MinIO 或本地）
	if video.ObjectKey != "" {
		if err := h.storage.Delete(r.Context(), video.ObjectKey); err != nil {
			log.Printf("⚠️ 删除存储文件失败 (ObjectKey: %s): %v", video.ObjectKey, err)
		} else {
			log.Printf("✅ 成功删除存储文件: %s", video.ObjectKey)
		}
	} else if video.FilePath != "" {
		// 向后兼容：处理旧的本地文件路径
		filePath := video.FilePath
		if strings.HasPrefix(filePath, "/uploads/") {
			filePath = filepath.Join(config.Envs.UploadDir, strings.TrimPrefix(filePath, "/uploads/"))
		}
		if err := os.Remove(filePath); err != nil {
			log.Printf("⚠️ 删除本地文件失败: %v", err)
		}
	}

	// 删除数据库记录（软删除）
	if err := h.store.DeleteVideo(id); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, fmt.Errorf("failed to delete video record: %v", err))
		return
	}

	// 删除缓存
	userID := auth.GetUserIDFromContext(r.Context())
	if err := h.cache.ClearUserVideosCache(userID); err != nil {
		log.Printf("警告：清除用户 %d 视频列表缓存失败: %v", userID, err)
	} else {
		log.Printf("✅ 成功清除用户 %d 视频列表缓存", userID)
	}
	if err := h.cache.ClearVideoCache(id); err != nil {
		log.Printf("警告：删除视频 ID %d 缓存失败: %v", id, err)
	} else {
		log.Printf("✅ 成功删除视频 ID %d 缓存", id)
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

// 新增：根据文件名检查是否为视频文件
func isValidVideoExtension(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	validExts := []string{".mp4", ".mpeg", ".mpg", ".mov", ".avi", ".wmv", ".webm", ".mkv"}

	for _, validExt := range validExts {
		if ext == validExt {
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
