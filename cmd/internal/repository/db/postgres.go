package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/sushkomihail/metric-aggregation-service/cmd/internal/models"
)

type DB interface {
	AddMetric(context.Context, *models.Metric) error
}

type Postgres struct {
	conn *pgx.Conn
	mu   sync.RWMutex
}

func NewPostgres(ctx context.Context) *Postgres {
	// TODO: make the port a constant
	url := fmt.Sprintf("postgres://%s:%s@%s:5432/%s",
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_ADDR"),
		os.Getenv("POSTGRES_DB"))
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

	// TODO: add all metric params
	query := `INSERT INTO metrics (name, value) VALUES ($1, $2)`
	_, err := p.conn.Exec(ctx, query, metric.Name, metric.Value)
	if err != nil {
		return err
	}

	fmt.Println("added metric")
	return nil
}
