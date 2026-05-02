package db

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sushkomihail/metric-aggregation-service/internal/config"
	"github.com/sushkomihail/metric-aggregation-service/pkg/models"
)

type DB interface {
	AddMetric(context.Context, *models.Metric) error
	AddHttpMetric(context.Context, *models.HttpMetric) error
	AddAggregatedMetric(context.Context, *models.AggregatedMetric) error
	GetUnprocessedMetrics(context.Context, time.Time, time.Time) ([]*models.Metric, error)
	GetHttpMetrics(context.Context, time.Time, time.Time) ([]*models.HttpMetric, error)
	GetAggregatedMetrics(context.Context, time.Time, time.Time) ([]*models.AggregatedMetric, error)
}

type Postgres struct {
	conn *pgx.Conn
	mu   sync.RWMutex
}

func NewPostgres(ctx context.Context, config config.PostgresConfig) *Postgres {
	url := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		config.User,
		config.Password,
		config.Addr,
		config.Port,
		config.DB)
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		log.Fatal(err)
	}

	return &Postgres{
		conn: conn,
	}
}

func (p *Postgres) CloseConnection(ctx context.Context) {
	err := p.conn.Close(ctx)
	if err != nil {
		log.Fatal(err)
	}
}

func (p *Postgres) AddMetric(ctx context.Context, metric *models.Metric) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	query :=
		`INSERT INTO metrics (name, value, type, tags) VALUES ($1, $2, $3, $4) RETURNING id`
	err := p.conn.QueryRow(ctx, query, metric.Name, metric.Value, int(metric.Type), metric.Tags).Scan(&metric.Id)
	if err != nil {
		return err
	}

	fmt.Println("postgres: added metric")
	return nil
}

func (p *Postgres) AddHttpMetric(ctx context.Context, metric *models.HttpMetric) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	query :=
		`INSERT INTO http_metrics (method, endpoint, code, duration, request_size, response_size)
		 VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := p.conn.Exec(ctx, query,
		metric.Method,
		metric.Endpoint,
		metric.Code,
		metric.Duration,
		metric.RequestSize,
		metric.ResponseSize)
	if err != nil {
		return err
	}

	fmt.Println("postgres: added http metric")
	return nil
}

func (p *Postgres) AddAggregatedMetric(ctx context.Context, aggregatedMetric *models.AggregatedMetric) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	query :=
		`INSERT INTO aggregated_metrics (name, count, rate, sum, min, max, p50, p95, p99)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := p.conn.Exec(ctx, query,
		aggregatedMetric.Name,
		aggregatedMetric.Count,
		aggregatedMetric.Rate,
		aggregatedMetric.Sum,
		aggregatedMetric.Min,
		aggregatedMetric.Max,
		aggregatedMetric.P50,
		aggregatedMetric.P95,
		aggregatedMetric.P99)
	if err != nil {
		return err
	}

	fmt.Println("postgres: added aggregated metric")
	return nil
}

func (p *Postgres) GetUnprocessedMetrics(ctx context.Context, start, end time.Time) ([]*models.Metric, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	query := `SELECT * FROM metrics WHERE created_at BETWEEN $1 AND $2 AND is_processed = FALSE`
	rows, err := p.conn.Query(ctx, query, start, end)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	metrics := make([]*models.Metric, 0)
	for rows.Next() {
		var metric models.Metric
		var isProcessed bool
		err = rows.Scan(
			&metric.Id,
			&metric.Name,
			&metric.Value,
			&metric.Type,
			&metric.Tags,
			&metric.Timestamp,
			&isProcessed)
		if err != nil {
			return nil, err
		}

		metrics = append(metrics, &metric)
	}

	return metrics, nil
}

func (p *Postgres) GetHttpMetrics(ctx context.Context, start, end time.Time) ([]*models.HttpMetric, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	query := `SELECT * FROM http_metrics WHERE created_at BETWEEN $1 AND $2`
	rows, err := p.conn.Query(ctx, query, start, end)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	metrics := make([]*models.HttpMetric, 0)
	for rows.Next() {
		var metric models.HttpMetric
		err = rows.Scan(
			&metric.Id,
			&metric.Method,
			&metric.Endpoint,
			&metric.Code,
			&metric.Duration,
			&metric.RequestSize,
			&metric.ResponseSize,
			&metric.Timestamp)
		if err != nil {
			return nil, err
		}

		metrics = append(metrics, &metric)
	}

	return metrics, nil
}

func (p *Postgres) GetAggregatedMetrics(ctx context.Context, start, end time.Time) ([]*models.AggregatedMetric, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	query := `SELECT * FROM aggregated_metrics WHERE created_at BETWEEN $1 AND $2`
	rows, err := p.conn.Query(ctx, query, start, end)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	metrics := make([]*models.AggregatedMetric, 0)
	for rows.Next() {
		var metric models.AggregatedMetric
		err = rows.Scan(
			&metric.Id,
			&metric.Name,
			&metric.Count,
			&metric.Rate,
			&metric.Sum,
			&metric.Min,
			&metric.Max,
			&metric.P50,
			&metric.P95,
			&metric.P99,
			&metric.CreatedAt)
		if err != nil {
			return nil, err
		}

		metrics = append(metrics, &metric)
	}

	return metrics, nil
}
