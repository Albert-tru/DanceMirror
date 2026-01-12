package video

import (
	"testing"

	"github.com/Albert-tru/DanceMirror/types"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestVideoStore_Integration 视频存储集成测试
func TestVideoStore_Integration(t *testing.T) {
	// 1. 连接到测试数据库
	dsn := "root:MySQL666@tcp(127.0.0.1:3306)/dancemirror_test?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // 使用 Silent 模式保持日志干净
	})
	if err != nil {
		t.Fatalf("无法连接到测试数据库: %v", err)
	}

	// 2. 创建一个贯穿所有子测试的共享用户
	testUser := &types.User{
		Phone:    "1234567890",
		Email:    "video.tester@example.com",
		Password: "testpass",
	}
	db.Create(testUser)

	// 3. 创建 Store 实例
	store := NewStore(db, nil)

	// 4. 在所有测试结束后清理用户
	t.Cleanup(func() {
		db.Exec("DELETE FROM videos") // 清理所有视频
		db.Exec("DELETE FROM users")
		t.Log("视频测试数据已清理")
	})

	// ========== 测试创建视频 ==========
	t.Run("should create a video successfully", func(t *testing.T) {
		video := &types.Video{
			UserID:   testUser.ID,
			Title:    "我的第一支舞",
			FilePath: "p", FileName: "f", FileSize: 1,
		}
		err := store.CreateVideo(video)
		assert.NoError(t, err)
		assert.NotZero(t, video.ID)

		// ⭐ 清理本次测试创建的数据
		t.Cleanup(func() {
			db.Unscoped().Delete(video)
		})
	})

	// ========== 测试通过 ID 获取视频 ==========
	t.Run("should get video by ID", func(t *testing.T) {
		// ⭐ 先创建本测试专用的视频
		video := &types.Video{UserID: testUser.ID, Title: "测试视频 - GetByID", FilePath: "p", FileName: "f", FileSize: 1}
		db.Create(video)
		t.Cleanup(func() { db.Unscoped().Delete(video) }) // 保证清理

		retrieved, err := store.GetVideoByID(video.ID)
		assert.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, video.ID, retrieved.ID)
	})

	t.Run("should return error for non-existent video ID", func(t *testing.T) {
		_, err := store.GetVideoByID(999999)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	// ========== 测试获取用户的所有视频 ==========
	t.Run("should get all videos for a user", func(t *testing.T) {
		// ⭐ 1. 为本测试创建独立的视频数据
		videosToCreate := []*types.Video{
			{UserID: testUser.ID, Title: "视频 1", FilePath: "p1", FileName: "f1", FileSize: 1},
			{UserID: testUser.ID, Title: "视频 2", FilePath: "p2", FileName: "f2", FileSize: 1},
			{UserID: testUser.ID, Title: "视频 3", FilePath: "p3", FileName: "f3", FileSize: 1},
		}
		db.Create(&videosToCreate)
		t.Cleanup(func() { db.Unscoped().Delete(&videosToCreate) }) // 清理这3个视频

		// ⭐ 2. 执行查询
		videos, err := store.GetVideos(testUser.ID)

		// ⭐ 3. 断言
		assert.NoError(t, err)
		assert.Equal(t, 3, len(videos))
	})

	// ========== 测试软删除视频 ==========
	t.Run("should soft delete a video", func(t *testing.T) {
		// ⭐ 1. 创建一个本测试专用的视频
		video := &types.Video{UserID: testUser.ID, Title: "待删除视频", FilePath: "p", FileName: "f", FileSize: 1}
		db.Create(video)
		t.Cleanup(func() { db.Unscoped().Delete(video) }) // 确保物理删除

		// ⭐ 2. 执行软删除
		err := store.DeleteVideo(video.ID)
		assert.NoError(t, err)

		// ⭐ 3. 验证：普通查询应该找不到
		_, err = store.GetVideoByID(video.ID)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

		// ⭐ 4. 验证：使用 Unscoped 可以查到，并且 DeletedAt 不为 nil
		var deletedVideo types.Video
		err = db.Unscoped().First(&deletedVideo, video.ID).Error
		assert.NoError(t, err)
		assert.NotNil(t, deletedVideo.DeletedAt)
	})

	// ========== 测试视频字段验证 ==========
	t.Run("should handle video with minimal fields", func(t *testing.T) {
		// 只提供必需字段
		video := &types.Video{
			UserID:   testUser.ID,
			Title:    "最小视频",
			FilePath: "/uploads/minimal.mp4",
			FileName: "minimal.mp4",
			FileSize: 100000,
			// 没有 Description、Duration、Thumbnail
		}

		err := store.CreateVideo(video)
		assert.NoError(t, err)

		retrieved, _ := store.GetVideoByID(video.ID)
		assert.Equal(t, "最小视频", retrieved.Title)
		assert.Empty(t, retrieved.Description)
		assert.Equal(t, 0.0, retrieved.Duration)
		assert.Empty(t, retrieved.Thumbnail)
	})

	// ========== 测试视频文件大小和时长 ==========
	t.Run("should handle large video files", func(t *testing.T) {
		video := &types.Video{
			UserID:      testUser.ID,
			Title:       "大文件视频",
			Description: "测试大文件",
			FilePath:    "/uploads/large.mp4",
			FileName:    "large.mp4",
			FileSize:    524288000, // 500MB
			Duration:    3600.5,    // 1小时
		}

		err := store.CreateVideo(video)
		assert.NoError(t, err)

		retrieved, _ := store.GetVideoByID(video.ID)
		assert.Equal(t, int64(524288000), retrieved.FileSize)
		assert.Equal(t, 3600.5, retrieved.Duration)
	})

	// ========== 测试多用户视频隔离 ==========
	t.Run("should isolate videos between users", func(t *testing.T) {
		db.Exec("DELETE FROM videos")

		// 创建第二个用户
		user2 := types.User{
			FirstName: "Second",
			LastName:  "User",
			Email:     "user2@example.com",
			Phone:     "2222222222",
			Password:  "pass",
		}
		db.Create(&user2)

		// 为第一个用户创建 2 个视频
		for i := 1; i <= 2; i++ {
			video := &types.Video{
				UserID:   testUser.ID,
				Title:    "User1 视频",
				FilePath: "/uploads/user1.mp4",
				FileName: "user1.mp4",
				FileSize: 1000000,
			}
			store.CreateVideo(video)
		}

		// 为第二个用户创建 3 个视频
		for i := 1; i <= 3; i++ {
			video := &types.Video{
				UserID:   user2.ID,
				Title:    "User2 视频",
				FilePath: "/uploads/user2.mp4",
				FileName: "user2.mp4",
				FileSize: 2000000,
			}
			store.CreateVideo(video)
		}

		// 验证隔离性
		user1Videos, _ := store.GetVideos(testUser.ID)
		user2Videos, _ := store.GetVideos(user2.ID)

		assert.Equal(t, 2, len(user1Videos))
		assert.Equal(t, 3, len(user2Videos))
	})

	// ========== 测试并发上传视频 ==========
	t.Run("should handle concurrent video uploads", func(t *testing.T) {
		db.Exec("DELETE FROM videos")

		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func(index int) {
				video := &types.Video{
					UserID:      testUser.ID,
					Title:       "并发视频",
					Description: "测试并发上传",
					FilePath:    "/uploads/concurrent.mp4",
					FileName:    "concurrent.mp4",
					FileSize:    1000000,
					Duration:    60.0,
				}
				store.CreateVideo(video)
				done <- true
			}(i)
		}

		// 等待所有 goroutine 完成
		for i := 0; i < 10; i++ {
			<-done
		}

		// 验证所有视频都被创建
		videos, _ := store.GetVideos(testUser.ID)
		assert.Equal(t, 10, len(videos))
	})

	// ========== 清理测试数据 ==========
	t.Cleanup(func() {
		db.Exec("DELETE FROM videos")
		db.Exec("DELETE FROM users")
		t.Log("视频测试数据已清理")
	})
}

