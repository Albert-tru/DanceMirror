package video

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"context"

	"github.com/Albert-tru/DanceMirror/config"
	"github.com/Albert-tru/DanceMirror/service/auth"
	"github.com/Albert-tru/DanceMirror/service/cache"
	"github.com/Albert-tru/DanceMirror/types"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// mockVideoStorage 模拟视频存储 VideoStorage 接口
type mockVideoStorage struct {
	files map[string][]byte // 模拟存储文件的映射
}

func newMockVideoStorage() *mockVideoStorage {
	return &mockVideoStorage{
		files: make(map[string][]byte),
	}
}

// 实现接口需要的 Upload 方法
func (m *mockVideoStorage) Upload(ctx context.Context, objectKey string, r io.Reader, size int64, contentType string) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.files[objectKey] = data
	return nil
}

// 实现接口需要的 Delete 方法
func (m *mockVideoStorage) Delete(ctx context.Context, objectKey string) error {
	delete(m.files, objectKey)
	return nil
}

// 实现接口需要的 PresignGet 方法
func (m *mockVideoStorage) GetPresignedURL(objectKey string, expiry time.Duration) (string, error) {

	if _, ok := m.files[objectKey]; !ok {
		return "", fmt.Errorf("file not found: %s", objectKey)
	}
	return fmt.Sprintf("https://mock-storage.example.com/%s?expires=%d", objectKey, int(expiry.Seconds())), nil
}

// 可选辅助方法（测试断言用）
func (m *mockVideoStorage) FileExists(objectKey string) bool {
	_, ok := m.files[objectKey]
	return ok
}

// setupTestHandler 创建测试用的 Handler 和路由
func setupTestHandler(t *testing.T) (*gorm.DB, *mux.Router, *types.User, string, *mockVideoStorage) {
	// 连接测试数据库
	dsn := "root:MySQL666@tcp(127.0.0.1:3306)/dancemirror_test?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("无法连接到测试数据库: %v", err)
	}

	// 禁用外键检查，防止删表时出错
	db.Exec("SET FOREIGN_KEY_CHECKS = 0")

	// 彻底删除旧表
	if err := db.Migrator().DropTable(&types.Video{}, &types.User{}); err != nil {
		t.Fatalf("无法删除旧表: %v", err)
	}

	// 恢复外键检查
	db.Exec("SET FOREIGN_KEY_CHECKS = 1")

	// 重新创建表
	if err := db.AutoMigrate(&types.User{}); err != nil {
		t.Fatalf("无法迁移 User 表: %v", err)
	}

	//  自动迁移确保 object_key 列存在
	if err := db.AutoMigrate(&types.Video{}); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}

	// 清空测试数据
	db.Exec("DELETE FROM videos")
	db.Exec("DELETE FROM users")

	// 创建测试用户
	hashedPassword, _ := auth.HashPassword("testpass")
	testUser := &types.User{
		Phone:     "13800138000",
		Email:     "video.test@example.com",
		Password:  hashedPassword,
		FirstName: "Video",
		LastName:  "Tester",
	}
	db.Create(testUser)

	// 创建测试上传目录
	testUploadDir := filepath.Join(os.TempDir(), "test_uploads")
	os.MkdirAll(testUploadDir, os.ModePerm)
	config.Envs.UploadDir = testUploadDir
	config.Envs.MaxUploadSize = 100 * 1024 * 1024 // 100MB

	// 创建 JWT token
	secret := []byte(config.Envs.JWTSecret)
	token, _ := auth.CreateJWT(secret, testUser.ID)

	// 创建redis缓存客户端
	// 假设本地 Redis 用于测试，使用 DB 1 以免与开发数据冲突
	redisAddr := "localhost:6379"
	redisClient := cache.NewRedisClient(redisAddr, "", 1)

	// 清空测试数据库
	rawRedisClient := redis.NewClient(&redis.Options{Addr: redisAddr, DB: 1})
	if err := rawRedisClient.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("无法清空 Redis 测试数据库: %v", err)
	}

	// 创建模拟存储
	videoStore := NewStore(db, nil)
	videoStorage := newMockVideoStorage()

	// 创建 Handler
	userStore := &mockUserStore{db: db}
	handler := NewHandler(videoStore, userStore, redisClient, videoStorage, nil, nil, nil)

	// 创建路由
	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	return db, router, testUser, token, videoStorage
}

