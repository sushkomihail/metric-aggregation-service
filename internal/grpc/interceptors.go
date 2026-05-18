package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/sushkomihail/metric-aggregation-service/internal/logger"
	"github.com/sushkomihail/metric-aggregation-service/pkg/metrics"
	"google.golang.org/grpc"
)

func LoggingUnaryInterceptor(log *logger.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)
		if err != nil {
			log.Error("gRPC unary call failed", "method", info.FullMethod, "error", err)
			metrics.ObserveGrpcRequestsTotal(info.FullMethod, "failed")
		} else {
			log.Info("gRPC unary call succeeded", "method", info.FullMethod, "duration", duration)
			metrics.ObserveGrpcRequestsTotal(info.FullMethod, "success")
			metrics.ObserveGrpcRequestDuration(info.FullMethod, duration)
		}

		return resp, err
	}
}

func LoggingStreamInterceptor(log *logger.Logger) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()
		err := handler(srv, ss)
		duration := time.Since(start)
		if err != nil {
			log.Error("gRPC stream call failed", "method", info.FullMethod, "error", err)
			metrics.ObserveGrpcRequestsTotal(info.FullMethod, "failed")
		} else {
			log.Info("gRPC stream call succeeded", "method", info.FullMethod, "duration", duration)
			metrics.ObserveGrpcRequestDuration(info.FullMethod, duration)
			metrics.ObserveGrpcRequestsTotal(info.FullMethod, "success")
		}

		return err
	}
}

func RecoveryUnaryInterceptor(log *logger.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("Panic in unary handler", "method", info.FullMethod, "panic", r)
				err = fmt.Errorf("internal server error")
			}
		}()

		return handler(ctx, req)
	}
}

func RecoveryStreamInterceptor(log *logger.Logger) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("Panic in stream handler", "method", info.FullMethod, "panic", r)
				err = fmt.Errorf("internal server error")
			}
		}()

		return handler(srv, ss)
	}
}
