-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    address VARCHAR(42) UNIQUE NOT NULL,
    onboarding_completed BOOLEAN DEFAULT FALSE,
    onboarding_points BIGINT DEFAULT 0,
    total_points BIGINT DEFAULT 0
);

-- Create swap_events table
CREATE TABLE IF NOT EXISTS swap_events (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id),
    transaction_hash VARCHAR(66) NOT NULL,
    amount_usd NUMERIC(20, 2) NOT NULL,
    timestamp TIMESTAMP NOT NULL
);

-- Create points_history table
CREATE TABLE IF NOT EXISTS points_history (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id),
    points BIGINT NOT NULL,
    reason VARCHAR(255) NOT NULL,
    timestamp TIMESTAMP NOT NULL
);

-- Create campaign_config table
CREATE TABLE IF NOT EXISTS campaign_config (
    id SERIAL PRIMARY KEY,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);

-- Create leaderboard table
CREATE TABLE IF NOT EXISTS leaderboard (
    address VARCHAR(42) PRIMARY KEY,
    points BIGINT NOT NULL DEFAULT 0
);

-- Create indexes
CREATE INDEX idx_swap_events_user_id ON swap_events(user_id);
CREATE INDEX idx_points_history_user_id ON points_history(user_id);
CREATE INDEX idx_leaderboard_points ON leaderboard(points DESC);

-- Insert default campaign configuration
INSERT INTO campaign_config (start_time, end_time, is_active)
VALUES (NOW(), NOW() + INTERVAL '4 weeks', TRUE);