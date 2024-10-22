package db

// DBService interface defines the methods we need from the database
type DBService interface {
	GetUserTasks(address string) (map[string]interface{}, error)
	GetUserPointsHistory(address string) ([]PointsHistory, error)
	GetLeaderboard(limit int) ([]LeaderboardEntry, error)
	GetCampaignConfig() (CampaignConfig, error)
	CalculateWeeklySharePoolPoints() error
	RecordSwapAndUpdatePoints(address string, usdValue float64, points int64, txHash string) error
	EndCampaign(campaignID int) error
	UpdateLeaderboard(address string, points int64) error // Add this method
	Close() error
}
