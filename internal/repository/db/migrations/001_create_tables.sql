CREATE TABLE IF NOT EXISTS metrics (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255),
    value FLOAT,
    type INTEGER,
    tags JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now(),
    is_processed BOOLEAN DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS http_metrics (
    id SERIAL PRIMARY KEY,
    method VARCHAR(255) NOT NULL,
    endpoint VARCHAR(255) NOT NULL,
    code INTEGER NOT NULL,
    duration FLOAT NOT NULL ,
    request_size INTEGER DEFAULT 0,
    response_size INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now()
);

CREATE TABLE IF NOT EXISTS aggregated_metrics (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255),
    count INTEGER,
    rate FLOAT,
    sum FLOAT,
    min FLOAT,
    max FLOAT,
    p50 FLOAT,
    p95 FLOAT,
    p99 FLOAT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now()
);
