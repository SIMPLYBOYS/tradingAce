package db

import (
	"database/sql"
	"time"
)

var DB *sql.DB

type User struct {
	ID                  int
	Address             string
	OnboardingCompleted bool
	OnboardingPoints    int
	TotalPoints         int64
}

type SwapEvent struct {
	ID              int
	UserID          int
	TransactionHash string
	AmountUSD       float64
	Timestamp       time.Time
}

type PointsHistory struct {
	ID        int
	UserID    int
	Points    int64
	Reason    string
	Timestamp time.Time
}

type CampaignConfig struct {
	ID        int
	StartTime time.Time
	EndTime   time.Time
	IsActive  bool
}

type LeaderboardEntry struct {
	Address string
	Points  int64
}
