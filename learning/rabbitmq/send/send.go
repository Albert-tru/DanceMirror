package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func bodyForm(args []string) string {
	var s string
	if (len(args) < 1) || args[0] == "" {
		s = "hello"
	} else {
		s = strings.Join(args[1:], "")
	}
	return s
}

func main() {
	// 1. 建立RabbitMQ连接
	// conn: 返回一个连接对象（*amqp.Connection）
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	// 2. 创建一个通道
	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	// 3. 声明一个队列
	q, err := ch.QueueDeclare(
		"task_queue", // name
		true,         // durable	【消息持久化：RabbitMQ 重启不丢失】
		false,        // delete when unused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
	)
	failOnError(err, "Failed to declare a queue")

	// 4. 键入消息内容
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	body := bodyForm(os.Args)

	// 5. 发布消息
	err = ch.PublishWithContext(
		ctx,
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "text/plain",
			Body:         []byte(body),
		})
	failOnError(err, "Failed to publish a message")
	log.Printf(" [x] Sent %s", body)
}
