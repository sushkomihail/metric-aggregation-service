package grpc

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	pb "github.com/sushkomihail/metric-aggregation-service/api/proto/generated/metrics"
	"github.com/sushkomihail/metric-aggregation-service/internal/logger"
	"github.com/sushkomihail/metric-aggregation-service/internal/metrics"
	"github.com/sushkomihail/metric-aggregation-service/internal/service"
	"github.com/sushkomihail/metric-aggregation-service/pkg/models"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type MetricsServer struct {
	pb.UnimplementedMetricsServiceServer
	aggregator *service.Aggregator
	processor  *service.Processor
	log        *logger.Logger
}

func NewMetricsServer(
	aggregator *service.Aggregator,
	processor *service.Processor,
	log *logger.Logger,
) *MetricsServer {
	return &MetricsServer{
		aggregator: aggregator,
		processor:  processor,
		log:        log,
	}
}

func (s *MetricsServer) SendMetric(ctx context.Context, req *pb.MetricRequest) (*pb.MetricResponse, error) {
	metrics.IncActiveConnections()
	defer metrics.DecActiveConnections()

	traceId := s.extractOrGenerateTraceID(ctx)
	startTime := time.Now()
	defer func() {
		metrics.ObserveProcessingDuration(req.GetName(), time.Since(startTime))
	}()

	metric, err := getDomainMetricStruct(req, traceId)
	if err != nil {
		s.log.Error("Failed to convert metric", "trace_id", traceId, "error", err)
		return nil, status.Error(codes.InvalidArgument, "invalid metric format")
	}

	err = s.aggregator.AddMetric(ctx, metric)
	if err != nil {
		s.log.Error("Failed to add metric", "trace_id", traceId, "error", err)
		return &pb.MetricResponse{
			Id:      -1,
			Success: false,
			Message: fmt.Sprintf("failed to add metric: %v", err),
		}, status.Error(codes.Internal, "failed to process metric")
	}

	return &pb.MetricResponse{
		Id:      int64(metric.Id),
		Success: true,
	}, nil
}

func (s *MetricsServer) StreamMetrics(stream pb.MetricsService_StreamMetricsServer) error {
	metrics.IncActiveStreams()
	defer metrics.DecActiveStreams()

	var (
		receivedCount int32
		failedCount   int32
		errors        []string
	)

	md, ok := metadata.FromIncomingContext(stream.Context())
	batchId := uuid.New().String()
	if ok {
		if ids := md.Get("trace-id"); len(ids) > 0 {
			batchId = ids[0]
		}
	}

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.StreamResponse{
				ReceivedCount: receivedCount,
				FailedCount:   failedCount,
				Errors:        errors,
			})
		}
		if err != nil {
			s.log.Error("Stream receive error", "batch_id", batchId, "error", err)
			return status.Error(codes.Internal, "stream receive error")
		}

		receivedCount++
		traceId := s.extractOrGenerateTraceID(stream.Context())
		metric, err := getDomainMetricStruct(req, traceId)
		if err != nil {
			failedCount++
			errMsg := fmt.Sprintf("metric %d: %v", receivedCount, err)
			errors = append(errors, errMsg)

			s.log.Warn("Failed to process stream metric",
				"batch_id", batchId,
				"trace_id", traceId,
				"error", err,
			)
			continue
		}

		if err = s.aggregator.AddMetric(stream.Context(), metric); err != nil {
			failedCount++
			errMsg := fmt.Sprintf("metric %d: %v", receivedCount, err)
			errors = append(errors, errMsg)

			s.log.Error("Failed to add stream metric",
				"batch_id", batchId,
				"trace_id", traceId,
				"error", err,
			)
			continue
		}
	}
}

func (s *MetricsServer) GetAggregatedMetrics(
	ctx context.Context,
	req *pb.AggregatedMetricsRequest,
) (*pb.AggregatedMetricsResponse, error) {
	traceId := s.extractOrGenerateTraceID(ctx)
	start, end := req.TimeWindowStart.AsTime(), req.TimeWindowEnd.AsTime()

	domainMetrics, err := s.aggregator.GetAggregatedMetrics(ctx, start, end, req.MetricName, req.Tags)
	if err != nil {
		s.log.Error("Failed to get aggregated metrics", "trace_id", traceId, "error", err)
		return nil, status.Error(codes.Internal, "failed to fetch metrics")
	}

	protoMetrics := make([]*pb.AggregatedMetric, 0, len(domainMetrics))
	for _, metric := range domainMetrics {
		protoMetrics = append(protoMetrics, getProtoAggregatedMetricStruct(metric, req.TimeWindowStart, req.TimeWindowEnd))
	}

	return &pb.AggregatedMetricsResponse{
		Metrics: protoMetrics,
	}, nil
}

func (s *MetricsServer) extractOrGenerateTraceID(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		if traceIds := md.Get("x-trace-id"); len(traceIds) > 0 {
			return traceIds[0]
		}
		if traceIds := md.Get("trace-id"); len(traceIds) > 0 {
			return traceIds[0]
		}
	}

	return uuid.New().String()
}

func getDomainMetricStruct(req *pb.MetricRequest, traceId string) (*models.Metric, error) {
	metricType, err := getServiceMetricType(req.GetType())
	if err != nil {
		return nil, err
	}

	timestamp := time.Now()
	if req.Timestamp != nil {
		timestamp = req.Timestamp.AsTime()
	}

	return &models.Metric{
		TraceId:   traceId,
		Name:      req.GetName(),
		Value:     req.GetValue(),
		Type:      metricType,
		Tags:      req.GetTags(),
		Timestamp: timestamp,
	}, nil
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
		Sum:             metric.Sum,
		Min:             metric.Min,
		Max:             metric.Max,
		P50:             metric.P50,
		P95:             metric.P95,
		P99:             metric.P99,
		Tags:            map[string]string{"trace_id": metric.TraceId},
	}
}

func getServiceMetricType(protoType pb.MetricType) (models.MetricType, error) {
	switch protoType {
	case pb.MetricType_COUNTER:
		return models.Counter, nil
	case pb.MetricType_GAUGE:
		return models.Gauge, nil
	case pb.MetricType_HISTOGRAM:
		return models.Histogram, nil
	default:
		return models.Unknown, fmt.Errorf("unknown metric type: %v", protoType)
	}
}
