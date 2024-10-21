package db

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testDBService is a helper struct to hold common test dependencies
type testDBService struct {
	mock   sqlmock.Sqlmock
	db     *sql.DB
	svc    *DBServiceImpl
	assert *assert.Assertions
}

// Mock implementation of DBOperations
type mockDBOperations struct {
	openFunc          func(driverName, dataSourceName string) (*sql.DB, error)
	runMigrationsFunc func(db *sql.DB) error
}

func (m *mockDBOperations) Open(driverName, dataSourceName string) (*sql.DB, error) {
	return m.openFunc(driverName, dataSourceName)
}

func (m *mockDBOperations) RunMigrations(db *sql.DB) error {
	return m.runMigrationsFunc(db)
}

// setupTestDB sets up a mock database and returns a testDBService
func setupTestDB(t *testing.T) *testDBService {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	return &testDBService{
		mock:   mock,
		db:     db,
		svc:    &DBServiceImpl{db: db},
		assert: assert.New(t),
	}
}

func (tdb *testDBService) close() {
	tdb.db.Close()
}

func TestNewDBService(t *testing.T) {
	// Mock environment variables
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "testuser")
	t.Setenv("DB_PASSWORD", "testpass")
	t.Setenv("DB_NAME", "testdb")

	// Create a mock database
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	// Create mock DBOperations
	mockOps := &mockDBOperations{
		openFunc: func(driverName, dataSourceName string) (*sql.DB, error) {
			return mockDB, nil
		},
		runMigrationsFunc: func(db *sql.DB) error {
			return nil
		},
	}

	// Set expectation for db.Ping()
	mock.ExpectPing()

	// Call the function under test
	service, err := NewDBService(mockOps)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, service)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserTasks(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.close()

	testCases := []struct {
		name          string
		address       string
		mockSetup     func()
		expectedTasks map[string]interface{}
		expectedError error
	}{
		{
			name:    "Successful retrieval",
			address: "0x1234567890123456789012345678901234567890",
			mockSetup: func() {
				tdb.mock.ExpectQuery("SELECT id, onboarding_completed, onboarding_points").
					WithArgs("0x1234567890123456789012345678901234567890").
					WillReturnRows(sqlmock.NewRows([]string{"id", "onboarding_completed", "onboarding_points", "total_swap_amount"}).
						AddRow(1, true, 100, 1000.0))

				tdb.mock.ExpectQuery("SELECT COALESCE").
					WithArgs(1, "0x1234567890123456789012345678901234567890").
					WillReturnRows(sqlmock.NewRows([]string{"amount", "points"}).AddRow(500.0, 50))
			},
			expectedTasks: map[string]interface{}{
				"onboarding": map[string]interface{}{
					"completed": true,
					"amount":    float64(1000),
					"points":    100,
				},
				"sharePool": map[string]interface{}{
					"completed": true,
					"amount":    float64(500),
					"points":    float64(50),
				},
			},
			expectedError: nil,
		},
		{
			name:    "User not found",
			address: "0x0000000000000000000000000000000000000000",
			mockSetup: func() {
				tdb.mock.ExpectQuery("SELECT id, onboarding_completed, onboarding_points").
					WithArgs("0x0000000000000000000000000000000000000000").
					WillReturnError(sql.ErrNoRows)
			},
			expectedTasks: nil,
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.mockSetup()

			tasks, err := tdb.svc.GetUserTasks(tc.address)

			if tc.expectedError != nil {
				tdb.assert.Error(err)
				tdb.assert.Equal(tc.expectedError, err)
			} else {
				tdb.assert.NoError(err)
				if tc.expectedTasks == nil {
					tdb.assert.Nil(tasks)
				} else {
					tdb.assert.Equal(tc.expectedTasks, tasks)
				}
			}

			tdb.assert.NoError(tdb.mock.ExpectationsWereMet())
		})
	}
}

func TestGetUserPointsHistory(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.close()

	address := "0x1234567890123456789012345678901234567890"
	now := time.Now()

	tdb.mock.ExpectQuery("SELECT id FROM users WHERE address").
		WithArgs(address).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	tdb.mock.ExpectQuery("SELECT points, reason, timestamp").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"points", "reason", "timestamp"}).
			AddRow(100, "Swap", now).
			AddRow(50, "Weekly Share Pool", now.Add(-24*time.Hour)))

	history, err := tdb.svc.GetUserPointsHistory(address)

	tdb.assert.NoError(err)
	tdb.assert.Len(history, 2)
	tdb.assert.Equal(int64(100), history[0].Points)
	tdb.assert.Equal("Swap", history[0].Reason)
	tdb.assert.Equal(now.Unix(), history[0].Timestamp.Unix())
	tdb.assert.Equal(int64(50), history[1].Points)
	tdb.assert.Equal("Weekly Share Pool", history[1].Reason)
	tdb.assert.Equal(now.Add(-24*time.Hour).Unix(), history[1].Timestamp.Unix())

	tdb.assert.NoError(tdb.mock.ExpectationsWereMet())
}

