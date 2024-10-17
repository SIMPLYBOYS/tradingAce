package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func SetupRouter(wsManager *WebSocketManager) *gin.Engine {
	r := gin.Default()

	r.GET("/user/:address/tasks", getUserTasks)
	r.GET("/user/:address/points", getUserPointsHistory)
	r.GET("/ethereum/price", getEthereumPrice)
	r.GET("/leaderboard", getLeaderboard)
	r.GET("/ws", func(c *gin.Context) {
		wsManager.HandleWebSocket(c.Writer, c.Request)
	})

	return r
}

func getUserTasks(c *gin.Context) {
	address := c.Param("address")

	tasks, err := GetUserTasks(address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user tasks"})
		return
	}

	c.JSON(http.StatusOK, tasks)
}

func getUserPointsHistory(c *gin.Context) {
	address := c.Param("address")

	pointsHistory, err := GetUserPointsHistory(address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user points history"})
		return
	}

	c.JSON(http.StatusOK, pointsHistory)
}

func getEthereumPrice(c *gin.Context) {
	price, err := GetEthereumPrice()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Ethereum price"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"price": price})
}

func getLeaderboard(c *gin.Context) {
	limit := 10 // Default limit
	if limitParam := c.Query("limit"); limitParam != "" {
		if parsedLimit, err := strconv.Atoi(limitParam); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	leaderboard, err := GetLeaderboard(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch leaderboard"})
		return
	}

	// Get the current campaign config for additional context
	campaignConfig, err := GetCampaignConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch campaign config"})
		return
	}

	response := gin.H{
		"leaderboard": leaderboard,
		"campaign_info": gin.H{
			"start_time":   campaignConfig.StartTime,
			"end_time":     campaignConfig.EndTime,
			"is_active":    campaignConfig.IsActive,
			"total_weeks":  4, // Assuming it's always 4 weeks
			"current_week": getCurrentWeek(campaignConfig.StartTime),
		},
	}

	c.JSON(http.StatusOK, response)
}

func getCurrentWeek(startTime time.Time) int {
	now := time.Now()
	if now.Before(startTime) {
		return 0
	}
	weeksPassed := int(now.Sub(startTime).Hours() / (24 * 7))
	if weeksPassed >= 4 {
		return 4
	}
	return weeksPassed + 1
}
