package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/Albert-tru/DanceMirror/config"
	"github.com/Albert-tru/DanceMirror/service/mq"
	"github.com/Albert-tru/DanceMirror/service/storage"
	"github.com/Albert-tru/DanceMirror/service/video"
"github.com/Albert-tru/DanceMirror/service/search"
	"github.com/Albert-tru/DanceMirror/types"
	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
)

type CropWorker struct {
	mqClient *mq.RabbitMQClient
	db       *gorm.DB
	storage  storage.VideoStorage
	queue    string
}

func NewCropWorker(mqClient *mq.RabbitMQClient, db *gorm.DB, storage storage.VideoStorage, queue string) *CropWorker {
	return &CropWorker{
		mqClient: mqClient,
		db:       db,
		storage:  storage,
		queue:    queue,
	}
}

// Start 启动 Worker Pool
// concurrency: 并发消费者数量
func (w *CropWorker) Start(concurrency int) {
	log.Printf("🚀 Starting CropWorker pool with %d workers...", concurrency)

	msgs, err := w.mqClient.Consume(w.queue)
	if err != nil {
		log.Fatalf("❌ Failed to consume queue: %v", err)
	}

	// 启动并发 Worker
	for i := 0; i < concurrency; i++ {
		go w.processMessages(i, msgs)
	}
}

func (w *CropWorker) processMessages(id int, msgs <-chan amqp.Delivery) {
	log.Printf("👷 Worker %d ready", id)

	for d := range msgs {
		log.Printf("👷 Worker %d received task", id)

		var task mq.CropTask
		if err := json.Unmarshal(d.Body, &task); err != nil {
			log.Printf("❌ Failed to parse task: %v", err)
			d.Nack(false, false) // 格式错误，不重试
			continue
		}

		// 处理裁剪逻辑
		if err := w.handleCropTask(task); err != nil {
			log.Printf("❌ Task failed (VideoID: %d): %v", task.VideoID, err)
			// 可以选择 d.Nack(false, true) 重试，或者记录失败状态
			w.updateVideoStatus(task.VideoID, "failed")
			d.Nack(false, false) // 暂时不重试，避免死循环
			//如果视频文件损坏导致ffmeg崩溃，就直接丢弃
		} else {
			log.Printf("✅ Task completed (VideoID: %d)", task.VideoID)
			w.updateVideoStatus(task.VideoID, "done")
			d.Ack(false)
		}
	}
}

func (w *CropWorker) handleCropTask(task mq.CropTask) error {
	w.updateVideoStatus(task.VideoID, "processing")

	// 1. 本地裁剪 (FFmpeg 生成到 OutputPath)
	// OutputPath 在发布任务时已被指定为本地临时路径
	if err := video.CropVideo(task.InputPath, task.OutputPath, task.Params); err != nil {
		return err
	}

	// 2. 处理结果文件
	// 如果是 MinIO 模式，需要把生成的本地文件上传上去
	// 如果是 Local 模式，CropVideo 已经生成在指定位置（可能在 temp），我们可能需要移动或者直接使用

	finalStorePath := ""

	if config.Envs.StorageDriver == "minio" {
		file, err := os.Open(task.OutputPath)
		if err != nil {
			return fmt.Errorf("failed to open processed file: %v", err)
		}
		defer file.Close()
		defer os.Remove(task.OutputPath) // 上传后清理本地临时文件

		stat, _ := file.Stat()
		objectKey := fmt.Sprintf("videos/%d/cropped_%d.mp4", task.UserID, time.Now().Unix())

		err = w.storage.Upload(context.Background(), objectKey, file, stat.Size(), "video/mp4")
		if err != nil {
			return fmt.Errorf("failed to upload cropped video to MinIO: %v", err)
		}
		finalStorePath = objectKey
	} else {
		// 本地模式：
		// 任务发布时 OutputPath 是 .../uploads/temp/crop_...mp4
		// 我们将其移动到 .../uploads/cropped/ 或者直接更新数据库指向 temp（不推荐）
		// 这里简单处理：保留在生成的位置，或者根据 Storage 逻辑

		// 假设我们不需要移动，直接使用生成的路径相对于 UploadDir 的路径
		// task.OutputPath 是绝对路径
		// 我们需要存储相对路径给前端访问
		// 简单起见，我们假设 finalStorePath 就是文件名，配合 static handler
		// 或者我们计算相对路径

		rel, err := filepath.Rel(config.Envs.UploadDir, task.OutputPath)
		if err == nil {
			finalStorePath = rel
		} else {
			finalStorePath = task.OutputPath
		}
	}

	// 3. 更新数据库中的视频记录 (例如更新一个新的字段，或者创建一个新视频记录)
	// 这里逻辑简单处理：更新原视频状态，实际业务可能需要保存裁剪后的视频作为新记录
	// 题目场景没有明确，假设是替换或者更新
	// 但通常裁剪是产生新文件。

	// 更新 Video 表的 StoragePath (如果有这个字段) 或者 ObjectKey needed?
	// 这里只更新状态，假设前端通过 ID 查询状态
	// 如果需要保存裁剪结果链接，应该更新 DB

	return w.saveCroppedVideoRecord(task.VideoID, finalStorePath)
}

func (w *CropWorker) updateVideoStatus(videoID int, status string) {
	if err := w.db.Model(&types.Video{}).Where("id = ?", videoID).Update("status", status).Error; err != nil {
		log.Printf("Database warning: failed to update status: %v", err)
	}
}

func (w *CropWorker) saveCroppedVideoRecord(originalVideoID int, newPath string) error {
	// 这里可以选择创建一个新视频记录，或者更新原记录的一个字段
	//为了简单展示，我们更新原记录的 Description 附加信息，或者假设有一个 CroppedPath 字段
	// 或者直接打印日志

	// 更好的做法：创建一个新的 Video 记录，关联到 User
	// 但是我们需要原始视频的信息来复制
	var original types.Video
	if err := w.db.First(&original, originalVideoID).Error; err != nil {
		return err
	}

	newVideo := types.Video{
		UserID:      original.UserID,
		Title:       original.Title + " (Cropped)",
		Description: "Cropped from " + original.FileName,
		ObjectKey:   newPath,
		StoragePath: newPath, // 兼容本地路径
		Status:      "ready",
		FileSize:    0, // 需要获取
		FileName:    filepath.Base(newPath),
	}

	if err := w.db.Create(&newVideo).Error; err != nil {
		return err
	}

	return nil
}

func StartESWorker(mqClient *mq.RabbitMQClient, esClient *search.ESClient, videoStore types.VideoStore) {
	msgs, err := mqClient.Consume("video_sync_es_queue")
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for d := range msgs {
			var msg mq.SyncVideoESMsg
			json.Unmarshal(d.Body, &msg)

			if msg.Action == "index" {
				// 从 DB 读取最新完整数据
				v, err := videoStore.GetVideoByID(msg.VideoID)
				if err == nil {
					// 写入 ES
					esClient.IndexVideo(v)
				}
			}
			d.Ack(false)
		}
	}()
}
