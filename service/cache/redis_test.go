package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRedisConnection(t *testing.T) {
	// 创建客户端（连接到本地 Redis）
	client := NewRedisClient("localhost:6379", "", 0)

	// 测试设置值
	err := client.Set(t.Context(), "test_key", "Hello Redis!", 10*time.Second)
	assert.NoError(t, err, "设置值失败")

	// 测试获取值
	var result string
	err = client.Get(t.Context(), "test_key", &result)
	assert.NoError(t, err, "获取值失败")
	assert.Equal(t, "Hello Redis!", result, "值不匹配")

	// 测试删除值
	err = client.Delete(t.Context(), "test_key")
	assert.NoError(t, err, "删除值失败")

	// 验证已删除
	exists, _ := client.Exists(t.Context(), "test_key")
	assert.False(t, exists, "键应该不存在")
}
