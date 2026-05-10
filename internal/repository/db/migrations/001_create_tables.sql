CREATE TABLE IF NOT EXISTS metrics (
    id SERIAL PRIMARY KEY,
    trace_id VARCHAR(36) NOT NULL,
    name VARCHAR(255) NOT NULL,
    value FLOAT,
    type INTEGER NOT NULL CHECK (type IN (0, 1, 2, 3)),
    tags JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now(),
    is_processed BOOLEAN DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_metrics_unprocessed ON metrics(is_processed, created_at) WHERE is_processed = FALSE;

CREATE TABLE IF NOT EXISTS http_metrics (
    id SERIAL PRIMARY KEY,
    trace_id VARCHAR(36) NOT NULL,
    method VARCHAR(10) NOT NULL,
    endpoint VARCHAR(500) NOT NULL,
    code INTEGER NOT NULL,
    duration FLOAT NOT NULL ,
    request_size INTEGER DEFAULT 0,
    response_size INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now(),
    is_processed BOOLEAN DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_http_metrics_unprocessed ON http_metrics(is_processed, created_at)
    WHERE is_processed = FALSE;

CREATE TABLE IF NOT EXISTS aggregated_metrics (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    count INTEGER NOT NULL CHECK (count >= 0),
    sum FLOAT NOT NULL,
    min FLOAT NOT NULL,
    max FLOAT NOT NULL,
    p50 FLOAT NOT NULL,
    p95 FLOAT NOT NULL,
    p99 FLOAT NOT NULL,
    source VARCHAR(10) NOT NULL CHECK (source IN ('grpc', 'http')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now()
);
