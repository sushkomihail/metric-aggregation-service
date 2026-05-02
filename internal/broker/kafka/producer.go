package kafka

import (
	"context"
	"strings"

	"github.com/segmentio/kafka-go"
	"github.com/sushkomihail/metric-aggregation-service/internal/config"
)

const (
	HttpTopic = "http-topic"
)

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(config config.KafkaConfig) *Producer {
	writer := &kafka.Writer{
		Addr:     kafka.TCP(strings.Split(config.Servers, ",")...),
		Balancer: &kafka.LeastBytes{},
	}

	return &Producer{
		writer: writer,
	}
}

func (p *Producer) Produce(ctx context.Context, message []byte, topic string) error {
	kafkaMsg := kafka.Message{
		Topic: topic,
		Value: message,
	}

	err := p.writer.WriteMessages(ctx, kafkaMsg)
	return err
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
