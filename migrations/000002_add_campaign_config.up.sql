CREATE TABLE IF NOT EXISTS campaign_config (
    id SERIAL PRIMARY KEY,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);

-- Insert a default campaign configuration
INSERT INTO campaign_config (start_time, end_time, is_active)
VALUES (NOW(), NOW() + INTERVAL '4 weeks', TRUE);