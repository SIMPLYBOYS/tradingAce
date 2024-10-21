package api

import (
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SIMPLYBOYS/trading_ace/internal/db"
	"github.com/SIMPLYBOYS/trading_ace/internal/errors"
	"github.com/SIMPLYBOYS/trading_ace/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDBService is a mock implementation of db.DBService
type MockDBService struct {
	mock.Mock
}

func (m *MockDBService) GetUserTasks(address string) (map[string]interface{}, error) {
	args := m.Called(address)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(map[string]interface{}), args.Error(1)
}

func (m *MockDBService) GetUserPointsHistory(address string) ([]db.PointsHistory, error) {
	args := m.Called(address)
	return args.Get(0).([]db.PointsHistory), args.Error(1)
}

func (m *MockDBService) GetLeaderboard(limit int) ([]db.LeaderboardEntry, error) {
	args := m.Called(limit)
	return args.Get(0).([]db.LeaderboardEntry), args.Error(1)
}

func (m *MockDBService) GetCampaignConfig() (db.CampaignConfig, error) {
	args := m.Called()
	return args.Get(0).(db.CampaignConfig), args.Error(1)
}

func (m *MockDBService) RecordSwapAndUpdatePoints(address string, usdValue float64, points int64, txHash string) error {
	args := m.Called(address, usdValue, points, txHash)
	return args.Error(0)
}

func (m *MockDBService) EndCampaign(campaignID int) error {
	args := m.Called(campaignID)
	return args.Error(0)
}

func (m *MockDBService) CalculateWeeklySharePoolPoints() error {
	args := m.Called()
	return args.Error(0)
}

// Add the new UpdateLeaderboard method
func (m *MockDBService) UpdateLeaderboard(address string, points int64) error {
	args := m.Called(address, points)
	return args.Error(0)
}

func (m *MockDBService) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockEthereumService is a mock implementation of ethereum.EthereumService
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

func (m *MockEthereumService) FetchSwapEvents(fromBlock, toBlock uint64) ([]types.SwapEvent, error) {
	args := m.Called(fromBlock, toBlock)
	return args.Get(0).([]types.SwapEvent), args.Error(1)
}

func (m *MockEthereumService) ParseSwapEvent(log interface{}) (*types.SwapEvent, error) {
	args := m.Called(log)
	return args.Get(0).(*types.SwapEvent), args.Error(1)
}

func (m *MockEthereumService) Close() {
	m.Called()
}

// Setup function to initialize a test Gin router with our handler
func setupTestRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/user/:address/tasks", h.GetUserTasks)
	r.GET("/user/:address/points", h.GetUserPointsHistory)
	r.GET("/ethereum/price", h.GetEthereumPrice)
	r.GET("/leaderboard", h.GetLeaderboard)
	return r
}

// Test GetUserTasks handler
func TestGetUserTasks(t *testing.T) {
	mockDB := new(MockDBService)
	mockEth := new(MockEthereumService)
	h := NewHandler(mockDB, mockEth)
	router := setupTestRouter(h)

	t.Run("Successful request", func(t *testing.T) {
		mockDB.On("GetUserTasks", "0x1234567890123456789012345678901234567890").Return(map[string]interface{}{
			"onboarding": map[string]interface{}{
				"completed": true,
				"amount":    1000,
				"points":    100,
			},
			"sharePool": map[string]interface{}{
				"completed": true,
				"amount":    500.0,
				"points":    50,
			},
		}, nil).Once()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/user/0x1234567890123456789012345678901234567890/tasks", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.NotNil(t, response["onboarding"])
		assert.NotNil(t, response["sharePool"])
	})

	t.Run("User not found", func(t *testing.T) {
		mockDB.On("GetUserTasks", "0x0000000000000000000000000000000000000000").Return(nil, &errors.NotFoundError{Resource: "user", Identifier: "0x0000000000000000000000000000000000000000"}).Once()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/user/0x0000000000000000000000000000000000000000/tasks", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "User not found", response["error"])
	})

	mockDB.AssertExpectations(t)
}

// ... other test functions remain the same ...
