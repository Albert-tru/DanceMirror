package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Albert-tru/DanceMirror/types"
	"github.com/redis/go-redis/v9"
)

// RedisClient 是 Redis 客户端的封装
type RedisClient struct {
	client *redis.Client // 底层 Redis 客户端
}

// NewRedisClient 创建 Redis 客户端
// addr 格式：localhost:6379
func NewRedisClient(addr string, password string, db int) *RedisClient {
	// 创建 Redis 客户端
	client := redis.NewClient(&redis.Options{
		Addr:     addr,     // Redis 地址
		Password: password, // 密码（如果没有就留空）
		DB:       db,       // 数据库编号（0-15）
	})

	// 测试连接
	ctx := context.Background() //返回一个非 nil 的空上下文
	if err := client.Ping(ctx).Err(); err != nil {
		panic("❌ Redis 连接失败: " + err.Error())
	}

	fmt.Println("✅ Redis 连接成功!")
	return &RedisClient{client: client}
}

// Set 设置键值对
// key: 键名
// value: 任意类型的值（会自动转为 JSON）
// expiration: 过期时间（0 表示永不过期）
func (r *RedisClient) Set(key string, value interface{}, expiration time.Duration) error {
	ctx := context.Background()

	// 将值转为 JSON 字符串
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("序列化失败: %w", err)
	}

	// 存入 Redis
	return r.client.Set(ctx, key, data, expiration).Err()
}

// Get 获取值
// key: 键名
// dest: 指针，用于接收结果
func (r *RedisClient) Get(key string, dest interface{}) error {
	ctx := context.Background()

	// 从 Redis 获取
	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return fmt.Errorf("键 %s 不存在", key)
	}
	if err != nil {
		return err
	}

	// 反序列化
	return json.Unmarshal([]byte(data), dest)
}

// Delete 删除键
func (r *RedisClient) Delete(key string) error {
	ctx := context.Background()
	return r.client.Del(ctx, key).Err()
}

// Exists 检查键是否存在
func (r *RedisClient) Exists(key string) (bool, error) {
	ctx := context.Background()
	count, err := r.client.Exists(ctx, key).Result()
	return count > 0, err
}

// 缓存用户的视频列表
func (r *RedisClient) CacheUserVideos(userID int, videos []*types.Video) error {
	key := fmt.Sprintf("user:%d:videos", userID)
	return r.Set(key, videos, 5*time.Minute) // 缓存5分钟
}

// 缓存单个视频
func (r *RedisClient) CacheVideoByID(video *types.Video) error {
	key := fmt.Sprintf("video:%d", video.ID)
	return r.Set(key, video, 10*time.Minute) // 缓存10分钟
}

// 获取单个视频缓存
func (r *RedisClient) GetVideoByID(videoID int) (*types.Video, error) {
	key := fmt.Sprintf("video:%d", videoID)
	var video types.Video
	err := r.Get(key, &video)
	if err != nil {
		return nil, err //缓存未命中
	}
	return &video, err
}

// 获取用户的视频列表缓存
func (r *RedisClient) GetUserVideos(userID int) ([]*types.Video, error) {
	key := fmt.Sprintf("user:%d:videos", userID)
	var videos []*types.Video
	err := r.Get(key, &videos)
	if err != nil {
		return nil, err //缓存未命中
	}
	return videos, err
}

// 清除用户列表缓存
func (r *RedisClient) ClearUserVideosCache(userID int) error {
	key := fmt.Sprintf("user:%d:videos", userID)
	return r.Delete(key)
}

// 清除单个视频缓存
func (r *RedisClient) ClearVideoCache(videoID int) error {
	key := fmt.Sprintf("video:%d", videoID)
	return r.Delete(key)
}
