package service

import (
	"context"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/sushkomihail/metric-aggregation-service/internal/repository/db"
	"github.com/sushkomihail/metric-aggregation-service/internal/repository/redis"
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
}

func NewProcessor(db db.DB, redis *redis.Client, timeWindow time.Duration) *Processor {
	return &Processor{
		db:         db,
		redis:      redis,
		timeWindow: timeWindow,
	}
}

func (p *Processor) Start(ctx context.Context) {
	ticker := time.NewTicker(p.timeWindow)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			go func() {
				err := p.processMetrics(ctx)
				if err != nil {
					log.Printf("error processing metrics: %v", err)
				}

				err = p.processHttpMetrics(ctx)
				if err != nil {
					log.Printf("error processing http metrics: %v", err)
				}
			}()
		}
	}
}

func (p *Processor) processMetrics(ctx context.Context) error {
	timeWindowEnd := time.Now()
	timeWindowStart := timeWindowEnd.Add(-p.timeWindow)
	// TODO: try get from redis first
	metrics, err := p.db.GetUnprocessedMetrics(ctx, timeWindowStart, timeWindowEnd)
	if err != nil {
		return err
	}

	metricsCount := len(metrics)
	groups := groupMetricsByName(metrics)
	for name, group := range groups {
		if len(group) == 0 {
			continue
		}

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
		}

		slices.Sort(values)
		rate := float64(len(group)) / float64(metricsCount)
		metric := &models.AggregatedMetric{
			Name:  name,
			Count: len(group),
			Rate:  rate,
			Sum:   sum,
			Min:   minValue,
			Max:   maxValue,
			P50:   getPercentile(values, 0.5),
			P95:   getPercentile(values, 0.95),
			P99:   getPercentile(values, 0.99),
		}

		err = p.db.AddAggregatedMetric(ctx, metric)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Processor) processHttpMetrics(ctx context.Context) error {
	timeWindowEnd := time.Now()
	timeWindowStart := timeWindowEnd.Add(-p.timeWindow)
	// TODO: try get from redis first
	metrics, err := p.db.GetHttpMetrics(ctx, timeWindowStart, timeWindowEnd)
	if err != nil {
		return err
	}

	groups := groupHttpMetricsByMethodAndEndpoint(metrics)
	for name, group := range groups {
		if len(group) == 0 {
			continue
		}

		groupSize := len(group)
		rate := float64(groupSize) / float64(len(metrics))

		durationValue := getAggregatedHttpMetricValue(group, models.Duration)
		if err = p.addAggregatedHttpMetricToDatabase(
			ctx,
			fmt.Sprintf("%s;%s", name, models.Duration),
			groupSize,
			rate,
			durationValue,
		); err != nil {
			return err
		}

		requestSizeValue := getAggregatedHttpMetricValue(group, models.RequestSize)
		if err = p.addAggregatedHttpMetricToDatabase(
			ctx,
			fmt.Sprintf("%s;%s", name, models.RequestSize),
			groupSize,
			rate,
			requestSizeValue,
		); err != nil {
			return err
		}

		responseSizeValue := getAggregatedHttpMetricValue(group, models.ResponseSize)
		if err = p.addAggregatedHttpMetricToDatabase(
			ctx,
			fmt.Sprintf("%s;%s", name, models.ResponseSize),
			groupSize,
			rate,
			responseSizeValue,
		); err != nil {
			return err
		}
	}
	return nil
}

func (p *Processor) addAggregatedHttpMetricToDatabase(
	ctx context.Context,
	name string,
	count int,
	rate float64,
	value *AggregatedHttpMetricValue,
) error {
	metric := &models.AggregatedMetric{
		Name:  name,
		Count: count,
		Rate:  rate,
		Sum:   value.sum,
		Min:   value.min,
		Max:   value.max,
		P50:   value.p50,
		P95:   value.p95,
		P99:   value.p99,
	}

	return p.db.AddAggregatedMetric(ctx, metric)
}

func groupMetricsByName(metrics []*models.Metric) map[string][]*models.Metric {
	groups := make(map[string][]*models.Metric)

	for _, metric := range metrics {
		if _, ok := groups[metric.Name]; !ok {
			groups[metric.Name] = make([]*models.Metric, 0)
		}

		groups[metric.Name] = append(groups[metric.Name], metric)
	}

	return groups
}

func groupHttpMetricsByMethodAndEndpoint(metrics []*models.HttpMetric) map[string][]*models.HttpMetric {
	groups := make(map[string][]*models.HttpMetric)

	for _, metric := range metrics {
		key := fmt.Sprintf("%s;%s", metric.Method, metric.Endpoint)
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
