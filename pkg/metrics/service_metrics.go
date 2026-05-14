package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var metricProcessingDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "metric_processing_duration_seconds",
	Help:    "Time spent processing individual metrics",
	Buckets: prometheus.DefBuckets,
}, []string{"metric_name"})

var metricsProduced = promauto.NewCounter(prometheus.CounterOpts{
	Name: "metric_produced",
	Help: "Number of metrics produced",
})

var metricsConsumed = promauto.NewCounter(prometheus.CounterOpts{
	Name: "metric_consumed",
	Help: "Number of metrics consumed",
})

var metricsProcessedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "metrics_processed_total",
	Help: "Total number of metrics processed",
}, []string{"status"})

func ObserveProcessingDuration(metricName string, duration time.Duration) {
	metricProcessingDuration.WithLabelValues(metricName).Observe(duration.Seconds())
}

func IncMetricsProduced() {
	metricsProduced.Inc()
}

func IncMetricsConsumed() {
	metricsConsumed.Inc()
}

func ObserveProcessedTotal(status string) {
	metricsProcessedTotal.WithLabelValues(status).Inc()
}
