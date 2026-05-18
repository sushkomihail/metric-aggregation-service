package db

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sushkomihail/metric-aggregation-service/internal/config"
	"github.com/sushkomihail/metric-aggregation-service/internal/logger"
	prommetrics "github.com/sushkomihail/metric-aggregation-service/pkg/metrics"
	"github.com/sushkomihail/metric-aggregation-service/pkg/models"
)

type DB interface {
	AddAggregatedMetric(context.Context, *models.AggregatedMetric) error
	GetAggregatedMetrics(ctx context.Context, start, end time.Time, metricName string, tags map[string]string) ([]*models.AggregatedMetric, error)
}

type Postgres struct {
	pool          *pgxpool.Pool
	mu            sync.RWMutex
	flushInterval time.Duration
	storageTime   time.Duration
	log           *logger.Logger
}

func NewPostgres(ctx context.Context, config config.PostgresConfig, log *logger.Logger) *Postgres {
	url := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		config.User,
		config.Password,
		config.Addr,
		config.Port,
		config.DB,
	)

	poolConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		panic(fmt.Sprintf("unable to parse database url: %v", err))
	}

	poolConfig.MaxConns = 20
	poolConfig.MinConns = 5
	poolConfig.MaxConnLifetime = 1 * time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		panic(fmt.Sprintf("unable to create connection pool: %v", err))
	}

	if err = pool.Ping(ctx); err != nil {
		panic(fmt.Sprintf("unable to ping database: %v", err))
	}

	return &Postgres{
		pool:          pool,
		flushInterval: config.FlushInterval,
		storageTime:   config.StorageTime,
		log:           log,
	}
}

func (p *Postgres) CloseConnection() {
	p.pool.Close()
}

func (p *Postgres) AddAggregatedMetric(ctx context.Context, metric *models.AggregatedMetric) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()

	query := `
		INSERT INTO aggregated_metrics (name, count, sum, min, max, p50, p95, p99, source, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`

	err := p.pool.QueryRow(
		ctx,
		query,
		metric.Name,
		metric.Count,
		metric.Sum,
		metric.Min,
		metric.Max,
		metric.P50,
		metric.P95,
		metric.P99,
		metric.Source,
		metric.CreatedAt,
	).Scan(&metric.Id)

	if err != nil {
		return fmt.Errorf("failed to insert aggregated metric: %w", err)
	}

	duration := time.Since(start)

	prommetrics.IncDatabaseOperationsTotal()
	prommetrics.ObserveDatabaseOperationDuration(duration)

	return nil
}

func (p *Postgres) GetAggregatedMetrics(
	ctx context.Context,
	start, end time.Time,
	metricName string,
	tags map[string]string,
) ([]*models.AggregatedMetric, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	operationStart := time.Now()

	query := `
		SELECT id, name, count, sum, min, max, p50, p95, p99, source, created_at
		FROM aggregated_metrics
		WHERE created_at BETWEEN $1 AND $2
	`
	args := []interface{}{start, end}
	argCount := 2

	if metricName != "" {
		argCount++
		query += fmt.Sprintf(" AND name = $%d", argCount)
		args = append(args, metricName)
	}

	if len(tags) > 0 {
		for key, value := range tags {
			argCount++
			query += fmt.Sprintf(" AND name LIKE $%d", argCount)
			args = append(args, fmt.Sprintf("%%:%s=%s%%", key, value))
		}
	}

	query += " ORDER BY created_at DESC LIMIT 1000"

	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query aggregated metrics: %w", err)
	}
	defer rows.Close()

	metrics := make([]*models.AggregatedMetric, 0)
	for rows.Next() {
		var metric models.AggregatedMetric
		err = rows.Scan(
			&metric.Id,
			&metric.Name,
			&metric.Count,
			&metric.Sum,
			&metric.Min,
			&metric.Max,
			&metric.P50,
			&metric.P95,
			&metric.P99,
			&metric.Source,
			&metric.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan aggregated metric row: %w", err)
		}

		metrics = append(metrics, &metric)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating aggregated metric rows: %w", err)
	}

	duration := time.Since(operationStart)

	prommetrics.IncDatabaseOperationsTotal()
	prommetrics.ObserveDatabaseOperationDuration(duration)

	return metrics, nil
}

func (p *Postgres) FlushWithInterval(ctx context.Context) {
	timer := time.NewTimer(p.flushInterval)
	defer timer.Stop()

	for {
		timer.Reset(p.flushInterval)

		select {
		case <-ctx.Done():
			p.log.Info("Postgres flushing stopped by context")
			return
		case <-timer.C:
			if err := p.flush(ctx); err != nil {
				p.log.Error("Error flushing metrics", "error", err)
				continue
			}

			p.log.Info("Postgres flushed successfully")
		}
	}
}

func (p *Postgres) flush(ctx context.Context) error {
	deleteBefore := time.Now().Add(-p.storageTime)

	err := p.deleteExpiredAggregatedMetrics(ctx, deleteBefore)
	if err != nil {
		return fmt.Errorf("failed to delete expired aggregated metrics: %w", err)
	}

	return nil
}

func (p *Postgres) deleteExpiredAggregatedMetrics(ctx context.Context, deleteBefore time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()

	query := `DELETE FROM aggregated_metrics WHERE created_at < $1`
	_, err := p.pool.Exec(ctx, query, deleteBefore)
	if err != nil {
		return fmt.Errorf("failed to delete expired metrics: %w", err)
	}

	duration := time.Since(start)

	prommetrics.IncDatabaseOperationsTotal()
	prommetrics.ObserveDatabaseOperationDuration(duration)

	return nil
}
