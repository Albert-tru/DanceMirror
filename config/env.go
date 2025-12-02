package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	PublicHost    string
	Port          string
	DBUser        string
	DBPassword    string
	DBAddress     string
	DBName        string
	JWTSecret     string
	JWTExpiration string
	UploadDir     string
	MaxUploadSize int64

	RedisAddress  string
	RedisPassword string
	RedisDB       int

	//存储
	StorageDriver   string // "local" 或 "minio"
	MinIOEndpoint   string // e.g., "play.min.io:9000"
	MinIOAccessKey  string //用来访问 MinIO 的 Access Key
	MinIOSecretKey  string //用来访问 MinIO 的 Secret Key
	MinIOBucket     string //存储桶名称
	MinIOUseSSL     bool   // 是否使用 SSL 连接 MinIO
	MinIORegion     string // MinIO 区域
	MinIOPresignMin int64  // 预签名 URL 有效期（分钟）
}

var Envs = initConfig()

func initConfig() Config {
	godotenv.Load()

	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "3306")

	var dbAddress string
	if dbPort == "" {
		// Socket 连接：直接使用 socket 路径
		dbAddress = dbHost
	} else {
		// TCP 连接：构建 host:port 格式
		dbAddress = fmt.Sprintf("%s:%s", dbHost, dbPort)
	}

	return Config{
		PublicHost:    getEnv("PUBLIC_HOST", "http://localhost:8080"),
		Port:          getEnv("APP_PORT", "8080"),
		DBUser:        getEnv("DB_USER", "root"),
		DBPassword:    getEnv("DB_PASSWORD", ""),
		DBAddress:     dbAddress,
		DBName:        getEnv("DB_NAME", "dancemirror"),
		JWTSecret:     getEnv("JWT_SECRET", "super-secret-jwt-key"),
		JWTExpiration: getEnv("JWT_EXPIRATION", "72h"),
		UploadDir:     getEnv("UPLOAD_DIR", "./uploads"),
		MaxUploadSize: getEnvAsInt64("MAX_UPLOAD_SIZE", 524288000),

		RedisAddress:  fmt.Sprintf("%s:%s", getEnv("REDIS_HOST", "localhost"), getEnv("REDIS_PORT", "6379")),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       int(getEnvAsInt64("REDIS_DB", 0)),

		//存储
		StorageDriver:   getEnv("STORAGE_DRIVER", "minio"),
		MinIOEndpoint:   getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey:  getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey:  getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinIOBucket:     getEnv("MINIO_BUCKET", "dancemirror"),
		MinIOUseSSL:     getEnv("MINIO_USE_SSL", "false") == "true",
		MinIORegion:     getEnv("MINIO_REGION", "us-east-1"),
		MinIOPresignMin: getEnvAsInt64("MINIO_PRESIGN_MIN", 15),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvAsInt64(key string, fallback int64) int64 {
	if value, ok := os.LookupEnv(key); ok {
		if i, err := strconv.ParseInt(value, 10, 64); err == nil {
			return i
		}
	}
	return fallback
}
