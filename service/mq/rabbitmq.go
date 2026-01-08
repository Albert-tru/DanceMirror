package mq

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Albert-tru/DanceMirror/service/video"
	amqp "github.com/rabbitmq/amqp091-go"
)

// CropTask 保持在这里作为 DTO (Data Transfer Object)
type CropTask struct {
	VideoID    int              `json:"video_id"`
	UserID     int              `json:"user_id"`
	InputPath  string           `json:"input_path"`
	OutputPath string           `json:"output_path"`
	Params     video.CropParams `json:"params"`
}

type RabbitMQClient struct {
	conn *amqp.Connection
	ch   *amqp.Channel
	mu   sync.Mutex // 保护并发访问
	url  string
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

	_, err := c.ch.QueueDeclare(
		queueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
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
			ContentType: "application/json",
			Body:        body,
		})
}

// Consume 获取消费通道
func (c *RabbitMQClient) Consume(queueName string) (<-chan amqp.Delivery, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.ch == nil {
		return nil, fmt.Errorf("channel is nil")
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
