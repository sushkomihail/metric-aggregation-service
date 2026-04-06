package models

import (
	"time"

	"github.com/google/uuid"
)

type MetricType int

const (
	COUNTER MetricType = iota
	GAUGE
	HISTOGRAM
)

type Metric struct {
	Id        uuid.UUID
	Name      string
	Value     float64
	Type      MetricType
	Tags      map[string]string
	CreatedAt time.Time
}

type AggregatedMetric struct {
	MetricId uuid.UUID
	Sum      int
	Count    int
	Rate     float64
	P50      float64
	P95      float64
	P99      float64
}
