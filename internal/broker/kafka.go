package broker

import (
	"context"
	"strings"

	"github.com/segmentio/kafka-go"
	"github.com/sushkomihail/metric-aggregation-service/internal/config"
)

const (
	HttpTopic = "http-topic"
)

type KafkaProducer struct {
	writer *kafka.Writer
}

func NewKafkaProducer(config config.KafkaConfig) *KafkaProducer {
	writer := &kafka.Writer{
		Addr:     kafka.TCP(strings.Split(config.Servers, ",")...),
		Balancer: &kafka.LeastBytes{},
	}

	return &KafkaProducer{
		writer: writer,
	}
}

func (p *KafkaProducer) Produce(ctx context.Context, message []byte, topic string) error {
	kafkaMsg := kafka.Message{
		Topic: topic,
		Value: message,
	}

	err := p.writer.WriteMessages(ctx, kafkaMsg)
	return err
}

func (p *KafkaProducer) Close() error {
	return p.writer.Close()
}
