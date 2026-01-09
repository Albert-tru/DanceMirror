package video

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Albert-tru/DanceMirror/config"
	"github.com/Albert-tru/DanceMirror/service/auth"
	"github.com/Albert-tru/DanceMirror/service/cache"
	"github.com/Albert-tru/DanceMirror/service/storage"
	"github.com/Albert-tru/DanceMirror/types"
	"github.com/Albert-tru/DanceMirror/utils"
	"github.com/gorilla/mux"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Handler struct {
	store       types.VideoStore
	userStore   types.UserStore
	cache       *cache.RedisClient
	storage     storage.VideoStorage
	publishCrop func(task map[string]interface{}) error
}

func NewHandler(store types.VideoStore, userStore types.UserStore, cache *cache.RedisClient, storage storage.VideoStorage, publishCrop func(task map[string]interface{}) error) *Handler {
	return &Handler{
		store:       store,
		userStore:   userStore,
		cache:       cache,
		storage:     storage,
		publishCrop: publishCrop,
	}
}

func (h *Handler) RegisterRoutes(router *mux.Router) {
	// ✅ 新增：视频流代理路由，避免 COEP 问题
	router.HandleFunc("/videos/stream/{key:.*}", h.handleProxyVideo).Methods(http.MethodGet)

	// 基础视频路由
	router.HandleFunc("/videos", auth.WithJWTAuth(h.handleGetVideos, h.userStore)).Methods(http.MethodGet)
	router.HandleFunc("/videos", auth.WithJWTAuth(h.handleUpload, h.userStore)).Methods(http.MethodPost)
	router.HandleFunc("/videos/{id}", auth.WithJWTAuth(h.handleGetVideo, h.userStore)).Methods(http.MethodGet)
	router.HandleFunc("/videos/{id}", auth.WithJWTAuth(h.handleDeleteVideo, h.userStore)).Methods(http.MethodDelete)
	router.HandleFunc("/videos/{id}/crop", auth.WithJWTAuth(h.handleDispatchCropTask, h.userStore)).Methods(http.MethodPost)

	// 静态文件服务
	if config.Envs.StorageDriver != "minio" {
		router.PathPrefix("/uploads/").Handler(http.StripPrefix("/uploads/", http.FileServer(http.Dir(config.Envs.UploadDir))))
	}
}

// ✅ 新增：代理 MinIO 视频流，避免浏览器 COEP 拦截
func (h *Handler) handleProxyVideo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"] // 例如 videos/1/cropped_25.mp4

	// 防止路径遍历攻击
	if key == "" || key[0] == '.' {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("invalid key"))
		return
	}

	// 从 MinIO 获取对象
	ctx := context.Background()
	obj, err := h.getMiniOClient().GetObject(ctx, config.Envs.MinIOBucket, key, minio.GetObjectOptions{})
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, fmt.Errorf("file not found"))
		return
	}
	defer obj.Close()

	// 获取对象信息（用于 Content-Length）
	objInfo, err := obj.Stat()
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, err)
		return
	}

	// 设置响应头（重要：允许浏览器读取跨域资源）
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", objInfo.Size))
	w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 流式传输
	if _, err := io.Copy(w, obj); err != nil {
		fmt.Printf("Error copying object: %v\n", err)
	}
}

// 辅助方法：获取 MinIO 客户端
func (h *Handler) getMiniOClient() *minio.Client {
	client, _ := minio.New(config.Envs.MinIOEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.Envs.MinIOAccessKey, config.Envs.MinIOSecretKey, ""),
		Secure: config.Envs.MinIOUseSSL,
	})
	return client
}

