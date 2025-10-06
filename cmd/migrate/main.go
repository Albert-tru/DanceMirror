package main

import (
	"fmt"
	"log"
	"os"

	"github.com/Albert-tru/DanceMirror/config"
	"github.com/Albert-tru/DanceMirror/db"
	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	// 1. 连接数据库
	database, err := db.NewMySQLStorage(config.Envs)
	if err != nil {
		log.Fatal(err)
	}

	// 2. 创建迁移驱动
	driver, err := mysql.WithInstance(database, &mysql.Config{})
	if err != nil {
		log.Fatal(err)
	}

	// 3. 创建迁移实例（从 migrations 目录读取 SQL 文件）
	m, err := migrate.NewWithDatabaseInstance(
		"file://cmd/migrate/migrations",
		"mysql",
		driver,
	)
	if err != nil {
		log.Fatal(err)
	}

	// 4. 执行命令（up 或 down）
	cmd := os.Args[len(os.Args)-1]

	switch cmd {
	case "up":
		// 升级：执行所有待执行的迁移
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatal(err)
		}
		fmt.Println("✅ 数据库迁移成功！")
	case "down":
		// 回滚：撤销最后一次迁移
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			log.Fatal(err)
		}
		fmt.Println("✅ 数据库回滚成功！")
	default:
		fmt.Println("用法: go run cmd/migrate/main.go [up|down]")
	}
}
