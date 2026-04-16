package grpc

import (
	"context"
	"fmt"
	"log"

	pb "github.com/sushkomihail/metric-aggregation-service/cmd/api/proto/generated/metrics"
	"github.com/sushkomihail/metric-aggregation-service/cmd/internal/models"
	"github.com/sushkomihail/metric-aggregation-service/cmd/internal/service"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	metric, err := getServiceMetricStruct(req)
	if err != nil {
		log.Println("can not convert proto to service struct")
		return nil, err
	}

	err = s.aggregator.AddMetric(ctx, metric)
	if err != nil {
		log.Println("can not add metric: ", err)
		return &pb.MetricResponse{
			Id:      -1,
			Success: false,
			Message: fmt.Sprintf("can not add metric: %v", err),
		}, err
	}

	return &pb.MetricResponse{
		Id:      int64(metric.Id),
		Success: true,
	}, nil
}

func (s *MetricsServer) GetAggregatedMetrics(
	ctx context.Context,
	req *pb.AggregatedMetricsRequest,
) (*pb.AggregatedMetricsResponse, error) {
	start, end := req.TimeWindowStart.AsTime(), req.TimeWindowEnd.AsTime()
	metrics, err := s.aggregator.GetAggregatedMetrics(ctx, start, end)
	if err != nil {
		return nil, err
	}

	protoMetrics := make([]*pb.AggregatedMetric, len(metrics))
	for i, metric := range metrics {
		protoMetrics[i] = getProtoAggregatedMetricStruct(metric, req.TimeWindowStart, req.TimeWindowEnd)
	}

	return &pb.AggregatedMetricsResponse{
		Metrics: protoMetrics,
	}, nil
}

func getServiceMetricStruct(req *pb.MetricRequest) (*models.Metric, error) {
	metric := &models.Metric{
		Name:  req.GetName(),
		Value: req.GetValue(),
		Tags:  req.GetTags(),
	}

	metricType, err := getServiceMetricType(req.GetType())
	if err != nil {
		return nil, err
	}

	metric.Type = metricType
	return metric, nil
}

func getProtoAggregatedMetricStruct(
	metric *models.AggregatedMetric,
	start, end *timestamppb.Timestamp,
) *pb.AggregatedMetric {
	return &pb.AggregatedMetric{
		MetricName:      metric.Name,
		TimeWindowStart: start,
		TimeWindowEnd:   end,
		Count:           int64(metric.Count),
		Rate:            metric.Rate,
		Sum:             metric.Sum,
		Min:             metric.Min,
		Max:             metric.Max,
		P50:             metric.P50,
		P95:             metric.P95,
		P99:             metric.P99,
	}
}

func getServiceMetricType(protoMetricType pb.MetricType) (models.MetricType, error) {
	metricType := models.Unknown

	switch protoMetricType {
	case pb.MetricType_COUNTER:
		metricType = models.Counter
	case pb.MetricType_GAUGE:
		metricType = models.Gauge
	case pb.MetricType_HISTOGRAM:
		metricType = models.Histogram
	default:
		return metricType, fmt.Errorf("unknown metric type: %s", protoMetricType)
	}

	return metricType, nil
}
