package service

import (
	"context"
	"fmt"
	"time"

	"github.com/sushkomihail/metric-aggregation-service/internal/logger"
	"github.com/sushkomihail/metric-aggregation-service/internal/repository/db"
	"github.com/sushkomihail/metric-aggregation-service/internal/repository/redis"
	"github.com/sushkomihail/metric-aggregation-service/pkg/models"
)

type Aggregator struct {
	db    db.DB
	redis *redis.Client
	log   *logger.Logger
}

func NewAggregator(db db.DB, redis *redis.Client, log *logger.Logger) *Aggregator {
	return &Aggregator{
		db:    db,
		redis: redis,
		log:   log,
	}
}

func (a *Aggregator) AddMetric(ctx context.Context, metric *models.Metric) error {
	if err := validateMetric(metric); err != nil {
		a.log.Error("Invalid metric", "trace_id", metric.TraceId, "error", err)
		return fmt.Errorf("invalid metric: %w", err)
	}

	if err := a.redis.ZAddWithUnixScore(ctx, "metrics:unprocessed", metric, time.Hour); err != nil {
		a.log.Warn("Failed to save metric to Redis", "trace_id", metric.TraceId, "error", err)
	}

	//if err := a.db.AddMetric(ctx, metric); err != nil {
	//	a.log.Error("Failed to save metric to PostgreSQL", "trace_id", metric.TraceId, "error", err)
	//	return fmt.Errorf("failed to save metric: %w", err)
	//}

	a.log.Info("Metric added successfully", "trace_id", metric.TraceId, "metric_id", metric.Id)
	return nil
}

func (a *Aggregator) AddHttpMetric(ctx context.Context, metric *models.HttpMetric) error {
	if err := validateHttpMetric(metric); err != nil {
		a.log.Error("Invalid HTTP metric", "trace_id", metric.TraceId, "error", err)
		return fmt.Errorf("invalid HTTP metric: %w", err)
	}

	if err := a.redis.ZAddWithUnixScore(ctx, "http_metrics:unprocessed", metric, time.Hour); err != nil {
		a.log.Warn("Failed to save HTTP metric to Redis", "trace_id", metric.TraceId, "error", err)
	}

	//if err := a.db.AddHttpMetric(ctx, metric); err != nil {
	//	a.log.Error("Failed to save HTTP metric to PostgreSQL", "trace_id", metric.TraceId, "error", err)
	//	return fmt.Errorf("failed to save http metric: %w", err)
	//}

	a.log.Info("HTTP metric added successfully",
		"trace_id", metric.TraceId,
		"method", metric.Method,
		"endpoint", metric.Endpoint,
	)
	return nil
}

func (a *Aggregator) GetAggregatedMetrics(
	ctx context.Context,
	start, end time.Time,
	metricName string,
	tags map[string]string,
) ([]*models.AggregatedMetric, error) {
	a.log.Debug("Fetching aggregated metrics",
		"start", start,
		"end", end,
		"metric_name", metricName,
		"tags", tags,
	)

	cacheKey := fmt.Sprintf("aggregated:%s:%d:%d", metricName, start.Unix(), end.Unix())
	var cachedMetrics []*models.AggregatedMetric

	if err := a.redis.HGetAll(ctx, cacheKey, &cachedMetrics); err == nil && len(cachedMetrics) > 0 {
		a.log.Debug("Retrieved metrics from cache",
			"cache_key", cacheKey,
			"count", len(cachedMetrics),
		)
		return cachedMetrics, nil
	}

	metrics, err := a.db.GetAggregatedMetrics(ctx, start, end, metricName, tags)
	if err != nil {
		a.log.Error("Failed to get aggregated metrics from database",
			"error", err,
		)
		return nil, fmt.Errorf("failed to get aggregated metrics: %w", err)
	}

	if len(metrics) > 0 {
		go func() {
			if err = a.redis.HSet(context.Background(), cacheKey, metrics, time.Hour); err != nil {
				a.log.Warn("Failed to cache metrics in Redis",
					"cache_key", cacheKey,
					"error", err,
				)
			}
		}()
	}

	a.log.Info("Aggregated metrics retrieved",
		"count", len(metrics),
		"metric_name", metricName,
	)

	return metrics, nil
}

func validateMetric(metric *models.Metric) error {
	if metric.Name == "" {
		return fmt.Errorf("metric name is required")
	}
	if metric.TraceId == "" {
		return fmt.Errorf("trace_id is required")
	}
	if metric.Type == models.Unknown {
		return fmt.Errorf("unknown metric type")
	}
	return nil
}

func validateHttpMetric(metric *models.HttpMetric) error {
	if metric.TraceId == "" {
		return fmt.Errorf("trace_id is required")
	}
	if metric.Method == "" {
		return fmt.Errorf("HTTP method is required")
	}
	if metric.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	return nil
}
