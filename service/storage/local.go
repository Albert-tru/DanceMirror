package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"
)

type LocalStorage struct {
	baseDir string
}

func NewLocalStorage(baseDir string) *LocalStorage {
	return &LocalStorage{baseDir: baseDir}
}

// GetPresignedURL 实现 VideoStorage 接口
// 对于本地存储，我们不需要真正的“签名”，只需要返回可以通过静态文件服务访问的 URL 路径
func (l *LocalStorage) GetPresignedURL(objectKey string, expiry time.Duration) (string, error) {
	// 假设在 api.go 中配置了 /uploads/ 路由指向本地存储目录
	// 这里的 objectKey 类似于 "videos/1/test.mp4"
	// 返回 "/uploads/videos/1/test.mp4"
	return "/uploads/" + objectKey, nil
}

func (l *LocalStorage) Upload(ctx context.Context, objectKey string, r io.Reader, size int64, contentType string) error {
	fullPath := filepath.Join(l.baseDir, objectKey)
	os.MkdirAll(filepath.Dir(fullPath), 0755)

	f, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, r)
	return err
}

func (l *LocalStorage) Delete(ctx context.Context, objectKey string) error {
	return os.Remove(filepath.Join(l.baseDir, objectKey))
}

func (l *LocalStorage) PresignGet(ctx context.Context, objectKey string, expiry time.Duration) (string, error) {
	// 本地存储返回相对路径
	return "/uploads/" + objectKey, nil
}
