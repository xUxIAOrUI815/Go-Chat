package kafka

import (
	"context"
	"go-chat/internal/config"
	myconfig "go-chat/internal/config"
	"go-chat/pkg/zlog"
	"time"

	"github.com/segmentio/kafka-go"
)

var ctx = context.Background()

type kafkaService struct {
	ChatWriter *kafka.Writer // Kafka 生产者 写消息到 Topic 中
	ChatReader *kafka.Reader // Kafka 消费者 读取 Topic 中的消息
	KafkaConn  *kafka.Conn   // Kafka 连接 用于创建 Topic 等管理操作
}

var KafkaService = new(kafkaService)

// KafkaInit 初始化
func (k *kafkaService) KafkaInit() {
	kafkaConfig := config.GetConfig().KafkaConfig
	k.ChatWriter = &kafka.Writer{
		Addr:                   kafka.TCP(kafkaConfig.HostPort),
		Topic:                  kafkaConfig.ChatTopic,
		Balancer:               &kafka.Hash{},
		WriteTimeout:           kafkaConfig.Timeout * time.Second,
		RequiredAcks:           kafka.RequireNone,
		AllowAutoTopicCreation: false,
	}
	k.ChatReader = kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{kafkaConfig.HostPort}, // 指定 Kafka 集群的 Broker 地址列表
		Topic:          kafkaConfig.ChatTopic,
		CommitInterval: kafkaConfig.Timeout * time.Second,
		GroupID:        "chat", // 消费者组ID
		StartOffset:    kafka.LastOffset,
	})
}

// KafkaClose 关闭连接并释放资源
func (k *kafkaService) KafkaClose() {
	if err := k.ChatWriter.Close(); err != nil {
		zlog.Error(err.Error())
	}
	if err := k.ChatReader.Close(); err != nil {
		zlog.Error(err.Error())
	}
}

// CreateTopic 创建 Topic
func (k *kafkaService) CreateTopic() {
	// 如果已经有topic了，就不创建了
	kafkaConfig := myconfig.GetConfig().KafkaConfig

	chatTopic := kafkaConfig.ChatTopic

	// 连接至任意kafka节点
	var err error
	k.KafkaConn, err = kafka.Dial("tcp", kafkaConfig.HostPort)
	if err != nil {
		zlog.Error(err.Error())
	}

	topicConfigs := []kafka.TopicConfig{
		{
			Topic:             chatTopic,
			NumPartitions:     kafkaConfig.Partition,
			ReplicationFactor: 1,
		},
	}

	// 创建topic
	if err = k.KafkaConn.CreateTopics(topicConfigs...); err != nil {
		zlog.Error(err.Error())
	}
}
