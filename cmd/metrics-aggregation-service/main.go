package main

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/joho/godotenv"
	pb "github.com/sushkomihail/metric-aggregation-service/cmd/api/proto/generated/metrics"
	srv "github.com/sushkomihail/metric-aggregation-service/cmd/internal/grpc"
	"github.com/sushkomihail/metric-aggregation-service/cmd/internal/metrics"
	"github.com/sushkomihail/metric-aggregation-service/cmd/internal/repository/db"
	"github.com/sushkomihail/metric-aggregation-service/cmd/internal/repository/redis"
	"github.com/sushkomihail/metric-aggregation-service/cmd/internal/service"
	"google.golang.org/grpc"
)

func main() {
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

	redisClient := redis.NewClient("localhost:6379", "", 0)
	defer func() {
		err = redisClient.Close()
		if err != nil {
			log.Fatalf("failed to close redis client: %v", err)
		}
	}()

	ctx := context.Background()
	postgres := db.NewPostgres(ctx)
	defer postgres.CloseConnection(ctx)

	aggregator := service.NewAggregator(postgres, redisClient)

	processor := service.NewProcessor(10 * time.Second)
	// processor.Run(context.Background())

	s := grpc.NewServer()
	pb.RegisterMetricsServiceServer(s, srv.NewMetricsServer(aggregator, processor))

	if err = s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
