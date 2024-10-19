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
func ProcessSwapEvents(logs []types.Log, wsManager WebSocketManager, dbService db.DBService) error {
	for _, vLog := range logs {
		// Check if this log is a Swap event
		if len(vLog.Topics) != 3 || vLog.Topics[0] != SwapEventSignature {
			continue
		}

		// Parse the swap event
		swapEvent, err := parseSwapEvent(vLog)
		if err != nil {
			return fmt.Errorf("failed to parse swap event: %w", err)
		}

		// Calculate USD value of the swap
		usdValue := calculateUSDValue(swapEvent.Amount0In, swapEvent.Amount1In, swapEvent.Amount0Out, swapEvent.Amount1Out)
		swapEvent.USDValue = usdValue

		// Broadcast the swap event
		err = wsManager.BroadcastSwapEvent(swapEvent)
		if err != nil {
			return fmt.Errorf("failed to broadcast swap event: %w", err)
		}

		// Calculate points for the swap
		points := calculatePointsForSwap(usdValue)

		// Record swap and update points
		usdValueFloat, _ := usdValue.Float64()
		err = dbService.RecordSwapAndUpdatePoints(swapEvent.Sender.Hex(), usdValueFloat, points, swapEvent.TxHash.Hex())
		if err != nil {
			return fmt.Errorf("failed to record swap and update points: %w", err)
		}

		// Broadcast user points update
		err = wsManager.BroadcastUserPointsUpdate(swapEvent.Sender.Hex(), points)
		if err != nil {
			return fmt.Errorf("failed to broadcast user points update: %w", err)
		}

		// Update the leaderboard
		err = dbService.UpdateLeaderboard(swapEvent.Sender.Hex(), points)
		if err != nil {
			return fmt.Errorf("failed to update leaderboard: %w", err)
		}

		// Get and broadcast the updated leaderboard
		leaderboard, err := dbService.GetLeaderboard(10) // Get top 10
		if err != nil {
			return fmt.Errorf("failed to get leaderboard: %w", err)
		}

		leaderboardMap := convertLeaderboard(leaderboard)
		err = wsManager.BroadcastLeaderboardUpdate(leaderboardMap)
		if err != nil {
			return fmt.Errorf("failed to broadcast leaderboard update: %w", err)
		}

		// Log the processed event
		logger.Info("Processed swap event: TX Hash: %s, Sender: %s, To: %s, USD Value: %.2f, Points: %d",
			vLog.TxHash.Hex(), swapEvent.Sender.Hex(), swapEvent.Recipient.Hex(), usdValueFloat, points)
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