// mockUserStore 模拟 UserStore
type mockUserStore struct {
	db *gorm.DB
}

func (m *mockUserStore) GetUserByPhone(phone string) (*types.User, error) {
	var user types.User
	err := m.db.Where("phone = ?", phone).First(&user).Error
	return &user, err
}

func (m *mockUserStore) GetUserByEmail(email string) (*types.User, error) {
	var user types.User
	err := m.db.Where("email = ?", email).First(&user).Error
	return &user, err
}

func (m *mockUserStore) GetUserByID(id int64) (*types.User, error) {
	var user types.User
	err := m.db.First(&user, id).Error
	return &user, err
}

func (m *mockUserStore) CreateUser(user types.User) error {
	return m.db.Create(&user).Error
}

// createTestVideoFile 创建测试视频文件
func createTestVideoFile(t *testing.T) (string, int64) {
	// 创建一个模拟的 MP4 文件（实际上是文本，但足够测试）
	content := []byte("fake video content for testing")
	tempFile, err := os.CreateTemp("", "test_video_*.mp4")
	if err != nil {
		t.Fatalf("无法创建临时文件: %v", err)
	}

	_, err = tempFile.Write(content)
	if err != nil {
		t.Fatalf("无法写入临时文件: %v", err)
	}

	filePath := tempFile.Name()
	fileSize := int64(len(content))
	tempFile.Close()

	return filePath, fileSize
}

// 视频上传接口 (POST /videos)
func TestHandleUpload(t *testing.T) {
	db, router, testUser, token, mockStorage := setupTestHandler(t)
	defer db.Exec("DELETE FROM videos")
	defer db.Exec("DELETE FROM users")
	defer os.RemoveAll(config.Envs.UploadDir)

	t.Run("should upload video successfully", func(t *testing.T) {
		// 创建测试文件
		testFilePath, _ := createTestVideoFile(t)
		defer os.Remove(testFilePath)

		// 创建 multipart 请求
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// 添加表单字段
		writer.WriteField("title", "测试视频")
		writer.WriteField("description", "这是一个测试视频")

		// 添加文件
		file, _ := os.Open(testFilePath)
		defer file.Close()
		part, _ := writer.CreateFormFile("video", "test.mp4")
		io.Copy(part, file)
		writer.Close()

		// 创建请求
		req := httptest.NewRequest(http.MethodPost, "/videos", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		// 如果失败，打印详细错误
		if rr.Code != http.StatusCreated {
			t.Logf("❌ 上传失败")
			t.Logf("状态码: %d (期望 %d)", rr.Code, http.StatusCreated)
			t.Logf("响应内容: %s", rr.Body.String())
		}

		// 断言
		assert.Equal(t, http.StatusCreated, rr.Code)

		var response types.Video
		json.Unmarshal(rr.Body.Bytes(), &response)
		assert.Equal(t, "测试视频", response.Title)
		assert.Equal(t, "这是一个测试视频", response.Description)
		assert.Equal(t, testUser.ID, response.UserID)
		assert.NotZero(t, response.ID)

		// 验证文件已上传到模拟存储
		assert.NotEmpty(t, response.ObjectKey, "应该生成 ObjectKey")
		assert.True(t, mockStorage.FileExists(response.ObjectKey), "文件应存在于模拟存储中")
	})

	t.Run("should upload video with file field name", func(t *testing.T) {
		// 测试使用 "file" 字段名上传（录制上传）
		testFilePath, _ := createTestVideoFile(t)
		defer os.Remove(testFilePath)

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.WriteField("title", "录制视频")
		writer.WriteField("description", "录制测试")

		file, _ := os.Open(testFilePath)
		defer file.Close()
		part, _ := writer.CreateFormFile("file", "recorded.mp4")
		io.Copy(part, file)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/videos", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Logf("❌ 录制上传失败: %s", rr.Body.String())
		}

		assert.Equal(t, http.StatusCreated, rr.Code)
	})

	t.Run("should fail without authentication", func(t *testing.T) {
		testFilePath, _ := createTestVideoFile(t)
		defer os.Remove(testFilePath)

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.WriteField("title", "测试视频")

		file, _ := os.Open(testFilePath)
		defer file.Close()
		part, _ := writer.CreateFormFile("video", "test.mp4")
		io.Copy(part, file)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/videos", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		// 不设置 Authorization 头

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		// 接受 401 或 403
		assert.True(t, rr.Code == http.StatusUnauthorized || rr.Code == http.StatusForbidden,
			"期望 401 或 403，实际得到 %d", rr.Code)
	})

	t.Run("should fail with missing title", func(t *testing.T) {
		testFilePath, _ := createTestVideoFile(t)
		defer os.Remove(testFilePath)

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		// 不设置 title

		file, _ := os.Open(testFilePath)
		defer file.Close()
		part, _ := writer.CreateFormFile("video", "test.mp4")
		io.Copy(part, file)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/videos", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)

		var response map[string]string
		json.Unmarshal(rr.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "title is required")
	})

	t.Run("should fail with missing file", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.WriteField("title", "测试视频")
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/videos", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("should fail with invalid file type", func(t *testing.T) {
		// 创建一个非视频文件
		tempFile, _ := os.CreateTemp("", "test_*.txt")
		tempFile.WriteString("not a video")
		tempFile.Close()
		defer os.Remove(tempFile.Name())

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.WriteField("title", "测试视频")

		file, _ := os.Open(tempFile.Name())
		defer file.Close()
		part, _ := writer.CreateFormFile("video", "test.txt")
		io.Copy(part, file)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/videos", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)

		var response map[string]string
		json.Unmarshal(rr.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "invalid file type")
	})
}

