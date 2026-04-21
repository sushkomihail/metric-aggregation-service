package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sushkomihail/metric-aggregation-service/internal/broker"
	"github.com/sushkomihail/metric-aggregation-service/pkg/models"
)

type responseWriter struct {
	http.ResponseWriter
	status int
	size   int64
	body   *bytes.Buffer
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.size += int64(size)
	rw.body.Write(b)
	return size, err
}

type HttpMetricMiddleware struct {
	producer *broker.KafkaProducer
}

func NewHttpMetricMiddleware(producer *broker.KafkaProducer) *HttpMetricMiddleware {
	return &HttpMetricMiddleware{
		producer: producer,
	}
}

func (m *HttpMetricMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		headers := make(map[string][]string)
		for k, v := range r.Header {
			headers[k] = v
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			body = []byte{}
		}

		r.Body = io.NopCloser(bytes.NewBuffer(body))
		wrapped := &responseWriter{
			ResponseWriter: w,
			status:         http.StatusOK,
			body:           &bytes.Buffer{},
		}
		next.ServeHTTP(wrapped, r)

		metric := models.HttpMetric{
			Method:       r.Method,
			Endpoint:     r.URL.Path,
			Headers:      headers,
			Code:         wrapped.status,
			Duration:     time.Since(startTime),
			RequestSize:  int64(len(body)),
			ResponseSize: wrapped.size,
			Timestamp:    startTime,
		}
		jsonData, err := json.Marshal(metric)
		if err != nil {
			fmt.Println(err)
			return
		}

		err = m.producer.Produce(context.Background(), jsonData, broker.HttpTopic)
		if err != nil {
			fmt.Println(err)
		}
	})
}
