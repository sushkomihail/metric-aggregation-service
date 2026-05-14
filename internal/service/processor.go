package service

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/sushkomihail/metric-aggregation-service/internal/logger"
	"github.com/sushkomihail/metric-aggregation-service/internal/repository/db"
	"github.com/sushkomihail/metric-aggregation-service/internal/repository/redis"
	prommetrics "github.com/sushkomihail/metric-aggregation-service/pkg/metrics"
	"github.com/sushkomihail/metric-aggregation-service/pkg/models"
)

type AggregatedHttpMetricValue struct {
	sum float64
	min float64
	max float64
	p50 float64
	p95 float64
	p99 float64
}

type Processor struct {
	db         db.DB
	redis      *redis.Client
	timeWindow time.Duration
	log        *logger.Logger
}

func NewProcessor(
	db db.DB,
	redis *redis.Client,
	timeWindow time.Duration,
	log *logger.Logger,
) *Processor {
	return &Processor{
		db:         db,
		redis:      redis,
		timeWindow: timeWindow,
		log:        log,
	}
}

func (p *Processor) Start(ctx context.Context) {
	ticker := time.NewTicker(p.timeWindow)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.log.Info("Metrics processor stopped by context")
			return
		case <-ticker.C:
			go p.processAllMetrics(ctx)
		}
	}
}

func (p *Processor) processAllMetrics(ctx context.Context) {
	startTime := time.Now()
	batchId := fmt.Sprintf("batch_%d", startTime.Unix())

	if err := p.processMetrics(ctx, batchId); err != nil {
		p.log.Error("Failed to process metrics",
			"batch_id", batchId,
			"error", err,
		)
	}

	if err := p.processHttpMetrics(ctx, batchId); err != nil {
		p.log.Error("Failed to process HTTP metrics",
			"batch_id", batchId,
			"error", err,
		)
	}
}

func (p *Processor) processMetrics(ctx context.Context, batchId string) error {
	end := time.Now()
	start := end.Add(-p.timeWindow)
	metrics, err := p.getUnprocessedMetrics(ctx, start, end)
	if err != nil {
		return fmt.Errorf("failed to get unprocessed metrics: %w", err)
	}

	if len(metrics) == 0 {
		p.log.Debug("No metrics to process", "batch_id", batchId)
		return nil
	}

	metricsCount := len(metrics)
	processedIds := make([]int, 0, metricsCount)
	groups := p.groupMetrics(metrics)
	for groupKey, group := range groups {
		if len(group) == 0 {
			continue
		}

		traceIds := make([]string, 0, len(group))

		values := make([]float64, 0, len(group))
		sum := 0.0
		minValue := group[0].Value
		maxValue := group[0].Value

		for _, metric := range group {
			values = append(values, metric.Value)
			sum += metric.Value

			if metric.Value < minValue {
				minValue = metric.Value
			}
			if metric.Value > maxValue {
				maxValue = metric.Value
			}

			if metric.TraceId != "" {
				traceIds = append(traceIds, metric.TraceId)
			}
		}

		slices.Sort(values)
		metric := &models.AggregatedMetric{
			TraceId: getPrimaryTraceId(traceIds),
			Name:    groupKey,
			Count:   len(group),
			Sum:     sum,
			Min:     minValue,
			Max:     maxValue,
			P50:     getPercentile(values, 0.5),
			P95:     getPercentile(values, 0.95),
			P99:     getPercentile(values, 0.99),
			Source:  models.Grpc.String(),
		}

		p.log.Debug("Created aggregated metric",
			"batch_id", batchId,
			"metric_name", groupKey,
			"count", metric.Count,
		)

		if err = p.db.AddAggregatedMetric(ctx, metric); err != nil {
			p.log.Error("Failed to save aggregated metric to database",
				"batch_id", batchId,
				"metric_name", metric.Name,
				"error", err,
			)
			continue
		}

		// TODO: remove this if it's not used
		cacheKey := fmt.Sprintf("aggregated:metric:%s", metric.Name)
		if err = p.redis.HSet(ctx, cacheKey, metric, time.Hour); err != nil {
			p.log.Warn("Failed to save aggregated metric to redis",
				"batch_id", batchId,
				"metric_name", metric.Name,
				"error", err,
			)
		}

		// TODO: create func
		prommetrics.ObserveAggregatedMetricCount(metric.Name, models.Grpc.String(), metric.Count)
		prommetrics.ObserveAggregatedMetricSum(metric.Name, models.Grpc.String(), metric.Sum)
		prommetrics.ObserveAggregatedMetricMin(metric.Name, models.Grpc.String(), metric.Min)
		prommetrics.ObserveAggregatedMetricMax(metric.Name, models.Grpc.String(), metric.Max)
		prommetrics.ObserveAggregatedMetricP50(metric.Name, models.Grpc.String(), metric.P50)
		prommetrics.ObserveAggregatedMetricP95(metric.Name, models.Grpc.String(), metric.P95)
		prommetrics.ObserveAggregatedMetricP99(metric.Name, models.Grpc.String(), metric.P99)

		processedIds = append(processedIds, metric.Id)
	}

	if err = p.db.MarkMetricsAsProcessed(ctx, processedIds); err != nil {
		p.log.Warn("Failed to mark metrics as processed", "batch_id", batchId, "error", err)
	}

	// TODO: write normal status or remove it
	prommetrics.ObserveProcessedTotal("ok")
	return nil
}

