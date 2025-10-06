package main

import (
	"database/sql"
	"log"

	"github.com/Albert-tru/DanceMirror/cmd/api"
	"github.com/Albert-tru/DanceMirror/config"
	"github.com/Albert-tru/DanceMirror/db"
)

func main() {
	// 1. 连接数据库
	database, err := db.NewMySQLStorage(config.Envs)
	if err != nil {
		log.Fatal(err)
	}

	// 2. 检查数据库连接
	initStorage(database)

	// 3. 启动 Web 服务器
	server := api.NewAPIServer(":"+config.Envs.Port, database)
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
