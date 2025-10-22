package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Albert-tru/DanceMirror/config"
	_ "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewMySQLStorage 创建并返回一个 MySQL 数据库连接
func NewMySQLStorage(cfg config.Config) (*gorm.DB, error) {
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

	//打开一个dirverName指定的数据库，dataSourceName指定数据源
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // 设置日志级别为 Info
		NowFunc: func() time.Time {
			return time.Now().In(time.FixedZone("CST", 8*3600)) // 设置为中国标准时间
		},
	})
	if err != nil {
		return nil, err
	}

	// 获取底层的 sql.DB 对象以进行进一步配置
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// 设置连接池参数（根据需要调整）
	sqlDB.SetMaxOpenConns(25)                 // 最大打开连接数
	sqlDB.SetMaxIdleConns(25)                 // 最大空闲连接数
	sqlDB.SetConnMaxLifetime(5 * time.Minute) // 连接最大生命周期

	log.Println("✅ Database connected successfully!")
	return db, nil
}

func InitStorage(db *sql.DB) error {
	// 这里可以执行一些初始化操作
	// 比如检查必要的表是否存在等
	return nil
}
