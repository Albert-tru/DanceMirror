package video

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Albert-tru/DanceMirror/config"
	"github.com/Albert-tru/DanceMirror/service/auth"
	"github.com/Albert-tru/DanceMirror/service/cache"
	"github.com/Albert-tru/DanceMirror/service/search"
	"github.com/Albert-tru/DanceMirror/service/storage"
	"github.com/Albert-tru/DanceMirror/types"
	"github.com/Albert-tru/DanceMirror/utils"
	"github.com/golang/groupcache/singleflight"
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
	esClient    *search.ESClient
	sf          singleflight.Group
}

func NewHandler(store types.VideoStore, userStore types.UserStore, cache *cache.RedisClient, storage storage.VideoStorage, publishCrop func(task map[string]interface{}) error, esClient *search.ESClient) *Handler {
	return &Handler{
		store:       store,
		userStore:   userStore,
		cache:       cache,
		storage:     storage,
		publishCrop: publishCrop,
		esClient:    esClient,
	}
}

func (h *Handler) RegisterRoutes(router *mux.Router) {

	// 基础视频路由
	router.HandleFunc("/videos/search", auth.WithJWTAuth(h.Search, h.userStore)).Methods(http.MethodGet)
	router.HandleFunc("/videos", auth.WithJWTAuth(h.handleGetVideos, h.userStore)).Methods(http.MethodGet)
	router.HandleFunc("/videos/search", auth.WithJWTAuth(h.Search, h.userStore)).Methods(http.MethodGet)
	router.HandleFunc("/videos", auth.WithJWTAuth(h.handleUpload, h.userStore)).Methods(http.MethodPost)
	router.HandleFunc("/videos/{id}", auth.WithJWTAuth(h.handleGetVideo, h.userStore)).Methods(http.MethodGet)
	router.HandleFunc("/videos/{id}", auth.WithJWTAuth(h.handleDeleteVideo, h.userStore)).Methods(http.MethodDelete)
	router.HandleFunc("/videos/{id}/crop", auth.WithJWTAuth(h.handleDispatchCropTask, h.userStore)).Methods(http.MethodPost)

	// 静态文件服务
	if config.Envs.StorageDriver != "minio" {
		router.PathPrefix("/uploads/").Handler(http.StripPrefix("/uploads/", http.FileServer(http.Dir(config.Envs.UploadDir))))
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
	var params types.CropParams
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
	h.store.UpdateVideo(r.Context(), video)

	utils.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Task queued successfully",
		"status":  "queued",
	})
}

// 修改：fileURL 改为生成预签名链接 (MinIO Presigned URL)
func (h *Handler) fileURL(ctx context.Context, key string) string {
	if key == "" {
		return ""
	}
	// 设置链接过期时间（例如 24 小时）
	expiry := time.Hour * 24

	// 设置请求参数（可选：例如强制下载或设置 Content-Type）
	reqParams := make(url.Values)
	// reqParams.Set("response-content-disposition", "attachment; filename=\"video.mp4\"")

	// 生成预签名 URL
	// 注意：PresignedGetObject 是离线计算操作，速度极快，不会产生网络 IO
	presignedURL, err := h.getMiniOClient().PresignedGetObject(ctx, config.Envs.MinIOBucket, key, expiry, reqParams)
	if err != nil {
		fmt.Printf("Error generating presigned url for key %s: %v\n", key, err)
		return ""
	}

	return presignedURL.String()
}

