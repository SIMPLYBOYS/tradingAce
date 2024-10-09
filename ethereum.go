package main

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	InfuraURL              = "https://mainnet.infura.io/v3/PROJECT_ID"
	UniswapV2PairAddress   = "0xB4e16d0168e52d35CaCD2c6185b44281Ec28C9Dc" // WETH/USDC pair
	ChainlinkETHUSDAddress = "0x5f4eC3Df9cbd43714FE2740f5E3616155c5b8419" // Ethereum Mainnet Chainlink Price Feed address for ETH/USD
)

var (
	Client       EthereumClient
	swapEventABI abi.ABI
	// getReservesSelector is the function selector for the getReserves() function
	getReservesSelector = crypto.Keccak256Hash([]byte("getReserves()")).Bytes()[:4]
)

// Extend the EthereumClient interface
type EthereumClient interface {
	CodeAt(ctx context.Context, contract common.Address, blockNumber *big.Int) ([]byte, error)
	CallContract(ctx context.Context, call ethereum.CallMsg, blockNumber *big.Int) ([]byte, error)
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
	BlockNumber(ctx context.Context) (uint64, error)
	FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error)
	// Add these methods to match bind.ContractBackend
	ChainID(ctx context.Context) (*big.Int, error)
	SendTransaction(ctx context.Context, tx *types.Transaction) error
	SuggestGasPrice(ctx context.Context) (*big.Int, error)
	EstimateGas(ctx context.Context, call ethereum.CallMsg) (gas uint64, err error)
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
}

type ClientCreator func(url string) (EthereumClient, error)

func defaultClientCreator(url string) (EthereumClient, error) {
	return ethclient.Dial(url)
}

func InitEthereumClient(creator ClientCreator) error {
	if creator == nil {
		creator = defaultClientCreator
	}
	var err error
	Client, err = creator(InfuraURL)
	if err != nil {
		return LogErrorf(err, "failed to connect to the Ethereum client")
	}
	LogInfo("Successfully connected to Ethereum client")
	return nil
}

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

// AggregatorV3Interface is a simplified ABI of the Chainlink Price Feed contract
type AggregatorV3Interface struct {
	LatestRoundData func() (roundId *big.Int, answer *big.Int, startedAt *big.Int, updatedAt *big.Int, answeredInRound *big.Int, err error)
}

// GetEthereumPrice fetches the latest ETH/USD price from Chainlink Price Feed
func GetEthereumPrice() (*big.Float, error) {
	address := common.HexToAddress(ChainlinkETHUSDAddress)

	// ABI for the latestRoundData function
	const abiJSON = `[{"inputs":[],"name":"latestRoundData","outputs":[{"internalType":"uint80","name":"roundId","type":"uint80"},{"internalType":"int256","name":"answer","type":"int256"},{"internalType":"uint256","name":"startedAt","type":"uint256"},{"internalType":"uint256","name":"updatedAt","type":"uint256"},{"internalType":"uint80","name":"answeredInRound","type":"uint80"}],"stateMutability":"view","type":"function"}]`
	parsedABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return nil, LogErrorf(err, "failed to parse ABI")
	}

	data, err := parsedABI.Pack("latestRoundData")
	if err != nil {
		return nil, LogErrorf(err, "failed to pack data for latestRoundData function call")
	}

	result, err := Client.CallContract(context.Background(), ethereum.CallMsg{
		To:   &address,
		Data: data,
	}, nil)
	if err != nil {
		return nil, LogErrorf(err, "failed to call latestRoundData function")
	}

	var (
		roundId         *big.Int
		answer          *big.Int
		startedAt       *big.Int
		updatedAt       *big.Int
		answeredInRound *big.Int
	)

	err = parsedABI.UnpackIntoInterface(&[]interface{}{&roundId, &answer, &startedAt, &updatedAt, &answeredInRound}, "latestRoundData", result)
	if err != nil {
		return nil, LogErrorf(err, "failed to unpack result")
	}

	// Chainlink price feeds for ETH/USD use 8 decimal places
	ethPrice := new(big.Float).SetInt(answer)
	ethPrice = ethPrice.Quo(ethPrice, big.NewFloat(1e8))

	return ethPrice, nil
}

