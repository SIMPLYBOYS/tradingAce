package main

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockEthereumClient is a mock of the EthereumClient interface
type MockEthereumClient struct {
	mock.Mock
}

func (m *MockEthereumClient) CodeAt(ctx context.Context, contract common.Address, blockNumber *big.Int) ([]byte, error) {
	args := m.Called(ctx, contract, blockNumber)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockEthereumClient) CallContract(ctx context.Context, call ethereum.CallMsg, blockNumber *big.Int) ([]byte, error) {
	args := m.Called(ctx, call, blockNumber)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockEthereumClient) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	args := m.Called(ctx, number)
	return args.Get(0).(*types.Header), args.Error(1)
}

func (m *MockEthereumClient) BlockNumber(ctx context.Context) (uint64, error) {
	args := m.Called(ctx)
	return args.Get(0).(uint64), args.Error(1)
}

func (m *MockEthereumClient) FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	args := m.Called(ctx, q)
	return args.Get(0).([]types.Log), args.Error(1)
}

func TestInitEthereumClient(t *testing.T) {
	mockClient := new(MockEthereumClient)

	mockClientCreator := func(url string) (EthereumClient, error) {
		return mockClient, nil
	}

	err := InitEthereumClient(mockClientCreator)
	assert.NoError(t, err)
	assert.Equal(t, mockClient, Client)
}

func TestCalculateUSDValue(t *testing.T) {
	swapEvent := &SwapEvent{
		Amount0In:  big.NewInt(1e18), // 1 WETH
		Amount1In:  big.NewInt(0),
		Amount0Out: big.NewInt(0),
		Amount1Out: big.NewInt(2000e6), // 2000 USDC
	}

	// Mock reserve values (100 WETH, 200,000 USDC)
	reserve0 := big.NewInt(100).Mul(big.NewInt(100), big.NewInt(1000000000000000000)) // 100 WETH
	reserve1 := big.NewInt(200000e6)                                                  // 200,000 USDC (assuming 6 decimals)

	usdValue, err := calculateUSDValue(swapEvent, reserve0, reserve1)
	assert.NoError(t, err)

	// Print intermediate values for debugging
	fmt.Printf("WETH in: %v\n", new(big.Float).SetInt(swapEvent.Amount0In))
	fmt.Printf("USDC out: %v\n", new(big.Float).SetInt(swapEvent.Amount1Out))
	fmt.Printf("Reserve WETH: %v\n", new(big.Float).SetInt(reserve0))
	fmt.Printf("Reserve USDC: %v\n", new(big.Float).SetInt(reserve1))
	fmt.Printf("Calculated USD value: %v\n", usdValue)

	expected := big.NewFloat(2000)
	diff := new(big.Float).Sub(expected, usdValue)
	absDiff := new(big.Float).Abs(diff)
	tolerance := big.NewFloat(1e-6)

	assert.True(t, absDiff.Cmp(tolerance) < 0, "USD value %v is not close enough to expected %v", usdValue, expected)
}

func TestGetPoolReserves(t *testing.T) {
	mockClient := new(MockEthereumClient)
	Client = mockClient

	blockNumber := uint64(12345)
	expectedReserve0 := big.NewInt(1000000)
	expectedReserve1 := big.NewInt(2000000)

	// Mock the CodeAt call
	mockClient.On("CodeAt", mock.Anything, mock.Anything, mock.Anything).Return([]byte{1}, nil)

	// Mock the CallContract call
	mockClient.On("CallContract", mock.Anything, mock.Anything, mock.Anything).Return(
		append(
			append(
				common.LeftPadBytes(expectedReserve0.Bytes(), 32),
				common.LeftPadBytes(expectedReserve1.Bytes(), 32)...,
			),
			common.LeftPadBytes(big.NewInt(int64(time.Now().Unix())).Bytes(), 32)...,
		),
		nil,
	)

	reserve0, reserve1, err := getPoolReserves(blockNumber)

	assert.NoError(t, err)
	assert.Equal(t, expectedReserve0, reserve0)
	assert.Equal(t, expectedReserve1, reserve1)

	mockClient.AssertExpectations(t)
}

func TestGetLatestBlockNumber(t *testing.T) {
	mockClient := new(MockEthereumClient)
	Client = mockClient

	expectedBlockNumber := uint64(12345)

	mockClient.On("HeaderByNumber", mock.Anything, (*big.Int)(nil)).Return(&types.Header{Number: big.NewInt(int64(expectedBlockNumber))}, nil)

	blockNumber, err := GetLatestBlockNumber()

	assert.NoError(t, err)
	assert.Equal(t, expectedBlockNumber, blockNumber)

	mockClient.AssertExpectations(t)
}

func TestFetchSwapEvents(t *testing.T) {
	mockClient := new(MockEthereumClient)
	Client = mockClient

	fromBlock := big.NewInt(12340)
	toBlock := big.NewInt(12345)

	expectedLogs := []types.Log{
		{
			Address: common.HexToAddress(UniswapV2PairAddress),
			Topics:  []common.Hash{common.HexToHash("0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822")},
			Data:    []byte{1, 2, 3, 4},
		},
	}

	mockClient.On("FilterLogs", mock.Anything, mock.Anything).Return(expectedLogs, nil)

	logs, err := FetchSwapEvents(fromBlock, toBlock)

	assert.NoError(t, err)
	assert.Equal(t, expectedLogs, logs)

	mockClient.AssertExpectations(t)
}
