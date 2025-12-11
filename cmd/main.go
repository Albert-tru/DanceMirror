package main

import (
	"database/sql"
	"log"
	"time"

	"github.com/Albert-tru/DanceMirror/cmd/api"
	"github.com/Albert-tru/DanceMirror/config"
	"github.com/Albert-tru/DanceMirror/db"
	"github.com/Albert-tru/DanceMirror/service/cache"
	"github.com/Albert-tru/DanceMirror/service/mq"
	"github.com/Albert-tru/DanceMirror/service/storage"
	"github.com/Albert-tru/DanceMirror/types"
)

func main() {
	// 1. 连接数据库
	database, err := db.NewMySQLStorage(config.Envs)
	if err != nil {
		log.Fatal(err)
	}

	// 2. 自动迁移数据库表
	if err := database.AutoMigrate(&types.User{}, &types.Video{}); err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	//初始化redis
	redisClient := cache.NewRedisClient(
		config.Envs.RedisAddress,
		config.Envs.RedisPassword,
		config.Envs.RedisDB,
	)

	mq.SetDB(database)
	// ⭐ 初始化存储客户端并注入到 mq（用于 worker 上传裁剪后文件）
	var storageClient storage.VideoStorage
	if config.Envs.StorageDriver == "minio" {
		minioClient, err := storage.NewMinIOStorage(
			config.Envs.MinIOEndpoint,
			config.Envs.MinIOAccessKey,
			config.Envs.MinIOSecretKey,
			config.Envs.MinIOBucket,
			config.Envs.MinIOUseSSL,
		)
		if err != nil {
			log.Fatalf("❌ MinIO 初始化失败: %v", err)
		}
		storageClient = minioClient
		log.Println("✅ MinIO 存储初始化成功 (main)")
	} else {
		storageClient = storage.NewLocalStorage(config.Envs.UploadDir)
		log.Println("✅ 本地存储初始化成功 (main)")
	}
	mq.SetStorageClient(storageClient)

	// 初始化消息队列
	rabbitMQURL := config.Envs.RabbitMQURL

	// ✅ 新增：重试逻辑
	var mqErr error
	for i := 0; i < 15; i++ {
		log.Printf("🐰 Connecting to RabbitMQ: %s (Attempt %d/15)...", rabbitMQURL, i+1)
		mqErr = mq.InitRabbitMQ(rabbitMQURL)
		if mqErr == nil {
			log.Println("✅ RabbitMQ connected successfully!")
			break
		}
		log.Printf("⚠️ RabbitMQ not ready: %v. Retrying in 2s...", mqErr)
		time.Sleep(2 * time.Second)
	}

	if mqErr != nil {
		log.Fatal("❌ Failed to initialize RabbitMQ after retries:", mqErr)
	}
	defer mq.Close()

	// ✅ 注入依赖并启动消费者
	mq.SetStorageClient(storageClient)
	mq.StartCropWorker()

	// 3. 启动 Web 服务器[api.go]
	server := api.NewAPIServer(":"+config.Envs.Port, database, redisClient, storageClient)
	if err := server.Run(); err != nil {
		log.Fatal(err)
	}
}

func initStorage(db *sql.DB) {
	err := db.Ping()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("✅ Database successfully connected!")
}
