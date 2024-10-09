CREATE TABLE IF NOT EXISTS leaderboard (
    address VARCHAR(42) PRIMARY KEY,
    points INT NOT NULL DEFAULT 0
);


-- Create an index on points for faster leaderboard queries
CREATE INDEX idx_leaderboard_points ON leaderboard(points DESC);