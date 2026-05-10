package client

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	pb "github.com/sushkomihail/metric-aggregation-service/api/proto/generated/metrics"
	"github.com/sushkomihail/metric-aggregation-service/internal/broker/kafka"
	"github.com/sushkomihail/metric-aggregation-service/internal/config"
	"github.com/sushkomihail/metric-aggregation-service/internal/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type GrpcClientOptions struct {
	Addr    string
	Port    int
	Timeout time.Duration
}

type GrpcClient struct {
	client   pb.MetricsServiceClient
	conn     *grpc.ClientConn
	producer *kafka.Producer
	log      *logger.Logger
}

func New(
	options GrpcClientOptions,
	config config.KafkaConfig,
	log *logger.Logger,
) (*GrpcClient, error) {
	p := kafka.NewProducer(config, log)

	target := fmt.Sprintf("%s:%d", options.Addr, options.Port)
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	client := pb.NewMetricsServiceClient(conn)

	log.Info("gRPC client created", "target", target)

	return &GrpcClient{
		client:   client,
		conn:     conn,
		producer: p,
		log:      log,
	}, nil
}

func (c *GrpcClient) SendMetric(ctx context.Context, req *pb.MetricRequest) (*pb.MetricResponse, error) {
	traceId := c.ensureTraceID(ctx)

	md := metadata.Pairs("x-trace-id", traceId)
	ctx = metadata.NewOutgoingContext(ctx, md)

	startTime := time.Now()
	resp, err := c.client.SendMetric(ctx, req)
	duration := time.Since(startTime)

	if err != nil {
		c.log.Error("Failed to send unary metric",
			"trace_id", traceId,
			"error", err,
			"duration", duration,
		)
		return nil, fmt.Errorf("unary metric send failed: %w", err)
	}

	c.log.Info("Unary metric sent successfully",
		"trace_id", traceId,
		"metric_id", resp.Id,
		"duration", duration,
	)

	return resp, nil
}

func (c *GrpcClient) StreamMetrics(
	ctx context.Context,
	metrics []*pb.MetricRequest,
) (*pb.StreamResponse, error) {
	traceID := c.ensureTraceID(ctx)
	batchID := uuid.New().String()

	c.log.Info("Starting metrics stream",
		"trace_id", traceID,
		"batch_id", batchID,
		"count", len(metrics),
	)

	md := metadata.Pairs(
		"x-trace-id", traceID,
		"batch-id", batchID,
	)
	ctx = metadata.NewOutgoingContext(ctx, md)

	stream, err := c.client.StreamMetrics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}

	startTime := time.Now()
	for i, metric := range metrics {
		metricTraceID := uuid.New().String()
		if metric.Tags == nil {
			metric.Tags = make(map[string]string)
		}
		metric.Tags["trace_id"] = metricTraceID
		metric.Tags["batch_id"] = batchID
		metric.Tags["sequence"] = fmt.Sprintf("%d", i+1)

		if err = stream.Send(metric); err != nil {
			c.log.Error("Failed to send metric in stream",
				"trace_id", metricTraceID,
				"batch_id", batchID,
				"sequence", i+1,
				"error", err,
			)
			return nil, fmt.Errorf("stream send failed at metric %d: %w", i+1, err)
		}

		c.log.Debug("Metric sent in stream",
			"trace_id", metricTraceID,
			"batch_id", batchID,
			"sequence", i+1,
			"name", metric.Name,
		)
	}

	resp, err := stream.CloseAndRecv()
	duration := time.Since(startTime)

	if err != nil && err != io.EOF {
		c.log.Error("Failed to close stream",
			"trace_id", traceID,
			"batch_id", batchID,
			"error", err,
			"duration", duration,
		)
		return nil, fmt.Errorf("stream close failed: %w", err)
	}

	if resp == nil {
		c.log.Info("Response is nil",
			"trace_id", traceID,
			"batch_id", batchID,
		)
		return nil, nil
	}

	c.log.Info("Metrics stream completed",
		"trace_id", traceID,
		"batch_id", batchID,
		"sent", len(metrics),
		"received", resp.ReceivedCount,
		"failed", resp.FailedCount,
		"duration", duration,
	)

	return resp, nil
}

func (c *GrpcClient) Producer() *kafka.Producer {
	return c.producer
}

func (c *GrpcClient) Close() error {
	c.log.Info("Closing gRPC client")

	if err := c.producer.Close(); err != nil {
		c.log.Warn("Failed to close Kafka producer", "error", err)
	}

	return c.conn.Close()
}

func (c *GrpcClient) ensureTraceID(ctx context.Context) string {
	md, ok := metadata.FromOutgoingContext(ctx)
	if ok {
		if traceIDs := md.Get("x-trace-id"); len(traceIDs) > 0 {
			return traceIDs[0]
		}
	}
	return uuid.New().String()
}
