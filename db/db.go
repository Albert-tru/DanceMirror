package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/Albert-tru/DanceMirror/config"
	_ "github.com/go-sql-driver/mysql"
)

func NewMySQLStorage(cfg config.Config) (*sql.DB, error) {
	var dsn string

	log.Printf("DBAddress: %s, DBUser: %s", cfg.DBAddress, cfg.DBUser)

	// 检查是否使用 socket 连接（DB_PORT 为空）
	if cfg.DBAddress == "" || cfg.DBAddress == ":" || strings.Contains(cfg.DBAddress, ".sock") {
		// Socket 连接格式: user:password@unix(/path/to/socket)/dbname
		dsn = fmt.Sprintf("%s:%s@unix(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.DBUser, cfg.DBPassword, cfg.DBAddress, cfg.DBName)
		log.Printf("Using socket connection: %s", dsn)
	} else {
		// TCP 连接格式: user:password@tcp(host:port)/dbname
		dsn = fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.DBUser, cfg.DBPassword, cfg.DBAddress, cfg.DBName)
		log.Printf("Using TCP connection: %s", dsn)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	log.Println("✅ Database connected successfully!")
	return db, nil
}

func InitStorage(db *sql.DB) error {
	// 这里可以执行一些初始化操作
	// 比如检查必要的表是否存在等
	return nil
}
