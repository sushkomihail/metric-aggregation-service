package main

import (
	"log"
	"net"

	"github.com/sushkomihail/metric-aggregation-service/cmd/internal/metrics"
	"github.com/sushkomihail/metric-aggregation-service/cmd/internal/repository/redis"
	"github.com/sushkomihail/metric-aggregation-service/cmd/internal/service"
	pb "github.com/sushkomihail/protos/gen/go/mas"
	"google.golang.org/grpc"
)

// client --(http)--> aggregation service --> redis

func main() {
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

	s := grpc.NewServer()
	pb.RegisterMetricAggregationServiceServer(s, service.NewServer(redisClient))

	if err = s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
