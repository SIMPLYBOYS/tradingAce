package main

import (
	"math/big"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
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
	// Mock DB for RecordSwap function
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()
	DB = db

	// Set up mock expectations for RecordSwap
	mock.ExpectQuery("SELECT id, start_time, end_time, is_active FROM campaign_config").
		WillReturnRows(sqlmock.NewRows([]string{"id", "start_time", "end_time", "is_active"}).
			AddRow(1, time.Now().Add(-7*24*time.Hour), time.Now().Add(21*24*time.Hour), true))
	mock.ExpectQuery("INSERT INTO users").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO swap_events").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT onboarding_completed FROM users").
		WillReturnRows(sqlmock.NewRows([]string{"onboarding_completed"}).AddRow(false))
	mock.ExpectExec("UPDATE users SET onboarding_completed").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO points_history").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

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

	// Mock getPoolReservesWrapper
	originalWrapper := getPoolReservesWrapper
	getPoolReservesWrapper = func(blockNumber uint64) (*big.Int, *big.Int, error) {
		return big.NewInt(100).Mul(big.NewInt(100), big.NewInt(1e18)), big.NewInt(200000e6), nil
	}
	defer func() { getPoolReservesWrapper = originalWrapper }()

	swapEvents := ProcessSwapEvents(logs)

	assert.Len(t, swapEvents, 1)
	assert.Equal(t, senderAddress, swapEvents[0].Sender)
	assert.Equal(t, 0, amount0In.Cmp(swapEvents[0].Amount0In), "Amount0In should be equal")
	assert.Equal(t, 0, amount1In.Cmp(swapEvents[0].Amount1In), "Amount1In should be equal")
	assert.Equal(t, 0, amount0Out.Cmp(swapEvents[0].Amount0Out), "Amount0Out should be equal")
	assert.Equal(t, 0, amount1Out.Cmp(swapEvents[0].Amount1Out), "Amount1Out should be equal")
	assert.Equal(t, recipientAddress, swapEvents[0].To)

	// Check if USDValue is set
	assert.NotNil(t, swapEvents[0].USDValue, "USDValue should not be nil")

	if swapEvents[0].USDValue != nil {
		// Additional check for USD value
		expectedUSDValue, _ := new(big.Float).SetString("2000")
		expectedFloat64, _ := expectedUSDValue.Float64()
		actualFloat64, _ := swapEvents[0].USDValue.Float64()
		assert.InDelta(t, expectedFloat64, actualFloat64, 0.01, "USD Value should be approximately 2000")
	}

	// Check if the mock expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
