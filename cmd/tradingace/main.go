package main

import (
	"context"
	"log"
	"math/big"
	"time"

	"github.com/SIMPLYBOYS/trading_ace/internal/api"
	"github.com/SIMPLYBOYS/trading_ace/internal/db"
	"github.com/SIMPLYBOYS/trading_ace/internal/errors"
	"github.com/SIMPLYBOYS/trading_ace/internal/ethereum"
	"github.com/SIMPLYBOYS/trading_ace/internal/websocket"
	"github.com/SIMPLYBOYS/trading_ace/pkg/logger"
)

func main() {
	// Initialize logger
	logger.SetLevel(logger.INFO)
	err := logger.EnableFileLogging("./logs")
	if err != nil {
		log.Fatalf("Failed to enable file logging: %v", err)
	}

	logger.Info("Trading Ace starting...")

	// Initialize database
	err = db.InitDB()
	if err != nil {
		logger.Fatal("Failed to initialize database: %v", err)
	}
	defer db.DB.Close()

	// Initialize Ethereum client
	err = ethereum.InitEthereumClient(nil)
	if err != nil {
		logger.Fatal("Failed to initialize Ethereum client: %v", err)
	}

	// Initialize WebSocket manager
	wsManager := websocket.NewWebSocketManager()
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	go wsManager.Run()

	// Set up and run the API server
	r := api.SetupRouter(wsManager)
	go func() {
		if err := r.Run(":8080"); err != nil {
			logger.Fatal("Failed to run server: %v", err)
		}
	}()

	// Start the campaign task scheduler
	go runCampaignTasks(wsManager)

	// Fetch and process swap events continuously
	go processSwapEvents(wsManager)

	// Keep the main goroutine running
	select {}
}

func runCampaignTasks(wsManager *websocket.WebSocketManager) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			logger.Info("Running daily campaign tasks...")

			// Check and update campaign status
			campaign, err := db.GetCampaignConfig()
			if err != nil {
				logger.Error("Failed to get campaign config: %v", err)
				continue
			}

			if campaign.IsActive && time.Now().After(campaign.EndTime) {
				logger.Info("Campaign has ended. Deactivating...")
				err = db.EndCampaign(campaign.ID)
				if err != nil {
					logger.Error("Failed to end campaign: %v", err)
				}
			}

			// Run weekly share pool calculation if it's Monday
			if time.Now().Weekday() == time.Monday {
				logger.Info("Calculating weekly share pool points...")
				err := db.CalculateWeeklySharePoolPoints()
				if err != nil {
					logger.Error("Failed to calculate weekly share pool points: %v", err)
				}
			}

			// Update and broadcast leaderboard
			dbLeaderboard, err := db.GetLeaderboard(100)
			if err != nil {
				logger.Error("Failed to get leaderboard: %v", err)
			} else {
				leaderboard := convertLeaderboard(dbLeaderboard)
				wsManager.BroadcastLeaderboardUpdate(leaderboard)
			}
		}
	}
}

// convertLeaderboard converts []db.LeaderboardEntry to []map[string]interface{}
func convertLeaderboard(dbLeaderboard []db.LeaderboardEntry) []map[string]interface{} {
	leaderboard := make([]map[string]interface{}, len(dbLeaderboard))
	for i, entry := range dbLeaderboard {
		leaderboard[i] = map[string]interface{}{
			"address": entry.Address,
			"points":  entry.Points,
		}
	}
	return leaderboard
}

func processSwapEvents(wsManager *websocket.WebSocketManager) {
	for {
		latestBlock, err := ethereum.Client.BlockNumber(context.Background())
		if err != nil {
			logger.Error("Failed to get latest block number: %v", err)
			time.Sleep(15 * time.Second)
			continue
		}

		fromBlock := big.NewInt(int64(latestBlock - 100))
		toBlock := big.NewInt(int64(latestBlock))

		logs, err := ethereum.FetchSwapEvents(fromBlock, toBlock)
		if err != nil {
			logger.Error("Failed to fetch swap events: %v", err)
			time.Sleep(15 * time.Second)
			continue
		}

		err = ethereum.ProcessSwapEvents(logs, wsManager)
		if err != nil {
			switch e := err.(type) {
			case *errors.EthereumError:
				logger.Error("Ethereum error: %v", e)
			case *errors.WebSocketError:
				logger.Error("WebSocket error: %v", e)
			case *errors.DatabaseError:
				logger.Error("Database error: %v", e)
			default:
				logger.Error("Unknown error: %v", e)
			}
		}

		time.Sleep(15 * time.Second)
	}
}
