CREATE TABLE metrics (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255),
    value FLOAT,
    type INTEGER,
    tags JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now(),
    is_processed BOOLEAN DEFAULT FALSE
);

CREATE TABLE aggregated_metrics (
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