// TestVideoStore_EdgeCases 边界情况测试
func TestVideoStore_EdgeCases(t *testing.T) {
	dsn := "root:MySQL666@tcp(127.0.0.1:3306)/dancemirror_test?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("无法连接到测试数据库: %v", err)
	}

	db.Exec("DELETE FROM videos")
	db.Exec("DELETE FROM users")

	// 创建测试用户
	testUser := types.User{
		FirstName: "Edge",
		LastName:  "Tester",
		Email:     "edge@example.com",
		Phone:     "5555555555",
		Password:  "pass",
	}
	db.Create(&testUser)

	store := NewStore(db, nil)

	t.Run("should handle very long title", func(t *testing.T) {
		longTitle := string(make([]byte, 300)) // 超长标题
		video := &types.Video{
			UserID:   testUser.ID,
			Title:    longTitle,
			FilePath: "/uploads/long.mp4",
			FileName: "long.mp4",
			FileSize: 1000000,
		}
		err := store.CreateVideo(video)
		// 可能会因为字段长度限制而失败
		if err != nil {
			t.Logf("预期的错误: %v", err)
		}
	})

	t.Run("should handle special characters in filename", func(t *testing.T) {
		video := &types.Video{
			UserID:      testUser.ID,
			Title:       "特殊字符测试",
			FilePath:    "/uploads/测试-视频_#1.mp4",
			FileName:    "测试-视频_#1.mp4",
			FileSize:    1000000,
			Description: "包含特殊字符: !@#$%^&*()",
		}
		err := store.CreateVideo(video)
		assert.NoError(t, err)

		retrieved, _ := store.GetVideoByID(video.ID)
		assert.Equal(t, "测试-视频_#1.mp4", retrieved.FileName)
	})

	t.Run("should handle zero duration", func(t *testing.T) {
		video := &types.Video{
			UserID:   testUser.ID,
			Title:    "零时长视频",
			FilePath: "/uploads/zero.mp4",
			FileName: "zero.mp4",
			FileSize: 1000,
			Duration: 0.0,
		}
		err := store.CreateVideo(video)
		assert.NoError(t, err)

		retrieved, _ := store.GetVideoByID(video.ID)
		assert.Equal(t, 0.0, retrieved.Duration)
	})

	t.Run("should handle negative user ID", func(t *testing.T) {
		videos, err := store.GetVideos(-1)
		assert.NoError(t, err) // 不应该报错，只是返回空列表
		assert.Equal(t, 0, len(videos))
	})

	t.Cleanup(func() {
		db.Exec("DELETE FROM videos")
		db.Exec("DELETE FROM users")
	})
}
