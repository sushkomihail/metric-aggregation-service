package kafka

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/segmentio/kafka-go"
	"github.com/sushkomihail/metric-aggregation-service/internal/config"
	"github.com/sushkomihail/metric-aggregation-service/internal/metrics"
	"github.com/sushkomihail/metric-aggregation-service/internal/service"
	"github.com/sushkomihail/metric-aggregation-service/pkg/models"
)

type Consumer struct {
	reader     *kafka.Reader
	aggregator *service.Aggregator
}

func NewConsumer(config config.KafkaConfig, aggregator *service.Aggregator, topic, groupId string) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  strings.Split(config.Servers, ","),
		GroupID:  groupId,
		Topic:    topic,
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})

	return &Consumer{
		reader:     reader,
		aggregator: aggregator,
	}
}

func (c *Consumer) Consume(ctx context.Context) {
	for {
		message, err := c.reader.ReadMessage(ctx)
		if err != nil {
			log.Printf("error reading kafka message: %s\n", err)
			continue
		}

		log.Printf("kafka message: %s\n", string(message.Value))

		var metric models.HttpMetric
		if err = json.Unmarshal(message.Value, &metric); err != nil {
			log.Printf("error unmarshalling http metric: %s\n", err)
			continue
		}

		metrics.ObserveHttpRequestsTotal(metric.Method, metric.Endpoint, metric.Code)
		metrics.ObserveHttpRequestDuration(metric.Method, metric.Endpoint, metric.Duration)
		metrics.ObserveHttpRequestSize(metric.Method, metric.Endpoint, metric.RequestSize)
		metrics.ObserveHttpResponseSize(metric.Method, metric.Endpoint, metric.ResponseSize)

		if err = c.aggregator.AddHttpMetric(ctx, &metric); err != nil {
			log.Printf("%v\n", err)
		}

		log.Printf("http metric consumed")
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
