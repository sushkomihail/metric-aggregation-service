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
	db               db.DB
	redis            *redis.Client
	batchSize        int
	metricsBatch     chan *models.Metric
	httpMetricsBatch chan *models.HttpMetric
	log              *logger.Logger
}

func NewAggregator(db db.DB, redis *redis.Client, batchSize int, log *logger.Logger) *Aggregator {
	return &Aggregator{
		db:               db,
		redis:            redis,
		batchSize:        batchSize,
		metricsBatch:     make(chan *models.Metric, batchSize),
		httpMetricsBatch: make(chan *models.HttpMetric, batchSize),
		log:              log,
	}
}

func (a *Aggregator) Start(ctx context.Context, flushInterval time.Duration) {
	go a.runMetricsWorker(ctx, flushInterval)
	go a.runHTTPMetricsWorker(ctx, flushInterval)
}

func (a *Aggregator) AddMetric(metric *models.Metric) error {
	if err := validateMetric(metric); err != nil {
		a.log.Error("Invalid metric", "trace_id", metric.TraceId, "error", err)
		return fmt.Errorf("invalid metric: %w", err)
	}

	select {
	case a.metricsBatch <- metric:
	default:
		a.log.Warn("Metrics buffer overflow, dropping metric", "trace_id", metric.TraceId)
		return fmt.Errorf("metrics buffer overflow")
	}

	return nil
}

func (a *Aggregator) AddHttpMetric(metric *models.HttpMetric) error {
	if err := validateHttpMetric(metric); err != nil {
		a.log.Error("Invalid HTTP metric", "trace_id", metric.TraceId, "error", err)
		return fmt.Errorf("invalid HTTP metric: %w", err)
	}

	select {
	case a.httpMetricsBatch <- metric:
	default:
		a.log.Warn("HTTP metrics buffer overflow, dropping metric", "trace_id", metric.TraceId)
		return fmt.Errorf("http metrics buffer overflow")
	}

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

func (a *Aggregator) runMetricsWorker(ctx context.Context, flushInterval time.Duration) {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	metrics := make([]interface{}, 0, a.batchSize)

	flush := func() {
		if len(metrics) == 0 {
			return
		}
		if err := a.redis.ZAddBatch(ctx, "metrics:unprocessed", metrics); err != nil {
			a.log.Error("Failed to add metrics batch to Redis", "error", err)
			return
		}

		metrics = metrics[:0]
		a.log.Info("Metrics successfully added to Redis")
	}

	for {
		select {
		case metric := <-a.metricsBatch:
			metrics = append(metrics, metric)
			if len(metrics) >= a.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-ctx.Done():
			flush()
			return
		}
	}
}

func (a *Aggregator) runHTTPMetricsWorker(ctx context.Context, flushInterval time.Duration) {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	httpMetrics := make([]interface{}, 0, a.batchSize)

	flush := func() {
		if len(httpMetrics) == 0 {
			return
		}
		if err := a.redis.ZAddBatch(ctx, "http_metrics:unprocessed", httpMetrics); err != nil {
			a.log.Warn("Failed to add http metrics batch to Redis", "error", err)
			return
		}

		httpMetrics = httpMetrics[:0]
		a.log.Info("HTTP metrics successfully added to Redis")
	}

	for {
		select {
		case httpMetric := <-a.httpMetricsBatch:
			httpMetrics = append(httpMetrics, httpMetric)
			if len(httpMetrics) >= a.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-ctx.Done():
			flush()
			return
		}
	}
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
