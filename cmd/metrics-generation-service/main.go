package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	pb "github.com/sushkomihail/metric-aggregation-service/api/proto/generated/metrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Client struct {
	client pb.MetricsServiceClient
	conn   *grpc.ClientConn
	stats  *Stats
	mu     sync.Mutex
}

type Stats struct {
	UnarySent     int64
	UnarySuccess  int64
	UnaryFailed   int64
	StreamSent    int64
	StreamSuccess int64
	StreamFailed  int64
	TotalDuration time.Duration
	StartTime     time.Time
}

type TestConfig struct {
	ServerAddr      string
	UnaryCount      int
	StreamCount     int
	StreamBatchSize int
	Duration        time.Duration
	Continuous      bool
	Interval        time.Duration
	Verbose         bool
}

func NewClient(serverAddr string) (*Client, error) {
	conn, err := grpc.NewClient(
		serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(10*1024*1024)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", serverAddr, err)
	}

	client := pb.NewMetricsServiceClient(conn)

	return &Client{
		client: client,
		conn:   conn,
		stats: &Stats{
			StartTime: time.Now(),
		},
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) SendUnaryMetric(
	ctx context.Context,
	metricName string,
	value float64,
	metricType pb.MetricType,
	tags map[string]string,
) error {
	traceID := uuid.New().String()

	md := metadata.Pairs("x-trace-id", traceID)
	ctx = metadata.NewOutgoingContext(ctx, md)

	req := &pb.MetricRequest{
		Name:      metricName,
		Type:      metricType,
		Value:     value,
		Tags:      tags,
		Timestamp: timestamppb.Now(),
	}

	start := time.Now()
	resp, err := c.client.SendMetric(ctx, req)
	duration := time.Since(start)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.stats.UnarySent++
	c.stats.TotalDuration += duration

	if err != nil {
		c.stats.UnaryFailed++
		return fmt.Errorf("unary send failed: %w", err)
	}

	if resp.Success {
		c.stats.UnarySuccess++
		return nil
	}

	c.stats.UnaryFailed++
	return fmt.Errorf("unary send failed: %s", resp.Message)
}

func (c *Client) StreamMetrics(ctx context.Context, metrics []*pb.MetricRequest) (*pb.StreamResponse, error) {
	batchID := uuid.New().String()
	traceID := uuid.New().String()

	md := metadata.Pairs(
		"x-trace-id", traceID,
		"batch-id", batchID,
	)
	ctx = metadata.NewOutgoingContext(ctx, md)

	stream, err := c.client.StreamMetrics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}

	for i, metric := range metrics {
		if metric.Tags == nil {
			metric.Tags = make(map[string]string)
		}
		metric.Tags["batch_id"] = batchID
		metric.Tags["sequence"] = fmt.Sprintf("%d", i+1)
		metric.Tags["trace_id"] = uuid.New().String()

		if err = stream.Send(metric); err != nil {
			return nil, fmt.Errorf("failed to send metric %d: %w", i+1, err)
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		return nil, fmt.Errorf("failed to close stream: %w", err)
	}

	c.mu.Lock()
	c.stats.StreamSent += int64(len(metrics))
	c.stats.StreamSuccess += int64(resp.ReceivedCount)
	c.stats.StreamFailed += int64(resp.FailedCount)
	c.mu.Unlock()

	return resp, nil
}

func GenerateTestMetrics(count int, metricType pb.MetricType) []*pb.MetricRequest {
	metrics := make([]*pb.MetricRequest, count)

	metricNames := []string{
		"cpu_usage_percent",
		"memory_usage_bytes",
		"disk_io_ops",
		"network_bytes_total",
		"request_latency_ms",
		"error_count",
		"queue_size",
		"active_connections",
		"throughput_per_sec",
		"response_time_ms",
	}

	for i := 0; i < count; i++ {
		name := metricNames[rand.Intn(len(metricNames))]

		var value float64
		switch metricType {
		case pb.MetricType_COUNTER:
			value = float64(rand.Intn(1000))
		case pb.MetricType_GAUGE:
			value = rand.Float64() * 100
		case pb.MetricType_HISTOGRAM:
			value = rand.ExpFloat64() * 10
		}

		tags := map[string]string{
			"host":      fmt.Sprintf("server-%d", rand.Intn(5)+1),
			"region":    []string{"us-east", "us-west", "eu-west", "ap-south"}[rand.Intn(4)],
			"env":       []string{"prod", "staging", "dev"}[rand.Intn(3)],
			"generator": "test-client",
		}

		metrics[i] = &pb.MetricRequest{
			Name:      fmt.Sprintf("%s_%d", name, i+1),
			Type:      metricType,
			Value:     value,
			Tags:      tags,
			Timestamp: timestamppb.Now(),
		}
	}

	return metrics
}

func (c *Client) GetAggregatedMetrics(ctx context.Context, metricName string, duration time.Duration) (*pb.AggregatedMetricsResponse, error) {
	end := time.Now()
	start := end.Add(-duration)

	req := &pb.AggregatedMetricsRequest{
		MetricName:      metricName,
		TimeWindowStart: timestamppb.New(start),
		TimeWindowEnd:   timestamppb.New(end),
	}

	resp, err := c.client.GetAggregatedMetrics(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get aggregated metrics: %w", err)
	}

	return resp, nil
}

func (c *Client) PrintStats() {
	c.mu.Lock()
	defer c.mu.Unlock()

	elapsed := time.Since(c.stats.StartTime)

	fmt.Println("\n" + "=")
	fmt.Println("📊 TEST CLIENT STATISTICS")
	fmt.Println("=")
	fmt.Printf("⏱️  Elapsed time: %v\n", elapsed.Round(time.Millisecond))
	fmt.Println("-")

	fmt.Println("🔹 UNARY METRICS:")
	fmt.Printf("  Sent:    %d\n", c.stats.UnarySent)
	fmt.Printf("  Success: %d (%.1f%%)\n",
		c.stats.UnarySuccess,
		float64(c.stats.UnarySuccess)/float64(c.stats.UnarySent)*100)
	fmt.Printf("  Failed:  %d (%.1f%%)\n",
		c.stats.UnaryFailed,
		float64(c.stats.UnaryFailed)/float64(c.stats.UnarySent)*100)

	fmt.Println("-")

	fmt.Println("🔹 STREAM METRICS:")
	fmt.Printf("  Sent:    %d\n", c.stats.StreamSent)
	fmt.Printf("  Success: %d\n", c.stats.StreamSuccess)
	fmt.Printf("  Failed:  %d\n", c.stats.StreamFailed)

	fmt.Println("-")

	total := c.stats.UnarySent + c.stats.StreamSent
	totalSuccess := c.stats.UnarySuccess + c.stats.StreamSuccess

	if total > 0 {
		fmt.Printf("📈 OVERALL SUCCESS RATE: %.1f%%\n",
			float64(totalSuccess)/float64(total)*100)
		fmt.Printf("⚡ AVERAGE RATE: %.2f metrics/sec\n",
			float64(total)/elapsed.Seconds())
	}

	fmt.Println("=")
}

func (c *Client) RunUnaryTest(ctx context.Context, config TestConfig) {
	fmt.Printf("\n🚀 Starting UNARY test: %d metrics\n", config.UnaryCount)
	fmt.Println("-")

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 100) // Ограничиваем параллелизм

	start := time.Now()

	metricTypes := []pb.MetricType{
		pb.MetricType_COUNTER,
		pb.MetricType_GAUGE,
		pb.MetricType_HISTOGRAM,
	}

	for i := 0; i < config.UnaryCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			metricType := metricTypes[rand.Intn(len(metricTypes))]

			var value float64
			switch metricType {
			case pb.MetricType_COUNTER:
				value = float64(rand.Intn(100))
			case pb.MetricType_GAUGE:
				value = rand.Float64() * 200
			case pb.MetricType_HISTOGRAM:
				value = rand.ExpFloat64() * 5
			}

			err := c.SendUnaryMetric(
				ctx,
				fmt.Sprintf("test_metric_%d", index),
				value,
				metricType,
				map[string]string{
					"test_id": uuid.New().String(),
				},
			)

			if config.Verbose {
				if err != nil {
					log.Printf("❌ Unary metric %d failed: %v", index, err)
				} else {
					log.Printf("✅ Unary metric %d sent successfully", index)
				}
			}
		}(i)

		if i%10 == 0 {
			time.Sleep(time.Millisecond)
		}
	}

	wg.Wait()

	duration := time.Since(start)
	fmt.Printf("✅ Unary test completed in %v\n", duration.Round(time.Millisecond))
}

