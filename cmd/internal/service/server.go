package service

import (
	"context"
	"fmt"

	"github.com/sushkomihail/metric-aggregation-service/cmd/internal/metrics"
	"github.com/sushkomihail/metric-aggregation-service/cmd/internal/repository/redis"
	pb "github.com/sushkomihail/protos/gen/go/mas"
)

type Server struct {
	redis *redis.Client
	pb.UnimplementedMetricAggregationServiceServer
}

func NewServer(redis *redis.Client) *Server {
	return &Server{
		redis: redis,
	}
}

func (s *Server) GetVisit(ctx context.Context, in *pb.VisitRequest) (*pb.VisitResponse, error) {
	defer func() {
		metrics.ObserveHttpRequestsCounter(200, "GET")
		s.redis.IncrCounter(ctx, "http_requests_count")
	}()

	fmt.Printf("page id: %d\n", in.GetPageId())
	return &pb.VisitResponse{MaxOnline: 1}, nil
}