func TestGetCampaignConfig(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.close()

	startTime := time.Now()
	endTime := startTime.Add(4 * 7 * 24 * time.Hour)

	tdb.mock.ExpectQuery("SELECT id, start_time, end_time, is_active").
		WillReturnRows(sqlmock.NewRows([]string{"id", "start_time", "end_time", "is_active"}).
			AddRow(1, startTime, endTime, true))

	config, err := tdb.svc.GetCampaignConfig()

	tdb.assert.NoError(err)
	tdb.assert.Equal(1, config.ID)
	tdb.assert.Equal(startTime.Unix(), config.StartTime.Unix())
	tdb.assert.Equal(endTime.Unix(), config.EndTime.Unix())
	tdb.assert.True(config.IsActive)

	tdb.assert.NoError(tdb.mock.ExpectationsWereMet())
}

func TestEndCampaign(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.close()

	campaignID := 1

	// Update the expectation to match the actual query
	tdb.mock.ExpectExec("UPDATE campaign_config").
		WithArgs(campaignID). // Only one argument: the campaign ID
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := tdb.svc.EndCampaign(campaignID)

	tdb.assert.NoError(err)
	tdb.assert.NoError(tdb.mock.ExpectationsWereMet())
}
func TestCalculateWeeklySharePoolPoints(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.close()

	tdb.mock.ExpectBegin()
	tdb.mock.ExpectQuery("SELECT COALESCE").
		WillReturnRows(sqlmock.NewRows([]string{"total_volume"}).AddRow(10000.0))

	tdb.mock.ExpectQuery("SELECT user_address, SUM").
		WillReturnRows(sqlmock.NewRows([]string{"user_address", "user_volume"}).
			AddRow("0x1111", 5000.0).
			AddRow("0x2222", 3000.0).
			AddRow("0x3333", 2000.0))

	// Expect updates for each user
	for _, user := range []struct {
		address string
		points  int64
	}{
		{"0x1111", 5000},
		{"0x2222", 3000},
		{"0x3333", 2000},
	} {
		tdb.mock.ExpectExec("UPDATE users").
			WithArgs(user.address, user.points).
			WillReturnResult(sqlmock.NewResult(1, 1))
		tdb.mock.ExpectExec("INSERT INTO points_history").
			WithArgs(user.address, user.points, "Weekly Share Pool"). // Removed sqlmock.AnyArg()
			WillReturnResult(sqlmock.NewResult(1, 1))
	}

	tdb.mock.ExpectCommit()

	err := tdb.svc.CalculateWeeklySharePoolPoints()

	tdb.assert.NoError(err)
	tdb.assert.NoError(tdb.mock.ExpectationsWereMet())
}

func TestUpdateLeaderboard(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.close()

	testCases := []struct {
		name          string
		address       string
		points        int64
		expectError   bool
		expectedError string
	}{
		{
			name:    "Successful update",
			address: "0x1234567890123456789012345678901234567890",
			points:  100,
		},
		{
			name:          "Database error",
			address:       "0x1234567890123456789012345678901234567890",
			points:        100,
			expectError:   true,
			expectedError: "failed to update leaderboard",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectError {
				tdb.mock.ExpectExec("INSERT INTO leaderboard").
					WithArgs(tc.address, tc.points).
					WillReturnError(fmt.Errorf("database error"))
			} else {
				tdb.mock.ExpectExec("INSERT INTO leaderboard").
					WithArgs(tc.address, tc.points).
					WillReturnResult(sqlmock.NewResult(1, 1))
			}

			err := tdb.svc.UpdateLeaderboard(tc.address, tc.points)

			if tc.expectError {
				tdb.assert.Error(err)
				tdb.assert.Contains(err.Error(), tc.expectedError)
			} else {
				tdb.assert.NoError(err)
			}

			tdb.assert.NoError(tdb.mock.ExpectationsWereMet())
		})
	}
}