func (c *Client) RunStreamTest(ctx context.Context, config TestConfig) {
	fmt.Printf("\n🚀 Starting STREAM test: %d metrics in batches of %d\n",
		config.StreamCount, config.StreamBatchSize)
	fmt.Println("-")

	batches := config.StreamCount / config.StreamBatchSize
	if config.StreamCount%config.StreamBatchSize != 0 {
		batches++
	}

	start := time.Now()

	for batch := 0; batch < batches; batch++ {
		batchSize := config.StreamBatchSize
		if (batch+1)*config.StreamBatchSize > config.StreamCount {
			batchSize = config.StreamCount - batch*config.StreamBatchSize
		}

		metricType := pb.MetricType_HISTOGRAM
		metrics := GenerateTestMetrics(batchSize, metricType)

		resp, err := c.StreamMetrics(ctx, metrics)

		if config.Verbose {
			if err != nil {
				log.Printf("❌ Stream batch %d failed: %v", batch+1, err)
			} else {
				log.Printf("✅ Stream batch %d completed: received=%d failed=%d errors=%v",
					batch+1, resp.ReceivedCount, resp.FailedCount, resp.Errors)
			}
		}

		if batch < batches-1 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	duration := time.Since(start)
	fmt.Printf("✅ Stream test completed in %v\n", duration.Round(time.Millisecond))
}

func (c *Client) RunContinuousTest(ctx context.Context, config TestConfig) {
	fmt.Printf("\n🔄 Starting CONTINUOUS test mode\n")
	fmt.Printf("   Interval: %v\n", config.Interval)
	fmt.Println("-")

	ticker := time.NewTicker(config.Interval)
	defer ticker.Stop()

	// Каждые N секунд отправляем метрики
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Отправляем unary метрики
			go func() {
				metricType := pb.MetricType_GAUGE
				err := c.SendUnaryMetric(
					context.Background(),
					"continuous_metric",
					rand.Float64()*100,
					metricType,
					map[string]string{
						"source": "continuous",
					},
				)
				if config.Verbose && err != nil {
					log.Printf("❌ Continuous metric failed: %v", err)
				}
			}()

			// Отправляем batch метрик в потоке
			go func() {
				batchSize := 10
				metrics := GenerateTestMetrics(batchSize, pb.MetricType_HISTOGRAM)
				_, err := c.StreamMetrics(context.Background(), metrics)
				if config.Verbose && err != nil {
					log.Printf("❌ Continuous stream failed: %v", err)
				}
			}()
		}
	}
}

