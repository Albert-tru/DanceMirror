package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type MinIOStorage struct {
	client *minio.Client
	bucket string
}

// NewMinIOStorage 创建MinIO存储客户端
func NewMinIOStorage(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*MinIOStorage, error) {
	// 初始化MinIO客户端
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("初始化MinIO失败: %w", err)
	}

	// 确保bucket存在
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("检查bucket失败: %w", err)
	}

	if !exists {
		err = client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("创建bucket失败: %w", err)
		}
	}

	// ⭐ 新增代码开始：自动将 Bucket 设置为公开只读 ⭐
	// 这样任何人都可以通过 http://localhost:9000/bucket/filename 直接访问文件
	policy := fmt.Sprintf(`{
"Version": "2012-10-17",
"Statement": [
{
"Effect": "Allow",
"Principal": {"AWS": ["*"]},
"Action": ["s3:GetObject"],
"Resource": ["arn:aws:s3:::%s/*"]
}
]
}`, bucket)

	err = client.SetBucketPolicy(ctx, bucket, policy)
	if err != nil {
		// 这里只打印警告，不阻断程序启动（防止因策略已存在报错）
		fmt.Printf("⚠️ 警告: 自动设置Bucket公开权限失败: %v\n", err)
	} else {
		fmt.Println("✅ 成功设置 Bucket 为公开访问模式")
	}
	// ⭐ 新增代码结束 ⭐

	return &MinIOStorage{
		client: client,
		bucket: bucket,
	}, nil
}

// Upload 上传文件到MinIO (带 Tracing)
func (m *MinIOStorage) Upload(ctx context.Context, objectKey string, r io.Reader, size int64, contentType string) error {
	// Start Span
	tracer := otel.Tracer("minio")
	ctx, span := tracer.Start(ctx, "MinIO Upload", trace.WithAttributes(
		attribute.String("minio.bucket", m.bucket),
		attribute.String("minio.object", objectKey),
		attribute.Int64("minio.size", size),
	))
	defer span.End()

	_, err := m.client.PutObject(ctx, m.bucket, objectKey, r, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("上传到MinIO失败: %w", err)
	}
	return nil
}

// Delete 从MinIO删除文件 (带 Tracing)
func (m *MinIOStorage) Delete(ctx context.Context, objectKey string) error {
	// Start Span
	tracer := otel.Tracer("minio")
	ctx, span := tracer.Start(ctx, "MinIO Delete", trace.WithAttributes(
		attribute.String("minio.bucket", m.bucket),
		attribute.String("minio.object", objectKey),
	))
	defer span.End()

	err := m.client.RemoveObject(ctx, m.bucket, objectKey, minio.RemoveObjectOptions{})
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("从MinIO删除失败: %w", err)
	}
	return nil
}

func (m *MinIOStorage) GetPresignedURL(objectKey string, expiry time.Duration) (string, error) {
	// ✅ 终极方案：直接返回宿主机可访问的公开链接
	// 这样既避开了 Docker 网络隔离问题，也避开了签名不匹配问题

	// 格式: http://localhost:9000/<bucket-name>/<object-key>
	publicURL := fmt.Sprintf("http://localhost:9000/%s/%s", m.bucket, objectKey)

	return publicURL, nil
}
