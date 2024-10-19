package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SIMPLYBOYS/trading_ace/internal/api"
	"github.com/SIMPLYBOYS/trading_ace/internal/db"
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
	dbService, err := db.NewDBService()
	if err != nil {
		logger.Fatal("Failed to initialize database: %v", err)
	}
	defer dbService.Close()

	// Initialize Ethereum client
	ethService, err := ethereum.NewEthereumService()
	if err != nil {
		logger.Fatal("Failed to initialize Ethereum client: %v", err)
	}
	defer ethService.Close()

	// Initialize WebSocket manager
	wsManager := websocket.NewWebSocketManager()
	go wsManager.Run()

	// Set up and run the API server
	r := api.SetupRouter(dbService, ethService, wsManager)

	// Create a new server
	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to run server: %v", err)
		}
	}()

	// Start the campaign task scheduler
	go runCampaignTasks(dbService, wsManager)

	// Fetch and process swap events continuously
	go processSwapEvents(ethService, dbService, wsManager)

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown: %v", err)
	}

	logger.Info("Server exiting")
}

func runCampaignTasks(dbService db.DBService, wsManager *websocket.WebSocketManager) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			logger.Info("Running daily campaign tasks...")

			// Check and update campaign status
			campaign, err := dbService.GetCampaignConfig()
			if err != nil {
				logger.Error("Failed to get campaign config: %v", err)
				continue
			}

			if campaign.IsActive && time.Now().After(campaign.EndTime) {
				logger.Info("Campaign has ended. Deactivating...")
				err = dbService.EndCampaign(campaign.ID)
				if err != nil {
					logger.Error("Failed to end campaign: %v", err)
				}
			}

			// Run weekly share pool calculation if it's Monday
			if time.Now().Weekday() == time.Monday {
				logger.Info("Calculating weekly share pool points...")
				err := dbService.CalculateWeeklySharePoolPoints()
				if err != nil {
					logger.Error("Failed to calculate weekly share pool points: %v", err)
				}
			}

			// Update and broadcast leaderboard
			leaderboard, err := dbService.GetLeaderboard(100)
			if err != nil {
				logger.Error("Failed to get leaderboard: %v", err)
			} else {
				// Convert leaderboard to the format expected by WebSocketManager
				leaderboardData := make([]map[string]interface{}, len(leaderboard))
				for i, entry := range leaderboard {
					leaderboardData[i] = map[string]interface{}{
						"address": entry.Address,
						"points":  entry.Points,
					}
				}
				wsManager.BroadcastLeaderboardUpdate(leaderboardData)
			}
		}
	}
}

func processSwapEvents(ethService ethereum.EthereumService, dbService db.DBService, wsManager *websocket.WebSocketManager) {
	for {
		latestBlock, err := ethService.GetLatestBlockNumber()
		if err != nil {
			logger.Error("Failed to get latest block number: %v", err)
			time.Sleep(15 * time.Second)
			continue
		}

		fromBlock := latestBlock - 100
		if fromBlock < 0 {
			fromBlock = 0
		}

		logs, err := ethService.FetchSwapEvents(fromBlock, latestBlock)
		if err != nil {
			logger.Error("Failed to fetch swap events: %v", err)
			time.Sleep(15 * time.Second)
			continue
		}

		for _, log := range logs {
			// Process each swap event
			usdValue, _ := log.USDValue.Float64()
			err := dbService.RecordSwapAndUpdatePoints(log.Sender.Hex(), usdValue, calculatePoints(usdValue), log.TxHash.Hex())
			if err != nil {
				logger.Error("Failed to record swap and update points: %v", err)
				continue
			}

			// Broadcast swap event
			wsManager.BroadcastSwapEvent(&log)

			// Update and broadcast leaderboard
			leaderboard, err := dbService.GetLeaderboard(100)
			if err != nil {
				logger.Error("Failed to get leaderboard: %v", err)
			} else {
				leaderboardData := make([]map[string]interface{}, len(leaderboard))
				for i, entry := range leaderboard {
					leaderboardData[i] = map[string]interface{}{
						"address": entry.Address,
						"points":  entry.Points,
					}
				}
				wsManager.BroadcastLeaderboardUpdate(leaderboardData)
			}
		}

		time.Sleep(15 * time.Second)
	}
}

func calculatePoints(usdValue float64) int64 {
	// Implement your points calculation logic here
	return int64(usdValue / 10) // Example: 1 point for every 10 USD
}