func (c *Client) QueryAggregatedMetrics(ctx context.Context) {
	fmt.Println("\n📊 Querying aggregated metrics...")
	fmt.Println("-")

	resp, err := c.GetAggregatedMetrics(ctx, "", 5*time.Minute)
	if err != nil {
		log.Printf("❌ Failed to get aggregated metrics: %v", err)
		return
	}

	if len(resp.Metrics) == 0 {
		fmt.Println("ℹ️  No aggregated metrics found in the last 5 minutes")
		return
	}

	fmt.Printf("✅ Found %d aggregated metrics:\n", len(resp.Metrics))
	for i, metric := range resp.Metrics {
		fmt.Printf("  %d. %s - count:%d avg:%.2f min:%.2f max:%.2f p95:%.2f\n",
			i+1,
			metric.MetricName,
			metric.Count,
			metric.Sum/float64(metric.Count),
			metric.Min,
			metric.Max,
			metric.P95,
		)
	}
}

func main() {
	config := TestConfig{}

	flag.StringVar(&config.ServerAddr, "addr", "localhost:8080", "gRPC server address")
	flag.IntVar(&config.UnaryCount, "unary", 100, "Number of unary metrics to send")
	flag.IntVar(&config.StreamCount, "stream", 1000, "Number of stream metrics to send")
	flag.IntVar(&config.StreamBatchSize, "batch", 100, "Batch size for stream metrics")
	flag.DurationVar(&config.Duration, "duration", 0, "Test duration (0 for one-shot)")
	flag.BoolVar(&config.Continuous, "continuous", false, "Run in continuous mode")
	flag.DurationVar(&config.Interval, "interval", 5*time.Second, "Interval for continuous mode")
	flag.BoolVar(&config.Verbose, "verbose", false, "Verbose output")

	flag.Parse()

	fmt.Println("🔌 Connecting to gRPC server:", config.ServerAddr)
	client, err := NewClient(config.ServerAddr)
	if err != nil {
		log.Fatalf("❌ Failed to create client: %v", err)
	}
	defer client.Close()

	fmt.Println("✅ Connected successfully!")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n\n⚠️  Received interrupt signal, stopping...")
		cancel()
		client.PrintStats()
		os.Exit(0)
	}()

	if config.Continuous {
		fmt.Println("\n📋 Test Plan: CONTINUOUS MODE")
		fmt.Printf("   Sending metrics every %v\n", config.Interval)
		fmt.Print("   Press Ctrl+C to stop\n\n")

		client.RunContinuousTest(ctx, config)

		<-ctx.Done()
	} else {
		fmt.Println("\n📋 Test Plan:")
		fmt.Printf("   Unary metrics: %d\n", config.UnaryCount)
		fmt.Printf("   Stream metrics: %d in batches of %d\n", config.StreamCount, config.StreamBatchSize)
		fmt.Println()

		var wg sync.WaitGroup

		if config.UnaryCount > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				client.RunUnaryTest(ctx, config)
			}()
		}

		if config.StreamCount > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				client.RunStreamTest(ctx, config)
			}()
		}

		wg.Wait()

		fmt.Println("\n⏳ Waiting for metrics to be processed...")
		time.Sleep(2 * time.Second)

		client.QueryAggregatedMetrics(ctx)
	}

	client.PrintStats()
}

func repeat(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
