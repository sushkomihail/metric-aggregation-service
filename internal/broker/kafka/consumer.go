package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/sushkomihail/metric-aggregation-service/internal/config"
	"github.com/sushkomihail/metric-aggregation-service/internal/logger"
	"github.com/sushkomihail/metric-aggregation-service/internal/metrics"
	"github.com/sushkomihail/metric-aggregation-service/internal/service"
	"github.com/sushkomihail/metric-aggregation-service/pkg/models"
)

type Consumer struct {
	reader      *kafka.Reader
	readTimeout time.Duration
	aggregator  *service.Aggregator
	log         *logger.Logger
}

func NewConsumer(
	config config.KafkaConfig,
	readTimeout time.Duration,
	aggregator *service.Aggregator,
	topic, groupId string,
	log *logger.Logger,
) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  strings.Split(config.Servers, ","),
		GroupID:  groupId,
		Topic:    topic,
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})

	return &Consumer{
		reader:      reader,
		readTimeout: readTimeout,
		aggregator:  aggregator,
		log:         log,
	}
}

func (c *Consumer) Consume(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			c.log.Info("Kafka consumer stopped by context")
			return
		default:
			if err := c.readMessage(ctx); err != nil {
				if ctx.Err() != nil {
					return
				}

				c.log.Error("Error consuming message", "error", err)

				select {
				case <-ctx.Done():
					return
				case <-time.After(c.readTimeout):
				}
			}
		}
	}
}

func (c *Consumer) readMessage(ctx context.Context) error {
	readCtx, cancel := context.WithTimeout(ctx, c.readTimeout)
	defer cancel()

	message, err := c.reader.ReadMessage(readCtx)
	if err != nil {
		if errors.Is(readCtx.Err(), context.DeadlineExceeded) {
			return nil
		}
		return fmt.Errorf("failed to read message: %w", err)
	}

	metrics.IncMetricsConsumed()

	var httpMetric models.HttpMetric
	if err = json.Unmarshal(message.Value, &httpMetric); err != nil {
		return fmt.Errorf("failed to unmarshal HTTP metric: %w", err)
	}

	if httpMetric.TraceId == "" {
		httpMetric.TraceId = fmt.Sprintf("kafka-%d", message.Offset)
		c.log.Warn("HTTP metric without TraceId, generated",
			"generated_trace_id", httpMetric.TraceId,
			"offset", message.Offset,
		)
	}

	metrics.ObserveHttpRequestsTotal(httpMetric.Method, httpMetric.Endpoint, httpMetric.Code)
	metrics.ObserveHttpRequestDuration(httpMetric.Method, httpMetric.Endpoint, httpMetric.Duration)
	metrics.ObserveHttpRequestSize(httpMetric.Method, httpMetric.Endpoint, httpMetric.RequestSize)
	metrics.ObserveHttpResponseSize(httpMetric.Method, httpMetric.Endpoint, httpMetric.ResponseSize)

	if err = c.aggregator.AddHttpMetric(ctx, &httpMetric); err != nil {
		return fmt.Errorf("failed to add HTTP metric to storage (trace_id: %s): %w", httpMetric.TraceId, err)
	}

	return nil
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
