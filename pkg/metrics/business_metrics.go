package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// TODO: http metrics may be replaced
var httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "http_requests_total",
	Help: "Total HTTP requests processed",
}, []string{"method", "endpoint", "code"})

var httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "http_request_duration_seconds",
	Help:    "HTTP request duration in seconds",
	Buckets: prometheus.DefBuckets,
}, []string{"method", "endpoint"})

var httpRequestSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "http_request_size_bytes",
	Buckets: prometheus.ExponentialBuckets(100, 10, 5),
}, []string{"method", "endpoint"})

var httpResponseSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "http_response_size_bytes",
	Buckets: prometheus.ExponentialBuckets(100, 10, 5),
}, []string{"method", "endpoint"})

var grpcRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "grpc_requests_total",
	Help: "Total number of gRPC requests",
}, []string{"method", "status"})

var grpcRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "grpc_request_duration_seconds",
	Help:    "gRPC request duration in seconds",
	Buckets: prometheus.DefBuckets,
}, []string{"method"})

var grpcRequestSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "grpc_request_size_bytes",
	Help:    "gRPC request size in bytes",
	Buckets: prometheus.ExponentialBuckets(100, 10, 5),
}, []string{"method"})

var grpcResponseSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "grpc_response_size_bytes",
	Help:    "gRPC response size in bytes",
	Buckets: prometheus.ExponentialBuckets(100, 10, 5),
}, []string{"method"})

var activeConnections = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "active_connections",
	Help: "Current number of active unary connections",
})

var activeStreams = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "active_streams",
	Help: "Current number of active streams",
})

var aggregatedMetricCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "aggregated_metric_count",
	Help: "Count of raw metrics in aggregated group",
}, []string{"name", "source"})

var aggregatedMetricSum = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "aggregated_metric_sum",
	Help: "Sum of values in aggregated group",
}, []string{"name", "source"})

var aggregatedMetricMin = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "aggregated_metric_min",
	Help: "Minimum value in aggregated group",
}, []string{"name", "source"})

var aggregatedMetricMax = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "aggregated_metric_max",
	Help: "Maximum value in aggregated group",
}, []string{"name", "source"})

var aggregatedMetricP50 = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "aggregated_metric_p50",
	Help: "P50 percentile of aggregated group",
}, []string{"name", "source"})

var aggregatedMetricP95 = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "aggregated_metric_p95",
	Help: "P95 percentile of aggregated group",
}, []string{"name", "source"})

var aggregatedMetricP99 = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "aggregated_metric_p99",
	Help: "P99 percentile of aggregated group",
}, []string{"name", "source"})

func ObserveHttpRequestsTotal(method, endpoint string, code int) {
	httpRequestsTotal.WithLabelValues(method, endpoint, strconv.Itoa(code)).Inc()
}

func ObserveHttpRequestDuration(method, endpoint string, duration time.Duration) {
	httpRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

func ObserveHttpRequestSize(method, endpoint string, size int64) {
	httpRequestSize.WithLabelValues(method, endpoint).Observe(float64(size))
}

func ObserveHttpResponseSize(method, endpoint string, size int64) {
	httpResponseSize.WithLabelValues(method, endpoint).Observe(float64(size))
}

func ObserveGrpcRequestsTotal(method string, status int) {
	grpcRequestsTotal.WithLabelValues(method, strconv.Itoa(status)).Inc()
}

func ObserveGrpcRequestDuration(method string, duration time.Duration) {
	grpcRequestDuration.WithLabelValues(method).Observe(duration.Seconds())
}

func ObserveGrpcRequestSize(method string, size int64) {
	grpcRequestSize.WithLabelValues(method).Observe(float64(size))
}

func ObserveGrpcResponseSize(method string, size int64) {
	grpcResponseSize.WithLabelValues(method).Observe(float64(size))
}

func IncActiveConnections() {
	activeConnections.Inc()
}

func DecActiveConnections() {
	activeConnections.Dec()
}

func IncActiveStreams() {
	activeStreams.Inc()
}

func DecActiveStreams() {
	activeStreams.Dec()
}

func ObserveAggregatedMetricCount(name, source string, count int) {
	aggregatedMetricCount.WithLabelValues(name, source).Set(float64(count))
}

func ObserveAggregatedMetricSum(name, source string, sum float64) {
	aggregatedMetricSum.WithLabelValues(name, source).Set(sum)
}

func ObserveAggregatedMetricMin(name, source string, min float64) {
	aggregatedMetricMin.WithLabelValues(name, source).Set(min)
}

func ObserveAggregatedMetricMax(name, source string, max float64) {
	aggregatedMetricMax.WithLabelValues(name, source).Set(max)
}

func ObserveAggregatedMetricP50(name, source string, p50 float64) {
	aggregatedMetricP50.WithLabelValues(name, source).Set(p50)
}

func ObserveAggregatedMetricP95(name, source string, p95 float64) {
	aggregatedMetricP95.WithLabelValues(name, source).Set(p95)
}

func ObserveAggregatedMetricP99(name, source string, p99 float64) {
	aggregatedMetricP99.WithLabelValues(name, source).Set(p99)
}
