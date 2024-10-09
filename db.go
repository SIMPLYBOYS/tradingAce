package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

var DB *sql.DB

type CampaignConfig struct {
	ID        int
	StartTime time.Time
	EndTime   time.Time
	IsActive  bool
}

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
	var user struct {
		ID                  int
		OnboardingCompleted bool
		OnboardingPoints    int
		OnboardingAmount    float64
	}
	err := DB.QueryRow(`
        SELECT id, onboarding_completed, onboarding_points, 
               COALESCE((SELECT amount_usd FROM swap_events WHERE user_id = users.id ORDER BY timestamp ASC LIMIT 1), 0) as onboarding_amount
        FROM users 
        WHERE address = $1`, address).Scan(&user.ID, &user.OnboardingCompleted, &user.OnboardingPoints, &user.OnboardingAmount)
	if err != nil {
		return nil, err
	}

	var sharePoolAmount, sharePoolPoints float64
	err = DB.QueryRow(`
        SELECT COALESCE(SUM(amount_usd), 0),
               COALESCE((SELECT SUM(points) FROM points_history WHERE user_id = $1 AND reason = 'Weekly Share Pool Task'), 0)
        FROM swap_events 
        WHERE user_id = $1`, user.ID).Scan(&sharePoolAmount, &sharePoolPoints)
	if err != nil {
		return nil, err
	}

	// Get the latest campaign config
	campaignConfig, err := GetCampaignConfig()
	if err != nil {
		return nil, err
	}

	// Check if the user is eligible for the current share pool distribution
	var latestDistribution time.Time
	err = DB.QueryRow(`
        SELECT COALESCE(MAX(timestamp), $1)
        FROM points_history
        WHERE user_id = $2 AND reason = 'Weekly Share Pool Task'`, campaignConfig.StartTime, user.ID).Scan(&latestDistribution)
	if err != nil {
		return nil, err
	}

	isEligibleForCurrentDistribution := latestDistribution.Before(time.Now().AddDate(0, 0, -7))

	tasks := map[string]interface{}{
		"onboarding": map[string]interface{}{
			"completed": user.OnboardingCompleted,
			"amount":    user.OnboardingAmount,
			"points":    user.OnboardingPoints,
		},
		"sharePool": map[string]interface{}{
			"completed": sharePoolAmount > 0,
			"amount":    sharePoolAmount,
			"points":    sharePoolPoints,
			"eligible":  isEligibleForCurrentDistribution,
		},
		"campaign": map[string]interface{}{
			"startTime": campaignConfig.StartTime,
			"endTime":   campaignConfig.EndTime,
			"isActive":  campaignConfig.IsActive,
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
	config, err := GetCampaignConfig()
	if err != nil {
		return LogErrorf(err, "failed to get campaign config")
	}

	now := time.Now()
	if !config.IsActive || now.Before(config.StartTime) || now.After(config.EndTime) {
		return nil // Silently ignore swaps outside the campaign timeframe
	}

	var userID int
	err = DB.QueryRow("INSERT INTO users (address) VALUES ($1) ON CONFLICT (address) DO UPDATE SET address = EXCLUDED.address RETURNING id", address).Scan(&userID)
	if err != nil {
		return LogErrorf(err, "failed to insert or get user")
	}

	tx, err := DB.Begin()
	if err != nil {
		return LogErrorf(err, "failed to begin transaction")
	}
	defer tx.Rollback()

	_, err = tx.Exec("INSERT INTO swap_events (user_id, transaction_hash, amount_usd, timestamp) VALUES ($1, $2, $3, $4)",
		userID, txHash, amountUSD, now)
	if err != nil {
		return LogErrorf(err, "failed to insert swap event")
	}

	if amountUSD >= 1000 {
		var onboardingCompleted bool
		err = tx.QueryRow("SELECT onboarding_completed FROM users WHERE id = $1", userID).Scan(&onboardingCompleted)
		if err != nil {
			return LogErrorf(err, "failed to check onboarding status")
		}

		if !onboardingCompleted {
			_, err = tx.Exec("UPDATE users SET onboarding_completed = true, onboarding_points = 100 WHERE id = $1", userID)
			if err != nil {
				return LogErrorf(err, "failed to update onboarding status")
			}

			_, err = tx.Exec("INSERT INTO points_history (user_id, points, reason, timestamp) VALUES ($1, 100, 'Onboarding task completed', $2)",
				userID, now)
			if err != nil {
				return LogErrorf(err, "failed to insert onboarding points history")
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		return LogErrorf(err, "failed to commit transaction")
	}

	return nil
}

func CalculateWeeklySharePoolPoints() error {
	config, err := GetCampaignConfig()
	if err != nil {
		return fmt.Errorf("failed to get campaign config: %v", err)
	}

	now := time.Now()
	if !config.IsActive || now.Before(config.StartTime) || now.After(config.EndTime) {
		log.Println("Campaign is not active or has ended, skipping point distribution")
		return nil
	}

	// Check if this is the last week of the campaign
	isLastWeek := now.Add(7 * 24 * time.Hour).After(config.EndTime)

	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Get the total swap volume for the week
	var totalVolume float64
	err = tx.QueryRow(`
        SELECT COALESCE(SUM(amount_usd), 0)
        FROM swap_events
        WHERE timestamp >= $1 AND timestamp < $2
    `, now.Add(-7*24*time.Hour), now).Scan(&totalVolume)
	if err != nil {
		return fmt.Errorf("failed to get total volume: %v", err)
	}

	if totalVolume == 0 {
		log.Println("No swaps this week, skipping point distribution")
		return nil
	}

	// Fetch all eligible users and their volumes
	rows, err := tx.Query(`
        SELECT u.id, u.address, COALESCE(SUM(se.amount_usd), 0) as volume
        FROM users u
        LEFT JOIN swap_events se ON u.id = se.user_id AND se.timestamp >= $1 AND se.timestamp < $2
        WHERE u.onboarding_completed = true
        GROUP BY u.id, u.address
        HAVING COALESCE(SUM(se.amount_usd), 0) > 0
        ORDER BY volume DESC
    `, now.Add(-7*24*time.Hour), now)
	if err != nil {
		return fmt.Errorf("failed to query user volumes: %v", err)
	}
	defer rows.Close()

	type UserData struct {
		ID      int
		Address string
		Volume  float64
	}

	var users []UserData
	for rows.Next() {
		var user UserData
		if err := rows.Scan(&user.ID, &user.Address, &user.Volume); err != nil {
			return fmt.Errorf("failed to scan user data: %v", err)
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating over user rows: %v", err)
	}

	totalPoints := 10000
	remainingPoints := totalPoints

	// Distribute points
	for i, user := range users {
		var points int
		if i == len(users)-1 {
			// Last user gets all remaining points
			points = remainingPoints
		} else {
			points = int((user.Volume / totalVolume) * float64(totalPoints))
			if points == 0 {
				points = 1 // Ensure users with very small volume get at least 1 point
			}
		}
		remainingPoints -= points

		_, err = tx.Exec(`
            INSERT INTO points_history (user_id, points, reason, timestamp)
            VALUES ($1, $2, $3, $4)
        `, user.ID, points, "Weekly Share Pool Task", now)
		if err != nil {
			return fmt.Errorf("failed to insert points history for user %s: %v", user.Address, err)
		}

		log.Printf("Awarded %d points to user %s for Weekly Share Pool Task", points, user.Address)
	}

	if isLastWeek {
		_, err = tx.Exec("UPDATE campaign_config SET is_active = false WHERE id = $1", config.ID)
		if err != nil {
			return fmt.Errorf("failed to deactivate campaign: %v", err)
		}
		log.Println("Campaign has ended. Deactivated in the database.")
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	log.Printf("Weekly share pool points calculated and distributed. Total points: %d, Users rewarded: %d", totalPoints, len(users))
	return nil
}
func GetCampaignConfig() (CampaignConfig, error) {
	var config CampaignConfig
	err := DB.QueryRow("SELECT id, start_time, end_time, is_active FROM campaign_config ORDER BY id DESC LIMIT 1").
		Scan(&config.ID, &config.StartTime, &config.EndTime, &config.IsActive)
	if err != nil {
		return CampaignConfig{}, fmt.Errorf("failed to get campaign config: %v", err)
	}
	return config, nil
}

func SetCampaignConfig(startTime time.Time) error {
	endTime := startTime.Add(4 * 7 * 24 * time.Hour) // 4 weeks
	_, err := DB.Exec("INSERT INTO campaign_config (start_time, end_time, is_active) VALUES ($1, $2, $3)",
		startTime, endTime, true)
	if err != nil {
		return fmt.Errorf("failed to set campaign config: %v", err)
	}
	return nil
}

func AwardOnboardingPoints(userID int) error {
	_, err := DB.Exec(`
        UPDATE users SET onboarding_completed = true, onboarding_points = 100
        WHERE id = $1 AND onboarding_completed = false
    `, userID)
	if err != nil {
		return fmt.Errorf("failed to award onboarding points: %v", err)
	}

	_, err = DB.Exec(`
        INSERT INTO points_history (user_id, points, reason, timestamp)
        VALUES ($1, 100, 'Onboarding task completed', NOW())
    `, userID)
	if err != nil {
		return fmt.Errorf("failed to record onboarding points: %v", err)
	}

	return nil
}
