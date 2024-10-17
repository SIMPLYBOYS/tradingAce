package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Config holds the application configuration
type Config struct {
	DatabaseURL string
	ServerPort  string
	InfuraURL   string
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}

func run() error {
	// Initialize logger
	if err := initLogger(); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize database
	if err := InitDB(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer DB.Close()

	// Initialize Ethereum client
	if err := InitEthereumClient(nil); err != nil {
		return fmt.Errorf("failed to initialize Ethereum client: %w", err)
	}

	// Initialize WebSocket manager
	wsManager := NewWebSocketManager()

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start WebSocket manager
	go wsManager.Run(ctx)

	// Set up and run the API server
	router := SetupRouter(wsManager)
	srv := &http.Server{
		Addr:    ":" + config.ServerPort,
		Handler: router,
	}

	go func() {
		LogInfo("Starting server on port %s", config.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			LogError("Server error: %v", err)
		}
	}()

	// Start swap event processor
	go runSwapEventProcessor(ctx, wsManager)

	// Start weekly share pool task
	go runWeeklySharePoolTask(ctx, wsManager)

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	LogInfo("Shutting down server...")

	// Shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	LogInfo("Server exiting")
	return nil
}

func initLogger() error {
	// Initialize logger with appropriate configuration
	// For simplicity, we're using the default logger here
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	return nil
}

func loadConfig() (*Config, error) {
	// Load configuration from environment variables
	return &Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		ServerPort:  os.Getenv("SERVER_PORT"),
		InfuraURL:   os.Getenv("INFURA_URL"),
	}, nil
}

func runSwapEventProcessor(ctx context.Context, wsManager *WebSocketManager) {
	LogInfo("Starting swap event processor")
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			LogInfo("Stopping swap event processor")
			return
		case <-ticker.C:
			latestBlock, err := GetLatestBlockNumber()
			if err != nil {
				LogError("Failed to get latest block number: %v", err)
				continue
			}

			fromBlock := latestBlock - 100 // Process last 100 blocks
			if fromBlock < 0 {
				fromBlock = 0
			}

			logs, err := FetchSwapEvents(big.NewInt(int64(fromBlock)), big.NewInt(int64(latestBlock)))
			if err != nil {
				LogError("Failed to fetch swap events: %v", err)
				continue
			}

			ProcessSwapEvents(logs, wsManager)
		}
	}
}

func runWeeklySharePoolTask(ctx context.Context, wsManager *WebSocketManager) {
	LogInfo("Starting weekly share pool task")
	ticker := time.NewTicker(24 * time.Hour) // Check daily, but only execute weekly
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			LogInfo("Stopping weekly share pool task")
			return
		case <-ticker.C:
			if time.Now().Weekday() == time.Monday {
				LogInfo("Calculating weekly share pool points")
				if err := CalculateWeeklySharePoolPoints(); err != nil {
					LogError("Failed to calculate weekly share pool points: %v", err)
					continue
				}

				// Update leaderboard after calculation
				leaderboard, err := GetLeaderboard(100) // Get top 100 for broadcasting
				if err != nil {
					LogError("Failed to get leaderboard: %v", err)
				} else {
					wsManager.BroadcastLeaderboardUpdate(leaderboard)
				}
			}
		}
	}
}

// Note: SetupRouter is now used from api.go, so we don't define it here
