package main

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	InfuraURL            = "https://mainnet.infura.io/v3/PROJECT-ID"
	UniswapV2PairAddress = "0xB4e16d0168e52d35CaCD2c6185b44281Ec28C9Dc" // WETH/USDC pair
)

var (
	Client       *ethclient.Client
	swapEventABI abi.ABI
)

// SwapEvent represents the data structure of a Swap event
type SwapEvent struct {
	Sender     common.Address
	Amount0In  *big.Int
	Amount1In  *big.Int
	Amount0Out *big.Int
	Amount1Out *big.Int
	To         common.Address
	USDValue   *big.Float
}

// Swap event signature
var SwapEventSignature = []byte("Swap(address,uint256,uint256,uint256,uint256,address)")

func init() {
	// Initialize the ABI for the Swap event
	const abiJSON = `[{"anonymous":false,"inputs":[{"indexed":true,"name":"sender","type":"address"},{"indexed":false,"name":"amount0In","type":"uint256"},{"indexed":false,"name":"amount1In","type":"uint256"},{"indexed":false,"name":"amount0Out","type":"uint256"},{"indexed":false,"name":"amount1Out","type":"uint256"},{"indexed":true,"name":"to","type":"address"}],"name":"Swap","type":"event"}]`
	var err error
	swapEventABI, err = abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		panic(err)
	}
}

func InitEthereumClient() error {
	var err error
	Client, err = ethclient.Dial(InfuraURL)
	if err != nil {
		return fmt.Errorf("failed to connect to the Ethereum client: %w", err)
	}
	LogInfo("Successfully connected to Ethereum client")
	return nil
}

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

	LogInfo("Successfully fetched %d swap events from block %s to %s", len(logs), fromBlock.String(), toBlock.String())
	return logs, nil
}

func calculateUSDValue(event *SwapEvent) (*big.Float, error) {
	// Assume USDC has 6 decimal places and WETH has 18
	usdcDecimals := big.NewFloat(1e6)
	wethDecimals := big.NewFloat(1e18)

	// Convert big.Int to big.Float
	amount0In := new(big.Float).SetInt(event.Amount0In)
	amount1In := new(big.Float).SetInt(event.Amount1In)
	amount0Out := new(big.Float).SetInt(event.Amount0Out)
	amount1Out := new(big.Float).SetInt(event.Amount1Out)

	// Calculate total WETH and USDC involved in the swap
	totalWETH := new(big.Float).Add(amount0In, amount0Out)
	totalUSDC := new(big.Float).Add(amount1In, amount1Out)

	// Adjust for decimals
	totalWETH.Quo(totalWETH, wethDecimals)
	totalUSDC.Quo(totalUSDC, usdcDecimals)

	// If USDC amount is 0, we can't calculate the price
	if totalUSDC.Cmp(big.NewFloat(0)) == 0 {
		return nil, fmt.Errorf("USDC amount is 0, cannot calculate price")
	}

	// Calculate WETH price in USDC
	wethPrice := new(big.Float).Quo(totalUSDC, totalWETH)

	// Calculate total USD value
	usdValue := new(big.Float).Mul(totalWETH, wethPrice)

	return usdValue, nil
}

func ProcessSwapEvents(logs []types.Log) []*SwapEvent {
	swapEvents := make([]*SwapEvent, 0)

	for _, vLog := range logs {
		var swapEvent SwapEvent
		err := swapEventABI.UnpackIntoInterface(&swapEvent, "Swap", vLog.Data)
		if err != nil {
			LogError("Error unpacking swap event: %v", err)
			continue
		}

		swapEvent.Sender = common.HexToAddress(vLog.Topics[1].Hex())
		swapEvent.To = common.HexToAddress(vLog.Topics[2].Hex())

		usdValue, err := calculateUSDValue(&swapEvent)
		if err != nil {
			LogError("Error calculating USD value for swap event %s: %v", vLog.TxHash.Hex(), err)
			continue
		}

		usdValueFloat64, _ := usdValue.Float64()

		err = RecordSwap(swapEvent.Sender.Hex(), usdValueFloat64, vLog.TxHash.Hex())
		if err != nil {
			LogError("Error recording swap event %s: %v", vLog.TxHash.Hex(), err)
			continue
		}

		swapEvents = append(swapEvents, &swapEvent)

		LogInfo("Processed swap event: TX Hash: %s, Sender: %s, To: %s, USD Value: %.2f",
			vLog.TxHash.Hex(), swapEvent.Sender.Hex(), swapEvent.To.Hex(), usdValueFloat64)
	}

	return swapEvents
}

func CalculateSwapVolume(event *SwapEvent) *big.Int {
	volume := new(big.Int).Add(event.Amount0In, event.Amount0Out)
	return volume
}

func GetLatestBlockNumber() (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	header, err := Client.HeaderByNumber(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get latest block number: %w", err)
	}

	LogInfo("Retrieved latest block number: %d", header.Number.Uint64())
	return header.Number.Uint64(), nil
}
