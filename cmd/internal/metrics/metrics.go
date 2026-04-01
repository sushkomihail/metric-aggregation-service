package metrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace = "aggregation_service"
)

var httpRequestsCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: namespace,
	Name:      "http_requests_total",
	Help:      "How many HTTP requests processed, partitioned by status code and HTTP method.",
}, []string{"code", "method"})

func registerMetrics() {
	prometheus.MustRegister(httpRequestsCounter)
}

func ObserveHttpRequestsCounter(code int, method string) {
	httpRequestsCounter.WithLabelValues(strconv.Itoa(code), method).Inc()
}
