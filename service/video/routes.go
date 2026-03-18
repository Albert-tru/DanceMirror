package video

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Albert-tru/DanceMirror/config"
	"github.com/Albert-tru/DanceMirror/pkg/ratelimit"
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
	store          types.VideoStore
	userStore      types.UserStore
	cache          *cache.RedisClient
	storage        storage.VideoStorage
	publishCrop    func(task map[string]interface{}) error
	publishAnalyze func(task map[string]interface{}) error
	esClient       *search.ESClient
	sf             singleflight.Group
	limiter        *ratelimit.Limiter
	cacheBypass    bool // 是否绕过缓存，主要用于调试
	perfMode       bool // 性能测试模式，开启后会记录更详细的日志
}

func NewHandler(
	store types.VideoStore,
	userStore types.UserStore,
	cache *cache.RedisClient,
	storage storage.VideoStorage,
	publishCrop func(task map[string]interface{}) error,
	publishAnalyze func(task map[string]interface{}) error,
	esClient *search.ESClient,
) *Handler {
	return &Handler{
		store:          store,
		userStore:      userStore,
		cache:          cache,
		storage:        storage,
		publishCrop:    publishCrop,
		publishAnalyze: publishAnalyze,
		esClient:       esClient,
		sf:             singleflight.Group{},
		limiter:        ratelimit.NewLimiter(),
		cacheBypass:    os.Getenv("CACHE_BYPASS") == "1", // 通过环境变量控制是否绕过缓存
		perfMode:       os.Getenv("PERF_MODE") == "1",    // 通过环境变量控制是否开启性能测试模式
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
	router.HandleFunc("/videos/{id}/analyze", auth.WithJWTAuth(h.handleDispatchAnalyze, h.userStore)).Methods(http.MethodPost) // 视频分析任务路由
	router.HandleFunc("/videos/{id}/analysis", auth.WithJWTAuth(h.handleGetAnalysis, h.userStore)).Methods(http.MethodGet)     // 获取分析结果路由

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

// 处理前端发来的裁剪指令
func (h *Handler) handleDispatchCropTask(w http.ResponseWriter, r *http.Request) {
	// 1. 获取 Video ID
	vars := mux.Vars(r)
	videoID, err := strconv.Atoi(vars["id"])
	userID := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("invalid video id"))
		return
	}

	// 限流：每个用户每分钟最多发起 2 个裁剪任务
	key := fmt.Sprintf("crop:%d:%d", userID, videoID)
	if ok, resetAt := h.limiter.Allow(key, 1, time.Minute); !ok {
		writeRateLimited(w, resetAt, "please wait before cropping again")
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

func (h *Handler) fileURL(ctx context.Context, key string) string {
	if key == "" {
		return ""
	}

	// ✅ 最稳妥的方案：直接返回公开 URL（不用签名）
	// 因为你的 minio.go 已经设置了 Bucket 为公开只读模式
	publicHost := config.Envs.PublicHost
	if publicHost == "" {
		publicHost = "localhost"
	}

	// 格式: http://localhost:9000/bucket/objectKey
	publicURL := fmt.Sprintf("http://%s:9000/%s/%s", publicHost, config.Envs.MinIOBucket, key)

	fmt.Printf("✅ Generated public URL: %s\n", publicURL)
	return publicURL
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

	// 4. 动态生成预签名 URL
	fmt.Printf("🔍 DEBUG: Processing %d videos...\n", len(videos)) // 👈 加这行
	for i := range videos {
		key := getVideoKey(videos[i]) // ✅ 使用辅助函数

		// 👈 加这些日志
		fmt.Printf("🔍 DEBUG Video[%d] ID=%d | ObjectKey='%s' | StoragePath='%s' | FinalKey='%s'\n",
			i, videos[i].ID, videos[i].ObjectKey, videos[i].StoragePath, key)

		if key != "" {
			url := h.fileURL(ctx, key)
			videos[i].FilePath = url
			fmt.Printf("   -> Generated URL: %s\n", url) // 👈 看看生成的 URL 对不对
		} else {
			fmt.Println("   -> ❌ Key is empty, skipping URL generation")
		}
	}

	utils.WriteJSON(w, http.StatusOK, videos)
}

// ✅ 新增：辅助函数，确保任何时候都能拿到 Key
func getVideoKey(v *types.Video) string {
	if v.ObjectKey != "" {
		return v.ObjectKey
	}
	return v.StoragePath
}

// 修改：详情页 Cache-Aside + 开关
func (h *Handler) handleGetVideo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])
	ctx := r.Context()

	var (
		video *types.Video
		err   error
	)

	// A组：绕过缓存（优化前）
	if h.cacheBypass {
		video, err = h.store.GetVideoByID(ctx, id)
		if err != nil {
			utils.WriteError(w, http.StatusNotFound, fmt.Errorf("video not found"))
			return
		}
	} else {
		// B组：正常缓存逻辑（优化后）
		video, err = h.cache.GetVideoByID(ctx, id)
		if err != nil {
			if errors.Is(err, cache.ErrCacheNull) {
				utils.WriteError(w, http.StatusNotFound, fmt.Errorf("video not found"))
				return
			}

			sfKey := fmt.Sprintf("sf_video_%d", id)
			var v interface{}
			v, err = h.sf.Do(sfKey, func() (interface{}, error) {
				dbVideo, dbErr := h.store.GetVideoByID(ctx, id)
				if dbErr != nil {
					_ = h.cache.CacheNullVideoByID(context.Background(), id)
					return nil, dbErr
				}
				_ = h.cache.CacheVideoByID(context.Background(), dbVideo)
				return dbVideo, nil
			})
			if err != nil {
				utils.WriteError(w, http.StatusNotFound, fmt.Errorf("video not found"))
				return
			}

			var ok bool
			video, ok = v.(*types.Video)
			if !ok || video == nil {
				utils.WriteError(w, http.StatusInternalServerError, fmt.Errorf("invalid video data"))
				return
			}
		}
	}

	userID := auth.GetUserIDFromContext(r.Context())
	if video.UserID != userID {
		utils.WriteError(w, http.StatusForbidden, fmt.Errorf("permission denied"))
		return
	}

	key := getVideoKey(video)
	if key != "" {
		video.FilePath = h.fileURL(ctx, key)
	}
	utils.WriteJSON(w, http.StatusOK, video)
}

