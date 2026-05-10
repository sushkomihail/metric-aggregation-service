package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	pb "github.com/sushkomihail/metric-aggregation-service/api/proto/generated/metrics"
	"github.com/sushkomihail/metric-aggregation-service/internal/broker/kafka"
	"github.com/sushkomihail/metric-aggregation-service/internal/config"
	srv "github.com/sushkomihail/metric-aggregation-service/internal/grpc"
	"github.com/sushkomihail/metric-aggregation-service/internal/logger"
	"github.com/sushkomihail/metric-aggregation-service/internal/metrics"
	"github.com/sushkomihail/metric-aggregation-service/internal/repository/db"
	redisrepo "github.com/sushkomihail/metric-aggregation-service/internal/repository/redis"
	"github.com/sushkomihail/metric-aggregation-service/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
)

func main() {
	if err := godotenv.Load(); err != nil {
		panic(fmt.Sprintf("error loading .env file: %v", err))
	}

	var cfg config.Config
	cfg.Load()

	log := logger.New(cfg.LogLevel)
	log.Info("Starting Metric Aggregation Service")

	go func() {
		log.Info("Starting metrics HTTP server", "port", cfg.MetricsPort)
		if err := metrics.Listen(cfg.MetricsPort); err != nil {
			log.Error("Failed to start metrics server", "error", err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	redisClient, err := redisrepo.NewClient(ctx, cfg.RedisConfig())
	if err != nil {
		panic(fmt.Sprintf("failed to create Redis client: %v", err))
	}
	defer func() {
		if err = redisClient.Close(); err != nil {
			log.Error("Failed to close Redis client", "error", err)
		}
	}()

	postgres := db.NewPostgres(ctx, cfg.PostgresConfig(), log)
	defer postgres.CloseConnection()
	go postgres.FlushWithInterval(ctx)

	aggregator := service.NewAggregator(postgres, redisClient, log)
	processor := service.NewProcessor(postgres, redisClient, 3*time.Second, log)
	go processor.Start(ctx)

	consumer := kafka.NewConsumer(
		cfg.KafkaConfig(),
		5*time.Second,
		aggregator,
		kafka.HttpTopic,
		"http-metrics-group",
		log,
	)
	go consumer.Consume(ctx)
	defer func() {
		if err = consumer.Close(); err != nil {
			log.Error("Failed to close Kafka consumer", "error", err)
		}
	}()

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			loggingUnaryInterceptor(log),
			metricsUnaryInterceptor(),
			recoveryUnaryInterceptor(log),
		),
		grpc.ChainStreamInterceptor(
			loggingStreamInterceptor(log),
			metricsStreamInterceptor(),
			recoveryStreamInterceptor(log),
		),
	)

	metricsServer := srv.NewMetricsServer(aggregator, processor, log)
	pb.RegisterMetricsServiceServer(grpcServer, metricsServer)

	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("metrics.MetricsService", grpc_health_v1.HealthCheckResponse_SERVING)

	reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", cfg.GrpcPort)
	if err != nil {
		panic(fmt.Sprintf("failed to listen on %s: %v", cfg.GrpcPort, err))
	}

	go func() {
		log.Info("gRPC server starting", "port", cfg.GrpcPort)
		if err = grpcServer.Serve(lis); err != nil {
			log.Error("Failed to serve gRPC", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	sig := <-quit
	log.Info("Received shutdown signal", "signal", sig.String())
	log.Info("Starting graceful shutdown...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	shutdownDone := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(shutdownDone)
	}()

	select {
	case <-shutdownDone:
		log.Info("gRPC server stopped gracefully")
	case <-shutdownCtx.Done():
		log.Warn("gRPC server stop timeout, forcing stop")
		grpcServer.Stop()
	}

	log.Info("Service stopped successfully")
}

func loggingUnaryInterceptor(log *logger.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()

		traceID := extractTraceID(ctx)

		log.Debug("gRPC unary request",
			"trace_id", traceID,
			"method", info.FullMethod,
		)

		resp, err := handler(ctx, req)

		log.Info("gRPC unary response",
			"trace_id", traceID,
			"method", info.FullMethod,
			"duration", time.Since(start),
			"error", err,
		)

		return resp, err
	}
}

func loggingStreamInterceptor(log *logger.Logger) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()
		traceID := extractTraceID(ss.Context())

		log.Debug("gRPC stream started",
			"trace_id", traceID,
			"method", info.FullMethod,
		)

		err := handler(srv, ss)

		log.Info("gRPC stream completed",
			"trace_id", traceID,
			"method", info.FullMethod,
			"duration", time.Since(start),
			"error", err,
		)

		return err
	}
}

func metricsUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		metrics.IncActiveConnections()
		defer metrics.DecActiveConnections()

		return handler(ctx, req)
	}
}

func metricsStreamInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		metrics.IncActiveStreams()
		defer metrics.DecActiveStreams()

		return handler(srv, ss)
	}
}

func recoveryUnaryInterceptor(log *logger.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("Panic in unary handler",
					"method", info.FullMethod,
					"panic", r,
				)
				err = fmt.Errorf("internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

func recoveryStreamInterceptor(log *logger.Logger) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("Panic in stream handler",
					"method", info.FullMethod,
					"panic", r,
				)
				err = fmt.Errorf("internal server error")
			}
		}()
		return handler(srv, ss)
	}
}

func extractTraceID(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if traceIDs := md.Get("x-trace-id"); len(traceIDs) > 0 {
			return traceIDs[0]
		}
	}
	return "unknown"
}