func TestGetLeaderboard(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.close()

	testCases := []struct {
		name           string
		limit          int
		mockSetup      func()
		expectedResult []LeaderboardEntry
		expectError    bool
	}{
		{
			name:  "Successful retrieval",
			limit: 3,
			mockSetup: func() {
				tdb.mock.ExpectQuery("SELECT address, points FROM leaderboard").
					WithArgs(3).
					WillReturnRows(sqlmock.NewRows([]string{"address", "points"}).
						AddRow("0x1111", 1000).
						AddRow("0x2222", 800).
						AddRow("0x3333", 600))
			},
			expectedResult: []LeaderboardEntry{
				{Address: "0x1111", Points: 1000},
				{Address: "0x2222", Points: 800},
				{Address: "0x3333", Points: 600},
			},
		},
		{
			name:  "Empty leaderboard",
			limit: 10,
			mockSetup: func() {
				tdb.mock.ExpectQuery("SELECT address, points FROM leaderboard").
					WithArgs(10).
					WillReturnRows(sqlmock.NewRows([]string{"address", "points"}))
			},
			expectedResult: []LeaderboardEntry{},
		},
		{
			name:  "Database error",
			limit: 5,
			mockSetup: func() {
				tdb.mock.ExpectQuery("SELECT address, points FROM leaderboard").
					WithArgs(5).
					WillReturnError(fmt.Errorf("database error"))
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.mockSetup()

			result, err := tdb.svc.GetLeaderboard(tc.limit)

			if tc.expectError {
				tdb.assert.Error(err)
			} else {
				tdb.assert.NoError(err)
				tdb.assert.Equal(tc.expectedResult, result)
			}

			tdb.assert.NoError(tdb.mock.ExpectationsWereMet())
		})
	}
}

func TestRecordSwapAndUpdatePoints(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.close()

	testCases := []struct {
		name        string
		address     string
		usdValue    float64
		points      int64
		txHash      string
		mockSetup   func()
		expectError bool
	}{
		{
			name:     "Successful swap and points update for existing user",
			address:  "0x1234567890123456789012345678901234567890",
			usdValue: 1000.0,
			points:   100,
			txHash:   "0xabcdef1234567890",
			mockSetup: func() {
				tdb.mock.ExpectBegin()
				// Expect the SELECT query to get the user ID
				tdb.mock.ExpectQuery("SELECT id FROM users WHERE address = \\$1").
					WithArgs("0x1234567890123456789012345678901234567890").
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
				// Expectation for INSERT INTO swap_events
				tdb.mock.ExpectExec("INSERT INTO swap_events").
					WithArgs(1, "0x1234567890123456789012345678901234567890", 1000.0, "0xabcdef1234567890").
					WillReturnResult(sqlmock.NewResult(1, 1))
				// Expectation for UPDATE users
				tdb.mock.ExpectExec("UPDATE users").
					WithArgs(1, 100).
					WillReturnResult(sqlmock.NewResult(1, 1))
				tdb.mock.ExpectExec("INSERT INTO points_history").
					WithArgs(1, 100, "Swap").
					WillReturnResult(sqlmock.NewResult(1, 1))
				tdb.mock.ExpectCommit()
			},
			expectError: false,
		},
		{
			name:     "Successful swap and points update for new user",
			address:  "0x0987654321098765432109876543210987654321",
			usdValue: 500.0,
			points:   50,
			txHash:   "0xfedcba9876543210",
			mockSetup: func() {
				tdb.mock.ExpectBegin()
				// Expect the SELECT query to get the user ID (user not found)
				tdb.mock.ExpectQuery("SELECT id FROM users WHERE address = \\$1").
					WithArgs("0x0987654321098765432109876543210987654321").
					WillReturnError(sql.ErrNoRows)
				// Expect INSERT INTO users for new user
				tdb.mock.ExpectQuery("INSERT INTO users").
					WithArgs("0x0987654321098765432109876543210987654321").
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(2))
				// Expectation for INSERT INTO swap_events
				tdb.mock.ExpectExec("INSERT INTO swap_events").
					WithArgs(2, "0x0987654321098765432109876543210987654321", 500.0, "0xfedcba9876543210").
					WillReturnResult(sqlmock.NewResult(1, 1))
				// Expectation for UPDATE users
				tdb.mock.ExpectExec("UPDATE users").
					WithArgs(2, 50).
					WillReturnResult(sqlmock.NewResult(1, 1))
				tdb.mock.ExpectExec("INSERT INTO points_history").
					WithArgs(2, 50, "Swap").
					WillReturnResult(sqlmock.NewResult(1, 1))
				tdb.mock.ExpectCommit()
			},
			expectError: false,
		},
		// Add more test cases here...
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.mockSetup()

			err := tdb.svc.RecordSwapAndUpdatePoints(tc.address, tc.usdValue, tc.points, tc.txHash)

			if tc.expectError {
				tdb.assert.Error(err)
			} else {
				tdb.assert.NoError(err)
			}

			tdb.assert.NoError(tdb.mock.ExpectationsWereMet())
		})
	}
}
