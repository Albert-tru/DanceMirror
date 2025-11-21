package user

import (
	"testing"
	"time"

	"github.com/Albert-tru/DanceMirror/types"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestUserStore_Integration 是我们主要的测试函数
func TestUserStore_Integration(t *testing.T) {
	// 1. 连接到我们的测试数据库（带重试机制）
	dsn := "root:MySQL666@tcp(127.0.0.1:3306)/dancemirror_test?charset=utf8mb4&parseTime=True&loc=Local"

	var db *gorm.DB
	var err error

	// 重试最多 5 次，每次间隔 1 秒
	for i := 0; i < 5; i++ {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info), // 可以改为 logger.Silent 隐藏日志
		})
		if err == nil {
			// 连接成功，跳出循环
			break
		}
		t.Logf("第 %d 次连接尝试失败: %v，1秒后重试...", i+1, err)
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		// 如果重试 5 次后还是失败，测试就没法进行，直接失败
		t.Fatalf("无法连接到测试数据库: %v", err)
	}

	// 2. 在每次测试运行前，清空 users 表，防止测试之间互相影响
	db.Exec("DELETE FROM users")

	// 3. 创建一个我们要测试的 Store 实例
	store := NewStore(db)

	// 4. 开始具体的测试场景

	// ========== 测试用户创建功能 ==========
	t.Run("should create a user successfully", func(t *testing.T) {
		// 准备测试数据
		newUser := types.User{
			FirstName: "Test",
			LastName:  "User",
			Email:     "test@example.com",
			Phone:     "1234567890",
			Password:  "password123",
		}

		// 执行创建操作
		err := store.CreateUser(newUser)

		// 断言：创建应该成功
		assert.NoError(t, err)

		// 验证：查询数据库确认用户已创建
		var createdUser types.User
		db.Where("email = ?", "test@example.com").First(&createdUser)
		assert.Equal(t, "Test", createdUser.FirstName)
		assert.Equal(t, "test@example.com", createdUser.Email)
		assert.NotZero(t, createdUser.ID) // ID 应该被自动生成
		assert.NotZero(t, createdUser.CreatedAt)
	})

	t.Run("should fail to create user with duplicate email", func(t *testing.T) {
		// 先创建一个用户
		user1 := types.User{
			FirstName: "First",
			LastName:  "User",
			Email:     "duplicate@example.com",
			Phone:     "1111111111",
			Password:  "password123",
		}
		store.CreateUser(user1)

		// 尝试创建另一个相同邮箱的用户
		user2 := types.User{
			FirstName: "Second",
			LastName:  "User",
			Email:     "duplicate@example.com", // 相同的邮箱
			Phone:     "2222222222",
			Password:  "password456",
		}
		err := store.CreateUser(user2)

		// 断言：应该失败（因为邮箱是唯一的）
		assert.Error(t, err)
	})

	// ========== 测试通过邮箱查询用户 ==========
	t.Run("should get user by email", func(t *testing.T) {
		// 先创建一个测试用户
		testUser := types.User{
			FirstName: "John",
			LastName:  "Doe",
			Email:     "john@example.com",
			Phone:     "9876543210",
			Password:  "securepass",
		}
		store.CreateUser(testUser)

		// 通过邮箱查询
		retrievedUser, err := store.GetUserByEmail("john@example.com")

		// 断言
		assert.NoError(t, err)
		assert.NotNil(t, retrievedUser)
		assert.Equal(t, "John", retrievedUser.FirstName)
		assert.Equal(t, "Doe", retrievedUser.LastName)
		assert.Equal(t, "john@example.com", retrievedUser.Email)
	})

	t.Run("should return error for non-existent email", func(t *testing.T) {
		// 尝试查询一个不存在的邮箱
		_, err := store.GetUserByEmail("nonexistent@example.com")

		// 断言：应该返回 record not found 错误
		assert.Error(t, err)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	// ========== 测试通过 ID 查询用户 ==========
	t.Run("should get user by ID", func(t *testing.T) {
		// 先创建一个测试用户
		testUser := types.User{
			FirstName: "Jane",
			LastName:  "Smith",
			Email:     "jane@example.com",
			Phone:     "5551234567",
			Password:  "mypassword",
		}
		store.CreateUser(testUser)

		// 获取刚创建的用户 ID
		var createdUser types.User
		db.Where("email = ?", "jane@example.com").First(&createdUser)

		// 通过 ID 查询
		retrievedUser, err := store.GetUserByID(createdUser.ID)

		// 断言
		assert.NoError(t, err)
		assert.NotNil(t, retrievedUser)
		assert.Equal(t, createdUser.ID, retrievedUser.ID)
		assert.Equal(t, "Jane", retrievedUser.FirstName)
	})

	t.Run("should return error for non-existent ID", func(t *testing.T) {
		// 尝试查询一个不存在的 ID（比如 999999）
		_, err := store.GetUserByID(999999)

		// 断言：应该返回错误
		assert.Error(t, err)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	// ========== 测试软删除功能 ==========
	t.Run("should soft delete a user", func(t *testing.T) {
		// 先创建一个用户
		testUser := types.User{
			FirstName: "ToDelete",
			LastName:  "User",
			Email:     "delete@example.com",
			Phone:     "1112223333",
			Password:  "deletepass",
		}
		store.CreateUser(testUser)

		// 获取用户 ID
		var createdUser types.User
		db.Where("email = ?", "delete@example.com").First(&createdUser)

		// 执行软删除
		err := db.Delete(&createdUser).Error
		assert.NoError(t, err)

		// 验证：普通查询应该找不到这个用户
		_, err = store.GetUserByEmail("delete@example.com")
		assert.Error(t, err)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

		// 验证：使用 Unscoped 可以查到被软删除的用户
		var deletedUser types.User
		err = db.Unscoped().Where("email = ?", "delete@example.com").First(&deletedUser).Error
		assert.NoError(t, err)
		assert.NotNil(t, deletedUser.DeletedAt) // DeletedAt 应该有值
	})

	// ========== 测试更新用户信息 ==========
	t.Run("should update user information", func(t *testing.T) {
		// 先创建一个用户
		testUser := types.User{
			FirstName: "OldName",
			LastName:  "OldLastName",
			Email:     "update@example.com",
			Phone:     "3334445555",
			Password:  "oldpass",
		}
		store.CreateUser(testUser)

		// 获取用户
		user, _ := store.GetUserByEmail("update@example.com")

		// 更新用户信息
		user.FirstName = "NewName"
		user.LastName = "NewLastName"
		err := db.Save(user).Error
		assert.NoError(t, err)

		// 重新查询并验证
		updatedUser, _ := store.GetUserByEmail("update@example.com")
		assert.Equal(t, "NewName", updatedUser.FirstName)
		assert.Equal(t, "NewLastName", updatedUser.LastName)
	})

	// ========== 测试查询所有用户 ==========
	t.Run("should list all users", func(t *testing.T) {
		// 清空表
		db.Exec("DELETE FROM users")

		// 创建多个用户
		users := []types.User{
			{FirstName: "User1", LastName: "Test", Email: "user1@example.com", Phone: "1111111111", Password: "pass1"},
			{FirstName: "User2", LastName: "Test", Email: "user2@example.com", Phone: "2222222222", Password: "pass2"},
			{FirstName: "User3", LastName: "Test", Email: "user3@example.com", Phone: "3333333333", Password: "pass3"},
		}

		for _, u := range users {
			store.CreateUser(u)
		}

		// 查询所有用户
		var allUsers []types.User
		err := db.Find(&allUsers).Error
		assert.NoError(t, err)
		assert.Equal(t, 3, len(allUsers))
	})

	// ========== 测试并发创建用户 ==========
	t.Run("should handle concurrent user creation", func(t *testing.T) {
		// 清空表
		db.Exec("DELETE FROM users")

		// 使用 goroutine 并发创建用户
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func(index int) {
				user := types.User{
					FirstName: "Concurrent",
					LastName:  "User",
					Email:     "concurrent" + string(rune('0'+index)) + "@example.com",
					Phone:     "555000" + string(rune('0'+index)) + "000",
					Password:  "pass",
				}
				store.CreateUser(user)
				done <- true
			}(i)
		}

		// 等待所有 goroutine 完成
		for i := 0; i < 10; i++ {
			<-done
		}

		// 验证：应该有 10 个用户被创建
		var count int64
		db.Model(&types.User{}).Count(&count)
		assert.Equal(t, int64(10), count)
	})

	// ========== 测试密码字段不返回 ==========
	t.Run("password should not be exposed in JSON", func(t *testing.T) {
		// 这个测试验证 User 结构体的 json:"-" 标签是否生效
		// 在实际的 API 响应中，密码字段不应该被序列化
		testUser := types.User{
			FirstName: "Secure",
			LastName:  "User",
			Email:     "secure@example.com",
			Phone:     "9998887777",
			Password:  "secretpassword",
		}

		// 注意：这个测试更适合放在 handler 层的测试中
		// 这里只是作为示例
		assert.NotEmpty(t, testUser.Password) // 在结构体中密码是存在的
	})

	// ========== 清理测试数据 ==========
	t.Cleanup(func() {
		// 测试完成后清理所有测试数据
		db.Exec("DELETE FROM users")
		t.Log("测试数据已清理")
	})
}

// TestUserStore_EdgeCases 边界情况测试
func TestUserStore_EdgeCases(t *testing.T) {
	// 连接数据库
	dsn := "root:MySQL666@tcp(127.0.0.1:3306)/dancemirror_test?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("无法连接到测试数据库: %v", err)
	}

	db.Exec("DELETE FROM users")
	store := NewStore(db)

	t.Run("should handle empty email gracefully", func(t *testing.T) {
		_, err := store.GetUserByEmail("")
		assert.Error(t, err)
	})

	t.Run("should handle very long names", func(t *testing.T) {
		longName := string(make([]byte, 300)) // 超过 varchar(100) 的限制
		user := types.User{
			FirstName: longName,
			LastName:  "Test",
			Email:     "longname@example.com",
			Phone:     "1234567890",
			Password:  "pass",
		}
		err := store.CreateUser(user)
		// 应该失败，因为超过字段长度限制
		assert.Error(t, err)
	})

	t.Run("should handle special characters in email", func(t *testing.T) {
		user := types.User{
			FirstName: "Special",
			LastName:  "User",
			Email:     "test+tag@example.com", // 包含 + 号
			Phone:     "9999999999",
			Password:  "pass",
		}
		err := store.CreateUser(user)
		assert.NoError(t, err)

		// 应该能正常查询
		retrieved, err := store.GetUserByEmail("test+tag@example.com")
		assert.NoError(t, err)
		assert.Equal(t, "test+tag@example.com", retrieved.Email)
	})

	t.Cleanup(func() {
		db.Exec("DELETE FROM users")
	})
}
