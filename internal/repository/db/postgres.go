package db

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sushkomihail/metric-aggregation-service/internal/config"
	"github.com/sushkomihail/metric-aggregation-service/internal/logger"
	"github.com/sushkomihail/metric-aggregation-service/pkg/models"
)

type DB interface {
	AddMetric(context.Context, *models.Metric) error
	AddHttpMetric(context.Context, *models.HttpMetric) error
	AddAggregatedMetric(context.Context, *models.AggregatedMetric) error
	GetUnprocessedMetrics(context.Context, time.Time, time.Time) ([]*models.Metric, error)
	GetUnprocessedHttpMetrics(context.Context, time.Time, time.Time) ([]*models.HttpMetric, error)
	GetAggregatedMetrics(ctx context.Context, start, end time.Time, metricName string, tags map[string]string) ([]*models.AggregatedMetric, error)
	MarkMetricsAsProcessed(ctx context.Context, ids []int) error
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

func (p *Postgres) AddMetric(ctx context.Context, metric *models.Metric) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	tagsJson, err := json.Marshal(metric.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	query := `
		INSERT INTO metrics (trace_id, name, value, type, tags, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`

	err = p.pool.QueryRow(
		ctx,
		query,
		metric.TraceId,
		metric.Name,
		metric.Value,
		int(metric.Type),
		tagsJson,
		time.Now(),
	).Scan(&metric.Id)

	if err != nil {
		return fmt.Errorf("failed to insert metric: %w", err)
	}

	return nil
}

func (p *Postgres) AddHttpMetric(ctx context.Context, metric *models.HttpMetric) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	query := `
		INSERT INTO http_metrics (trace_id, method, endpoint, code, duration, request_size, response_size, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`

	err := p.pool.QueryRow(
		ctx,
		query,
		metric.TraceId,
		metric.Method,
		metric.Endpoint,
		metric.Code,
		float64(metric.Duration.Milliseconds()),
		metric.RequestSize,
		metric.ResponseSize,
		metric.Timestamp,
	).Scan(&metric.Id)

	if err != nil {
		return fmt.Errorf("failed to insert HTTP metric: %w", err)
	}

	return nil
}

func (p *Postgres) AddAggregatedMetric(ctx context.Context, metric *models.AggregatedMetric) error {
	p.mu.Lock()
	defer p.mu.Unlock()

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

	return nil
}

func (p *Postgres) GetUnprocessedMetrics(ctx context.Context, start, end time.Time) ([]*models.Metric, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	query := `
		SELECT id, trace_id, name, value, type, tags, created_at FROM metrics
        WHERE created_at BETWEEN $1 AND $2 AND is_processed = FALSE
	`

	rows, err := p.pool.Query(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query unprocessed metrics: %w", err)
	}
	defer rows.Close()

	metrics := make([]*models.Metric, 0)
	for rows.Next() {
		var metric models.Metric
		err = rows.Scan(
			&metric.Id,
			&metric.TraceId,
			&metric.Name,
			&metric.Value,
			&metric.Type,
			&metric.Tags,
			&metric.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan metric row: %w", err)
		}

		metrics = append(metrics, &metric)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating metric rows: %w", err)
	}

	return metrics, nil
}

func (p *Postgres) GetUnprocessedHttpMetrics(ctx context.Context, start, end time.Time) ([]*models.HttpMetric, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	query := `SELECT * FROM http_metrics WHERE created_at BETWEEN $1 AND $2 AND is_processed = FALSE`

	rows, err := p.pool.Query(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query HTTP metrics: %w", err)
	}
	defer rows.Close()

	metrics := make([]*models.HttpMetric, 0)
	for rows.Next() {
		var metric models.HttpMetric
		var duration float64

		err = rows.Scan(
			&metric.Id,
			&metric.TraceId,
			&metric.Method,
			&metric.Endpoint,
			&metric.Code,
			&duration,
			&metric.RequestSize,
			&metric.ResponseSize,
			&metric.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan HTTP metric row: %w", err)
		}

		metric.Duration = time.Duration(duration) * time.Millisecond
		metrics = append(metrics, &metric)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating HTTP metric rows: %w", err)
	}

	return metrics, nil
}

func (p *Postgres) GetAggregatedMetrics(
	ctx context.Context,
	start, end time.Time,
	metricName string,
	tags map[string]string,
) ([]*models.AggregatedMetric, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

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

	return metrics, nil
}

func (p *Postgres) MarkMetricsAsProcessed(ctx context.Context, ids []int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(ids) == 0 {
		return nil
	}

	query := `
		UPDATE metrics
		SET is_processed = TRUE
		WHERE id = ANY($1)
	`

	_, err := p.pool.Exec(ctx, query, ids)
	if err != nil {
		return fmt.Errorf("failed to mark metrics as processed: %w", err)
	}

	return nil
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
	interval := pgtype.Interval{
		Microseconds: p.storageTime.Microseconds(),
		Valid:        true,
	}

	err := p.deleteExpiredMetrics(ctx, interval)
	if err != nil {
		return fmt.Errorf("failed to delete expired metrics: %w", err)
	}

	err = p.deleteExpiredHttpMetrics(ctx, interval)
	if err != nil {
		return fmt.Errorf("failed to delete expired http metrics: %w", err)
	}

	err = p.deleteExpiredAggregatedMetrics(ctx, interval)
	if err != nil {
		return fmt.Errorf("failed to delete expired aggregated metrics: %w", err)
	}

	return nil
}

func (p *Postgres) deleteExpiredMetrics(ctx context.Context, interval pgtype.Interval) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	query := `DELETE FROM metrics WHERE created_at < $1 - $2::interval`
	_, err := p.pool.Exec(ctx, query, time.Now(), interval)
	if err != nil {
		return fmt.Errorf("failed to delete expired metrics: %w", err)
	}

	return nil
}

func (p *Postgres) deleteExpiredHttpMetrics(ctx context.Context, interval pgtype.Interval) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	query := `DELETE FROM http_metrics WHERE created_at < $1 - $2::interval`
	_, err := p.pool.Exec(ctx, query, time.Now(), interval)
	if err != nil {
		return fmt.Errorf("failed to delete expired metrics: %w", err)
	}

	return nil
}

func (p *Postgres) deleteExpiredAggregatedMetrics(ctx context.Context, interval pgtype.Interval) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	query := `DELETE FROM aggregated_metrics WHERE created_at < $1 - $2::interval`
	_, err := p.pool.Exec(ctx, query, time.Now(), interval)
	if err != nil {
		return fmt.Errorf("failed to delete expired metrics: %w", err)
	}

	return nil
}
