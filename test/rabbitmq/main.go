package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/streadway/amqp"
)

const (
	rabbitMQHost     = "192.168.152.128"
	rabbitMQPort     = "5672"
	rabbitMQUser     = "admin"
	rabbitMQPassword = "myMQ#@g3359Blue#@test.com"
	exchangeName     = "test-exchange"
	queueName        = "test-queue"
	routingKey       = "test-key"
)

// buildRabbitMQURL 构建RabbitMQ连接URL（自动编码特殊字符）
func buildRabbitMQURL() string {
	// URL编码用户名和密码
	encodedUser := url.QueryEscape(rabbitMQUser)
	encodedPassword := url.QueryEscape(rabbitMQPassword)

	return fmt.Sprintf("amqp://%s:%s@%s:%s/", encodedUser, encodedPassword, rabbitMQHost, rabbitMQPort)
}

// Producer 生产者 - 每100ms发送一条消息
func Producer(ctx context.Context) error {
	rabbitMQURL := buildRabbitMQURL()

	// 连接RabbitMQ
	conn, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		return fmt.Errorf("连接RabbitMQ失败: %w", err)
	}
	defer conn.Close()

	// 创建通道
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("创建通道失败: %w", err)
	}
	defer ch.Close()

	// 声明交换机
	err = ch.ExchangeDeclare(
		exchangeName, // 交换机名称
		"direct",     // 类型
		true,         // 持久化
		false,        // 自动删除
		false,        // 内部
		false,        // 无等待
		nil,          // 参数
	)
	if err != nil {
		return fmt.Errorf("声明交换机失败: %w", err)
	}

	// 声明队列
	_, err = ch.QueueDeclare(
		queueName, // 队列名称
		true,      // 持久化
		false,     // 自动删除
		false,     // 排他
		false,     // 无等待
		nil,       // 参数
	)
	if err != nil {
		return fmt.Errorf("声明队列失败: %w", err)
	}

	// 绑定队列到交换机
	err = ch.QueueBind(
		queueName,    // 队列名称
		routingKey,   // 路由键
		exchangeName, // 交换机名称
		false,        // 无等待
		nil,          // 参数
	)
	if err != nil {
		return fmt.Errorf("绑定队列失败: %w", err)
	}

	log.Println("[生产者] 已启动，每100ms发送一条消息...")

	// 每100ms发送一条消息
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	counter := 0
	for {
		select {
		case <-ctx.Done():
			log.Println("[生产者] 已停止")
			return nil
		case <-ticker.C:
			counter++
			message := fmt.Sprintf("消息 #%d - 时间: %s", counter, time.Now().Format("2006-01-02 15:04:05.000"))

			err = ch.Publish(
				exchangeName, // 交换机
				routingKey,   // 路由键
				false,        // 强制
				false,        // 立即
				amqp.Publishing{
					ContentType:  "text/plain",
					Body:         []byte(message),
					DeliveryMode: amqp.Persistent, // 持久化消息
					Timestamp:    time.Now(),
				},
			)
			if err != nil {
				log.Printf("[生产者] 发送消息失败: %v", err)
				continue
			}

			log.Printf("[生产者] ✓ 发送: %s", message)
		}
	}
}

// Consumer 消费者
func Consumer(ctx context.Context) error {
	rabbitMQURL := buildRabbitMQURL()

	// 连接RabbitMQ
	conn, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		return fmt.Errorf("连接RabbitMQ失败: %w", err)
	}
	defer conn.Close()

	// 创建通道
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("创建通道失败: %w", err)
	}
	defer ch.Close()

	// 声明队列（确保队列存在）
	_, err = ch.QueueDeclare(
		queueName, // 队列名称
		true,      // 持久化
		false,     // 自动删除
		false,     // 排他
		false,     // 无等待
		nil,       // 参数
	)
	if err != nil {
		return fmt.Errorf("声明队列失败: %w", err)
	}

	// 设置QoS（每次只处理一条消息，确保公平分发）
	err = ch.Qos(
		1,     // 预取数量
		0,     // 预取大小
		false, // 全局
	)
	if err != nil {
		return fmt.Errorf("设置QoS失败: %w", err)
	}

	// 消费消息
	msgs, err := ch.Consume(
		queueName, // 队列名称
		"",        // 消费者标签
		false,     // 自动确认（false表示手动确认）
		false,     // 排他
		false,     // 无本地
		false,     // 无等待
		nil,       // 参数
	)
	if err != nil {
		return fmt.Errorf("注册消费者失败: %w", err)
	}

	log.Println("[消费者] 已启动，等待消息...")

	// 处理消息
	for {
		select {
		case <-ctx.Done():
			log.Println("[消费者] 已停止")
			return nil
		case msg, ok := <-msgs:
			if !ok {
				log.Println("[消费者] 消息通道已关闭")
				return nil
			}

			log.Printf("[消费者] ✓ 接收: %s", string(msg.Body))

			// 模拟处理时间
			time.Sleep(50 * time.Millisecond)

			// 手动确认消息
			err = msg.Ack(false)
			if err != nil {
				log.Printf("[消费者] 确认消息失败: %v", err)
			}
		}
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动生产者（goroutine）
	go func() {
		if err := Producer(ctx); err != nil {
			log.Fatalf("[生产者] 错误: %v", err)
		}
	}()

	// 等待一下让生产者先启动
	time.Sleep(500 * time.Millisecond)

	// 启动消费者（goroutine）
	go func() {
		if err := Consumer(ctx); err != nil {
			log.Fatalf("[消费者] 错误: %v", err)
		}
	}()

	// 运行30秒后停止
	log.Println("==========================================")
	log.Println("程序运行中，30秒后自动停止...")
	log.Println("==========================================")
	time.Sleep(30 * time.Second)

	log.Println("正在停止...")
	cancel()

	// 等待goroutine结束
	time.Sleep(1 * time.Second)
	log.Println("程序已停止")
}
