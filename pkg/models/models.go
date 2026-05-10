package models

import (
	"time"
)

type MetricType int
type MetricSource string
type HttpAggregationValue string

const (
	Unknown MetricType = iota
	Counter
	Gauge
	Histogram
)

const (
	Grpc MetricSource = "grpc"
	Http MetricSource = "http"
)

func (s MetricSource) String() string {
	return string(s)
}

const (
	Duration     HttpAggregationValue = "duration"
	RequestSize  HttpAggregationValue = "request_size"
	ResponseSize HttpAggregationValue = "response_size"
)

func (v HttpAggregationValue) String() string {
	return string(v)
}

type Metric struct {
	Id        int               `json:"id" redis:"id"`
	TraceId   string            `json:"trace_id" redis:"trace_id"`
	Name      string            `json:"name" redis:"name"`
	Value     float64           `json:"value" redis:"value"`
	Type      MetricType        `json:"type" redis:"type"`
	Tags      map[string]string `json:"tags" redis:"tags"`
	Timestamp time.Time         `json:"timestamp" redis:"timestamp"`
}

type HttpMetric struct {
	Id           int           `json:"id" redis:"id"`
	TraceId      string        `json:"trace_id" redis:"trace_id"`
	Method       string        `json:"method" redis:"method"`
	Endpoint     string        `json:"endpoint" redis:"endpoint"`
	Code         int           `json:"code" redis:"code"`
	Duration     time.Duration `json:"duration" redis:"duration"`
	RequestSize  int64         `json:"request_size" redis:"request_size"`
	ResponseSize int64         `json:"response_size" redis:"response_size"`
	Timestamp    time.Time     `json:"timestamp" redis:"timestamp"`
}

type AggregatedMetric struct {
	Id        int       `json:"id" redis:"id"`
	TraceId   string    `json:"trace_id,omitempty" redis:"trace_id"`
	Name      string    `json:"name" redis:"name"`
	Count     int       `json:"count" redis:"count"`
	Sum       float64   `json:"sum" redis:"sum"`
	Min       float64   `json:"min" redis:"min"`
	Max       float64   `json:"max" redis:"max"`
	P50       float64   `json:"p50" redis:"p50"`
	P95       float64   `json:"p95" redis:"p95"`
	P99       float64   `json:"p99" redis:"p99"`
	CreatedAt time.Time `json:"created_at" redis:"created_at"`
	Source    string    `json:"source" redis:"source"`
}
