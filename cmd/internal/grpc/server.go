package grpc

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	pb "github.com/sushkomihail/metric-aggregation-service/cmd/api/proto/generated/metrics"
	"github.com/sushkomihail/metric-aggregation-service/cmd/internal/models"
	"github.com/sushkomihail/metric-aggregation-service/cmd/internal/service"
)

type MetricsServer struct {
	pb.UnimplementedMetricsServiceServer
	aggregator *service.Aggregator
	processor  *service.Processor
}

func NewMetricsServer(aggregator *service.Aggregator, processor *service.Processor) *MetricsServer {
	return &MetricsServer{
		aggregator: aggregator,
		processor:  processor,
	}
}

func (s *MetricsServer) SendMetric(ctx context.Context, req *pb.MetricRequest) (*pb.MetricResponse, error) {
	metric, err := s.convertProtoToServiceStruct(req)
	if err != nil {
		log.Println("can not convert proto to service struct")
		return nil, err
	}

	err = s.aggregator.AddMetric(ctx, metric)
	if err != nil {
		log.Println("can not add metric")
		return &pb.MetricResponse{
			Id:      metric.Id.String(),
			Success: false,
		}, err
	}

	return &pb.MetricResponse{
		Id:      metric.Id.String(),
		Success: true,
	}, nil
}

func (s *MetricsServer) GetAggregatedMetrics(
	ctx context.Context,
	req *pb.AggregatedMetricsRequest,
) (*pb.AggregatedMetricsResponse, error) {
	return &pb.AggregatedMetricsResponse{}, nil
}

func (s *MetricsServer) convertProtoToServiceStruct(req *pb.MetricRequest) (*models.Metric, error) {
	metric := &models.Metric{
		Id:        uuid.New(),
		Name:      req.GetName(),
		Value:     req.GetValue(),
		Tags:      req.GetTags(),
		CreatedAt: time.Now(),
	}

	switch req.GetType() {
	case pb.MetricType_COUNTER:
		metric.Type = models.COUNTER
	case pb.MetricType_GAUGE:
		metric.Type = models.GAUGE
	case pb.MetricType_HISTOGRAM:
		metric.Type = models.HISTOGRAM
	default:
		return nil, fmt.Errorf("unknown metric type: %s", req.GetType())
	}

	return metric, nil
}
