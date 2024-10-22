package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/SIMPLYBOYS/trading_ace/internal/api"
	"github.com/SIMPLYBOYS/trading_ace/internal/db"
	"github.com/SIMPLYBOYS/trading_ace/internal/ethereum"
	"github.com/SIMPLYBOYS/trading_ace/internal/websocket"
	"github.com/SIMPLYBOYS/trading_ace/pkg/logger"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

// RealDBOperations implements the DBOperations interface
type RealDBOperations struct{}

func (RealDBOperations) Open(driverName, dataSourceName string) (*sql.DB, error) {
	return sql.Open(driverName, dataSourceName)
}

func (RealDBOperations) RunMigrations(db *sql.DB) error {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("could not create the postgres driver: %v", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations", // This should be the path to your migration files
		"postgres", driver)
	if err != nil {
		return fmt.Errorf("migration failed: %v", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("an error occurred while syncing the database: %v", err)
	}

	return nil
}

func main() {
	// Initialize logger
	logLevel := logger.INFO
	if os.Getenv("DEBUG") == "true" {
		logLevel = logger.DEBUG
	}
	logger.SetLevel(logLevel)

	// Create a real DBOperations implementation
	dbOps := RealDBOperations{}

	// Initialize database service
	dbService, err := db.NewDBService(dbOps)
	if err != nil {
		logger.Fatal("Failed to initialize database service: %v", err)
	}
	defer dbService.Close()

	// Initialize Ethereum service
	ethService, err := ethereum.NewEthereumService()
	if err != nil {
		logger.Fatal("Failed to initialize Ethereum service: %v", err)
	}
	defer ethService.Close()

	// Initialize WebSocket manager
	wsManager := websocket.NewWebSocketManager()
	go wsManager.Run()

	// Setup router
	router := api.SetupRouter(dbService, ethService, wsManager)

	// Get port from environment variable or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Create a new http.Server
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the server in a goroutine
	go func() {
		logger.Info("Server is running on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server: %v", err)
		}
	}()

	// Start processing Ethereum events in a separate goroutine
	go processEthereumEvents(ctx, ethService, dbService, wsManager)

	// Start the campaign management goroutine
	go manageCampaign(ctx, dbService)

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	// Cancel the context to stop the Ethereum event processing and campaign management
	cancel()

	// Create a deadline to wait for
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	// Doesn't block if no connections, but will otherwise wait until the timeout deadline
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Fatal("Server forced to shutdown: %v", err)
	}

	logger.Info("Server exiting")
}

func processEthereumEvents(ctx context.Context, ethService ethereum.EthereumService, dbService db.DBService, wsManager *websocket.WebSocketManager) {
	ticker := time.NewTicker(15 * time.Second) // Poll for new events every 15 seconds
	defer ticker.Stop()

	var lastProcessedBlock uint64
	var mu sync.Mutex

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			func() {
				mu.Lock()
				defer mu.Unlock()

				latestBlock, err := ethService.GetLatestBlockNumber()
				if err != nil {
					logger.Error("Failed to get latest block number: %v", err)
					return
				}

				if lastProcessedBlock == 0 {
					lastProcessedBlock = latestBlock - 100 // Start from 100 blocks ago on first run
				}

				if latestBlock <= lastProcessedBlock {
					return // No new blocks to process
				}

				logs, err := ethService.FetchSwapEvents(lastProcessedBlock+1, latestBlock)
				if err != nil {
					logger.Error("Failed to fetch swap events: %v", err)
					return
				}

				err = ethereum.ProcessSwapEvents(logs, wsManager, dbService)
				if err != nil {
					logger.Error("Failed to process swap events: %v", err)
					return
				}

				lastProcessedBlock = latestBlock
			}()
		}
	}
}

func manageCampaign(ctx context.Context, dbService db.DBService) {
	ticker := time.NewTicker(24 * time.Hour) // Check campaign status daily
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			campaign, err := dbService.GetCampaignConfig()
			if err != nil {
				logger.Error("Failed to get campaign config: %v", err)
				continue
			}

			now := time.Now()

			if campaign.IsActive && now.After(campaign.EndTime) {
				// End the campaign
				err := dbService.EndCampaign(campaign.ID)
				if err != nil {
					logger.Error("Failed to end campaign: %v", err)
				} else {
					logger.Info("Campaign ended: ID %d", campaign.ID)
				}
			}

			// Check if it's time to calculate weekly share pool points
			if now.Weekday() == time.Sunday && now.Hour() == 0 {
				err := dbService.CalculateWeeklySharePoolPoints()
				if err != nil {
					logger.Error("Failed to calculate weekly share pool points: %v", err)
				} else {
					logger.Info("Calculated weekly share pool points")
				}
			}
		}
	}
}
