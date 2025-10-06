package db

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/Albert-tru/DanceMirror/config"
	_ "github.com/go-sql-driver/mysql"
)

// NewMySQLStorage 创建并返回 MySQL 数据库连接
func NewMySQLStorage(cfg config.Config) (*sql.DB, error) {
	// 构建 DSN (Data Source Name) 连接字符串
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.DBUser, cfg.DBPassword, cfg.DBAddress, cfg.DBName)

	// 打开数据库连接
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	// 测试连接是否成功
	if err := db.Ping(); err != nil {
		return nil, err
	}

	log.Println("✅ Database connected successfully!")
	return db, nil
}
