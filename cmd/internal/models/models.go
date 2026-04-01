package models

import (
	"time"

	"github.com/google/uuid"
)

type Metric struct {
	Uuid      uuid.UUID
	Name      string
	Value     float64
	CreatedAt time.Time
}

type AggregatedMetric struct {
	MetricUuid uuid.UUID
	Sum        int
	Count      int
	Rate       float64
	P50        float64
	P95        float64
	P99        float64
}
