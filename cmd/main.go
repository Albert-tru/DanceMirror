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
	"github.com/Albert-tru/DanceMirror/service/search"
	"github.com/Albert-tru/DanceMirror/service/storage"
	"github.com/Albert-tru/DanceMirror/service/worker"
	"github.com/Albert-tru/DanceMirror/types"
)

const QueueName = "video_crop_queue"

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

	// 3. 初始化存储
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

	// 初始化es
	esClient, err := search.NewESClient(config.Envs.ElasticsearchURL)
	if err != nil {
		log.Fatalf("❌ ElasticSearch 初始化失败: %v", err)
	}

	// 4. 初始化消息队列 (无全局变量)
	mqClient := mq.NewRabbitMQClient(config.Envs.RabbitMQURL)
	var mqErr error
	for i := 0; i < 15; i++ {
		log.Printf("🐰 Connecting to RabbitMQ: %s (Attempt %d/15)...", config.Envs.RabbitMQURL, i+1)
		mqErr = mqClient.Connect()
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
	defer mqClient.Close()

	// 确保队列存在
	if err := mqClient.EnsureQueue(QueueName); err != nil {
		log.Fatal("❌ Failed to declare queue:", err)
	}

	// 5. 启动 Worker (并发 5)
	cropWorker := worker.NewCropWorker(mqClient, database, storageClient, QueueName)
	cropWorker.Start(5)

	// 6. 启动 Web 服务器[api.go]
	server := api.NewAPIServer(":"+config.Envs.Port, database, redisClient, storageClient, mqClient, esClient)
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
