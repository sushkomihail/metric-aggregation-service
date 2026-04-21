package middleware

import (
	"bytes"
	"io"
	"net/http"
)

func prometheusMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// start := time.Now()

			body, err := io.ReadAll(r.Body)
			if err != nil {
				body = []byte{}
			}
			r.Body = io.NopCloser(bytes.NewBuffer(body))

			//metrics.ObserveHttpRequestsTotal(r.Method, r.URL.Path, wrapped.status)
			//metrics.ObserveHttpRequestDuration(r.Method, r.URL.Path, time.Since(start))
			//metrics.ObserveHttpRequestSize(r.Method, r.URL.Path, int64(len(body)))
			//metrics.ObserveHttpResponseSize(r.Method, r.URL.Path, wrapped.size)
			//
			//metrics.IncActiveConnections()
			//defer metrics.DecActiveConnections()
		})
	}
}