// 修改：使用 cache 包封装好的方法实现 Cache-Aside
func (h *Handler) handleGetVideos(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	ctx := r.Context()

	// 1. 尝试从缓存读取 (利用 redis.go 中封装好的 GetUserVideos)
	// 同时也修正了之前提到的 routes.go 里错误的 Get 调用方式
	videos, err := h.cache.GetUserVideos(ctx, userID)

	// 2. 如果缓存未命中 (err != nil)，查询数据库
	// 注意：你的 Get 实现里，key不存在会返回 error，所以这里判断 err != nil 即可视为 Miss
	if err != nil {
		videos, err = h.store.GetVideos(ctx, userID)
		if err != nil {
			utils.WriteError(w, http.StatusInternalServerError, err)
			return
		}

		// 3. 写入缓存 (利用 redis.go 中封装好的 CacheUserVideos)
		// 这一步是异步的，不应该阻塞主流程，可以容忍偶尔失败
		go func() {
			h.cache.CacheUserVideos(context.Background(), userID, videos)
		}()
	}

	// 4. 动态生成预签名 URL (这步必须在缓存读取之后每次实时生成)
	for i := range videos {
		key := videos[i].StoragePath
		if key == "" {
			key = videos[i].ObjectKey
		}
		videos[i].FilePath = h.fileURL(ctx, key)
	}

	utils.WriteJSON(w, http.StatusOK, videos)
}

// 修改：详情页 Cache-Aside
func (h *Handler) handleGetVideo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])
	ctx := r.Context()

	// 1. 尝试从缓存获取 (利用 redis.go 中封装好的 GetVideoByID)
	// GetVideoByID 内部调用了 r.Get(ctx, key, &video)
	video, err := h.cache.GetVideoByID(ctx, id)

	// 2. 缓存未命中，查 DB
	if err != nil {
		// 构造一个针对该 VideoID 的唯一 Key
		sfKey := fmt.Sprintf("sf_video_%d", id)

		// ✅ Do 方法保证：同一时刻，对同一个 sfKey，fn 函数只会执行一次
		// 其他并发请求会阻塞在这里等待结果
		v, err, shared := h.sf.Do(sfKey, func() (interface{}, error) {
			// --- 这里是真正查 DB 的逻辑 (只会有 1 个请求进来) ---

			// a. 查 DB
			dbVideo, err := h.store.GetVideoByID(ctx, id)
			if err != nil {
				return nil, err
			}

			// b. 回写缓存 (Cache-Aside)
			// 使用 context.Background() 确保请求取消不影响回写缓存
			h.cache.CacheVideoByID(context.Background(), dbVideo)

			return dbVideo, nil
			// ----------------------------------------------------
		})
		if err != nil {
			utils.WriteError(w, http.StatusNotFound, fmt.Errorf("video not found"))
			return
		}

		// 如果是共享结果，说明是从缓存回写中获取的
		if shared {
			fmt.Printf("⚡ Singleflight: Video ID %d shared result\n", id)
		}

		// 类型断言：interface{} -> *types.Video
		video = v.(*types.Video)
	}

	// 4. 权限检查
	userID := auth.GetUserIDFromContext(r.Context())
	if video.UserID != userID {
		utils.WriteError(w, http.StatusForbidden, fmt.Errorf("permission denied"))
		return
	}

	// 5. 生成签名 URL (针对每个用户动态生成，不能缓存)
	key := video.StoragePath
	if key == "" {
		key = video.ObjectKey
	}
	video.FilePath = h.fileURL(ctx, key)

	utils.WriteJSON(w, http.StatusOK, video)
}

