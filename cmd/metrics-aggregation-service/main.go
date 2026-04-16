package main

import (
	"context"
	"log"
	"net"
	"os"
	"time"

	"github.com/joho/godotenv"
	pb "github.com/sushkomihail/metric-aggregation-service/cmd/api/proto/generated/metrics"
	srv "github.com/sushkomihail/metric-aggregation-service/cmd/internal/grpc"
	"github.com/sushkomihail/metric-aggregation-service/cmd/internal/metrics"
	"github.com/sushkomihail/metric-aggregation-service/cmd/internal/repository/db"
	"github.com/sushkomihail/metric-aggregation-service/cmd/internal/repository/redis"
	"github.com/sushkomihail/metric-aggregation-service/cmd/internal/service"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
)

func parseRedisConfig() redis.Config {
	data, err := os.ReadFile("redis.yml")
	if err != nil {
		log.Fatal(err)
	}

	var config redis.Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatal(err)
	}

	return config
}

func main() {
	redisConfig := parseRedisConfig()

	err := godotenv.Load()
	if err != nil {
		log.Fatal("error loading .env file")
	}

	lis, err := net.Listen("tcp", ":8080")

	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	go func() {
		err = metrics.Listen(":8081")
		if err != nil {
			log.Fatalf("failed to start metrics server: %v", err)
		}
	}()

	ctx := context.Background()
	redisClient, err := redis.NewClient(ctx, redisConfig)
	if err != nil {
		log.Fatalf("failed to create redis client: %v", err)
	}

	defer func() {
		err = redisClient.Close()
		if err != nil {
			log.Fatalf("failed to close redis client: %v", err)
		}
	}()

	postgres := db.NewPostgres(ctx)
	defer postgres.CloseConnection(ctx)

	aggregator := service.NewAggregator(postgres, redisClient)

	processor := service.NewProcessor(postgres, redisClient, 3*time.Second)
	go func() {
		processor.Run(ctx)
	}()

	s := grpc.NewServer()
	pb.RegisterMetricsServiceServer(s, srv.NewMetricsServer(aggregator, processor))

	if err = s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
