package ethereum

import (
	"context"
	"errors" // Import the standard errors package
	"fmt"
	"math/big"
	"time"

	"github.com/SIMPLYBOYS/trading_ace/internal/db"
	customErrors "github.com/SIMPLYBOYS/trading_ace/internal/errors"
	customtypes "github.com/SIMPLYBOYS/trading_ace/internal/types"
	"github.com/SIMPLYBOYS/trading_ace/pkg/logger"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

var SwapEventSignature = []byte("Swap(address,uint256,uint256,uint256,uint256,address)")

type WebSocketManager interface {
	BroadcastSwapEvent(event *customtypes.SwapEvent) error
	BroadcastUserPointsUpdate(address string, points int64) error
	BroadcastLeaderboardUpdate(leaderboard []map[string]interface{}) error
}

// FetchSwapEvents retrieves swap events from the specified block range
func FetchSwapEvents(fromBlock, toBlock *big.Int) ([]types.Log, error) {
	contractAddress := common.HexToAddress(UniswapV2PairAddress)
	query := ethereum.FilterQuery{
		FromBlock: fromBlock,
		ToBlock:   toBlock,
		Addresses: []common.Address{contractAddress},
		Topics:    [][]common.Hash{{crypto.Keccak256Hash(SwapEventSignature)}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logs, err := Client.FilterLogs(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to filter logs: %w", err)
	}

	logger.Info("Successfully fetched %d swap events from block %s to %s",
		len(logs), fromBlock.String(), toBlock.String())

	return logs, nil
}

// ProcessSwapEvents processes the fetched swap events
func ProcessSwapEvents(logs []types.Log, wsManager WebSocketManager) error {
	for _, vLog := range logs {
		// Unpack the event data
		var swapEvent customtypes.SwapEvent
		err := swapEventABI.UnpackIntoInterface(&swapEvent, "Swap", vLog.Data)
		if err != nil {
			return &customErrors.EthereumError{Operation: "unpack swap event", Err: err}
		}

		// Set the sender and recipient addresses
		swapEvent.Sender = common.HexToAddress(vLog.Topics[1].Hex())
		swapEvent.To = common.HexToAddress(vLog.Topics[2].Hex())

		// Calculate USD value of the swap
		usdValue, err := calculateUSDValue(&swapEvent)
		if err != nil {
			return &customErrors.EthereumError{Operation: "calculate USD value", Err: err}
		}
		swapEvent.USDValue = usdValue

		// Broadcast the swap event
		err = wsManager.BroadcastSwapEvent(&swapEvent)
		if err != nil {
			return &customErrors.WebSocketError{Operation: "broadcast swap event", Err: err}
		}

		// Calculate points for the swap
		points := calculatePointsForSwap(usdValue)

		// Correct usage of Float64()
		usdValueFloat, _ := usdValue.Float64()
		err = db.RecordSwapAndUpdatePoints(swapEvent.Sender.Hex(), usdValueFloat, points, swapEvent.TxHash.Hex())
		if err != nil {
			return &customErrors.DatabaseError{Operation: "record swap and update points", Err: err}
		}
		// Broadcast user points update
		err = wsManager.BroadcastUserPointsUpdate(swapEvent.Sender.Hex(), points)
		if err != nil {
			return &customErrors.WebSocketError{Operation: "broadcast user points update", Err: err}
		}

		// Update the leaderboard
		err = db.UpdateLeaderboard(swapEvent.Sender.Hex(), points)
		if err != nil {
			return &customErrors.DatabaseError{Operation: "update leaderboard", Err: err}
		}

		// Get and broadcast the updated leaderboard
		leaderboard, err := db.GetLeaderboard(10) // Get top 10
		if err != nil {
			return &customErrors.DatabaseError{Operation: "get leaderboard", Err: err}
		}

		// Convert leaderboard to []map[string]interface{}
		leaderboardMap := make([]map[string]interface{}, len(leaderboard))
		for i, entry := range leaderboard {
			leaderboardMap[i] = map[string]interface{}{
				"address": entry.Address,
				"points":  entry.Points,
			}
		}
		err = wsManager.BroadcastLeaderboardUpdate(leaderboardMap)
		if err != nil {
			return &customErrors.WebSocketError{Operation: "broadcast leaderboard update", Err: err}
		}

		logger.Info("Processed swap event: TX Hash: %s, Sender: %s, To: %s, USD Value: %.2f, Points: %d",
			vLog.TxHash.Hex(), swapEvent.Sender.Hex(), swapEvent.To.Hex(), usdValueFloat, points)
	}

	return nil
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

// calculateUSDValue calculates the USD value of a swap event
func calculateUSDValue(event *customtypes.SwapEvent) (*big.Float, error) {
	ethPrice, err := GetEthereumPrice()
	if err != nil {
		return nil, &customErrors.EthereumError{Operation: "get Ethereum price", Err: err}
	}

	wethDecimals := new(big.Float).SetInt64(1e18)
	usdcDecimals := new(big.Float).SetInt64(1e6)

	amount0In := new(big.Float).Quo(new(big.Float).SetInt(event.Amount0In), wethDecimals)
	amount1In := new(big.Float).Quo(new(big.Float).SetInt(event.Amount1In), usdcDecimals)
	amount0Out := new(big.Float).Quo(new(big.Float).SetInt(event.Amount0Out), wethDecimals)
	amount1Out := new(big.Float).Quo(new(big.Float).SetInt(event.Amount1Out), usdcDecimals)

	var usdValue *big.Float

	if amount0In.Cmp(big.NewFloat(0)) > 0 {
		// WETH was input
		usdValue = new(big.Float).Mul(amount0In, ethPrice)
	} else if amount1Out.Cmp(big.NewFloat(0)) > 0 {
		// USDC was output
		usdValue = amount1Out
	} else if amount1In.Cmp(big.NewFloat(0)) > 0 {
		// USDC was input
		usdValue = amount1In
	} else if amount0Out.Cmp(big.NewFloat(0)) > 0 {
		// WETH was output
		usdValue = new(big.Float).Mul(amount0Out, ethPrice)
	} else {
		return nil, &customErrors.EthereumError{
			Operation: "calculate USD value",
			Err:       errors.New("invalid swap event: no input or output"),
		}
	}

	return usdValue.Abs(usdValue), nil
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