func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	// 1. 限制上传大小 (例如 500MB)
	r.Body = http.MaxBytesReader(w, r.Body, 500<<20)
	if err := r.ParseMultipartForm(500 << 20); err != nil {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("file too big or invalid format"))
		return
	}

	// 2. 获取文件流
	file, header, err := r.FormFile("video")
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("missing video file"))
		return
	}
	defer file.Close()

	// 3. 构建视频元数据对象
	userID := auth.GetUserIDFromContext(r.Context())
	uniqueFileName := fmt.Sprintf("%d_%d_%s", userID, time.Now().UnixNano(), header.Filename)

	// 4. 上传到存储 (MinIO/Local)
	// 注意：这里调用 storage 接口上传，该接口内部应该处理具体的 PutObject 逻辑
	if err := h.storage.Upload(r.Context(), uniqueFileName, file, header.Size, header.Header.Get("Content-Type")); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	storagePath := uniqueFileName // 兼容旧字段，实际存储在 MinIO 的 ObjectKey

	// 5. 准备数据库记录
	video := &types.Video{
		UserID:      userID,
		Title:       r.FormValue("title"),
		Description: r.FormValue("description"),
		ObjectKey:   uniqueFileName, // 用于 MinIO 查找的 Key
		StoragePath: storagePath,    // 兼容旧字段
		FileName:    header.Filename,
		FileSize:    header.Size,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 6. 写入数据库 (Source of Truth)
	if err := h.store.CreateVideo(r.Context(), video); err != nil {
		// 如果 DB 写入失败，理论上应该尝试回滚删除 MinIO 里的文件，防止孤儿文件
		// h.storage.Delete(context.Background(), uniqueFileName)
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	// 7. ✅ Cache-Aside: 此时 DB 已更新，必须清除该用户的视频列表缓存
	// 使用 goroutine 异步清理，防止阻塞 HTTP 响应
	// 必须使用 context.Background()，因为 request context 在函数返回后会取消
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := h.cache.ClearUserVideosCache(ctx, userID); err != nil {
			fmt.Printf("⚠️ Failed to clear cache for user %d: %v\n", userID, err)
		}
	}()

	// 8. 返回成功响应
	utils.WriteJSON(w, http.StatusCreated, video)
}

func (h *Handler) handleDeleteVideo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	// DB 查权
	video, err := h.checkVideoOwnership(w, r, id)
	if err != nil {
		utils.WriteError(w, http.StatusForbidden, err)
		return
	}

	// MinIO 删除
	if video.ObjectKey != "" {
		h.storage.Delete(r.Context(), video.ObjectKey)
	}

	// DB 删除
	h.store.DeleteVideo(r.Context(), id)

	// ✅ 修改：使用封装好的清除方法，保持一致性
	// 防止脏读：删库后必须删缓存
	go func() {
		ctx := context.Background()
		h.cache.ClearUserVideosCache(ctx, video.UserID)
		h.cache.ClearVideoCache(ctx, id)
	}()

	utils.WriteJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

func (h *Handler) checkVideoOwnership(w http.ResponseWriter, r *http.Request, videoID int) (*types.Video, error) {
	userID := auth.GetUserIDFromContext(r.Context())
	video, err := h.store.GetVideoByID(r.Context(), videoID)
	if err != nil {
		return nil, fmt.Errorf("video not found")
	}
	if video.UserID != userID {
		return nil, fmt.Errorf("permission denied")
	}
	return video, nil
}

func (h *Handler) SearchOld(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		h.handleGetVideos(w, r) // 如果没有关键词，回退到普通列表
		return
	}

	// 调用 ES 搜索拿到 ID 列表
	ids, err := h.esClient.SearchVideos(r.Context(), query, 1, 20)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	// 根据 ID 回 MySQL 查完整数据 (或者直接从 ES 返回部分字段)
	// 这里简单演示回表查，保证数据最新
	videos, err := h.store.GetVideosByIDs(r.Context(), ids)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	utils.WriteJSON(w, http.StatusOK, videos)
}

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size < 1 {
		size = 20
	}
	sort := r.URL.Query().Get("sort")

	videos, total, err := h.store.SearchVideos(r.Context(), query, page, size, sort)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	for i := range videos {
		key := videos[i].StoragePath
		if key == "" {
			key = videos[i].ObjectKey
		}
		videos[i].FilePath = h.fileURL(r.Context(), key)
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"items": videos,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

func (s *Store) IncrementVideoViews(ctx context.Context, id int, count int) error {
	// 原生 SQL 是最高效的
	query := "UPDATE videos SET views = views + ? WHERE id = ?"
	_, err := s.db.ExecContext(ctx, query, count, id)
	return err
}
