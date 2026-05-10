package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/sushkomihail/metric-aggregation-service/internal/broker/kafka"
	"github.com/sushkomihail/metric-aggregation-service/internal/logger"
	"github.com/sushkomihail/metric-aggregation-service/pkg/models"
)

type responseWriter struct {
	http.ResponseWriter
	status      int
	size        int64
	body        *bytes.Buffer
	wroteHeader bool
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		status:         http.StatusOK,
		body:           &bytes.Buffer{},
	}
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}

	rw.status = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}

	size, err := rw.ResponseWriter.Write(b)
	rw.size += int64(size)
	rw.body.Write(b)
	return size, err
}

type HttpMetricMiddleware struct {
	producer *kafka.Producer
	log      *logger.Logger
}

func NewHttpMetricMiddleware(producer *kafka.Producer, log *logger.Logger) *HttpMetricMiddleware {
	return &HttpMetricMiddleware{
		producer: producer,
		log:      log,
	}
}

func (m *HttpMetricMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		traceID := r.Header.Get("X-Trace-ID")
		if traceID == "" {
			traceID = r.Header.Get("X-Request-ID")
		}
		if traceID == "" {
			traceID = uuid.New().String()
		}

		w.Header().Set("X-Trace-ID", traceID)

		m.log.Debug("HTTP request started",
			"trace_id", traceID,
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
		)

		var requestBody []byte
		if r.Body != nil {
			var err error
			requestBody, err = io.ReadAll(r.Body)
			if err != nil {
				m.log.Warn("Failed to read request body",
					"trace_id", traceID,
					"error", err,
				)
				requestBody = []byte{}
			}
			r.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		wrapped := newResponseWriter(w)
		next.ServeHTTP(wrapped, r)

		duration := time.Since(startTime)
		metric := models.HttpMetric{
			TraceId:      traceID,
			Method:       r.Method,
			Endpoint:     r.URL.Path,
			Code:         wrapped.status,
			Duration:     duration,
			RequestSize:  int64(len(requestBody)),
			ResponseSize: wrapped.size,
			Timestamp:    startTime,
		}

		m.log.Info("HTTP request completed",
			"trace_id", traceID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.status,
			"duration_ms", duration.Milliseconds(),
			"request_size", metric.RequestSize,
			"response_size", metric.ResponseSize,
		)

		go func() {
			if err := m.sendMetricToKafka(&metric); err != nil {
				m.log.Error("Failed to send HTTP metric to Kafka",
					"trace_id", traceID,
					"error", err,
				)
			}
		}()
	})
}

func (m *HttpMetricMiddleware) sendMetricToKafka(metric *models.HttpMetric) error {
	jsonData, err := json.Marshal(metric)
	if err != nil {
		return fmt.Errorf("failed to marshal HTTP metric: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err = m.producer.Produce(ctx, jsonData, kafka.HttpTopic); err != nil {
		return fmt.Errorf("failed to produce to Kafka: %w", err)
	}

	m.log.Debug("HTTP metric sent to Kafka",
		"trace_id", metric.TraceId,
		"topic", kafka.HttpTopic,
	)

	return nil
}
