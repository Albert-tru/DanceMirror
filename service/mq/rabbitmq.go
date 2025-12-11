package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"mime"
	"os"
	"path/filepath"
	"time"

	"github.com/Albert-tru/DanceMirror/service/storage"
	"github.com/Albert-tru/DanceMirror/service/video"
	"github.com/Albert-tru/DanceMirror/types"
	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
)

// ✅ 定义常量，确保生产者和消费者使用同一个队列
const QueueName = "video_crop_queue"

var (
	conn          *amqp.Connection
	ch            *amqp.Channel
	db            *gorm.DB
	storageClient storage.VideoStorage
)

type CropTask struct {
	VideoID    int              `json:"video_id"`
	UserID     int              `json:"user_id"`
	InputPath  string           `json:"input_path"`
	OutputPath string           `json:"output_path"`
	Params     video.CropParams `json:"params"`
}

func SetDB(database *gorm.DB) {
	db = database
}

func SetStorageClient(s storage.VideoStorage) {
	storageClient = s
}

func InitRabbitMQ(url string) error {
	var err error
	conn, err = amqp.Dial(url)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %v", err)
	}

	ch, err = conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open a channel: %v", err)
	}

	// ✅ 使用常量声明队列
	_, err = ch.QueueDeclare(
		QueueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare a queue: %v", err)
	}

	return nil
}

func Close() {
	if ch != nil {
		ch.Close()
	}
	if conn != nil {
		conn.Close()
	}
}

func PublishCropTask(task CropTask) error {
	if ch == nil {
		return fmt.Errorf("RabbitMQ channel is not initialized")
	}

	body, err := json.Marshal(task)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// ✅ 使用常量发布消息
	err = ch.PublishWithContext(ctx,
		"",        // exchange
		QueueName, // routing key
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})

	if err == nil {
		log.Printf("📤 [MQ] Task published for VideoID %d", task.VideoID)
	}
	return err
}

func StartCropWorker() {
	if ch == nil {
		log.Println("⚠️ RabbitMQ channel is nil, worker cannot start")
		return
	}

	// ✅ 使用常量消费消息
	msgs, err := ch.Consume(
		QueueName, // queue
		"",        // consumer
		false,     // auto-ack (我们要手动ack)
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	if err != nil {
		log.Printf("❌ Failed to register consumer: %v", err)
		return
	}

	go func() {
		log.Println("👷 Worker started, waiting for tasks on queue:", QueueName)
		for d := range msgs {
			var task CropTask
			if err := json.Unmarshal(d.Body, &task); err != nil {
				log.Printf("❌ JSON Error: %v", err)
				d.Nack(false, false)
				continue
			}

			log.Printf("📥 [Worker] Got task: VideoID=%d", task.VideoID)

			// 1. 更新状态: processing
			if db != nil {
				db.Model(&types.Video{}).Where("id = ?", task.VideoID).Update("status", "processing")
			}

			// 2. 执行裁剪
			log.Println("✂️ [Worker] FFmpeg starting...")
			err := video.CropVideo(task.InputPath, task.OutputPath, task.Params)
			if err != nil {
				log.Printf("❌ [Worker] FFmpeg failed: %v", err)
				if db != nil {
					db.Model(&types.Video{}).Where("id = ?", task.VideoID).Update("status", "failed")
				}
				d.Nack(false, false)
				continue
			}
			log.Println("✅ [Worker] FFmpeg done!")

			// 3. 上传结果
			updates := map[string]interface{}{
				"status":     "completed",
				"outputPath": task.OutputPath,
			}

			if storageClient != nil {
				log.Println("☁️ [Worker] Uploading to storage...")
				objectKey := fmt.Sprintf("videos/%d/cropped_%d%s", task.UserID, task.VideoID, filepath.Ext(task.OutputPath))

				f, err := os.Open(task.OutputPath)
				if err == nil {
					fi, _ := f.Stat()
					ext := filepath.Ext(task.OutputPath)
					contentType := mime.TypeByExtension(ext)
					if contentType == "" {
						contentType = "video/mp4"
					}

					err = storageClient.Upload(context.Background(), objectKey, f, fi.Size(), contentType)
					f.Close()

					if err != nil {
						log.Printf("❌ [Worker] Upload failed: %v", err)
						if db != nil {
							db.Model(&types.Video{}).Where("id = ?", task.VideoID).Update("status", "failed")
						}
						d.Nack(false, false)
						continue
					}

					updates["storagePath"] = objectKey
					os.Remove(task.OutputPath) // 删除临时文件
				}
			}

			// 4. 更新 DB 完成
			if db != nil {
				db.Model(&types.Video{}).Where("id = ?", task.VideoID).Updates(updates)
			}

			log.Printf("🎉 [Worker] Task %d completed!", task.VideoID)
			d.Ack(false)
		}
	}()
}
