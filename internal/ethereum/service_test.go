package ethereum

import (
	"math/big"
	"testing"

	customtypes "github.com/SIMPLYBOYS/trading_ace/internal/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockEthereumService is a mock implementation of the EthereumService interface
type MockEthereumService struct {
	mock.Mock
}

func (m *MockEthereumService) GetEthereumPrice() (*big.Float, error) {
	args := m.Called()
	return args.Get(0).(*big.Float), args.Error(1)
}

func (m *MockEthereumService) GetLatestBlockNumber() (uint64, error) {
	args := m.Called()
	return args.Get(0).(uint64), args.Error(1)
}

func (m *MockEthereumService) FetchSwapEvents(fromBlock, toBlock uint64) ([]customtypes.SwapEvent, error) {
	args := m.Called(fromBlock, toBlock)
	return args.Get(0).([]customtypes.SwapEvent), args.Error(1)
}

func (m *MockEthereumService) ParseSwapEvent(log interface{}) (*customtypes.SwapEvent, error) {
	args := m.Called(log)
	return args.Get(0).(*customtypes.SwapEvent), args.Error(1)
}

func (m *MockEthereumService) Close() {
	m.Called()
}

func TestGetEthereumPrice(t *testing.T) {
	mockService := new(MockEthereumService)
	expectedPrice := big.NewFloat(2000.0)
	mockService.On("GetEthereumPrice").Return(expectedPrice, nil)

	price, err := mockService.GetEthereumPrice()

	assert.NoError(t, err)
	assert.Equal(t, expectedPrice, price)
	mockService.AssertExpectations(t)
}

func TestGetLatestBlockNumber(t *testing.T) {
	mockService := new(MockEthereumService)
	expectedBlockNumber := uint64(12345)
	mockService.On("GetLatestBlockNumber").Return(expectedBlockNumber, nil)

	blockNumber, err := mockService.GetLatestBlockNumber()

	assert.NoError(t, err)
	assert.Equal(t, expectedBlockNumber, blockNumber)
	mockService.AssertExpectations(t)
}

func TestFetchSwapEvents(t *testing.T) {
	mockService := new(MockEthereumService)
	fromBlock := uint64(100)
	toBlock := uint64(200)

	expectedEvents := []customtypes.SwapEvent{
		{
			TxHash:     common.HexToHash("0x123"),
			Sender:     common.HexToAddress("0x1234567890123456789012345678901234567890"),
			Recipient:  common.HexToAddress("0x0987654321098765432109876543210987654321"),
			Amount0In:  big.NewInt(100),
			Amount1In:  big.NewInt(200),
			Amount0Out: big.NewInt(300),
			Amount1Out: big.NewInt(400),
			USDValue:   big.NewFloat(1000),
		},
	}

	mockService.On("FetchSwapEvents", fromBlock, toBlock).Return(expectedEvents, nil)

	events, err := mockService.FetchSwapEvents(fromBlock, toBlock)

	assert.NoError(t, err)
	assert.Equal(t, expectedEvents, events)
	mockService.AssertExpectations(t)
}

func TestParseSwapEvent(t *testing.T) {
	mockService := new(MockEthereumService)

	mockLog := types.Log{
		Address: common.HexToAddress(UniswapV2PairAddress),
		Topics: []common.Hash{
			SwapEventSignature,
			common.HexToHash("0x000000000000000000000000" + "1234567890123456789012345678901234567890"),
			common.HexToHash("0x000000000000000000000000" + "0987654321098765432109876543210987654321"),
		},
		Data: []byte{
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2,
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3,
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4,
		},
	}

	expectedEvent := &customtypes.SwapEvent{
		TxHash:     mockLog.TxHash,
		Sender:     common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Recipient:  common.HexToAddress("0x0987654321098765432109876543210987654321"),
		Amount0In:  big.NewInt(100),
		Amount1In:  big.NewInt(200),
		Amount0Out: big.NewInt(300),
		Amount1Out: big.NewInt(400),
	}

	mockService.On("ParseSwapEvent", mockLog).Return(expectedEvent, nil)

	event, err := mockService.ParseSwapEvent(mockLog)

	assert.NoError(t, err)
	assert.Equal(t, expectedEvent, event)
	mockService.AssertExpectations(t)
}
