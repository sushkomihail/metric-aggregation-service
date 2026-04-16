package service

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strconv"
	"time"

	"github.com/sushkomihail/metric-aggregation-service/cmd/internal/models"
	"github.com/sushkomihail/metric-aggregation-service/cmd/internal/repository/db"
	"github.com/sushkomihail/metric-aggregation-service/cmd/internal/repository/redis"
)

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

func (p *Processor) Run(ctx context.Context) {
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
			}()
		}
	}
}

func (p *Processor) processMetrics(ctx context.Context) error {
	timeWindowEnd := time.Now()
	timeWindowStart := timeWindowEnd.Add(-p.timeWindow)
	metrics, err := p.db.GetUnprocessedMetrics(ctx, timeWindowStart, timeWindowEnd)
	if err != nil {
		return err
	}

	metricsCount := len(metrics)
	groups := groupMetricsByName(metrics)
	for name, group := range groups {
		values := make([]float64, len(group))
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

		err = p.redis.HSet(ctx, strconv.Itoa(metric.Id), metric)
		if err != nil {
			return err
		}

		fmt.Println("aggregated metric added to redis")
	}

	return nil
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
