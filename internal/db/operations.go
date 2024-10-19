package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/SIMPLYBOYS/trading_ace/internal/errors"
	"github.com/SIMPLYBOYS/trading_ace/pkg/logger"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

// Error handling in InitDB
func InitDB() error {
	connStr := "host=localhost port=5432 user=user password=password dbname=tradingace sslmode=disable"
	var err error
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		return &errors.DatabaseError{Operation: "open connection", Err: err}
	}

	err = DB.Ping()
	if err != nil {
		return &errors.DatabaseError{Operation: "ping database", Err: err}
	}

	return RunMigrations(DB)
}

// GetUserTasks retrieves the tasks status for a given user
func GetUserTasks(address string) (map[string]interface{}, error) {
	var user User
	err := DB.QueryRow(`
        SELECT id, onboarding_completed, onboarding_points, 
               COALESCE((SELECT SUM(amount_usd) FROM swap_events WHERE user_id = users.id), 0) as total_swap_amount
        FROM users 
        WHERE address = $1`, address).Scan(&user.ID, &user.OnboardingCompleted, &user.OnboardingPoints, &user.TotalPoints)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, &errors.NotFoundError{Resource: "user", Identifier: address}
		}
		return nil, &errors.DatabaseError{Operation: "get user tasks", Err: err}
	}
	var sharePoolAmount, sharePoolPoints float64
	err = DB.QueryRow(`
        SELECT COALESCE(SUM(amount_usd), 0),
               COALESCE((SELECT SUM(points) FROM points_history WHERE user_id = $1 AND reason = 'Weekly Share Pool Task'), 0)
        FROM swap_events 
        WHERE user_id = $1`, user.ID).Scan(&sharePoolAmount, &sharePoolPoints)
	if err != nil {
		return nil, &errors.DatabaseError{Operation: "failed to get share pool info", Err: err}
	}

	campaign, err := GetCampaignConfig()
	if err != nil {
		return nil, &errors.DatabaseError{Operation: "failed to get campaign config", Err: err}
	}

	var latestDistribution time.Time
	err = DB.QueryRow(`
        SELECT COALESCE(MAX(timestamp), $1)
        FROM points_history
        WHERE user_id = $2 AND reason = 'Weekly Share Pool Task'`, campaign.StartTime, user.ID).Scan(&latestDistribution)
	if err != nil {
		return nil, &errors.DatabaseError{Operation: "failed to get latest distribution", Err: err}
	}

	isEligibleForCurrentDistribution := latestDistribution.Before(time.Now().AddDate(0, 0, -7))

	tasks := map[string]interface{}{
		"onboarding": map[string]interface{}{
			"completed": user.OnboardingCompleted,
			"amount":    user.TotalPoints,
			"points":    user.OnboardingPoints,
		},
		"sharePool": map[string]interface{}{
			"completed": sharePoolAmount > 0,
			"amount":    sharePoolAmount,
			"points":    sharePoolPoints,
			"eligible":  isEligibleForCurrentDistribution,
		},
		"campaign": map[string]interface{}{
			"startTime": campaign.StartTime,
			"endTime":   campaign.EndTime,
			"isActive":  campaign.IsActive,
		},
	}

	return tasks, nil
}

// GetUserPointsHistory retrieves the points history for a given user
func GetUserPointsHistory(address string) ([]PointsHistory, error) {
	rows, err := DB.Query(`
        SELECT points, reason, timestamp 
        FROM points_history 
        WHERE user_id = (SELECT id FROM users WHERE address = $1) 
        ORDER BY timestamp DESC`, address)
	if err != nil {
		return nil, fmt.Errorf("failed to get user points history: %w", err)
	}
	defer rows.Close()

	var history []PointsHistory
	for rows.Next() {
		var ph PointsHistory
		err := rows.Scan(&ph.Points, &ph.Reason, &ph.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("failed to scan points history: %w", err)
		}
		history = append(history, ph)
	}

	return history, nil
}