func (p *Processor) processHttpMetrics(ctx context.Context, batchId string) error {
	end := time.Now()
	start := end.Add(-p.timeWindow)
	metrics, err := p.getUnprocessedHttpMetrics(ctx, start, end)
	if err != nil {
		return fmt.Errorf("failed to get unprocessed http metrics: %w", err)
	}

	if len(metrics) == 0 {
		p.log.Debug("No HTTP metrics to process", "batch_id", batchId)
		return nil
	}

	metricsCount := len(metrics)
	processedIds := make([]int, 0, metricsCount)
	groups := p.groupHttpMetrics(metrics)
	for groupKey, group := range groups {
		if len(group) == 0 {
			continue
		}

		groupSize := len(group)
		aggregations := []models.HttpAggregationValue{
			models.Duration,
			models.RequestSize,
			models.ResponseSize,
		}

		for _, v := range aggregations {
			value := getAggregatedHttpMetricValue(group, v)
			if value == nil {
				continue
			}

			metric := &models.AggregatedMetric{
				TraceId:   getPrimaryTraceId(extractTraceIdsFromHttp(group)),
				Name:      fmt.Sprintf("%s:%s", groupKey, v.String()),
				Count:     groupSize,
				Sum:       value.sum,
				Min:       value.min,
				Max:       value.max,
				P50:       value.p50,
				P95:       value.p95,
				P99:       value.p99,
				CreatedAt: time.Now(),
				Source:    models.Http.String(),
			}

			p.log.Debug("Created HTTP aggregated metric",
				"batch_id", batchId,
				"metric_name", metric.Name,
				"count", metric.Count,
			)

			if err = p.db.AddAggregatedMetric(ctx, metric); err != nil {
				p.log.Error("Failed to save HTTP aggregated metric",
					"batch_id", batchId,
					"metric_name", metric.Name,
					"error", err,
				)
				continue
			}

			// TODO: remove this if it's not used
			cacheKey := fmt.Sprintf("aggregated:http:%s", metric.Name)
			if err = p.redis.HSet(ctx, cacheKey, metric, time.Hour); err != nil {
				p.log.Warn("Failed to cache HTTP aggregated metric",
					"batch_id", batchId,
					"metric_name", metric.Name,
					"error", err,
				)
			}

			processedIds = append(processedIds, metric.Id)
		}
	}

	if err = p.db.MarkMetricsAsProcessed(ctx, processedIds); err != nil {
		p.log.Warn("Failed to mark metrics as processed", "batch_id", batchId, "error", err)
	}

	// TODO: write normal status or remove it
	prommetrics.ObserveProcessedTotal("ok")
	return nil
}

func (p *Processor) groupMetrics(metrics []*models.Metric) map[string][]*models.Metric {
	groups := make(map[string][]*models.Metric)

	for _, metric := range metrics {
		key := fmt.Sprintf("%s:%d", metric.Name, metric.Type)
		if _, ok := groups[key]; !ok {
			groups[key] = make([]*models.Metric, 0)
		}
		groups[key] = append(groups[key], metric)
	}

	return groups
}

