package main

import (
	"context"
	"log"
	"math/big"
	"time"
)

func main() {
	LogInfo("Trading Ace starting...")

	err := InitDB()
	if err != nil {
		LogFatal("Failed to initialize database: %v", err)
	}
	defer DB.Close()

	err = InitEthereumClient(nil)
	if err != nil {
		LogFatal("Failed to initialize Ethereum client: %v", err)
	}

	wsManager := NewWebSocketManager()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go wsManager.Run(ctx)

	// Set up and run the API server
	r := SetupRouter(wsManager)
	go func() {
		if err := r.Run(":8080"); err != nil {
			log.Fatalf("Failed to run server: %v", err)
		}
	}()

	// Start the weekly share pool task
	go runWeeklySharePoolTask(wsManager)

	// Fetch and process swap events continuously
	go func() {
		for {
			latestBlock, err := Client.BlockNumber(context.Background())
			if err != nil {
				log.Printf("Failed to get latest block number: %v", err)
				time.Sleep(15 * time.Second)
				continue
			}

			fromBlock := big.NewInt(int64(latestBlock - 100))
			toBlock := big.NewInt(int64(latestBlock))

			logs, err := FetchSwapEvents(fromBlock, toBlock)
			if err != nil {
				log.Printf("Failed to fetch swap events: %v", err)
				time.Sleep(15 * time.Second)
				continue
			}

			swapEvents := ProcessSwapEvents(logs)
			for _, event := range swapEvents {
				wsManager.BroadcastSwapEvent(event)
			}

			time.Sleep(15 * time.Second)
		}
	}()

	select {}
}

func runWeeklySharePoolTask(wsManager *WebSocketManager) {
	for {
		nextMonday := getNextMonday()
		time.Sleep(time.Until(nextMonday))

		log.Println("Starting weekly share pool calculation")
		err := CalculateWeeklySharePoolPoints()
		if err != nil {
			log.Printf("Error calculating weekly share pool points: %v", err)
		}

		// After calculating points, update the leaderboard
		leaderboard, err := GetLeaderboard(100) // Get top 100 for broadcasting
		if err != nil {
			log.Printf("Error getting leaderboard: %v", err)
		} else {
			wsManager.BroadcastLeaderboardUpdate(leaderboard)
		}
	}
}

func getNextMonday() time.Time {
	now := time.Now().UTC()
	weekday := int(now.Weekday())
	daysUntilMonday := 7 - weekday
	if weekday == 0 { // If it's Sunday
		daysUntilMonday = 1
	}
	nextMonday := now.AddDate(0, 0, daysUntilMonday)
	return time.Date(nextMonday.Year(), nextMonday.Month(), nextMonday.Day(), 0, 0, 0, 0, time.UTC)
}