// RecordSwapAndUpdatePoints records a swap event, updates user points, and updates the leaderboard
func (s *DBServiceImpl) RecordSwapAndUpdatePoints(address string, usdValue float64, points int64, txHash string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Record swap event
	_, err = tx.Exec(`
		INSERT INTO swap_events (sender, amount_usd, transaction_hash, timestamp)
		VALUES ($1, $2, $3, NOW())
	`, address, usdValue, txHash)
	if err != nil {
		return fmt.Errorf("failed to record swap event: %w", err)
	}

	// Update user points
	_, err = tx.Exec(`
		INSERT INTO users (address, total_points)
		VALUES ($1, $2)
		ON CONFLICT (address) DO UPDATE
		SET total_points = users.total_points + $2
	`, address, points)
	if err != nil {
		return fmt.Errorf("failed to update user points: %w", err)
	}

	// Record points history
	_, err = tx.Exec(`
		INSERT INTO points_history (user_id, points, reason, timestamp)
		SELECT id, $2, 'Swap', NOW()
		FROM users
		WHERE address = $1
	`, address, points)
	if err != nil {
		return fmt.Errorf("failed to record points history: %w", err)
	}

	// Update leaderboard
	_, err = tx.Exec(`
		INSERT INTO leaderboard (address, points)
		VALUES ($1, $2)
		ON CONFLICT (address) DO UPDATE
		SET points = leaderboard.points + $2
	`, address, points)
	if err != nil {
		return fmt.Errorf("failed to update leaderboard: %w", err)
	}

	return tx.Commit()
}

// GetLeaderboard retrieves the current leaderboard
func (s *DBServiceImpl) GetLeaderboard(limit int) ([]LeaderboardEntry, error) {
	rows, err := s.db.Query("SELECT address, points FROM leaderboard ORDER BY points DESC LIMIT $1", limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query leaderboard: %w", err)
	}
	defer rows.Close()

	var leaderboard []LeaderboardEntry
	for rows.Next() {
		var entry LeaderboardEntry
		if err := rows.Scan(&entry.Address, &entry.Points); err != nil {
			return nil, fmt.Errorf("failed to scan leaderboard entry: %w", err)
		}
		leaderboard = append(leaderboard, entry)
	}

	return leaderboard, nil
}

// GetCampaignConfig retrieves the current campaign configuration
func GetCampaignConfig() (CampaignConfig, error) {
	var config CampaignConfig
	err := DB.QueryRow("SELECT id, start_time, end_time, is_active FROM campaign_config ORDER BY id DESC LIMIT 1").
		Scan(&config.ID, &config.StartTime, &config.EndTime, &config.IsActive)
	if err != nil {
		if err == sql.ErrNoRows {
			return CampaignConfig{}, &errors.NotFoundError{Resource: "campaign config", Identifier: "latest"}
		}
		return CampaignConfig{}, &errors.DatabaseError{Operation: "get campaign config", Err: err}
	}
	return config, nil
}

// SetCampaignConfig sets a new campaign configuration
func SetCampaignConfig(startTime time.Time) error {
	endTime := startTime.Add(4 * 7 * 24 * time.Hour) // 4 weeks
	_, err := DB.Exec("INSERT INTO campaign_config (start_time, end_time, is_active) VALUES ($1, $2, $3)",
		startTime, endTime, true)
	if err != nil {
		return &errors.DatabaseError{Operation: "failed to set campaign config", Err: err}
	}
	return nil
}

// EndCampaign marks a campaign as inactive
func EndCampaign(campaignID int) error {
	_, err := DB.Exec("UPDATE campaign_config SET is_active = false WHERE id = $1", campaignID)
	if err != nil {
		return &errors.DatabaseError{Operation: "failed to end campaign", Err: err}
	}
	return nil
}

// CalculateWeeklySharePoolPoints calculates and distributes weekly share pool points
func CalculateWeeklySharePoolPoints() error {
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get total swap volume for the week
	var totalVolume float64
	err = tx.QueryRow(`
        SELECT COALESCE(SUM(amount_usd), 0)
        FROM swap_events
        WHERE timestamp >= NOW() - INTERVAL '7 days'`).Scan(&totalVolume)
	if err != nil {
		return &errors.DatabaseError{Operation: "failed to get total volume", Err: err}
	}

	if totalVolume == 0 {
		logger.Info("No swaps this week, skipping point distribution")
		return nil
	}

	// Get all eligible users and their volumes
	rows, err := tx.Query(`
        SELECT u.id, u.address, COALESCE(SUM(se.amount_usd), 0) as volume
        FROM users u
        LEFT JOIN swap_events se ON u.id = se.user_id AND se.timestamp >= NOW() - INTERVAL '7 days'
        WHERE u.onboarding_completed = true
        GROUP BY u.id, u.address
        HAVING COALESCE(SUM(se.amount_usd), 0) > 0
        ORDER BY volume DESC`)
	if err != nil {
		return &errors.DatabaseError{Operation: "failed to query user volumes", Err: err}
	}
	defer rows.Close()

	totalPoints := 10000
	for rows.Next() {
		var userID int
		var address string
		var volume float64
		err := rows.Scan(&userID, &address, &volume)
		if err != nil {
			return &errors.DatabaseError{Operation: "failed to scan user data", Err: err}
		}

		points := int64(float64(totalPoints) * (volume / totalVolume))
		if points == 0 {
			points = 1 // Minimum 1 point
		}

		_, err = tx.Exec(`
            INSERT INTO points_history (user_id, points, reason, timestamp)
            VALUES ($1, $2, $3, $4)`, userID, points, "Weekly Share Pool Task", time.Now())
		if err != nil {
			return &errors.DatabaseError{Operation: "failed to insert points history", Err: err}
		}

		_, err = tx.Exec(`
            UPDATE users SET total_points = total_points + $1 WHERE id = $2`, points, userID)
		if err != nil {
			return &errors.DatabaseError{Operation: "failed to update user points", Err: err}
		}

		err = UpdateLeaderboard(address, points)
		if err != nil {
			return &errors.DatabaseError{Operation: "failed to update leaderboard", Err: err}
		}

		logger.Info("Awarded %d points to user %s for Weekly Share Pool Task", points, address)
	}

	return tx.Commit()
}

