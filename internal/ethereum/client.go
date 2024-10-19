package ethereum

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/SIMPLYBOYS/trading_ace/pkg/logger"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	UniswapV2PairAddress   = "0xB4e16d0168e52d35CaCD2c6185b44281Ec28C9Dc" // WETH/USDC pair
	ChainlinkETHUSDAddress = "0x5f4eC3Df9cbd43714FE2740f5E3616155c5b8419" // Ethereum Mainnet Chainlink Price Feed address for ETH/USD
)

var (
	Client       EthereumClient
	InfuraURL    string
	swapEventABI abi.ABI
)

type EthereumClient interface {
	BlockNumber(ctx context.Context) (uint64, error)
	FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error)
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
	CallContract(ctx context.Context, call ethereum.CallMsg, blockNumber *big.Int) ([]byte, error)
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
		return fmt.Errorf("failed to connect to the Ethereum client: %w", err)
	}
	logger.Info("Successfully connected to Ethereum client")
	return nil
}

func init() {
	projectID := os.Getenv("INFURA_PROJECT_ID")
	if projectID == "" {
		logger.Fatal("INFURA_PROJECT_ID environment variable is not set")
	}
	InfuraURL = fmt.Sprintf("https://mainnet.infura.io/v3/%s", projectID)

	const abiJSON = `[{"anonymous":false,"inputs":[{"indexed":true,"name":"sender","type":"address"},{"indexed":false,"name":"amount0In","type":"uint256"},{"indexed":false,"name":"amount1In","type":"uint256"},{"indexed":false,"name":"amount0Out","type":"uint256"},{"indexed":false,"name":"amount1Out","type":"uint256"},{"indexed":true,"name":"to","type":"address"}],"name":"Swap","type":"event"}]`
	var err error
	swapEventABI, err = abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		panic(err)
	}
}

func GetLatestBlockNumber() (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	return Client.BlockNumber(ctx)
}

func GetEthereumPrice() (*big.Float, error) {
	address := common.HexToAddress(ChainlinkETHUSDAddress)

	const abiJSON = `[{"inputs":[],"name":"latestRoundData","outputs":[{"internalType":"uint80","name":"roundId","type":"uint80"},{"internalType":"int256","name":"answer","type":"int256"},{"internalType":"uint256","name":"startedAt","type":"uint256"},{"internalType":"uint256","name":"updatedAt","type":"uint256"},{"internalType":"uint80","name":"answeredInRound","type":"uint80"}],"stateMutability":"view","type":"function"}]`
	parsedABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	data, err := parsedABI.Pack("latestRoundData")
	if err != nil {
		return nil, fmt.Errorf("failed to pack data for latestRoundData function call: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := Client.CallContract(ctx, ethereum.CallMsg{
		To:   &address,
		Data: data,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call latestRoundData function: %w", err)
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
		return nil, fmt.Errorf("failed to unpack result: %w", err)
	}

	// Chainlink price feeds for ETH/USD use 8 decimal places
	ethPrice := new(big.Float).SetInt(answer)
	ethPrice = ethPrice.Quo(ethPrice, big.NewFloat(1e8))

	return ethPrice, nil
}
