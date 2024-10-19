package api

import (
	"net/http"
	"time"

	"github.com/SIMPLYBOYS/trading_ace/internal/db"
	"github.com/SIMPLYBOYS/trading_ace/internal/errors"
	"github.com/SIMPLYBOYS/trading_ace/internal/ethereum"
	"github.com/SIMPLYBOYS/trading_ace/pkg/logger"
	"github.com/gin-gonic/gin"
)

// GetUserTasks handles the GET request for user tasks
func GetUserTasks(c *gin.Context) {
	address := c.Param("address")

	tasks, err := db.GetUserTasks(address)
	if err != nil {
		c.Error(&errors.APIError{
			StatusCode: 500,
			Message:    "Failed to fetch user tasks",
			Err:        err,
		})
		return
	}

	c.JSON(http.StatusOK, tasks)
}

// GetUserPointsHistory handles the GET request for user points history
func GetUserPointsHistory(c *gin.Context) {
	address := c.Param("address")

	pointsHistory, err := db.GetUserPointsHistory(address)
	if err != nil {
		logger.Error("Failed to fetch user points history: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user points history"})
		return
	}

	c.JSON(http.StatusOK, pointsHistory)
}

// GetEthereumPrice handles the GET request for current Ethereum price
func GetEthereumPrice(c *gin.Context) {
	price, err := ethereum.GetEthereumPrice()
	if err != nil {
		logger.Error("Failed to fetch Ethereum price: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Ethereum price"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"price": price})
}

// GetLeaderboard handles the GET request for the leaderboard
func GetLeaderboard(c *gin.Context) {
	leaderboard, err := db.GetLeaderboard(10) // Get top 10 for now
	if err != nil {
		logger.Error("Failed to fetch leaderboard: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch leaderboard"})
		return
	}

	campaign, err := db.GetCampaignConfig()
	if err != nil {
		logger.Error("Failed to fetch campaign config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch campaign config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"leaderboard": leaderboard,
		"campaign": gin.H{
			"start_time":   campaign.StartTime,
			"end_time":     campaign.EndTime,
			"is_active":    campaign.IsActive,
			"total_weeks":  4, // Assuming 4-week campaign
			"current_week": getCurrentWeek(campaign.StartTime),
		},
	})
}

// getCurrentWeek calculates the current week of the campaign
func getCurrentWeek(startTime time.Time) int {
	duration := time.Since(startTime)
	week := int(duration.Hours()/(7*24)) + 1
	if week > 4 {
		week = 4
	}
	return week
}
