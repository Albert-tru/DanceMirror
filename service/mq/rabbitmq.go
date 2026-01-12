package mq

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Albert-tru/DanceMirror/types"
	amqp "github.com/rabbitmq/amqp091-go"
)

// CropTask 保持在这里作为 DTO (Data Transfer Object)
type CropTask struct {
	VideoID    int              `json:"video_id"`
	UserID     int              `json:"user_id"`
	InputPath  string           `json:"input_path"`
	OutputPath string           `json:"output_path"`
	Params     types.CropParams `json:"params"`
}

// RabbitMQClient RabbitMQ 客户端结构体
type RabbitMQClient struct {
	conn *amqp.Connection
	ch   *amqp.Channel
	mu   sync.Mutex // 保护并发访问
	url  string
}

// SyncVideoESMsg 用于同步视频到 Elasticsearch 的消息结构
type SyncVideoESMsg struct {
	VideoID int    `json:"video_id"`
	Action  string `json:"action"` // "create", "update", "delete"
}

func NewRabbitMQClient(url string) *RabbitMQClient {
	return &RabbitMQClient{
		url: url,
	}
}

// Connect 建立连接
func (c *RabbitMQClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var err error
	c.conn, err = amqp.Dial(c.url)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %v", err)
	}

	c.ch, err = c.conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open a channel: %v", err)
	}

	return nil
}

// EnsureQueue 声明队列
func (c *RabbitMQClient) EnsureQueue(queueName string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.ch == nil {
		return fmt.Errorf("channel not initialized")
	}

	// 1. 声明死信交换机
	const dlxName = "dlx_exchange"
	err := c.ch.ExchangeDeclare(
		dlxName,
		"direct",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare dead-letter exchange: %v", err)
	}

	// 2. 声明死信队列
	dlqName := queueName + "_dlq"
	_, err = c.ch.QueueDeclare(
		dlqName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare dead-letter queue: %v", err)
	}

	// 3. 绑定死信队列到死信交换机
	err = c.ch.QueueBind(
		dlqName,
		dlqName, // routing key
		dlxName,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to bind dead-letter queue: %v", err)
	}

	// 4. 声明主队列，设置死信交换机参数
	args := amqp.Table{
		"x-dead-letter-exchange":    dlxName,
		"x-dead-letter-routing-key": dlqName,
	}

	_, err = c.ch.QueueDeclare(
		queueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		args,  // arguments
	)

	return err
}

// Publish 发送消息 (线程安全)
func (c *RabbitMQClient) Publish(queueName string, body []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.ch == nil {
		return fmt.Errorf("channel is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return c.ch.PublishWithContext(ctx,
		"",        // exchange
		queueName, // routing key
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent, // 持久化
		})
}

// Consume 获取消费通道
func (c *RabbitMQClient) Consume(queueName string) (<-chan amqp.Delivery, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.ch == nil {
		return nil, fmt.Errorf("channel is nil")
	}

	//负载均衡
	err := c.ch.Qos(
		1,     // prefetch count(每次只分发一个消息)
		0,     // prefetch size（0表示不限制大小）
		false, // global（false表示应用于当前通道）
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set QoS: %v", err)
	}

	return c.ch.Consume(
		queueName,
		"",    // consumer
		false, // auto-ack (手动管理)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
}

func (c *RabbitMQClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.ch != nil {
		c.ch.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}