// ✅ 核心修复：处理前端发来的裁剪指令
func (h *Handler) handleDispatchCropTask(w http.ResponseWriter, r *http.Request) {
	// 1. 获取 Video ID
	vars := mux.Vars(r)
	videoID, err := strconv.Atoi(vars["id"])
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("invalid video id"))
		return
	}

	// 2. 验证权限并获取视频信息
	video, err := h.checkVideoOwnership(w, r, videoID)
	if err != nil {
		utils.WriteError(w, http.StatusForbidden, err)
		return
	}

	// 3. 解析裁剪参数 (JSON Body)
	var params CropParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("invalid crop params"))
		return
	}

	// 4. 构造 Worker 能访问的输入路径
	var inputPath string
	if config.Envs.StorageDriver == "minio" {
		// ⚠️ 关键修复：Worker 在 Docker 内部，必须用 minio 服务名访问，不能用 localhost
		// 格式: http://minio:9000/bucket/objectKey
		inputPath = fmt.Sprintf("http://minio:9000/%s/%s", config.Envs.MinIOBucket, video.ObjectKey)
	} else {
		// 本地存储模式
		inputPath = filepath.Join(config.Envs.UploadDir, video.ObjectKey)
	}

	// 5. 构造输出路径 (Worker 里的临时路径)
	outputFilename := fmt.Sprintf("crop_%d_%d.mp4", video.UserID, time.Now().Unix())
	// 注意：这里假设 Worker 也有 /app/uploads 目录
	outputPath := filepath.Join(config.Envs.UploadDir, "temp", outputFilename)

	// 6. 发布任务
	task := map[string]interface{}{
		"video_id":    video.ID,
		"user_id":     video.UserID,
		"input_path":  inputPath,  // ✅ 内部网络地址
		"output_path": outputPath, // ✅ 容器内临时路径
		"params":      params,
	}

	if h.publishCrop != nil {
		if err := h.publishCrop(task); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, fmt.Errorf("failed to queue task"))
			return
		}
	} else {
		utils.WriteError(w, http.StatusInternalServerError, fmt.Errorf("mq not configured"))
		return
	}

	// 7. 更新 DB 状态
	video.Status = "queued"
	h.store.UpdateVideo(video)

	utils.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Task queued successfully",
		"status":  "queued",
	})
}

// --- 以下是原有的辅助函数，保持不变 ---

func (h *Handler) fileURL(key string) string {
	if key == "" {
		return ""
	}
	return fmt.Sprintf("/api/v1/videos/stream/%s", key)
}

func (h *Handler) handleGetVideos(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	videos, err := h.store.GetVideos(userID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	for i := range videos {
		key := videos[i].StoragePath
		if key == "" {
			key = videos[i].ObjectKey
		}
		videos[i].FilePath = h.fileURL(key)
	}
	utils.WriteJSON(w, http.StatusOK, videos)
}

func (h *Handler) handleGetVideo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])
	video, err := h.checkVideoOwnership(w, r, id)
	if err != nil {
		utils.WriteError(w, http.StatusForbidden, err)
		return
	}
	key := video.StoragePath
	if key == "" {
		key = video.ObjectKey
	}
	video.FilePath = h.fileURL(key)
	utils.WriteJSON(w, http.StatusOK, video)
}

func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	r.Body = http.MaxBytesReader(w, r.Body, config.Envs.MaxUploadSize)
	if err := r.ParseMultipartForm(config.Envs.MaxUploadSize); err != nil {
		utils.WriteError(w, http.StatusBadRequest, err)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		file, header, err = r.FormFile("file") // 兼容
		if err != nil {
			utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("missing file"))
			return
		}
	}
	defer file.Close()

	objectKey := fmt.Sprintf("videos/%d/%d_%s", userID, time.Now().UnixNano(), header.Filename)

	// 上传到 MinIO
	if err := h.storage.Upload(context.Background(), objectKey, file, header.Size, header.Header.Get("Content-Type")); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	// 创建 DB 记录
	video := &types.Video{
		UserID:      userID,
		Title:       r.FormValue("title"),
		Description: r.FormValue("description"),
		ObjectKey:   objectKey,
		FileName:    header.Filename,
		FileSize:    header.Size,
		Status:      "pending", // 初始状态
	}

	if err := h.store.CreateVideo(video); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	h.cache.ClearUserVideosCache(r.Context(), userID)
	utils.WriteJSON(w, http.StatusCreated, video)
}

func (h *Handler) handleDeleteVideo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])
	video, err := h.checkVideoOwnership(w, r, id)
	if err != nil {
		utils.WriteError(w, http.StatusForbidden, err)
		return
	}

	if video.ObjectKey != "" {
		h.storage.Delete(context.Background(), video.ObjectKey)
	}
	h.store.DeleteVideo(id)
	h.cache.ClearUserVideosCache(r.Context(), video.UserID)
	h.cache.ClearVideoCache(r.Context(), id)

	utils.WriteJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

func (h *Handler) checkVideoOwnership(w http.ResponseWriter, r *http.Request, videoID int) (*types.Video, error) {
	userID := auth.GetUserIDFromContext(r.Context())
	video, err := h.store.GetVideoByID(videoID)
	if err != nil {
		return nil, fmt.Errorf("video not found")
	}
	if video.UserID != userID {
		return nil, fmt.Errorf("permission denied")
	}
	return video, nil
}
