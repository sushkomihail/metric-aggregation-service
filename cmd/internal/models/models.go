package models

import (
	"time"
)

type MetricType int

const (
	Unknown MetricType = iota
	Counter
	Gauge
	Histogram
)

type Metric struct {
	Id        int
	Name      string
	Value     float64
	Type      MetricType
	Tags      map[string]string
	CreatedAt time.Time
}

type AggregatedMetric struct {
	Id        int       `redis:"id"`
	Name      string    `redis:"name"`
	Count     int       `redis:"count"`
	Rate      float64   `redis:"rate"`
	Sum       float64   `redis:"sum"`
	Min       float64   `redis:"min"`
	Max       float64   `redis:"max"`
	P50       float64   `redis:"p50"`
	P95       float64   `redis:"p95"`
	P99       float64   `redis:"p99"`
	CreatedAt time.Time `redis:"created_at"`
}
