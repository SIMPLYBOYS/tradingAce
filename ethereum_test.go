package main

import (
	"math/big"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
)

func TestCalculateUSDValue(t *testing.T) {
	swapEvent := &SwapEvent{
		Amount0In:  big.NewInt(1e18), // 1 WETH
		Amount1In:  big.NewInt(0),
		Amount0Out: big.NewInt(0),
		Amount1Out: big.NewInt(2000e6), // 2000 USDC
	}

	usdValue, err := calculateUSDValue(swapEvent)
	assert.NoError(t, err)

	expected := big.NewFloat(2000)
	diff := new(big.Float).Sub(expected, usdValue)
	absDiff := new(big.Float).Abs(diff)
	tolerance := big.NewFloat(1e-6)

	assert.True(t, absDiff.Cmp(tolerance) < 0, "USD value %v is not close enough to expected %v", usdValue, expected)
}

func TestProcessSwapEvents(t *testing.T) {
	// Set up mock database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()
	DB = db

	// Mock the GetCampaignConfig call
	mock.ExpectQuery("SELECT id, start_time, end_time, is_active FROM campaign_config").
		WillReturnRows(sqlmock.NewRows([]string{"id", "start_time", "end_time", "is_active"}).
			AddRow(1, time.Now(), time.Now().Add(4*7*24*time.Hour), true))

	// Mock the insert or get user query
	mock.ExpectQuery("INSERT INTO users").
		WithArgs("0xaBcDef1234567890123456789012345678901234").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	// Mock the database calls for RecordSwap
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO swap_events").
		WithArgs(1, "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", 200.0, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	logs := []types.Log{
		{
			Address: common.HexToAddress(UniswapV2PairAddress),
			Topics: []common.Hash{
				common.HexToHash("0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822"),
				common.HexToHash("0x000000000000000000000000aBcDef1234567890123456789012345678901234"),
				common.HexToHash("0x0000000000000000000000001234567890123456789012345678901234aBCDef"),
			},
			Data:   common.Hex2Bytes("0000000000000000000000000000000000000000000000000000e0e7bdc7f30900000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000bebc200"),
			TxHash: common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
		},
	}

	swapEvents := ProcessSwapEvents(logs)

	// Check if mock expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}

	// Now check the results
	assert.Len(t, swapEvents, 1)
	if len(swapEvents) > 0 {
		assert.Equal(t, common.HexToAddress("0xaBcDef1234567890123456789012345678901234"), swapEvents[0].Sender)
		assert.Equal(t, common.HexToAddress("0x1234567890123456789012345678901234aBCDef"), swapEvents[0].To)
		assert.Equal(t, big.NewInt(247285926064905), swapEvents[0].Amount0In)
		assert.True(t, swapEvents[0].Amount1In.Cmp(big.NewInt(0)) == 0, "Amount1In should be zero")
		assert.True(t, swapEvents[0].Amount0Out.Cmp(big.NewInt(0)) == 0, "Amount0Out should be zero")
		assert.Equal(t, big.NewInt(200000000), swapEvents[0].Amount1Out)
	}
}
