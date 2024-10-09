package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitDB() error {
	connStr := "host=localhost port=5432 user=user password=password dbname=tradingace sslmode=disable"
	var err error
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %v", err)
	}

	err = DB.Ping()
	if err != nil {
		return fmt.Errorf("failed to ping database: %v", err)
	}

	log.Println("Successfully connected to database")

	// Run migrations
	err = runMigrations(DB)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %v", err)
	}

	return nil
}

func GetUserTasks(address string) (map[string]interface{}, error) {
	var onboardingCompleted bool
	var onboardingPoints int
	err := DB.QueryRow("SELECT onboarding_completed, onboarding_points FROM users WHERE address = $1", address).Scan(&onboardingCompleted, &onboardingPoints)
	if err != nil {
		return nil, err
	}

	var sharePoolAmount float64
	err = DB.QueryRow("SELECT COALESCE(SUM(amount_usd), 0) FROM swap_events WHERE user_id = (SELECT id FROM users WHERE address = $1)", address).Scan(&sharePoolAmount)
	if err != nil {
		return nil, err
	}

	tasks := map[string]interface{}{
		"onboarding": map[string]interface{}{
			"completed": onboardingCompleted,
			"amount":    0, // This should be the amount swapped for onboarding
			"points":    onboardingPoints,
		},
		"sharePool": map[string]interface{}{
			"completed": sharePoolAmount > 0,
			"amount":    sharePoolAmount,
			"points":    0, // This should be calculated based on the user's share of the pool
		},
	}

	return tasks, nil
}

func GetUserPointsHistory(address string) ([]map[string]interface{}, error) {
	rows, err := DB.Query("SELECT points, reason, timestamp FROM points_history WHERE user_id = (SELECT id FROM users WHERE address = $1) ORDER BY timestamp DESC", address)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pointsHistory []map[string]interface{}
	for rows.Next() {
		var points int
		var reason string
		var timestamp string
		err := rows.Scan(&points, &reason, &timestamp)
		if err != nil {
			return nil, err
		}
		pointsHistory = append(pointsHistory, map[string]interface{}{
			"timestamp": timestamp,
			"points":    points,
			"reason":    reason,
		})
	}

	return pointsHistory, nil
}

func RecordSwap(address string, amountUSD float64, txHash string) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert or update user
	var userID int
	err = tx.QueryRow("INSERT INTO users (address) VALUES ($1) ON CONFLICT (address) DO UPDATE SET address = EXCLUDED.address RETURNING id", address).Scan(&userID)
	if err != nil {
		return err
	}

	// Record swap event
	_, err = tx.Exec("INSERT INTO swap_events (user_id, transaction_hash, amount_usd, timestamp) VALUES ($1, $2, $3, $4)",
		userID, txHash, amountUSD, time.Now())
	if err != nil {
		return err
	}

	// Check and update onboarding task
	var onboardingCompleted bool
	err = tx.QueryRow("SELECT onboarding_completed FROM users WHERE id = $1", userID).Scan(&onboardingCompleted)
	if err != nil {
		return err
	}

	if !onboardingCompleted && amountUSD >= 1000 {
		_, err = tx.Exec("UPDATE users SET onboarding_completed = true, onboarding_points = 100 WHERE id = $1", userID)
		if err != nil {
			return err
		}

		// Record points for onboarding task
		_, err = tx.Exec("INSERT INTO points_history (user_id, points, reason, timestamp) VALUES ($1, 100, 'Onboarding task completed', $2)",
			userID, time.Now())
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func CalculateSharePoolPoints() error {
	// This function should be run weekly
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get total swap volume for the week
	var totalVolume float64
	err = tx.QueryRow("SELECT COALESCE(SUM(amount_usd), 0) FROM swap_events WHERE timestamp >= NOW() - INTERVAL '1 week'").Scan(&totalVolume)
	if err != nil {
		return err
	}

	if totalVolume == 0 {
		// No swaps this week, nothing to do
		return nil
	}

	// Calculate and award points for each user
	rows, err := tx.Query(`
        SELECT u.id, u.address, COALESCE(SUM(se.amount_usd), 0) as volume
        FROM users u
        LEFT JOIN swap_events se ON u.id = se.user_id AND se.timestamp >= NOW() - INTERVAL '1 week'
        WHERE u.onboarding_completed = true
        GROUP BY u.id, u.address
    `)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var userID int
		var address string
		var volume float64
		err := rows.Scan(&userID, &address, &volume)
		if err != nil {
			return err
		}

		if volume > 0 {
			points := int((volume / totalVolume) * 10000) // 10000 is the total points pool
			_, err = tx.Exec("INSERT INTO points_history (user_id, points, reason, timestamp) VALUES ($1, $2, 'Share pool task - Weekly', $3)",
				userID, points, time.Now())
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}
