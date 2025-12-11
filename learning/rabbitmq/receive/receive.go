package main

import (
	"bytes"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

func main() {
	// 1. 建立RabbitMQ连接
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
		true,         // durable
		false,        // delete when unused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
	)
	failOnError(err, "Failed to declare a queue")

	// 公平分派（告诉 RabbitMQ 一次不要向一个工作进程发送超过一条消息。
	// 			换句话说，在工作进程处理并确认上一条消息之前，不要向其分发新消息
	err = ch.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	failOnError(err, "Failed to set QoS")

	// 4. 消费消息
	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack (use manual ack so we can call d.Ack)
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	// 5. 处理接收到的消息
	forever := make(chan struct{})

	// 启动一个 goroutine 来处理消息
	go func() {
		for d := range msgs {
			log.Printf("Received a message: %s", d.Body)
			dotCount := bytes.Count(d.Body, []byte("."))
			t := dotCount
			log.Printf(" [x] Sleeping %d seconds", t)
			// 模拟处理时间
			// time.Sleep(time.Duration(t) * time.Second)
			log.Printf("Done")
			d.Ack(false) // 手动确认消息已处理
		}
	}()

	// 阻塞主线程，直到程序被终止
	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever
}
