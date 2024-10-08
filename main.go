package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
)

func main() {
	fmt.Println("Trading Ace starting...")

	err := InitEthereumClient()
	if err != nil {
		log.Fatalf("Failed to initialize Ethereum client: %v", err)
	}

	// Fetch swap events for the last 100 blocks
	latestBlock, err := Client.BlockNumber(context.Background())
	if err != nil {
		log.Fatalf("Failed to get latest block number: %v", err)
	}

	fmt.Println(latestBlock)

	fromBlock := big.NewInt(int64(latestBlock - 100))
	toBlock := big.NewInt(int64(latestBlock))

	logs, err := FetchSwapEvents(fromBlock, toBlock)
	if err != nil {
		log.Fatalf("Failed to fetch swap events: %v", err)
	}

	ProcessSwapEvents(logs)
}
