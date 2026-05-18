package metrics

import (
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sushkomihail/metric-aggregation-service/pkg/models"
)

var metricsProcessedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "metrics_processed_total",
	Help: "Total number of metrics processed",
})

var aggregatedMetricsTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "aggregated_metrics_total",
	Help: "Total number of aggregated metrics",
})

var metricProcessingDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "metric_processing_duration_seconds",
	Help:    "Time spent processing individual metrics",
	Buckets: prometheus.DefBuckets,
}, []string{"metric_name"})

var httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "http_requests_total",
	Help: "Total HTTP requests processed",
}, []string{"method", "endpoint", "code"})

var httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "http_request_duration_seconds",
	Help:    "HTTP request duration in seconds",
	Buckets: prometheus.DefBuckets,
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

var activeConnections = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "active_connections",
	Help: "Current number of active unary connections",
})

var activeStreams = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "active_streams",
	Help: "Current number of active streams",
})

var metricsProduced = promauto.NewCounter(prometheus.CounterOpts{
	Name: "metric_produced",
	Help: "Number of metrics produced",
})

var metricsConsumed = promauto.NewCounter(prometheus.CounterOpts{
	Name: "metric_consumed",
	Help: "Number of metrics consumed",
})

var databaseOperationsTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "database_operations_total",
	Help: "Total number of database operations",
})

var databaseOperationDuration = promauto.NewHistogram(prometheus.HistogramOpts{
	Name:    "database_operation_duration_seconds",
	Help:    "Time spent database operations",
	Buckets: prometheus.DefBuckets,
})

var redisUsedMemory = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "redis_used_memory",
	Help: "Used memory in bytes",
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

func AddMetricsProcessedTotal(count int) {
	metricsProcessedTotal.Add(float64(count))
}

func IncAggregatedMetricsTotal() {
	aggregatedMetricsTotal.Inc()
}

func ObserveProcessingDuration(metricName string, duration time.Duration) {
	metricProcessingDuration.WithLabelValues(metricName).Observe(duration.Seconds())
}

func ObserveHttpRequestsTotal(method, endpoint string, code int) {
	httpRequestsTotal.WithLabelValues(method, endpoint, strconv.Itoa(code)).Inc()
}

func ObserveHttpRequestDuration(method, endpoint string, duration time.Duration) {
	httpRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

func ObserveGrpcRequestsTotal(method string, status string) {
	grpcRequestsTotal.WithLabelValues(method, status).Inc()
}

func ObserveGrpcRequestDuration(method string, duration time.Duration) {
	grpcRequestDuration.WithLabelValues(method).Observe(duration.Seconds())
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

func IncMetricsProduced() {
	metricsProduced.Inc()
}

func IncMetricsConsumed() {
	metricsConsumed.Inc()
}

func IncDatabaseOperationsTotal() {
	databaseOperationsTotal.Inc()
}

func ObserveDatabaseOperationDuration(duration time.Duration) {
	databaseOperationDuration.Observe(duration.Seconds())
}

func ObserveRedisUsedMemory(memory string) {
	usedMemoryStr := strings.Split(strings.Split(memory, "\r\n")[1], ":")[1]
	usedMemoryInt, err := strconv.Atoi(usedMemoryStr)
	if err != nil {
		return
	}

	redisUsedMemory.Set(float64(usedMemoryInt))
}

func ObserveAggregatedMetric(metric *models.AggregatedMetric, source string) {
	aggregatedMetricCount.WithLabelValues(metric.Name, source).Set(float64(metric.Count))
	aggregatedMetricSum.WithLabelValues(metric.Name, source).Set(metric.Sum)
	aggregatedMetricMin.WithLabelValues(metric.Name, source).Set(metric.Min)
	aggregatedMetricMax.WithLabelValues(metric.Name, source).Set(metric.Max)
	aggregatedMetricP50.WithLabelValues(metric.Name, source).Set(metric.P50)
	aggregatedMetricP95.WithLabelValues(metric.Name, source).Set(metric.P95)
	aggregatedMetricP99.WithLabelValues(metric.Name, source).Set(metric.P99)
}
