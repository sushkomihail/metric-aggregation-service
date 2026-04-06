CREATE TABLE metrics (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255),
    value FLOAT,
    type VARCHAR(50),
    tags JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    is_processed BOOLEAN DEFAULT FALSE
)