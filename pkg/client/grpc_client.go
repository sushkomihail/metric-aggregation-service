package client

import (
	"fmt"
	"time"

	pb "github.com/sushkomihail/metric-aggregation-service/api/proto/generated/metrics"
	"github.com/sushkomihail/metric-aggregation-service/internal/broker/kafka"
	"github.com/sushkomihail/metric-aggregation-service/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GrpcClient struct {
	client   pb.MetricsServiceClient
	conn     *grpc.ClientConn
	producer *kafka.Producer
}

type GrpcClientOptions struct {
	Addr string
	Port int
	// TODO: use timeout
	Timeout time.Duration
}

func New(options GrpcClientOptions, config config.KafkaConfig) (*GrpcClient, error) {
	p := kafka.NewProducer(config)

	target := fmt.Sprintf("%s:%d", options.Addr, options.Port)
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	client := pb.NewMetricsServiceClient(conn)
	return &GrpcClient{
		client:   client,
		conn:     conn,
		producer: p,
	}, nil
}

func (c *GrpcClient) Producer() *kafka.Producer {
	return c.producer
}

func (c *GrpcClient) Close() error {
	err := c.producer.Close()
	if err != nil {
		return err
	}

	return c.conn.Close()
}
