package main

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetUserTasks(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	DB = db

	// Mock the user query
	userRows := sqlmock.NewRows([]string{"id", "onboarding_completed", "onboarding_points", "onboarding_amount"}).
		AddRow(1, true, 100, 1000.0)

	mock.ExpectQuery("SELECT id, onboarding_completed, onboarding_points, COALESCE").
		WithArgs("0x1234567890123456789012345678901234567890").
		WillReturnRows(userRows)

	// Mock the swap events query
	swapRows := sqlmock.NewRows([]string{"total_amount", "total_points"}).
		AddRow(5000.0, 500)

	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(amount_usd\\), 0\\), COALESCE").
		WithArgs(1).
		WillReturnRows(swapRows)

	// Mock the campaign config query
	configRows := sqlmock.NewRows([]string{"id", "start_time", "end_time", "is_active"}).
		AddRow(1, time.Now().Add(-7*24*time.Hour), time.Now().Add(21*24*time.Hour), true)

	mock.ExpectQuery("SELECT id, start_time, end_time, is_active FROM campaign_config").
		WillReturnRows(configRows)

	// Mock the latest distribution query
	distRows := sqlmock.NewRows([]string{"latest_distribution"}).
		AddRow(time.Now().Add(-8 * 24 * time.Hour))

	mock.ExpectQuery("SELECT COALESCE\\(MAX\\(timestamp\\), \\$1\\)").
		WithArgs(sqlmock.AnyArg(), 1).
		WillReturnRows(distRows)

	tasks, err := GetUserTasks("0x1234567890123456789012345678901234567890")
	assert.NoError(t, err)

	assert.NotNil(t, tasks)
	assert.Equal(t, true, tasks["onboarding"].(map[string]interface{})["completed"])
	assert.Equal(t, 100, tasks["onboarding"].(map[string]interface{})["points"])
	assert.Equal(t, 1000.0, tasks["onboarding"].(map[string]interface{})["amount"])
	assert.Equal(t, 5000.0, tasks["sharePool"].(map[string]interface{})["amount"])
	assert.Equal(t, 500.0, tasks["sharePool"].(map[string]interface{})["points"])
	assert.True(t, tasks["sharePool"].(map[string]interface{})["eligible"].(bool))
	assert.NotNil(t, tasks["campaign"])

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestGetUserPointsHistory(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	DB = db

	rows := sqlmock.NewRows([]string{"points", "reason", "timestamp"}).
		AddRow(100, "Onboarding", time.Now()).
		AddRow(200, "Weekly Share", time.Now())

	mock.ExpectQuery("SELECT points, reason, timestamp FROM points_history").
		WithArgs("0x1234567890123456789012345678901234567890").
		WillReturnRows(rows)

	history, err := GetUserPointsHistory("0x1234567890123456789012345678901234567890")
	assert.NoError(t, err)
	assert.Len(t, history, 2)
	assert.Equal(t, 100, history[0]["points"])
	assert.Equal(t, "Onboarding", history[0]["reason"])
}

