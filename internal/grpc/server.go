package grpc

import (
	"context"
	"fmt"
	"log"

	"github.com/sushkomihail/metric-aggregation-service/api/proto/generated/metrics"
	"github.com/sushkomihail/metric-aggregation-service/internal/service"
	"github.com/sushkomihail/metric-aggregation-service/pkg/models"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type MetricsServer struct {
	metrics.UnimplementedMetricsServiceServer
	aggregator *service.Aggregator
	processor  *service.Processor
}

func NewMetricsServer(aggregator *service.Aggregator, processor *service.Processor) *MetricsServer {
	return &MetricsServer{
		aggregator: aggregator,
		processor:  processor,
	}
}

func (s *MetricsServer) SendMetric(ctx context.Context, req *metrics.MetricRequest) (*metrics.MetricResponse, error) {
	metric, err := getServiceMetricStruct(req)
	if err != nil {
		log.Println("can not convert proto to service struct")
		return nil, err
	}

	err = s.aggregator.AddMetric(ctx, metric)
	if err != nil {
		log.Println("can not add metric: ", err)
		return &metrics.MetricResponse{
			Id:      -1,
			Success: false,
			Message: fmt.Sprintf("can not add metric: %v", err),
		}, err
	}

	return &metrics.MetricResponse{
		Id:      int64(metric.Id),
		Success: true,
	}, nil
}

func (s *MetricsServer) GetAggregatedMetrics(
	ctx context.Context,
	req *metrics.AggregatedMetricsRequest,
) (*metrics.AggregatedMetricsResponse, error) {
	start, end := req.TimeWindowStart.AsTime(), req.TimeWindowEnd.AsTime()
	domainMetrics, err := s.aggregator.GetAggregatedMetrics(ctx, start, end)
	if err != nil {
		return nil, err
	}

	protoMetrics := make([]*metrics.AggregatedMetric, len(domainMetrics))
	for i, metric := range domainMetrics {
		protoMetrics[i] = getProtoAggregatedMetricStruct(metric, req.TimeWindowStart, req.TimeWindowEnd)
	}

	return &metrics.AggregatedMetricsResponse{
		Metrics: protoMetrics,
	}, nil
}

func getServiceMetricStruct(req *metrics.MetricRequest) (*models.Metric, error) {
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
) *metrics.AggregatedMetric {
	return &metrics.AggregatedMetric{
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

func getServiceMetricType(protoMetricType metrics.MetricType) (models.MetricType, error) {
	metricType := models.Unknown

	switch protoMetricType {
	case metrics.MetricType_COUNTER:
		metricType = models.Counter
	case metrics.MetricType_GAUGE:
		metricType = models.Gauge
	case metrics.MetricType_HISTOGRAM:
		metricType = models.Histogram
	default:
		return metricType, fmt.Errorf("unknown metric type: %s", protoMetricType)
	}

	return metricType, nil
}