// 获取视频列表接口 (GET /videos)
func TestHandleGetVideos(t *testing.T) {
	db, router, testUser, token, mockStorage := setupTestHandler(t)
	defer db.Exec("DELETE FROM videos")
	defer db.Exec("DELETE FROM users")

	t.Run("should get user's videos", func(t *testing.T) {
		// 创建测试视频
		video1 := &types.Video{
			UserID:    testUser.ID,
			Title:     "视频 1",
			ObjectKey: "videos/1/test1.mp4",
			FilePath:  "videos/1/test1.mp4",
			FileName:  "test1.mp4",
			FileSize:  1000,
		}
		video2 := &types.Video{
			UserID:    testUser.ID,
			Title:     "视频 2",
			ObjectKey: "videos/1/test2.mp4",
			FilePath:  "videos/1/test2.mp4",
			FileName:  "test2.mp4",
			FileSize:  2000,
		}

		db.Create(video1)
		db.Create(video2)

		// 模拟文件存在于存储中
		mockStorage.files[video1.ObjectKey] = []byte("content1")
		mockStorage.files[video2.ObjectKey] = []byte("content2")

		t.Cleanup(func() {
			db.Delete(video1)
			db.Delete(video2)
		})

		req := httptest.NewRequest(http.MethodGet, "/videos", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Logf("❌ 获取视频列表失败: %s", rr.Body.String())
		}

		assert.Equal(t, http.StatusOK, rr.Code)

		var response []*types.Video
		json.Unmarshal(rr.Body.Bytes(), &response)
		assert.Equal(t, 2, len(response))

		//验证返回的预签名 URL
		for _, v := range response {
			assert.Contains(t, v.FilePath, "https://mock-storage.example.com/")
			assert.Contains(t, v.FilePath, "expires=")
		}
	})

	t.Run("should return empty list for user with no videos", func(t *testing.T) {
		// ❌ 不要再用 db.Exec("DELETE FROM videos")，这会影响其他并发测试。
		// 我们需要的是隔离，而不是粗暴地清空。

		// ⭐ 1. 为这个测试创建一个全新的、一次性的用户。
		//    这个新用户（比如 ID 是 125）保证在数据库和缓存中都是完全干净的。
		hashedPassword, _ := auth.HashPassword("newpass")
		newUser := &types.User{Phone: "13900139000", Email: "new.user@example.com", Password: hashedPassword}
		db.Create(newUser)
		t.Cleanup(func() { db.Unscoped().Delete(newUser) }) // 确保测试后物理删除这个临时用户

		// ⭐ 2. 为这个新用户生成一个专用的 token。
		secret := []byte(config.Envs.JWTSecret)
		newToken, _ := auth.CreateJWT(secret, newUser.ID)

		// ⭐ 3. 使用新用户的 token 发起请求。
		req := httptest.NewRequest(http.MethodGet, "/videos", nil)
		req.Header.Set("Authorization", "Bearer "+newToken)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Logf("❌ 获取空列表失败: %s", rr.Body.String())
		}

		assert.Equal(t, http.StatusOK, rr.Code)

		// ⭐ 4. 使用 JSONEq 更精确地判断响应是否为 "[]"
		assert.JSONEq(t, `[]`, rr.Body.String(), "对于一个没有任何视频的新用户，应该返回一个空的 JSON 数组")
	})

	t.Run("should fail without authentication", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/videos", nil)
		// 不设置 Authorization

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		// 接受 401 或 403
		assert.True(t, rr.Code == http.StatusUnauthorized || rr.Code == http.StatusForbidden,
			"期望 401 或 403，实际得到 %d", rr.Code)
	})
}

