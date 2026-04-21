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
	Timestamp time.Time
}

type HttpMetric struct {
	Method   string `json:"method"`
	Endpoint string `json:"endpoint"`
	// TODO: remove headers
	Headers      map[string][]string `json:"headers"`
	Code         int                 `json:"code"`
	Duration     time.Duration       `json:"duration"`
	RequestSize  int64               `json:"request_size"`
	ResponseSize int64               `json:"response_size"`
	Timestamp    time.Time           `json:"timestamp"`
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
