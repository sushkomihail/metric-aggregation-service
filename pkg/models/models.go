package models

import (
	"time"
)

type MetricType int
type HttpAggregationValue string

const (
	Unknown MetricType = iota
	Counter
	Gauge
	Histogram
)

const (
	Duration     HttpAggregationValue = "duration"
	RequestSize  HttpAggregationValue = "request_size"
	ResponseSize HttpAggregationValue = "response_size"
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
	Id           int           `json:"id" redis:"id"`
	Method       string        `json:"method" redis:"method"`
	Endpoint     string        `json:"endpoint" redis:"endpoint"`
	Code         int           `json:"code" redis:"code"`
	Duration     time.Duration `json:"duration" redis:"duration"`
	RequestSize  int64         `json:"request_size" redis:"request_size"`
	ResponseSize int64         `json:"response_size" redis:"response_size"`
	Timestamp    time.Time     `json:"timestamp" redis:"timestamp"`
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

type AggregatedHttpMetric struct {
}
