package user

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Albert-tru/DanceMirror/service/auth"
	"github.com/Albert-tru/DanceMirror/types"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTestHandler 创建测试用的 Handler 和路由
func setupTestHandler(t *testing.T) (*Handler, *gorm.DB, *mux.Router) {
	// 连接测试数据库
	dsn := "root:MySQL666@tcp(127.0.0.1:3306)/dancemirror_test?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("无法连接到测试数据库: %v", err)
	}

	// 清空测试数据
	db.Exec("DELETE FROM users")

	// 创建 Store 和 Handler
	store := NewStore(db)
	handler := NewHandler(store)

	// 创建路由
	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	return handler, db, router
}

// TestHandleRegister 测试用户注册
func TestHandleRegister(t *testing.T) {
	_, db, router := setupTestHandler(t)
	defer db.Exec("DELETE FROM users")

	t.Run("should register a new user successfully", func(t *testing.T) {
		// 准备请求数据
		payload := types.RegisterUserPayload{
			Phone:     "13800138000",
			Email:     "test@example.com",
			Password:  "password123",
			FirstName: "张",
			LastName:  "三",
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		// 创建响应记录器
		rr := httptest.NewRecorder()

		// 执行请求
		router.ServeHTTP(rr, req)

		// 断言响应
		assert.Equal(t, http.StatusCreated, rr.Code)

		var response map[string]string
		json.Unmarshal(rr.Body.Bytes(), &response)
		assert.Equal(t, "user created successfully", response["message"])

		// 验证用户已创建
		var user types.User
		db.Where("phone = ?", "13800138000").First(&user)
		assert.Equal(t, "张", user.FirstName)
		assert.Equal(t, "三", user.LastName)
		assert.Equal(t, "test@example.com", user.Email)
		assert.NotEmpty(t, user.Password)                // 密码应该被哈希
		assert.NotEqual(t, "password123", user.Password) // 不应该是明文
	})

	t.Run("should fail with duplicate phone number", func(t *testing.T) {
		// 先创建一个用户
		hashedPass, _ := auth.HashPassword("testpass")
		db.Create(&types.User{
			Phone:     "13900139000",
			Email:     "unique1@example.com",
			Password:  hashedPass,
			FirstName: "Test",
			LastName:  "User",
		})

		// 尝试用相同手机号注册
		payload := types.RegisterUserPayload{
			Phone:     "13900139000",         // 重复的手机号
			Email:     "unique2@example.com", // 不同的邮箱
			Password:  "password123",
			FirstName: "Another",
			LastName:  "User",
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		// 应该返回 400 错误
		assert.Equal(t, http.StatusBadRequest, rr.Code)

		var response map[string]string
		json.Unmarshal(rr.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "已被注册")
	})

	t.Run("should fail with invalid payload - missing phone", func(t *testing.T) {
		payload := map[string]interface{}{
			// Phone 缺失
			"password":  "password123",
			"firstName": "Test",
			"lastName":  "User",
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)

		var response map[string]string
		json.Unmarshal(rr.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "invalid payload")
	})

	t.Run("should fail with invalid payload - missing password", func(t *testing.T) {
		payload := map[string]interface{}{
			"phone": "13800138001",
			// Password 缺失
			"firstName": "Test",
			"lastName":  "User",
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)

		var response map[string]string
		json.Unmarshal(rr.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "invalid payload")
	})

	t.Run("should fail with invalid payload - missing firstName", func(t *testing.T) {
		payload := map[string]interface{}{
			"phone":    "13800138002",
			"password": "password123",
			// firstName 缺失
			"lastName": "User",
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("should fail with invalid payload - missing lastName", func(t *testing.T) {
		payload := map[string]interface{}{
			"phone":     "13800138003",
			"password":  "password123",
			"firstName": "Test",
			// lastName 缺失
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("should fail with invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("should fail with empty request body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer([]byte{}))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("should fail with short password", func(t *testing.T) {
		payload := types.RegisterUserPayload{
			Phone:     "13800138004",
			Password:  "123", // 太短的密码
			FirstName: "Test",
			LastName:  "User",
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)

		var response map[string]string
		json.Unmarshal(rr.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "invalid payload")
	})
}

// TestHandleLogin 测试用户登录
func TestHandleLogin(t *testing.T) {
	_, db, router := setupTestHandler(t)
	defer db.Exec("DELETE FROM users")

	// 先创建一个测试用户
	testPhone := "13800138000"
	testPassword := "password123"
	hashedPassword, _ := auth.HashPassword(testPassword)

	db.Create(&types.User{
		Phone:     testPhone,
		Password:  hashedPassword,
		FirstName: "Test",
		LastName:  "User",
	})

	t.Run("should login successfully with correct credentials", func(t *testing.T) {
		loginPayload := types.LoginUserPayload{
			Phone:    testPhone,
			Password: testPassword,
		}

		body, _ := json.Marshal(loginPayload)
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		// 断言响应
		assert.Equal(t, http.StatusOK, rr.Code)

		var response map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &response)

		// 验证返回了 token
		assert.NotEmpty(t, response["token"])

		// 验证返回了用户信息
		user := response["user"].(map[string]interface{})
		assert.Equal(t, testPhone, user["phone"])
		assert.Equal(t, "Test", user["firstName"])
		assert.Equal(t, "User", user["lastName"])
		assert.NotZero(t, user["id"])

		// 密码不应该被返回
		_, hasPassword := user["password"]
		assert.False(t, hasPassword, "密码不应该在响应中返回")
	})

	t.Run("should fail with wrong password", func(t *testing.T) {
		loginPayload := types.LoginUserPayload{
			Phone:    testPhone,
			Password: "wrongpassword",
		}

		body, _ := json.Marshal(loginPayload)
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)

		var response map[string]string
		json.Unmarshal(rr.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "手机号或密码错误")
	})

	t.Run("should fail with non-existent phone", func(t *testing.T) {
		loginPayload := types.LoginUserPayload{
			Phone:    "99999999999",
			Password: testPassword,
		}

		body, _ := json.Marshal(loginPayload)
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)

		var response map[string]string
		json.Unmarshal(rr.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "手机号或密码错误")
	})

	t.Run("should fail with invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("should fail with missing phone", func(t *testing.T) {
		loginPayload := map[string]interface{}{
			// Phone 缺失
			"password": testPassword,
		}

		body, _ := json.Marshal(loginPayload)
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)

		var response map[string]string
		json.Unmarshal(rr.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "invalid payload")
	})

	t.Run("should fail with missing password", func(t *testing.T) {
		loginPayload := map[string]interface{}{
			"phone": testPhone,
			// Password 缺失
		}

		body, _ := json.Marshal(loginPayload)
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)

		var response map[string]string
		json.Unmarshal(rr.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "invalid payload")
	})

	t.Run("should fail with empty phone", func(t *testing.T) {
		loginPayload := types.LoginUserPayload{
			Phone:    "",
			Password: testPassword,
		}

		body, _ := json.Marshal(loginPayload)
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("should fail with empty password", func(t *testing.T) {
		loginPayload := types.LoginUserPayload{
			Phone:    testPhone,
			Password: "",
		}

		body, _ := json.Marshal(loginPayload)
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

// TestRegisterAndLogin 测试完整的注册登录流程
func TestRegisterAndLogin(t *testing.T) {
	_, db, router := setupTestHandler(t)
	defer db.Exec("DELETE FROM users")

	// 第一步：注册新用户
	registerPayload := types.RegisterUserPayload{
		Phone:     "13800138888",
		Password:  "securepassword",
		FirstName: "完整",
		LastName:  "测试",
	}

	body, _ := json.Marshal(registerPayload)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	// 第二步：用刚注册的账号登录
	loginPayload := types.LoginUserPayload{
		Phone:    "13800138888",
		Password: "securepassword",
	}

	body, _ = json.Marshal(loginPayload)
	req = httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	// 验证登录成功
	assert.NotEmpty(t, response["token"])
	user := response["user"].(map[string]interface{})
	assert.Equal(t, "13800138888", user["phone"])
	assert.Equal(t, "完整", user["firstName"])
	assert.Equal(t, "测试", user["lastName"])

	t.Log("✅ 注册 → 登录流程测试通过")
}

// TestConcurrentRegistrations 测试并发注册
func TestConcurrentRegistrations(t *testing.T) {
	_, db, router := setupTestHandler(t)
	defer db.Exec("DELETE FROM users")

	// 并发注册 10 个不同的用户
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(index int) {
			payload := types.RegisterUserPayload{
				Phone:     "1380013800" + string(rune('0'+index)),
				Password:  "password123",
				FirstName: "User",
				LastName:  string(rune('A' + index)),
			}

			body, _ := json.Marshal(payload)
			req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			done <- true
		}(i)
	}

	// 等待所有请求完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证所有用户都被创建
	var count int64
	db.Model(&types.User{}).Count(&count)
	assert.Equal(t, int64(10), count)
}