func (p *Processor) groupHttpMetrics(metrics []*models.HttpMetric) map[string][]*models.HttpMetric {
	groups := make(map[string][]*models.HttpMetric)

	for _, metric := range metrics {
		key := fmt.Sprintf("%s:%s", metric.Method, metric.Endpoint)
		if _, ok := groups[key]; !ok {
			groups[key] = make([]*models.HttpMetric, 0)
		}
		groups[key] = append(groups[key], metric)
	}

	return groups
}

func getAggregatedHttpMetricValue(
	metrics []*models.HttpMetric,
	aggregationValue models.HttpAggregationValue,
) *AggregatedHttpMetricValue {
	var getValue func(*models.HttpMetric) float64

	switch aggregationValue {
	case models.Duration:
		getValue = func(m *models.HttpMetric) float64 { return float64(m.Duration) }
	case models.RequestSize:
		getValue = func(m *models.HttpMetric) float64 { return float64(m.RequestSize) }
	case models.ResponseSize:
		getValue = func(m *models.HttpMetric) float64 { return float64(m.ResponseSize) }
	}

	return aggregateHttpMetricValue(metrics, getValue)
}

func aggregateHttpMetricValue(
	metrics []*models.HttpMetric,
	getValue func(*models.HttpMetric) float64,
) *AggregatedHttpMetricValue {
	values := make([]float64, len(metrics))
	sum := 0.0
	first := getValue(metrics[0])
	minValue, maxValue := first, first

	for i, metric := range metrics {
		value := getValue(metric)
		values[i] = value
		sum += value

		if value < minValue {
			minValue = value
		}
		if value > maxValue {
			maxValue = value
		}
	}

	slices.Sort(values)
	return &AggregatedHttpMetricValue{
		sum: sum,
		min: minValue,
		max: maxValue,
		p50: getPercentile(values, 0.5),
		p95: getPercentile(values, 0.95),
		p99: getPercentile(values, 0.99),
	}
}

func (p *Processor) getUnprocessedMetrics(ctx context.Context, start, end time.Time) ([]*models.Metric, error) {
	results, err := p.redis.ZRangeByUnixScore(ctx, "metrics:unprocessed", start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics from redis: %w", err)
	}

	if len(results) == 0 {
		return p.db.GetUnprocessedMetrics(ctx, start, end)
	}

	metrics, err := zRangeResultsToDomainModels[models.Metric](results)
	if err != nil {
		p.log.Warn("Failed to get metrics from redis", "error", err)
		return p.db.GetUnprocessedMetrics(ctx, start, end)
	}

	return metrics, nil
}

func (p *Processor) getUnprocessedHttpMetrics(ctx context.Context, start, end time.Time) ([]*models.HttpMetric, error) {
	results, err := p.redis.ZRangeByUnixScore(ctx, "http_metrics:unprocessed", start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics from redis: %w", err)
	}

	if len(results) == 0 {
		return p.db.GetUnprocessedHttpMetrics(ctx, start, end)
	}

	metrics, err := zRangeResultsToDomainModels[models.HttpMetric](results)
	if err != nil {
		p.log.Warn("Failed to get metrics from redis", "error", err)
		return p.db.GetUnprocessedHttpMetrics(ctx, start, end)
	}

	return metrics, nil
}

func zRangeResultsToDomainModels[T any](results []string) ([]*T, error) {
	domainModels := make([]*T, 0, len(results))

	for _, result := range results {
		var model T
		if err := json.Unmarshal([]byte(result), &model); err != nil {
			return nil, fmt.Errorf("failed to unmarshal result: %w", err)
		}

		domainModels = append(domainModels, &model)
	}

	return domainModels, nil
}

func getPercentile(values []float64, percent float64) float64 {
	if len(values) == 0 || values == nil {
		return 0
	}

	idx := int(float64(len(values)) * percent)
	if idx >= len(values) {
		idx = len(values) - 1
	}

	return values[idx]
}

func getPrimaryTraceId(traceIds []string) string {
	if len(traceIds) == 0 {
		return ""
	}

	return traceIds[0]
}

func extractTraceIdsFromHttp(metrics []*models.HttpMetric) []string {
	traceIds := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		if metric.TraceId != "" {
			traceIds = append(traceIds, metric.TraceId)
		}
	}

	return traceIds
}
