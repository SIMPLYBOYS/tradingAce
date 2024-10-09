CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    address VARCHAR(42) UNIQUE NOT NULL,
    onboarding_completed BOOLEAN DEFAULT FALSE,
    onboarding_points INT DEFAULT 0
);

CREATE TABLE IF NOT EXISTS swap_events (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id),
    transaction_hash VARCHAR(66) NOT NULL,
    amount_usd NUMERIC(20, 2) NOT NULL,
    timestamp TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS points_history (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id),
    points BIGINT NOT NULL, 
    reason VARCHAR(255) NOT NULL,
    timestamp TIMESTAMP NOT NULL
);

-- If the table already exists, alter the column type
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name='points_history' AND column_name='points') THEN
        ALTER TABLE points_history ALTER COLUMN points TYPE BIGINT;
    END IF;
END $$;