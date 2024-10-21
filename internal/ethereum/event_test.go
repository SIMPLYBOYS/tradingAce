package ethereum

import (
	"math/big"
	"testing"

	"github.com/SIMPLYBOYS/trading_ace/internal/db"
	"github.com/SIMPLYBOYS/trading_ace/internal/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockWebSocketManager is a mock implementation of the WebSocketManager interface
type MockWebSocketManager struct {
	mock.Mock
}

func (m *MockWebSocketManager) BroadcastSwapEvent(event *types.SwapEvent) error {
	args := m.Called(event)
	return args.Error(0)
}

func (m *MockWebSocketManager) BroadcastUserPointsUpdate(address string, points int64) error {
	args := m.Called(address, points)
	return args.Error(0)
}

func (m *MockWebSocketManager) BroadcastLeaderboardUpdate(leaderboard []map[string]interface{}) error {
	args := m.Called(leaderboard)
	return args.Error(0)
}

// MockDBService is a mock implementation of the db.DBService interface
type MockDBService struct {
	mock.Mock
}

func (m *MockDBService) RecordSwapAndUpdatePoints(address string, usdValue float64, points int64, txHash string) error {
	args := m.Called(address, usdValue, points, txHash)
	return args.Error(0)
}

func (m *MockDBService) UpdateLeaderboard(address string, points int64) error {
	args := m.Called(address, points)
	return args.Error(0)
}

func (m *MockDBService) GetLeaderboard(limit int) ([]db.LeaderboardEntry, error) {
	args := m.Called(limit)
	return args.Get(0).([]db.LeaderboardEntry), args.Error(1)
}

func (m *MockDBService) GetUserTasks(address string) (map[string]interface{}, error) {
	args := m.Called(address)
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockDBService) GetUserPointsHistory(address string) ([]db.PointsHistory, error) {
	args := m.Called(address)
	return args.Get(0).([]db.PointsHistory), args.Error(1)
}

func (m *MockDBService) GetCampaignConfig() (db.CampaignConfig, error) {
	args := m.Called()
	return args.Get(0).(db.CampaignConfig), args.Error(1)
}

func (m *MockDBService) EndCampaign(campaignID int) error {
	args := m.Called(campaignID)
	return args.Error(0)
}

func (m *MockDBService) CalculateWeeklySharePoolPoints() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDBService) Close() error {
	args := m.Called()
	return args.Error(0)
}

// Implement other methods of db.DBService as needed...

func TestProcessSwapEvents(t *testing.T) {
	mockWS := new(MockWebSocketManager)
	mockDB := new(MockDBService)

	events := []types.SwapEvent{
		{
			TxHash:     common.HexToHash("0x123"),
			Sender:     common.HexToAddress("0x456"),
			Recipient:  common.HexToAddress("0x789"),
			Amount0In:  big.NewInt(100),
			Amount1In:  big.NewInt(200),
			Amount0Out: big.NewInt(300),
			Amount1Out: big.NewInt(400),
			USDValue:   big.NewFloat(1000),
		},
	}

	mockWS.On("BroadcastSwapEvent", mock.Anything).Return(nil)
	mockWS.On("BroadcastUserPointsUpdate", mock.Anything, mock.Anything).Return(nil)
	mockDB.On("RecordSwapAndUpdatePoints", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockDB.On("UpdateLeaderboard", mock.Anything, mock.Anything).Return(nil)
	mockDB.On("GetLeaderboard", 10).Return([]db.LeaderboardEntry{}, nil)
	mockWS.On("BroadcastLeaderboardUpdate", mock.Anything).Return(nil)

	// Add expectations for other methods if they are called in ProcessSwapEvents
	mockDB.On("GetUserTasks", mock.Anything).Return(map[string]interface{}{}, nil).Maybe()
	mockDB.On("GetUserPointsHistory", mock.Anything).Return([]db.PointsHistory{}, nil).Maybe()
	mockDB.On("GetCampaignConfig").Return(db.CampaignConfig{}, nil).Maybe()
	mockDB.On("EndCampaign", mock.Anything).Return(nil).Maybe()
	mockDB.On("CalculateWeeklySharePoolPoints").Return(nil).Maybe()
	mockDB.On("Close").Return(nil).Maybe()

	err := ProcessSwapEvents(events, mockWS, mockDB)

	assert.NoError(t, err)
	mockWS.AssertExpectations(t)
	mockDB.AssertExpectations(t)
}

func TestCalculatePointsForSwap(t *testing.T) {
	testCases := []struct {
		name     string
		usdValue *big.Float
		expected int64
	}{
		{"100 USD", big.NewFloat(100), 10},
		{"1000 USD", big.NewFloat(1000), 100},
		{"5 USD", big.NewFloat(5), 1}, // Should return minimum 1 point
		{"0 USD", big.NewFloat(0), 1}, // Should return minimum 1 point
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			points := calculatePointsForSwap(tc.usdValue)
			assert.Equal(t, tc.expected, points)
		})
	}
}

func TestCalculateSwapVolume(t *testing.T) {
	event := &types.SwapEvent{
		Amount0In:  big.NewInt(100),
		Amount0Out: big.NewInt(200),
		Amount1In:  big.NewInt(300),
		Amount1Out: big.NewInt(400),
	}

	volume := CalculateSwapVolume(event)
	expected := big.NewInt(300) // 100 + 200

	assert.Equal(t, expected, volume)
}
