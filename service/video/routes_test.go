package video

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Albert-tru/DanceMirror/config"
	"github.com/Albert-tru/DanceMirror/service/auth"
	"github.com/Albert-tru/DanceMirror/types"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTestHandler 创建测试用的 Handler 和路由
func setupTestHandler(t *testing.T) (*gorm.DB, *mux.Router, *types.User, string) {
	// 连接测试数据库
	dsn := "root:MySQL666@tcp(127.0.0.1:3306)/dancemirror_test?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("无法连接到测试数据库: %v", err)
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

	// 创建 Store 和 Handler
	videoStore := NewStore(db)
	userStore := &mockUserStore{db: db}
	handler := NewHandler(videoStore, userStore)

	// 创建路由
	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	return db, router, testUser, token
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

func (m *mockUserStore) GetUserByID(id int) (*types.User, error) {
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

// TestHandleUpload 测试视频上传
func TestHandleUpload(t *testing.T) {
	db, router, testUser, token := setupTestHandler(t)
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
		assert.NotEmpty(t, response.FilePath)
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

// TestHandleGetVideos 测试获取视频列表
func TestHandleGetVideos(t *testing.T) {
	db, router, testUser, token := setupTestHandler(t)
	defer db.Exec("DELETE FROM videos")
	defer db.Exec("DELETE FROM users")

	t.Run("should get user's videos", func(t *testing.T) {
		// 创建测试视频
		videos := []*types.Video{
			{
				UserID:      testUser.ID,
				Title:       "视频 1",
				Description: "描述 1",
				FilePath:    "/uploads/video1.mp4",
				FileName:    "video1.mp4",
				FileSize:    1000,
			},
			{
				UserID:      testUser.ID,
				Title:       "视频 2",
				Description: "描述 2",
				FilePath:    "/uploads/video2.mp4",
				FileName:    "video2.mp4",
				FileSize:    2000,
			},
		}

		for _, v := range videos {
			db.Create(v)
		}

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
	})

	t.Run("should return empty list for user with no videos", func(t *testing.T) {
		db.Exec("DELETE FROM videos")

		req := httptest.NewRequest(http.MethodGet, "/videos", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Logf("❌ 获取空列表失败: %s", rr.Body.String())
		}

		assert.Equal(t, http.StatusOK, rr.Code)

		var response []*types.Video
		json.Unmarshal(rr.Body.Bytes(), &response)
		assert.Equal(t, 0, len(response))
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

// ... 其余测试保持不变