func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {

	// 限流: 每个用户每分钟最多上传 2 个视频
	userID := auth.GetUserIDFromContext(r.Context())
	if ok, resetAt := h.limiter.Allow(fmt.Sprintf("upload:user:%d", userID), 1000, time.Minute); !ok {
		writeRateLimited(w, resetAt, "upload limit exceeded for user")
		return
	}

	// 限流: 每个IP每分钟最多上传 10 个视频（防止恶意攻击/带宽耗尽）
	clientIP := ratelimit.ClientIP(r)
	if ok, resetAt := h.limiter.Allow("upload:ip:"+clientIP, 5000, time.Minute); !ok {
		writeRateLimited(w, resetAt, "upload limit exceeded for IP")
		return
	}

	// 临时放宽全局限流
	if ok, resetAt := h.limiter.Allow("upload:global", 10000, time.Minute); !ok {
		writeRateLimited(w, resetAt, "global upload limit exceeded")
		return
	}

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
	// a. 获取文件后缀 (如 .mp4)
	ext := filepath.Ext(header.Filename)

	// b. 获取不带后缀的文件名
	rawName := strings.TrimSuffix(header.Filename, ext)

	// c. 清理文件名：替换空格和特殊符号，防止 URL 编码问题
	safeName := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, rawName)

	// d. ✅ 核心：格式为 videos/{userID}/{纳秒时间戳}_{随机数}_{清理后的文件名}{后缀}
	// 纳秒级时间戳几乎不可能冲突，加上原有文件名方便在后台存储桶里肉眼识别
	uniqueFileName := fmt.Sprintf("videos/%d/%d_%s%s", userID, time.Now().UnixNano(), safeName, ext)

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

	// 推送 AI 分析任务到 video_analyze_queue
	if h.publishAnalyze != nil {
		inputURL := fmt.Sprintf("http://minio:9000/%s/%s", config.Envs.MinIOBucket, video.ObjectKey)
		task := map[string]interface{}{
			"video_id":   video.ID,
			"user_id":    video.UserID,
			"input_path": inputURL,
		}
		if err := h.publishAnalyze(task); err != nil {
			fmt.Printf("⚠️ enqueue analyze task failed: %v\n", err)
		} else {
			fmt.Printf("✅ analyze task enqueued: video_id=%d\n", video.ID)
		}
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
		key := getVideoKey(videos[i]) // ✅ 之前这里用了 &videos[i] 且逻辑不一致，需修正
		if key != "" {
			videos[i].FilePath = h.fileURL(r.Context(), key)
		}
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
	return s.db.WithContext(ctx).Exec(query, count, id).Error
}