func TestAwardOnboardingPoints(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	DB = db

	mock.ExpectBegin()

	mock.ExpectExec("UPDATE users SET onboarding_completed = true, onboarding_points = 100").
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec("INSERT INTO points_history").
		WithArgs(1, 100, "Onboarding task completed", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	err = AwardOnboardingPoints(1)
	assert.NoError(t, err)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestSetCampaignConfig(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	DB = db

	startTime := time.Now()
	endTime := startTime.Add(4 * 7 * 24 * time.Hour)

	mock.ExpectExec("INSERT INTO campaign_config").
		WithArgs(startTime, endTime, true).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = SetCampaignConfig(startTime)
	assert.NoError(t, err)
}

func TestCalculateSwapVolume(t *testing.T) {
	event := &SwapEvent{
		Amount0In:  big.NewInt(1000000),
		Amount0Out: big.NewInt(500000),
	}

	volume := CalculateSwapVolume(event)
	assert.Equal(t, big.NewInt(1500000), volume)
}

type MockWebSocketManager struct {
	mock.Mock
}

// Ensure MockWebSocketManager implements WebSocketManagerInterface
var _ WebSocketManagerInterface = (*MockWebSocketManager)(nil)

// Implement all methods of WebSocketManagerInterface
func (m *MockWebSocketManager) Run(ctx context.Context) {
	m.Called(ctx)
}

func (m *MockWebSocketManager) Stop() {
	m.Called()
}

func (m *MockWebSocketManager) BroadcastToTopic(topic string, message []byte) {
	m.Called(topic, message)
}

func (m *MockWebSocketManager) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	m.Called(w, r)
}

func (m *MockWebSocketManager) BroadcastLeaderboardUpdate(leaderboard []map[string]interface{}) {
	m.Called(leaderboard)
}

func (m *MockWebSocketManager) BroadcastUserPointsUpdate(address string, points int64) {
	m.Called(address, points)
}

func (m *MockWebSocketManager) BroadcastSwapEvent(event *SwapEvent) {
	m.Called(event)
}

func (m *MockWebSocketManager) BroadcastCampaignUpdate(campaignInfo map[string]interface{}) {
	m.Called(campaignInfo)
}

// func TestProcessSwapEvents(t *testing.T) {
// 	// Set up mock DB
// 	db, dbMock, err := sqlmock.New()
// 	if err != nil {
// 		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
// 	}
// 	defer db.Close()
// 	DB = db

// 	// Set up mock Ethereum client
// 	mockClient := new(MockEthereumClient)
// 	Client = mockClient

// 	// Mock getPoolReserves
// 	mockClient.On("CodeAt", mock.Anything, mock.Anything, mock.Anything).Return([]byte{1}, nil)
// 	mockClient.On("CallContract", mock.Anything, mock.Anything, mock.Anything).Return(
// 		append(
// 			common.LeftPadBytes(big.NewInt(1e18).Bytes(), 32),
// 			append(
// 				common.LeftPadBytes(big.NewInt(2000e6).Bytes(), 32),
// 				common.LeftPadBytes(big.NewInt(int64(time.Now().Unix())).Bytes(), 32)...,
// 			)...,
// 		),
// 		nil,
// 	)

// 	// Mock calculateUSDValue
// 	oldCalculateUSDValue := calculateUSDValue
// 	calculateUSDValue = func(event *SwapEvent, reserve0, reserve1 *big.Int) (*big.Float, error) {
// 		return big.NewFloat(2000), nil
// 	}
// 	defer func() { calculateUSDValue = oldCalculateUSDValue }()

// 	// Set up expectations for database operations
// 	dbMock.ExpectQuery("SELECT onboarding_completed FROM users WHERE address = \\$1").
// 		WithArgs("0x1234567890123456789012345678901234567890").
// 		WillReturnRows(sqlmock.NewRows([]string{"onboarding_completed"}).AddRow(false))

// 	dbMock.ExpectBegin()
// 	dbMock.ExpectQuery("INSERT INTO users").
// 		WithArgs("0x1234567890123456789012345678901234567890", 200).
// 		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

// 	dbMock.ExpectExec("INSERT INTO swap_events").
// 		WithArgs(1, "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", 2000.0, sqlmock.AnyArg()).
// 		WillReturnResult(sqlmock.NewResult(1, 1))

// 	dbMock.ExpectExec("INSERT INTO points_history").
// 		WithArgs(1, 200, "Swap event", sqlmock.AnyArg()).
// 		WillReturnResult(sqlmock.NewResult(1, 1))

// 	dbMock.ExpectCommit()

// 	// Mock UpdateLeaderboard
// 	dbMock.ExpectExec("INSERT INTO leaderboard").
// 		WithArgs("0x1234567890123456789012345678901234567890", 200).
// 		WillReturnResult(sqlmock.NewResult(1, 1))

// 	// Mock GetLeaderboard
// 	dbMock.ExpectQuery("SELECT address, points FROM leaderboard ORDER BY points DESC LIMIT \\$1").
// 		WithArgs(10).
// 		WillReturnRows(sqlmock.NewRows([]string{"address", "points"}).
// 			AddRow("0x1234567890123456789012345678901234567890", 200))

// 	// Set up mock WebSocket manager
// 	mockWSManager := new(MockWebSocketManager)
// 	mockWSManager.On("BroadcastSwapEvent", mock.Anything).Return()
// 	mockWSManager.On("BroadcastUserPointsUpdate", "0x1234567890123456789012345678901234567890", int64(200)).Return()
// 	mockWSManager.On("BroadcastLeaderboardUpdate", mock.Anything).Return()

// 	// Create a sample Swap event log
// 	senderAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")
// 	recipientAddress := common.HexToAddress("0x0987654321098765432109876543210987654321")
// 	amount0In := big.NewInt(1e18) // 1 WETH
// 	amount1In := big.NewInt(0)
// 	amount0Out := big.NewInt(0)
// 	amount1Out := big.NewInt(2000e6) // 2000 USDC

// 	swapEventSignature := []byte("Swap(address,uint256,uint256,uint256,uint256,address)")
// 	swapEventTopic := crypto.Keccak256Hash(swapEventSignature)

// 	swapEventData := common.LeftPadBytes(amount0In.Bytes(), 32)
// 	swapEventData = append(swapEventData, common.LeftPadBytes(amount1In.Bytes(), 32)...)
// 	swapEventData = append(swapEventData, common.LeftPadBytes(amount0Out.Bytes(), 32)...)
// 	swapEventData = append(swapEventData, common.LeftPadBytes(amount1Out.Bytes(), 32)...)

// 	sampleLog := types.Log{
// 		Address: common.HexToAddress(UniswapV2PairAddress),
// 		Topics: []common.Hash{
// 			swapEventTopic,
// 			common.BytesToHash(senderAddress.Bytes()),
// 			common.BytesToHash(recipientAddress.Bytes()),
// 		},
// 		Data:        swapEventData,
// 		BlockNumber: 12345,
// 		TxHash:      common.HexToHash("0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
// 		TxIndex:     0,
// 		BlockHash:   common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
// 		Index:       0,
// 		Removed:     false,
// 	}

// 	logs := []types.Log{sampleLog}

// 	// Call the function being tested
// 	ProcessSwapEvents(logs, mockWSManager)

// 	// Assert that all expected database calls were made
// 	if err := dbMock.ExpectationsWereMet(); err != nil {
// 		t.Errorf("there were unfulfilled database expectations: %s", err)
// 	}

// 	// Assert that all expected Ethereum client calls were made
// 	mockClient.AssertExpectations(t)

//		// Assert that all expected WebSocket broadcasts were made
//		mockWSManager.AssertExpectations(t)
//	}

func TestGetLeaderboard(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	DB = db

	rows := sqlmock.NewRows([]string{"address", "points"}).
		AddRow("0x1234", 1000).
		AddRow("0x5678", 800).
		AddRow("0x9abc", 600)

	mock.ExpectQuery("SELECT address, points FROM leaderboard ORDER BY points DESC LIMIT \\$1").
		WithArgs(10).
		WillReturnRows(rows)

	leaderboard, err := GetLeaderboard(10)
	assert.NoError(t, err)
	assert.Len(t, leaderboard, 3)

	assert.Equal(t, "0x1234", leaderboard[0]["address"])
	assert.Equal(t, int64(1000), leaderboard[0]["points"])

	assert.Equal(t, "0x5678", leaderboard[1]["address"])
	assert.Equal(t, int64(800), leaderboard[1]["points"])

	assert.Equal(t, "0x9abc", leaderboard[2]["address"])
	assert.Equal(t, int64(600), leaderboard[2]["points"])

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestGetLeaderboardAPI(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	DB = db

	// Mock the leaderboard query
	rows := sqlmock.NewRows([]string{"address", "points"}).
		AddRow("0x1234", 1000).
		AddRow("0x5678", 800)

	mock.ExpectQuery("SELECT address, points FROM leaderboard ORDER BY points DESC LIMIT \\$1").
		WithArgs(10).
		WillReturnRows(rows)

	// Mock the campaign config query
	configRows := sqlmock.NewRows([]string{"id", "start_time", "end_time", "is_active"}).
		AddRow(1, time.Now().Add(-14*24*time.Hour), time.Now().Add(14*24*time.Hour), true)

	mock.ExpectQuery("SELECT id, start_time, end_time, is_active FROM campaign_config").
		WillReturnRows(configRows)

	// Create a mock WebSocketManager
	mockWSManager := &WebSocketManager{
		clients:    make(map[*WSClient]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
	}

	// Set up the Gin router with the mock WebSocketManager
	router := SetupRouter(mockWSManager)

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/leaderboard", nil)
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, 200, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	leaderboard, ok := response["leaderboard"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, leaderboard, 2)

	campaignInfo, ok := response["campaign_info"].(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, campaignInfo["start_time"])
	assert.NotNil(t, campaignInfo["end_time"])
	assert.Equal(t, true, campaignInfo["is_active"])
	assert.Equal(t, float64(4), campaignInfo["total_weeks"])
	assert.Equal(t, float64(3), campaignInfo["current_week"])

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
