package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/sushkomihail/metric-aggregation-service/internal/config"
	"github.com/sushkomihail/metric-aggregation-service/internal/middleware"
	"github.com/sushkomihail/metric-aggregation-service/pkg/client"
)

func testHandler(w http.ResponseWriter, r *http.Request) {
	_, err := w.Write([]byte("Hello World"))
	if err != nil {
		fmt.Println(err)
	}
}

func main() {
	var cfg config.Config
	cfg.Load()

	metricsClient, err := client.New(client.GrpcClientOptions{
		Addr:    "localhost",
		Port:    50051,
		Timeout: 5 * time.Second,
	}, cfg.KafkaConfig())
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = metricsClient.Close(); err != nil {
			panic(err)
		}
	}()

	mw := middleware.NewHttpMetricMiddleware(metricsClient.Producer())
	router := http.NewServeMux()
	router.HandleFunc("/test", testHandler)
	err = http.ListenAndServe(":5050", mw.Handler(router))
	if err != nil {
		return
	}
}
