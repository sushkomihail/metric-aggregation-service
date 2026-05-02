CREATE TABLE IF NOT EXISTS aggregated_http_metrics (
    id SERIAL PRIMARY KEY,
    method VARCHAR(255),
    endpoint VARCHAR(255),
    duration_metric_id INTEGER NOT NULL,
    request_size_metric_id INTEGER NOT NULL,
    response_size_metric_id INTEGER NOT NULL,
    errors_count INTEGER NOT NULL
)