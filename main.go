package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"
)

func main() {
	fmt.Println("Trading Ace starting...")

	err := InitDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer DB.Close()

	err = InitEthereumClient()
	if err != nil {
		log.Fatalf("Failed to initialize Ethereum client: %v", err)
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
		time.Sleep(7 * 24 * time.Hour) // Wait for a week
		err := CalculateSharePoolPoints()
		if err != nil {
			log.Printf("Error calculating share pool points: %v", err)
		}
	}
}
