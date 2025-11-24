package main

import (
	"database/sql"
	"log"

	"github.com/Albert-tru/DanceMirror/cmd/api"
	"github.com/Albert-tru/DanceMirror/config"
	"github.com/Albert-tru/DanceMirror/db"
	"github.com/Albert-tru/DanceMirror/service/cache"
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

	// 3. 启动 Web 服务器[api.go]
	server := api.NewAPIServer(":"+config.Envs.Port, database, redisClient)
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