// 处理视频 AI 分析任务调度
func (h *Handler) handleDispatchAnalyze(w http.ResponseWriter, r *http.Request) {
	// 1. 获取 Video ID
	vars := mux.Vars(r)
	videoID, err := strconv.Atoi(vars["id"])
	userID := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, fmt.Errorf("invalid video id"))
		return
	}

	// 非压测模式才启用 analyze 限流
	if !h.perfMode {
		key := fmt.Sprintf("analyze:%d:%d", userID, videoID)
		if ok, resetAt := h.limiter.Allow(key, 1, 5*time.Minute); !ok {
			writeRateLimited(w, resetAt, "analysis already in progress or concurrent limit")
			return
		}

		userKey := fmt.Sprintf("analyze:user:%d", userID)
		if ok, resetAt := h.limiter.Allow(userKey, 2, time.Minute); !ok {
			writeRateLimited(w, resetAt, "too many analysis requests")
			return
		}
	}

	// 2. 验证权限
	video, err := h.checkVideoOwnership(w, r, videoID)
	if err != nil {
		utils.WriteError(w, http.StatusForbidden, err)
		return
	}

	// 3. 构造 Python Worker 可访问的路径
	var inputPath string
	if config.Envs.StorageDriver == "minio" {
		// 使用 Docker 内部网络地址 (http://minio:9000/...)
		inputPath = fmt.Sprintf("http://minio:9000/%s/%s", config.Envs.MinIOBucket, video.ObjectKey)
	} else {
		// 本地模式 (仅开发环境)
		inputPath = filepath.Join(config.Envs.UploadDir, video.ObjectKey)
	}

	// 4. 构造任务消息
	// 注意：我们要发送给 MQ 队列，这里复用了 map[string]interface{} 格式
	// 也可以直接使用 types.AnalyzeTask 结构体序列化
	task := map[string]interface{}{
		"video_id":   video.ID,
		"user_id":    video.UserID,
		"input_path": inputPath,
	}

	// 5. 发送任务到 MQ
	if h.publishAnalyze != nil {
		if err := h.publishAnalyze(task); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, fmt.Errorf("failed to queue analysis task"))
			return
		}
	} else {
		utils.WriteError(w, http.StatusInternalServerError, fmt.Errorf("analysis mq not configured"))
		return
	}

	// 6. 返回结果（前端收到后应该显示“AI 分析中...”并开始轮询）
	utils.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "AI analysis started",
		"status":  "analyzing",
	})
}

// 获取视频分析结果
func (h *Handler) handleGetAnalysis(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	videoID, _ := strconv.Atoi(vars["id"])

	// 1. 验证权限 (防止看别人的分析报告)
	_, err := h.checkVideoOwnership(w, r, videoID)
	if err != nil {
		utils.WriteError(w, http.StatusForbidden, err)
		return
	}

	// 2. 从 Redis 获取结果
	// Python Worker 存入的 Key 应该是 "analysis:video:{id}"
	key := fmt.Sprintf("analysis:video:%d", videoID)

	// 使用 map 接收 JSON 数据
	var result map[string]interface{}
	err = h.cache.Get(r.Context(), key, &result)

	// 3. 处理缓存未命中 (还没分析完，或者过期了)
	if err != nil {
		// 这里虽然 err != nil，但在业务上代表“处理中”或“无记录”
		// 我们可以返回一个特定的状态码，或者返回 status: processing
		utils.WriteJSON(w, http.StatusOK, map[string]string{
			"status":  "processing",
			"message": "Analysis is in progress or not found",
		})
		return
	}

	// 4. 返回分析报告
	utils.WriteJSON(w, http.StatusOK, result)
}

func writeRateLimited(w http.ResponseWriter, resetAt time.Time, msg string) {
	retryAfter := int(time.Until(resetAt).Seconds())
	if retryAfter < 1 {
		retryAfter = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	utils.WriteError(w, http.StatusTooManyRequests, fmt.Errorf("%s", msg))
}
