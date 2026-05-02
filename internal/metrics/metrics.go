package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var httpRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "http_requests_total",
	Help: "How many HTTP requests processed, partitioned by status code and HTTP method.",
}, []string{"method", "endpoint", "code"})

var httpRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name: "http_request_duration_seconds",
}, []string{"method", "endpoint"})

var httpRequestSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "http_request_size_bytes",
	Buckets: []float64{100, 1000, 10000, 100000, 1000000},
}, []string{"method", "endpoint"})

var httpResponseSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "http_response_size_bytes",
	Buckets: []float64{100, 1000, 10000, 100000, 1000000},
}, []string{"method", "endpoint"})

var activeConnections = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "active_connections",
})

var activeStreams = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "active_streams",
})

var queueSize = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "queue_size",
})

func Register() {
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(httpRequestSize)
	prometheus.MustRegister(httpResponseSize)
	prometheus.MustRegister(activeConnections)
}

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

func IncActiveConnections() {
	activeConnections.Inc()
}

func DecActiveConnections() {
	activeConnections.Dec()
}
