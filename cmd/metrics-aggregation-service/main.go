package main

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/joho/godotenv"
	pb "github.com/sushkomihail/metric-aggregation-service/api/proto/generated/metrics"
	"github.com/sushkomihail/metric-aggregation-service/internal/config"
	srv "github.com/sushkomihail/metric-aggregation-service/internal/grpc"
	"github.com/sushkomihail/metric-aggregation-service/internal/metrics"
	"github.com/sushkomihail/metric-aggregation-service/internal/repository/db"
	redis2 "github.com/sushkomihail/metric-aggregation-service/internal/repository/redis"
	service2 "github.com/sushkomihail/metric-aggregation-service/internal/service"
	"google.golang.org/grpc"
)

func main() {
	var cfg config.Config
	cfg.Load()

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
	redisClient, err := redis2.NewClient(ctx, cfg.RedisConfig())
	if err != nil {
		log.Fatalf("failed to create redis client: %v", err)
	}

	defer func() {
		err = redisClient.Close()
		if err != nil {
			log.Fatalf("failed to close redis client: %v", err)
		}
	}()

	postgres := db.NewPostgres(ctx, cfg.PostgresConfig())
	defer postgres.CloseConnection(ctx)

	aggregator := service2.NewAggregator(postgres, redisClient)

	processor := service2.NewProcessor(postgres, redisClient, 3*time.Second)
	go func() {
		processor.Run(ctx)
	}()

	s := grpc.NewServer()
	pb.RegisterMetricsServiceServer(s, srv.NewMetricsServer(aggregator, processor))

	if err = s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
