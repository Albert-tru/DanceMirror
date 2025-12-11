package mq

import (
	"os"
	"testing"
	"time"

	"github.com/Albert-tru/DanceMirror/service/video"
	"github.com/Albert-tru/DanceMirror/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// MockStorage 模拟存储客户端，避免真的上传到 MinIO
type MockStorage struct{}

func (m *MockStorage) Upload(ctx interface{}, objectKey string, file interface{}, size int64, contentType string) error {
	return nil // 假装上传成功
}
func (m *MockStorage) GetPresignedURL(objectKey string, expiry time.Duration) (string, error) {
	return "http://mock-url/" + objectKey, nil
}

// 确保 MockStorage 实现了接口 (根据你的实际接口定义调整)
// func (m *MockStorage) Delete(...) error { return nil }

func TestRabbitMQIntegration(t *testing.T) {
	// 1. 跳过测试如果环境变量没设置 (避免在没有 Docker 的 CI 环境报错)
	mqURL := os.Getenv("RABBITMQ_URL")
	if mqURL == "" {
		// 默认尝试本地 Docker
		mqURL = "amqp://guest:guest@localhost:5672/"
	}

	// 2. 初始化内存数据库 (SQLite) 用于测试
	// 这样不会污染你的真实 MySQL 数据
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to test DB: %v", err)
	}
	// 自动迁移表结构
	db.AutoMigrate(&types.Video{})

	// 3. 插入一条测试用的视频记录
	testVideo := types.Video{
		ID:     999,
		UserID: 1,
		Status: "pending",
	}
	db.Create(&testVideo)

	// 4. 注入依赖
	SetDB(db)
	// 这里我们暂时不注入 StorageClient，或者注入一个 Mock，
	// 因为我们主要测试消息流转，不想真的上传文件
	// SetStorageClient(&MockStorage{})

	// 5. 初始化 RabbitMQ
	err = InitRabbitMQ(mqURL)
	if err != nil {
		t.Logf("⚠️ 无法连接 RabbitMQ (%s), 跳过集成测试: %v", mqURL, err)
		t.Skip("RabbitMQ not available")
		return
	}
	defer Close()

	// 6. 启动消费者 Worker
	StartCropWorker()

	// 7. 构造一个“假”任务
	// 注意：InputPath 需要是一个真实存在的文件，否则 CropVideo 会报错
	// 这里我们创建一个临时文件来模拟
	tmpFile, _ := os.CreateTemp("", "test_video_*.mp4")
	defer os.Remove(tmpFile.Name()) // 测试完删除

	task := CropTask{
		VideoID:    999,
		UserID:     1,
		InputPath:  tmpFile.Name(), // 使用临时文件路径
		OutputPath: tmpFile.Name() + "_cropped.mp4",
		Params:     video.CropParams{X: 0, Y: 0, Width: 100, Height: 100},
	}

	// 8. 发送消息 (生产者)
	err = PublishCropTask(task)
	if err != nil {
		t.Fatalf("Failed to publish task: %v", err)
	}
	t.Log("✅ 消息已发送")

	// 9. 等待 Worker 处理 (异步的，所以要等)
	// 因为我们没有真的 FFmpeg 环境或者文件可能无效，Worker 可能会报错并把状态改为 failed
	// 或者如果文件存在且 FFmpeg 可用，状态会变成 completed
	// 我们主要验证状态发生了“变化”，不再是 "pending"

	t.Log("⏳ 等待 Worker 处理...")
	time.Sleep(2 * time.Second)

	// 10. 检查数据库状态
	var updatedVideo types.Video
	db.First(&updatedVideo, 999)

	t.Logf("视频最终状态: %s", updatedVideo.Status)

	if updatedVideo.Status == "pending" {
		t.Errorf("测试失败: 视频状态仍然是 pending，说明 Worker 没有消费消息")
	} else {
		t.Log("✅ 测试通过: Worker 成功消费了消息并更新了数据库")
	}
}
