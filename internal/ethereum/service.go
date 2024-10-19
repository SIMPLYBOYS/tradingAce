// File: internal/ethereum/service.go

package ethereum

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"

	customtypes "github.com/SIMPLYBOYS/trading_ace/internal/types"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// SwapEventSignature is the Keccak256 hash of "Swap(address,uint256,uint256,uint256,uint256,address)"
var SwapEventSignature = common.HexToHash("0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822")

// EthereumServiceImpl implements the EthereumService interface
type EthereumServiceImpl struct {
	client *ethclient.Client
	abi    abi.ABI
}

// NewEthereumService creates and returns a new EthereumService
func NewEthereumService() (EthereumService, error) {
	infuraProjectID := os.Getenv("INFURA_PROJECT_ID")
	if infuraProjectID == "" {
		return nil, fmt.Errorf("INFURA_PROJECT_ID environment variable is not set")
	}

	infuraURL := fmt.Sprintf("https://mainnet.infura.io/v3/%s", infuraProjectID)

	client, err := ethclient.Dial(infuraURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum client: %w", err)
	}

	uniswapABI, err := abi.JSON(strings.NewReader(UniswapV2PairABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse Uniswap V2 Pair ABI: %w", err)
	}

	return &EthereumServiceImpl{
		client: client,
		abi:    uniswapABI,
	}, nil
}

// GetEthereumPrice retrieves the current Ethereum price
func (s *EthereumServiceImpl) GetEthereumPrice() (*big.Float, error) {
	// In a real implementation, you would call a price feed contract or use an API
	// This is a placeholder returning a fixed value
	return big.NewFloat(2000.0), nil
}

// GetLatestBlockNumber retrieves the latest block number from the Ethereum blockchain
func (s *EthereumServiceImpl) GetLatestBlockNumber() (uint64, error) {
	header, err := s.client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch latest block header: %w", err)
	}
	return header.Number.Uint64(), nil
}

// FetchSwapEvents fetches swap events from the specified block range
func (s *EthereumServiceImpl) FetchSwapEvents(fromBlock, toBlock uint64) ([]customtypes.SwapEvent, error) {
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(fromBlock)),
		ToBlock:   big.NewInt(int64(toBlock)),
		Addresses: []common.Address{
			common.HexToAddress(UniswapV2PairAddress),
		},
	}

	logs, err := s.client.FilterLogs(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to filter logs: %w", err)
	}

	log.Printf("Fetched %d logs", len(logs))

	var events []customtypes.SwapEvent
	for i, vLog := range logs {
		log.Printf("Processing log %d: Topics: %d, Data length: %d", i, len(vLog.Topics), len(vLog.Data))

		// Check if this log is a Swap event
		if len(vLog.Topics) != 3 || vLog.Topics[0] != SwapEventSignature {
			log.Printf("Log %d is not a Swap event, skipping", i)
			continue
		}

		event, err := s.ParseSwapEvent(vLog)
		if err != nil {
			log.Printf("Failed to parse log %d: %v", i, err)
			continue
		}
		events = append(events, *event)
	}

	log.Printf("Successfully parsed %d swap events", len(events))

	return events, nil
}

// ParseSwapEvent parses a raw log into a SwapEvent
func (s *EthereumServiceImpl) ParseSwapEvent(log interface{}) (*customtypes.SwapEvent, error) {
	vLog, ok := log.(types.Log)
	if !ok {
		return nil, fmt.Errorf("invalid log type")
	}

	if len(vLog.Data) != 128 {
		return nil, fmt.Errorf("invalid data length for Swap event: got %d, want 128", len(vLog.Data))
	}

	event := &customtypes.SwapEvent{
		TxHash:     vLog.TxHash,
		Sender:     common.HexToAddress(vLog.Topics[1].Hex()),
		Recipient:  common.HexToAddress(vLog.Topics[2].Hex()),
		Amount0In:  new(big.Int).SetBytes(vLog.Data[0:32]),
		Amount1In:  new(big.Int).SetBytes(vLog.Data[32:64]),
		Amount0Out: new(big.Int).SetBytes(vLog.Data[64:96]),
		Amount1Out: new(big.Int).SetBytes(vLog.Data[96:128]),
	}

	// Calculate USD value (this is a placeholder, replace with actual calculation)
	event.USDValue = calculateUSDValue(event.Amount0In, event.Amount1In, event.Amount0Out, event.Amount1Out)

	return event, nil
}

func calculateUSDValue(amount0In, amount1In, amount0Out, amount1Out *big.Int) *big.Float {
	// This is a simplified calculation and should be replaced with a more accurate implementation
	// Here we're assuming amount1 is USDC (6 decimals) and amount0 is WETH (18 decimals)
	inValue := new(big.Float).SetInt(amount1In)
	inValue = inValue.Quo(inValue, big.NewFloat(1e6))

	outValue := new(big.Float).SetInt(amount1Out)
	outValue = outValue.Quo(outValue, big.NewFloat(1e6))

	totalValue := new(big.Float).Add(inValue, outValue)
	return totalValue
}

// Close closes the Ethereum client connection
func (s *EthereumServiceImpl) Close() {
	if s.client != nil {
		s.client.Close()
	}
}

// UniswapV2PairABI is the ABI for the Uniswap V2 Pair contract
const UniswapV2PairABI = `[{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"sender","type":"address"},{"indexed":false,"internalType":"uint256","name":"amount0In","type":"uint256"},{"indexed":false,"internalType":"uint256","name":"amount1In","type":"uint256"},{"indexed":false,"internalType":"uint256","name":"amount0Out","type":"uint256"},{"indexed":false,"internalType":"uint256","name":"amount1Out","type":"uint256"},{"indexed":true,"internalType":"address","name":"to","type":"address"}],"name":"Swap","type":"event"}]`
