// File: internal/db/service.go

package db

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/SIMPLYBOYS/trading_ace/pkg/logger"
	_ "github.com/lib/pq" // PostgreSQL driver
)

// DBServiceImpl implements the DBService interface
type DBServiceImpl struct {
	db *sql.DB
}

type DBOperations interface {
	Open(driverName, dataSourceName string) (*sql.DB, error)
	RunMigrations(db *sql.DB) error
}

// NewDBService creates and returns a new DBService
func NewDBService(ops DBOperations) (DBService, error) {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := ops.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Run migrations
	if err := ops.RunMigrations(db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &DBServiceImpl{db: db}, nil
}

// GetUserPointsHistory retrieves the points history for a given user
func (s *DBServiceImpl) GetUserPointsHistory(address string) ([]PointsHistory, error) {
	// First, get the user ID
	var userID int
	err := s.db.QueryRow("SELECT id FROM users WHERE address = $1", address).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Info("No user found for address: %s", address)
			return nil, nil
		}
		return nil, fmt.Errorf("error fetching user ID: %w", err)
	}

	// Now fetch the points history
	rows, err := s.db.Query(`
		SELECT points, reason, timestamp 
		FROM points_history 
		WHERE user_id = $1 
		ORDER BY timestamp DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching points history: %w", err)
	}
	defer rows.Close()

	var history []PointsHistory
	for rows.Next() {
		var ph PointsHistory
		err := rows.Scan(&ph.Points, &ph.Reason, &ph.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("error scanning points history row: %w", err)
		}
		history = append(history, ph)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating points history rows: %w", err)
	}

	return history, nil
}

func (s *DBServiceImpl) GetCampaignConfig() (CampaignConfig, error) {
	query := `
		SELECT id, start_time, end_time, is_active
		FROM campaign_config
		ORDER BY id DESC
		LIMIT 1
	`
	var cc CampaignConfig
	err := s.db.QueryRow(query).Scan(&cc.ID, &cc.StartTime, &cc.EndTime, &cc.IsActive)
	if err != nil {
		return CampaignConfig{}, fmt.Errorf("failed to get campaign config: %w", err)
	}
	return cc, nil
}

func (s *DBServiceImpl) EndCampaign(campaignID int) error {
	_, err := s.db.Exec(`
		UPDATE campaign_config
		SET is_active = false
		WHERE id = $1
	`, campaignID)
	if err != nil {
		return fmt.Errorf("failed to end campaign: %w", err)
	}
	return nil
}

func (s *DBServiceImpl) CalculateWeeklySharePoolPoints() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var totalVolume float64
	err = tx.QueryRow(`
		SELECT COALESCE(SUM(usd_value), 0)
		FROM swap_events
		WHERE timestamp >= NOW() - INTERVAL '7 days'
	`).Scan(&totalVolume)
	if err != nil {
		return fmt.Errorf("failed to get total swap volume: %w", err)
	}

	if totalVolume == 0 {
		return nil // No swaps this week
	}

	rows, err := tx.Query(`
		SELECT user_address, SUM(usd_value) as user_volume
		FROM swap_events
		WHERE timestamp >= NOW() - INTERVAL '7 days'
		GROUP BY user_address
	`)
	if err != nil {
		return fmt.Errorf("failed to query user volumes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var address string
		var userVolume float64
		if err := rows.Scan(&address, &userVolume); err != nil {
			return fmt.Errorf("failed to scan user volume row: %w", err)
		}

		points := int64((userVolume / totalVolume) * 10000) // 10000 total points to distribute
		if points == 0 {
			points = 1 // Minimum 1 point
		}

		// Update user points
		_, err = tx.Exec(`
			UPDATE users
			SET total_points = total_points + $2
			WHERE address = $1
		`, address, points)
		if err != nil {
			return fmt.Errorf("failed to update user points: %w", err)
		}

		// Record points history
		_, err = tx.Exec(`
			INSERT INTO points_history (user_address, points, reason)
			VALUES ($1, $2, $3)
		`, address, points, "Weekly Share Pool")
		if err != nil {
			return fmt.Errorf("failed to record points history: %w", err)
		}
	}

	return tx.Commit()
}

// UpdateLeaderboard updates the leaderboard with new points
func (s *DBServiceImpl) UpdateLeaderboard(address string, points int64) error {
	_, err := s.db.Exec(`
		INSERT INTO leaderboard (address, points) 
		VALUES ($1, $2) 
		ON CONFLICT (address) DO UPDATE 
		SET points = leaderboard.points + $2`, address, points)
	if err != nil {
		return fmt.Errorf("failed to update leaderboard: %w", err)
	}
	return nil
}

func (s *DBServiceImpl) Close() error {
	return s.db.Close()
}