// TestVideoCache 测试视频相关的缓存逻辑
func TestVideoCache(t *testing.T) {
	db, router, testUser, token, _ := setupTestHandler(t)
	defer db.Exec("DELETE FROM videos")
	defer db.Exec("DELETE FROM users")

	// 0. 初始化一个 Redis 客户端用于直接检查缓存
	redisAddr := "localhost:6379"
	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr, DB: 1})
	ctx := context.Background()
	userVideosCacheKey := "user:" + strconv.FormatInt(testUser.ID, 10) + ":videos"

	// 1. 首次请求视频列表，应该是缓存未命中
	t.Run("should miss cache on first request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/videos", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		// 验证缓存已被创建
		exists, err := redisClient.Exists(ctx, userVideosCacheKey).Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(1), exists, "首次请求后，用户视频列表缓存应存在")
	})

	// 2. 再次请求，应该是缓存命中
	t.Run("should hit cache on second request", func(t *testing.T) {
		// 为了确保是缓存命中，我们可以在数据库中创建一个视频，但缓存不更新
		// 如果返回的是空列表，说明读的是旧缓存
		db.Create(&types.Video{UserID: testUser.ID, Title: "临时视频", FilePath: "p", FileName: "f", FileSize: 1})

		req := httptest.NewRequest(http.MethodGet, "/videos", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var response []*types.Video
		json.Unmarshal(rr.Body.Bytes(), &response)
		assert.Equal(t, 0, len(response), "第二次请求应命中缓存，返回空列表")

		// 清理临时视频
		db.Exec("DELETE FROM videos WHERE title = '临时视频'")
	})

	// 3. 上传一个新视频，缓存应该被清除
	t.Run("should invalidate cache after uploading a video", func(t *testing.T) {
		// 先确保缓存存在
		redisClient.Set(ctx, userVideosCacheKey, "old_cache_data", 10*time.Minute)

		// 模拟上传
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.WriteField("title", "新上传的视频")
		part, _ := writer.CreateFormFile("video", "new.mp4")
		part.Write([]byte("new video"))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/videos", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)

		// 验证缓存已被清除
		exists, err := redisClient.Exists(ctx, userVideosCacheKey).Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(0), exists, "上传视频后，用户视频列表缓存应被清除")

		// 再次请求列表，应该能获取到新视频
		req = httptest.NewRequest(http.MethodGet, "/videos", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr = httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		var response []*types.Video
		json.Unmarshal(rr.Body.Bytes(), &response)
		assert.Equal(t, 1, len(response))
		assert.Equal(t, "新上传的视频", response[0].Title)
	})

	// 4. 删除视频，相关缓存应该被清除
	t.Run("should invalidate cache after deleting a video", func(t *testing.T) {
		// 获取刚上传的视频
		var video types.Video
		db.First(&video, "title = ?", "新上传的视频")
		videoCacheKey := "video:" + strconv.Itoa(video.ID)

		// 手动设置缓存以供测试
		redisClient.Set(ctx, userVideosCacheKey, "some_data", 10*time.Minute)
		redisClient.Set(ctx, videoCacheKey, "some_data", 10*time.Minute)

		// 发起删除请求
		req := httptest.NewRequest(http.MethodDelete, "/videos/"+strconv.Itoa(video.ID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		// 验证用户列表缓存和单个视频缓存都已被清除
		userCacheExists, _ := redisClient.Exists(ctx, userVideosCacheKey).Result()
		assert.Equal(t, int64(0), userCacheExists, "删除视频后，用户视频列表缓存应被清除")

		videoCacheExists, _ := redisClient.Exists(ctx, videoCacheKey).Result()
		assert.Equal(t, int64(0), videoCacheExists, "删除视频后，单个视频缓存应被清除")
	})
}
