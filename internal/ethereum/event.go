// File: internal/ethereum/event.go

package ethereum

import (
	"fmt"
	"math/big"

	"github.com/SIMPLYBOYS/trading_ace/internal/db"
	customtypes "github.com/SIMPLYBOYS/trading_ace/internal/types"
	"github.com/SIMPLYBOYS/trading_ace/pkg/logger"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type WebSocketManager interface {
	BroadcastSwapEvent(event *customtypes.SwapEvent) error
	BroadcastUserPointsUpdate(address string, points int64) error
	BroadcastLeaderboardUpdate(leaderboard []map[string]interface{}) error
}

// ProcessSwapEvents processes the fetched swap events
func ProcessSwapEvents(events []customtypes.SwapEvent, wsManager WebSocketManager, dbService db.DBService) error {
	for _, event := range events {
		// Calculate USD value of the swap (if not already calculated)
		if event.USDValue == nil {
			event.USDValue = calculateUSDValue(event.Amount0In, event.Amount1In, event.Amount0Out, event.Amount1Out)
		}

		// Broadcast the swap event
		err := wsManager.BroadcastSwapEvent(&event)
		if err != nil {
			logger.Error("Failed to broadcast swap event: %v", err)
			// Continue processing instead of returning
		}

		// Calculate points for the swap
		points := calculatePointsForSwap(event.USDValue)

		// Convert common.Address to string
		senderAddress := event.Sender.Hex()

		// Record swap and update points
		usdValueFloat, _ := event.USDValue.Float64()
		err = dbService.RecordSwapAndUpdatePoints(senderAddress, usdValueFloat, points, event.TxHash.Hex())
		if err != nil {
			logger.Error("Failed to record swap and update points: %v", err)
			continue // Continue processing other events instead of returning
		}

		// Broadcast user points update
		err = wsManager.BroadcastUserPointsUpdate(senderAddress, points)
		if err != nil {
			logger.Error("Failed to broadcast user points update: %v", err)
			// Continue processing instead of returning
		}

		// Update the leaderboard
		err = dbService.UpdateLeaderboard(senderAddress, points)
		if err != nil {
			logger.Error("Failed to update leaderboard: %v", err)
			continue // Continue processing other events instead of returning
		}

		// Get and broadcast the updated leaderboard
		leaderboard, err := dbService.GetLeaderboard(10) // Get top 10
		if err != nil {
			logger.Error("Failed to get leaderboard: %v", err)
			continue // Continue processing other events instead of returning
		}

		leaderboardMap := convertLeaderboard(leaderboard)
		err = wsManager.BroadcastLeaderboardUpdate(leaderboardMap)
		if err != nil {
			logger.Error("Failed to broadcast leaderboard update: %v", err)
			// Continue processing instead of returning
		}

		// Log the processed event
		logger.Info("Processed swap event: TX Hash: %s, Sender: %s, To: %s, USD Value: %.2f, Points: %d",
			event.TxHash.Hex(), senderAddress, event.Recipient.Hex(), usdValueFloat, points)
	}

	return nil
}

// parseSwapEvent parses a raw Ethereum log into a SwapEvent
func parseSwapEvent(vLog types.Log) (*customtypes.SwapEvent, error) {
	event := new(customtypes.SwapEvent)
	err := swapEventABI.UnpackIntoInterface(event, "Swap", vLog.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack swap event: %w", err)
	}

	event.TxHash = vLog.TxHash
	event.Sender = common.BytesToAddress(vLog.Topics[1].Bytes())
	event.Recipient = common.BytesToAddress(vLog.Topics[2].Bytes())

	return event, nil
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

// calculatePointsForSwap calculates the points for a swap based on its USD value
func calculatePointsForSwap(usdValue *big.Float) int64 {
	points, _ := usdValue.Quo(usdValue, big.NewFloat(10)).Int64() // 1 point for every 10 USD
	if points < 1 {
		points = 1 // Minimum 1 point per swap
	}
	return points
}

// CalculateSwapVolume calculates the total volume of a swap event
func CalculateSwapVolume(event *customtypes.SwapEvent) *big.Int {
	volume := new(big.Int).Add(event.Amount0In, event.Amount0Out)
	return volume
}
