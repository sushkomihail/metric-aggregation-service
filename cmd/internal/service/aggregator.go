package service

import (
	"context"

	"github.com/sushkomihail/metric-aggregation-service/cmd/internal/models"
	"github.com/sushkomihail/metric-aggregation-service/cmd/internal/repository/db"
	"github.com/sushkomihail/metric-aggregation-service/cmd/internal/repository/redis"
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
	err := a.db.AddMetric(ctx, metric)
	if err != nil {
		return err
	}

	return nil
}
