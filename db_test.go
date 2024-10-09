package main

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestGetCampaignConfig(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	DB = db

	rows := sqlmock.NewRows([]string{"id", "start_time", "end_time", "is_active"}).
		AddRow(1, time.Now(), time.Now().Add(4*7*24*time.Hour), true)

	mock.ExpectQuery("SELECT id, start_time, end_time, is_active FROM campaign_config").
		WillReturnRows(rows)

	config, err := GetCampaignConfig()
	assert.NoError(t, err)
	assert.True(t, config.IsActive)
	assert.Equal(t, 1, config.ID)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestRecordSwap(t *testing.T) {
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
		WithArgs("0x1234567890123456789012345678901234567890").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO swap_events").
		WithArgs(1, "0xabcdef1234567890", 1000.0, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT onboarding_completed FROM users").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"onboarding_completed"}).AddRow(false))
	mock.ExpectExec("UPDATE users SET onboarding_completed").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO points_history").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = RecordSwap("0x1234567890123456789012345678901234567890", 1000.0, "0xabcdef1234567890")
	assert.NoError(t, err)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestCalculateWeeklySharePoolPoints(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	DB = db

	mock.ExpectQuery("SELECT id, start_time, end_time, is_active FROM campaign_config").
		WillReturnRows(sqlmock.NewRows([]string{"id", "start_time", "end_time", "is_active"}).
			AddRow(1, time.Now().Add(-7*24*time.Hour), time.Now().Add(21*24*time.Hour), true))

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COALESCE").
		WillReturnRows(sqlmock.NewRows([]string{"total_volume"}).AddRow(10000.0))
	mock.ExpectQuery("SELECT u.id, u.address, COALESCE").
		WillReturnRows(sqlmock.NewRows([]string{"id", "address", "volume"}).
			AddRow(1, "0x1234", 5000.0).
			AddRow(2, "0x5678", 5000.0))
	mock.ExpectExec("INSERT INTO points_history").
		WithArgs(1, 5000, "Weekly Share Pool Task", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO points_history").
		WithArgs(2, 5000, "Weekly Share Pool Task", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	err = CalculateWeeklySharePoolPoints()
	assert.NoError(t, err)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