// UpdateLeaderboard updates the leaderboard with new points
func UpdateLeaderboard(address string, points int64) error {
	_, err := DB.Exec(`
        INSERT INTO leaderboard (address, points) 
        VALUES ($1, $2) 
        ON CONFLICT (address) DO UPDATE 
        SET points = leaderboard.points + $2`, address, points)
	if err != nil {
		return &errors.DatabaseError{Operation: "failed to update leaderboard", Err: err}
		return fmt.Errorf("failed to update leaderboard: %w", err)
	}
	return nil
}

// GetLeaderboard retrieves the current leaderboard
func GetLeaderboard(limit int) ([]LeaderboardEntry, error) {
	rows, err := DB.Query("SELECT address, points FROM leaderboard ORDER BY points DESC LIMIT $1", limit)
	if err != nil {
		return nil, &errors.DatabaseError{Operation: "failed to query leaderboard", Err: err}
	}
	defer rows.Close()

	var leaderboard []LeaderboardEntry
	for rows.Next() {
		var entry LeaderboardEntry
		if err := rows.Scan(&entry.Address, &entry.Points); err != nil {
			return nil, &errors.DatabaseError{Operation: "failed to scan leaderboard entry", Err: err}
		}
		leaderboard = append(leaderboard, entry)
	}

	return leaderboard, nil
}

// GetUserByAddress retrieves a user by their address
func GetUserByAddress(address string) (User, error) {
	var user User
	err := DB.QueryRow("SELECT id, address, onboarding_completed, onboarding_points, total_points FROM users WHERE address = $1", address).
		Scan(&user.ID, &user.Address, &user.OnboardingCompleted, &user.OnboardingPoints, &user.TotalPoints)
	if err != nil {
		return User{}, &errors.DatabaseError{Operation: "failed to get user by address", Err: err}
	}
	return user, nil
}

// CreateUser creates a new user
func CreateUser(address string) (User, error) {
	var user User
	err := DB.QueryRow("INSERT INTO users (address) VALUES ($1) RETURNING id, address, onboarding_completed, onboarding_points, total_points", address).
		Scan(&user.ID, &user.Address, &user.OnboardingCompleted, &user.OnboardingPoints, &user.TotalPoints)
	if err != nil {
		return User{}, &errors.DatabaseError{Operation: "failed to create user", Err: err}
	}
	return user, nil
}

// GetUserSwaps retrieves all swap events for a user
func GetUserSwaps(userID int) ([]SwapEvent, error) {
	rows, err := DB.Query("SELECT id, user_id, transaction_hash, amount_usd, timestamp FROM swap_events WHERE user_id = $1 ORDER BY timestamp DESC", userID)
	if err != nil {
		return nil, &errors.DatabaseError{Operation: "failed to query user swaps", Err: err}
	}
	defer rows.Close()

	var swaps []SwapEvent
	for rows.Next() {
		var swap SwapEvent
		if err := rows.Scan(&swap.ID, &swap.UserID, &swap.TransactionHash, &swap.AmountUSD, &swap.Timestamp); err != nil {
			return nil, &errors.DatabaseError{Operation: "failed to scan swap event", Err: err}
		}
		swaps = append(swaps, swap)
	}

	return swaps, nil
}

// RunMigrations runs the database migrations
func RunMigrations(db *sql.DB) error {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return &errors.DatabaseError{Operation: "could not create the postgres driver", Err: err}
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations", // This should be the path to your migration files
		"postgres", driver)
	if err != nil {
		return &errors.DatabaseError{Operation: "could not create migrate instance", Err: err}
	}

	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return &errors.DatabaseError{Operation: "an error occurred while syncing the database", Err: err}
	}

	logger.Info("Database migrations completed successfully")
	return nil
}
