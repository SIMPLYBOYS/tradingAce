package main

import (
	"context"
	"fmt"
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

	err = InitEthereumClient(nil) // Use the default client creator
	if err != nil {
		LogFatal("Failed to initialize Ethereum client: %v", err)
	}
	// Set up and run the API server
	r := SetupRouter()
	go func() {
		if err := r.Run(":8080"); err != nil {
			log.Fatalf("Failed to run server: %v", err)
		}
	}()

	// Start the weekly share pool task
	go runWeeklySharePoolTask()

	// Fetch and process swap events continuously
	go func() {
		for {
			// Fetch swap events for the last 100 blocks
			latestBlock, err := Client.BlockNumber(context.Background())
			if err != nil {
				log.Printf("Failed to get latest block number: %v", err)
				time.Sleep(15 * time.Second)
				continue
			}

			fmt.Println("Processing blocks up to:", latestBlock)

			fromBlock := big.NewInt(int64(latestBlock - 100))
			toBlock := big.NewInt(int64(latestBlock))

			logs, err := FetchSwapEvents(fromBlock, toBlock)
			if err != nil {
				log.Printf("Failed to fetch swap events: %v", err)
				time.Sleep(15 * time.Second)
				continue
			}

			ProcessSwapEvents(logs)

			time.Sleep(15 * time.Second) // Wait for 15 seconds before next fetch
		}
	}()

	// Keep the main goroutine running
	select {}
}

func runWeeklySharePoolTask() {
	for {
		// Wait until the next Monday at 00:00 UTC
		nextMonday := getNextMonday()
		time.Sleep(time.Until(nextMonday))

		log.Println("Starting weekly share pool calculation")
		err := CalculateWeeklySharePoolPoints()
		if err != nil {
			log.Printf("Error calculating weekly share pool points: %v", err)
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
