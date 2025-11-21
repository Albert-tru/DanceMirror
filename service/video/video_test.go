package video

import (
    "testing"
    "time"

    "github.com/Albert-tru/DanceMirror/types"
    "github.com/stretchr/testify/assert"
    "gorm.io/driver/mysql"
    "gorm.io/gorm"
    "gorm.io/gorm/logger"
)

// TestVideoStore_Integration 视频存储集成测试
func TestVideoStore_Integration(t *testing.T) {
    // 1. 连接到测试数据库（带重试机制）
    dsn := "root:MySQL666@tcp(127.0.0.1:3306)/dancemirror_test?charset=utf8mb4&parseTime=True&loc=Local"

    var db *gorm.DB
    var err error

    // 重试最多 5 次，每次间隔 1 秒
    for i := 0; i < 5; i++ {
        db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
            Logger: logger.Default.LogMode(logger.Info),
        })
        if err == nil {
            break
        }
        t.Logf("第 %d 次连接尝试失败: %v，1秒后重试...", i+1, err)
        time.Sleep(1 * time.Second)
    }

    if err != nil {
        t.Fatalf("无法连接到测试数据库: %v", err)
    }

    // 2. 清空测试数据
    db.Exec("DELETE FROM videos")
    db.Exec("DELETE FROM users")

    // 3. 创建测试用户（因为 videos 表有外键约束）
    testUser := types.User{
        FirstName: "Video",
        LastName:  "Tester",
        Email:     "video.tester@example.com",
        Phone:     "1234567890",
        Password:  "testpass",
    }
    db.Create(&testUser)

    // 4. 创建 Store 实例
    store := NewStore(db)

    // ========== 测试创建视频 ==========
    t.Run("should create a video successfully", func(t *testing.T) {
        video := &types.Video{
            UserID:      testUser.ID,
            Title:       "我的第一支舞",
            Description: "这是一个测试视频",
            FilePath:    "/uploads/videos/test-video.mp4",
            FileName:    "test-video.mp4",
            FileSize:    1024000,
            Duration:    120.5,
            Thumbnail:   "/uploads/thumbnails/test-thumb.jpg",
        }

        err := store.CreateVideo(video)
        assert.NoError(t, err)
        assert.NotZero(t, video.ID)
        assert.NotZero(t, video.CreatedAt)
    })

    // ========== 测试通过 ID 获取视频 ==========
    t.Run("should get video by ID", func(t *testing.T) {
        video := &types.Video{
            UserID:      testUser.ID,
            Title:       "测试视频 - GetByID",
            Description: "用于测试查询功能",
            FilePath:    "/uploads/videos/test-get.mp4",
            FileName:    "test-get.mp4",
            FileSize:    2048000,
            Duration:    180.0,
        }
        store.CreateVideo(video)

        retrieved, err := store.GetVideoByID(video.ID)

        assert.NoError(t, err)
        assert.NotNil(t, retrieved)
        assert.Equal(t, video.ID, retrieved.ID)
        assert.Equal(t, "测试视频 - GetByID", retrieved.Title)
    })

    t.Run("should return error for non-existent video ID", func(t *testing.T) {
        _, err := store.GetVideoByID(999999)
        assert.Error(t, err)
        assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
    })

    t.Cleanup(func() {
        db.Exec("DELETE FROM videos")
        db.Exec("DELETE FROM users")
        t.Log("视频测试数据已清理")
    })
}