// NewAggregatorV3Interface creates a new instance of AggregatorV3Interface
func NewAggregatorV3Interface(address common.Address, backend bind.ContractBackend) (*AggregatorV3Interface, error) {
	abi, err := abi.JSON(strings.NewReader(`[{"inputs":[],"name":"latestRoundData","outputs":[{"internalType":"uint80","name":"roundId","type":"uint80"},{"internalType":"int256","name":"answer","type":"int256"},{"internalType":"uint256","name":"startedAt","type":"uint256"},{"internalType":"uint256","name":"updatedAt","type":"uint256"},{"internalType":"uint80","name":"answeredInRound","type":"uint80"}],"stateMutability":"view","type":"function"}]`))
	if err != nil {
		return nil, err
	}

	contract := bind.NewBoundContract(address, abi, backend, backend, backend)

	return &AggregatorV3Interface{
		LatestRoundData: func() (*big.Int, *big.Int, *big.Int, *big.Int, *big.Int, error) {
			var out []interface{}
			err := contract.Call(nil, &out, "latestRoundData")
			if err != nil {
				return nil, nil, nil, nil, nil, err
			}
			return out[0].(*big.Int), out[1].(*big.Int), out[2].(*big.Int), out[3].(*big.Int), out[4].(*big.Int), nil
		},
	}, nil
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

func FetchSwapEvents(fromBlock, toBlock *big.Int) ([]types.Log, error) {
	contractAddress := common.HexToAddress(UniswapV2PairAddress)
	query := ethereum.FilterQuery{
		FromBlock: fromBlock,
		ToBlock:   toBlock,
		Addresses: []common.Address{contractAddress},
		Topics:    [][]common.Hash{{crypto.Keccak256Hash(SwapEventSignature)}},
	}

	logs, err := Client.FilterLogs(context.Background(), query)
	if err != nil {
		return nil, LogErrorf(err, "failed to filter logs")
	}

	LogInfo("Successfully fetched %d swap events from block %s to %s",
		len(logs), fromBlock.String(), toBlock.String())

	return logs, nil
}

var calculateUSDValue = func(event *SwapEvent, reserve0, reserve1 *big.Int) (*big.Float, error) {
	// Assume USDC has 6 decimal places and WETH has 18
	usdcDecimals := big.NewFloat(1e6)
	wethDecimals := big.NewFloat(1e18)

	// Convert big.Int to big.Float
	amount0In := new(big.Float).SetInt(event.Amount0In)
	amount1In := new(big.Float).SetInt(event.Amount1In)
	amount0Out := new(big.Float).SetInt(event.Amount0Out)
	amount1Out := new(big.Float).SetInt(event.Amount1Out)

	// Adjust for decimals
	amount0In.Quo(amount0In, wethDecimals)
	amount1In.Quo(amount1In, usdcDecimals)
	amount0Out.Quo(amount0Out, wethDecimals)
	amount1Out.Quo(amount1Out, usdcDecimals)

	// Calculate pool price (USDC per WETH)
	reserveWETH := new(big.Float).Quo(new(big.Float).SetInt(reserve0), wethDecimals)
	reserveUSDC := new(big.Float).Quo(new(big.Float).SetInt(reserve1), usdcDecimals)
	poolPrice := new(big.Float).Quo(reserveUSDC, reserveWETH)

	var usdValue *big.Float

	if amount0In.Cmp(big.NewFloat(0)) > 0 {
		// WETH was input
		usdValue = new(big.Float).Mul(amount0In, poolPrice)
	} else if amount1Out.Cmp(big.NewFloat(0)) > 0 {
		// USDC was output
		usdValue = amount1Out
	} else if amount1In.Cmp(big.NewFloat(0)) > 0 {
		// USDC was input
		usdValue = amount1In
	} else if amount0Out.Cmp(big.NewFloat(0)) > 0 {
		// WETH was output
		usdValue = new(big.Float).Mul(amount0Out, poolPrice)
	} else {
		return nil, fmt.Errorf("invalid swap event: no input or output")
	}

	return usdValue.Abs(usdValue), nil
}

var getPoolReservesWrapper = func(blockNumber uint64) (*big.Int, *big.Int, error) {
	return getPoolReserves(blockNumber)
}

func ProcessSwapEvents(logs []types.Log, wsManager WebSocketManagerInterface) {
	for _, vLog := range logs {
		var swapEvent SwapEvent
		err := swapEventABI.UnpackIntoInterface(&swapEvent, "Swap", vLog.Data)
		if err != nil {
			LogError("Error unpacking swap event: %v", err)
			continue
		}

		swapEvent.Sender = common.HexToAddress(vLog.Topics[1].Hex())
		swapEvent.To = common.HexToAddress(vLog.Topics[2].Hex())

		// Add this logging
		LogInfo("Processing swap event: TX Hash: %s, Amount0In: %s, Amount1In: %s, Amount0Out: %s, Amount1Out: %s",
			vLog.TxHash.Hex(), swapEvent.Amount0In, swapEvent.Amount1In, swapEvent.Amount0Out, swapEvent.Amount1Out)

		// Get the block number for this event
		blockNumber := vLog.BlockNumber

		// Fetch pool reserves for the block
		reserve0, reserve1, err := getPoolReserves(blockNumber)
		if err != nil {
			LogError("Error fetching pool reserves for block %d: %v", blockNumber, err)
			continue
		}

		usdValue, err := calculateUSDValue(&swapEvent, reserve0, reserve1)
		if err != nil {
			LogError("Error calculating USD value for swap event %s: %v", vLog.TxHash.Hex(), err)
			continue
		}

		swapEvent.USDValue = usdValue

		usdValueFloat64, _ := usdValue.Float64()

		points, err := CalculatePointsForSwap(swapEvent.Sender.Hex(), usdValueFloat64)
		if err != nil {
			LogError("Error calculating points for swap event %s: %v", vLog.TxHash.Hex(), err)
			continue
		}
		// Ensure points is int64
		points64 := int64(points)

		// Update user's points and record the swap
		err = RecordSwapAndUpdatePoints(swapEvent.Sender.Hex(), usdValueFloat64, points64, vLog.TxHash.Hex())
		if err != nil {
			LogError("Error recording swap and updating points for event %s: %v", vLog.TxHash.Hex(), err)
			continue
		}

		// Broadcast the swap event and updated points
		wsManager.BroadcastSwapEvent(&swapEvent)
		wsManager.BroadcastUserPointsUpdate(swapEvent.Sender.Hex(), points)

		// Update and broadcast leaderboard
		err = UpdateLeaderboard(swapEvent.Sender.Hex(), points)
		if err != nil {
			LogError("Error updating leaderboard for event %s: %v", vLog.TxHash.Hex(), err)
		} else {
			leaderboard, err := GetLeaderboard(10) // Get top 10 for broadcasting
			if err == nil {
				wsManager.BroadcastLeaderboardUpdate(leaderboard)
			}
		}

		LogInfo("Processed swap event: TX Hash: %s, Sender: %s, To: %s, USD Value: %.2f, Points: %d",
			vLog.TxHash.Hex(), swapEvent.Sender.Hex(), swapEvent.To.Hex(), usdValueFloat64, points)
	}
}

func calculateUSDValueWithEthPrice(event *SwapEvent, ethPrice *big.Float) (*big.Float, error) {
	wethDecimals := new(big.Float).SetInt64(1e18)
	usdcDecimals := new(big.Float).SetInt64(1e6)

	var usdValue *big.Float

	if event.Amount0In.Cmp(big.NewInt(0)) > 0 {
		// WETH was input
		wethAmount := new(big.Float).Quo(new(big.Float).SetInt(event.Amount0In), wethDecimals)
		usdValue = new(big.Float).Mul(wethAmount, ethPrice)
	} else if event.Amount1Out.Cmp(big.NewInt(0)) > 0 {
		// USDC was output
		usdValue = new(big.Float).Quo(new(big.Float).SetInt(event.Amount1Out), usdcDecimals)
	} else if event.Amount1In.Cmp(big.NewInt(0)) > 0 {
		// USDC was input
		usdValue = new(big.Float).Quo(new(big.Float).SetInt(event.Amount1In), usdcDecimals)
	} else if event.Amount0Out.Cmp(big.NewInt(0)) > 0 {
		// WETH was output
		wethAmount := new(big.Float).Quo(new(big.Float).SetInt(event.Amount0Out), wethDecimals)
		usdValue = new(big.Float).Mul(wethAmount, ethPrice)
	} else {
		return nil, fmt.Errorf("invalid swap event: no input or output")
	}

	return usdValue, nil
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

func getPoolReserves(blockNumber uint64) (*big.Int, *big.Int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	contractAddress := common.HexToAddress(UniswapV2PairAddress)

	// Check if the contract exists at the given block number
	code, err := Client.CodeAt(ctx, contractAddress, big.NewInt(int64(blockNumber)))
	if err != nil {
		return nil, nil, LogErrorf(err, "failed to check contract code")
	}
	if len(code) == 0 {
		return nil, nil, LogErrorf(nil, "no contract found at the specified address for block %d", blockNumber)
	}

	// Create ABI
	const abiJSON = `[{"constant":true,"inputs":[],"name":"getReserves","outputs":[{"internalType":"uint112","name":"_reserve0","type":"uint112"},{"internalType":"uint112","name":"_reserve1","type":"uint112"},{"internalType":"uint32","name":"_blockTimestampLast","type":"uint32"}],"payable":false,"stateMutability":"view","type":"function"}]`
	parsedABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return nil, nil, LogErrorf(err, "failed to parse ABI")
	}

	// Pack the function call
	data, err := parsedABI.Pack("getReserves")
	if err != nil {
		return nil, nil, LogErrorf(err, "failed to pack function call")
	}

	msg := ethereum.CallMsg{
		To:   &contractAddress,
		Data: data,
	}

	result, err := Client.CallContract(ctx, msg, big.NewInt(int64(blockNumber)))
	if err != nil {
		return nil, nil, LogErrorf(err, "failed to call getReserves")
	}

	// Unpack the result
	unpacked, err := parsedABI.Unpack("getReserves", result)
	if err != nil {
		return nil, nil, LogErrorf(err, "failed to unpack result")
	}

	if len(unpacked) < 2 {
		return nil, nil, LogErrorf(nil, "unexpected result length: got %d, want at least 2", len(unpacked))
	}

	reserve0, ok := unpacked[0].(*big.Int)
	if !ok {
		return nil, nil, LogErrorf(nil, "failed to convert reserve0 to *big.Int")
	}

	reserve1, ok := unpacked[1].(*big.Int)
	if !ok {
		return nil, nil, LogErrorf(nil, "failed to convert reserve1 to *big.Int")
	}

	LogInfo("Successfully fetched pool reserves for block %d: reserve0=%s, reserve1=%s",
		blockNumber, reserve0.String(), reserve1.String())

	return reserve0, reserve1, nil
}
