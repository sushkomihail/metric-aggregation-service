package service

import (
	"context"
	"fmt"
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
	errChan := make(chan error, 1)

	go func() {
		err := a.db.AddMetric(ctx, metric)
		errChan <- err
	}()

	return <-errChan
}

func (a *Aggregator) AddHttpMetric(ctx context.Context, metric *models.HttpMetric) error {
	key := fmt.Sprintf("http_metric:%s", metric.Timestamp.Format("2006-04-02 15-01-05"))
	if err := a.redis.HSet(ctx, key, metric); err != nil {
		return fmt.Errorf("error setting http metric to redis: %v", err)
	}

	if err := a.db.AddHttpMetric(ctx, metric); err != nil {
		return fmt.Errorf("error adding http metric to db: %v", err)
	}

	return nil
}

func (a *Aggregator) GetAggregatedMetrics(
	ctx context.Context, start, end time.Time) ([]*models.AggregatedMetric, error) {
	type result struct {
		metrics []*models.AggregatedMetric
		err     error
	}

	resChan := make(chan result, 1)

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
