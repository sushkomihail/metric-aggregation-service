package service

import (
	"context"
	"time"

	"github.com/sushkomihail/metric-aggregation-service/internal/repository/db"
	"github.com/sushkomihail/metric-aggregation-service/internal/repository/redis"
	"github.com/sushkomihail/metric-aggregation-service/pkg/models"
)

type Aggregator struct {
	db    db.DB
	redis *redis.Client
}

func NewAggregator(db db.DB, redis *redis.Client) *Aggregator {
	return &Aggregator{
		db:    db,
		redis: redis,
	}
}

func (a *Aggregator) AddMetric(ctx context.Context, metric *models.Metric) error {
	errChan := make(chan error)

	go func() {
		err := a.db.AddMetric(ctx, metric)
		errChan <- err
	}()

	err := <-errChan
	return err
}

func (a *Aggregator) AddHttpMetric(ctx context.Context, metric *models.HttpMetric) error {
	return nil
}

func (a *Aggregator) GetAggregatedMetrics(
	ctx context.Context, start, end time.Time) ([]*models.AggregatedMetric, error) {
	type result struct {
		metrics []*models.AggregatedMetric
		err     error
	}

	resChan := make(chan result)

	go func() {
		// TODO: try read from redis first

		metrics, err := a.db.GetAggregatedMetrics(ctx, start, end)
		resChan <- result{
			metrics: metrics,
			err:     err,
		}
	}()

	res := <-resChan
	return res.metrics, nil
}
