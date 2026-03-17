package main

import (
	"context"
	"fmt"
	"log"
	"net"

	pb "github.com/sushkomihail/protos/gen/go/mas"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedMetricAggregationServiceServer
}

func (s *server) GetVisit(ctx context.Context, in *pb.VisitRequest) (*pb.VisitResponse, error) {
	fmt.Printf("page id: %d", in.GetPageId())
	return &pb.VisitResponse{MaxOnline: 1}, nil
}

func main() {
	lis, err := net.Listen("tcp", ":8080")

	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterMetricAggregationServiceServer(s, &server{})

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
