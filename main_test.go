package main

import (
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
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

func TestProcessSwapEvents(t *testing.T) {
	// Set up mock DB
	db, dbMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()
	DB = db

	// Set up mock expectations for RecordSwap
	dbMock.ExpectQuery("SELECT id, start_time, end_time, is_active FROM campaign_config").
		WillReturnRows(sqlmock.NewRows([]string{"id", "start_time", "end_time", "is_active"}).
			AddRow(1, time.Now().Add(-7*24*time.Hour), time.Now().Add(21*24*time.Hour), true))

	dbMock.ExpectQuery("INSERT INTO users").
		WithArgs("0x1234567890123456789012345678901234567890").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	dbMock.ExpectBegin()
	dbMock.ExpectExec("INSERT INTO swap_events").
		WithArgs(1, "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", 2000.0, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	dbMock.ExpectQuery("SELECT onboarding_completed FROM users").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"onboarding_completed"}).AddRow(false))

	dbMock.ExpectExec("UPDATE users SET onboarding_completed").
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Update the mock expectation for points_history insertion
	dbMock.ExpectExec("INSERT INTO points_history \\(user_id, points, reason, timestamp\\) VALUES \\(\\$1, 100, 'Onboarding task completed', \\$2\\)").
		WithArgs(1, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	dbMock.ExpectCommit()

	// Set up mock Ethereum client
	mockClient := new(MockEthereumClient)
	Client = mockClient

	// Mock the CallContract method for GetEthereumPrice
	ethPrice := big.NewInt(2000e8) // 2000 USD per ETH, with 8 decimal places
	mockClient.On("CallContract", mock.Anything, mock.MatchedBy(func(call ethereum.CallMsg) bool {
		return call.To.Hex() == ChainlinkETHUSDAddress
	}), mock.Anything).Return(
		append(
			make([]byte, 32), // roundId
			append(
				common.LeftPadBytes(ethPrice.Bytes(), 32), // price
				make([]byte, 32*3)...,                     // startedAt, updatedAt, answeredInRound
			)...,
		),
		nil,
	)

	// Create a sample Swap event log
	senderAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")
	recipientAddress := common.HexToAddress("0x0987654321098765432109876543210987654321")
	amount0In := big.NewInt(1e18) // 1 WETH
	amount1In := big.NewInt(0)
	amount0Out := big.NewInt(0)
	amount1Out := big.NewInt(2000e6) // 2000 USDC

	swapEventSignature := []byte("Swap(address,uint256,uint256,uint256,uint256,address)")
	swapEventTopic := crypto.Keccak256Hash(swapEventSignature)

	swapEventData := common.LeftPadBytes(amount0In.Bytes(), 32)
	swapEventData = append(swapEventData, common.LeftPadBytes(amount1In.Bytes(), 32)...)
	swapEventData = append(swapEventData, common.LeftPadBytes(amount0Out.Bytes(), 32)...)
	swapEventData = append(swapEventData, common.LeftPadBytes(amount1Out.Bytes(), 32)...)

	sampleLog := types.Log{
		Address: common.HexToAddress(UniswapV2PairAddress),
		Topics: []common.Hash{
			swapEventTopic,
			common.BytesToHash(senderAddress.Bytes()),
			common.BytesToHash(recipientAddress.Bytes()),
		},
		Data:        swapEventData,
		BlockNumber: 12345,
		TxHash:      common.HexToHash("0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
		TxIndex:     0,
		BlockHash:   common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
		Index:       0,
		Removed:     false,
	}

	logs := []types.Log{sampleLog}

	swapEvents := ProcessSwapEvents(logs)

	assert.Len(t, swapEvents, 1, "Expected 1 swap event to be processed")
	if len(swapEvents) > 0 {
		assert.Equal(t, senderAddress, swapEvents[0].Sender)
		assert.Equal(t, 0, amount0In.Cmp(swapEvents[0].Amount0In), "Amount0In should be equal")
		assert.Equal(t, 0, amount1In.Cmp(swapEvents[0].Amount1In), "Amount1In should be equal")
		assert.Equal(t, 0, amount0Out.Cmp(swapEvents[0].Amount0Out), "Amount0Out should be equal")
		assert.Equal(t, 0, amount1Out.Cmp(swapEvents[0].Amount1Out), "Amount1Out should be equal")
		assert.Equal(t, recipientAddress, swapEvents[0].To)

		// Check if USDValue is set and correct
		assert.NotNil(t, swapEvents[0].USDValue, "USDValue should not be nil")
		if swapEvents[0].USDValue != nil {
			expectedUSDValue, _ := new(big.Float).SetString("2000")
			actualUSDValue, _ := swapEvents[0].USDValue.Float64()
			expectedFloat64, _ := expectedUSDValue.Float64()
			assert.InDelta(t, expectedFloat64, actualUSDValue, 0.01, "USD Value should be close to 2000")
		}
	}

	// Check if the mock expectations were met
	mockClient.AssertExpectations(t)
	if err := dbMock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled database expectations: %s", err)
	}
}

func TestGetLeaderboard(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	DB = db

	rows := sqlmock.NewRows([]string{"address", "total_points", "onboarding_points", "share_pool_points", "total_swap_volume", "weeks_participated"}).
		AddRow("0x1234", 1000, 100, 900, 5000.0, 4).
		AddRow("0x5678", 800, 100, 700, 4000.0, 3).
		AddRow("0x9abc", 600, 100, 500, 3000.0, 2)

	mock.ExpectQuery("SELECT (.+) FROM users").
		WithArgs(10).
		WillReturnRows(rows)

	leaderboard, err := GetLeaderboard(10)
	assert.NoError(t, err)
	assert.Len(t, leaderboard, 3)

	assert.Equal(t, "0x1234", leaderboard[0]["address"])
	assert.Equal(t, 1000, leaderboard[0]["total_points"])
	assert.Equal(t, 100, leaderboard[0]["onboarding_points"])
	assert.Equal(t, 900, leaderboard[0]["share_pool_points"])
	assert.Equal(t, 5000.0, leaderboard[0]["total_swap_volume"])
	assert.Equal(t, 4, leaderboard[0]["weeks_participated"])

	assert.Equal(t, "0x5678", leaderboard[1]["address"])
	assert.Equal(t, "0x9abc", leaderboard[2]["address"])

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestGetLeaderboardAPI(t *testing.T) {
	// Set up
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	DB = db

	// Mock the leaderboard query
	rows := sqlmock.NewRows([]string{"address", "total_points", "onboarding_points", "share_pool_points", "total_swap_volume", "weeks_participated"}).
		AddRow("0x1234", 1000, 100, 900, 5000.0, 4).
		AddRow("0x5678", 800, 100, 700, 4000.0, 3)

	mock.ExpectQuery("SELECT (.+) FROM users").
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
