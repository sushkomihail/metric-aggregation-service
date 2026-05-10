package kafka

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/sushkomihail/metric-aggregation-service/internal/config"
	"github.com/sushkomihail/metric-aggregation-service/internal/logger"
	"github.com/sushkomihail/metric-aggregation-service/internal/metrics"
)

const (
	HttpTopic = "http-topic"
)

type Producer struct {
	writer *kafka.Writer
	log    *logger.Logger
}

func NewProducer(config config.KafkaConfig, log *logger.Logger) *Producer {
	brokers := strings.Split(config.Servers, ",")

	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    100,
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafka.RequireOne,
		Compression:  kafka.Snappy,
		Async:        false,
	}

	return &Producer{
		writer: writer,
		log:    log,
	}
}

func (p *Producer) Produce(ctx context.Context, message []byte, topic string) error {
	kafkaMsg := kafka.Message{
		Topic: topic,
		Value: message,
	}

	if err := p.writer.WriteMessages(ctx, kafkaMsg); err != nil {
		return fmt.Errorf("failed to write message to kafka (topic: %s): %w", topic, err)
	}

	metrics.IncMetricsProduced()

	p.log.Debug("Message produced successfully", "topic", topic)
	return nil
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
