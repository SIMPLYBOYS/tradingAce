package api

import (
	"net/http"

	"github.com/SIMPLYBOYS/trading_ace/internal/db"
	"github.com/SIMPLYBOYS/trading_ace/internal/ethereum"
	"github.com/SIMPLYBOYS/trading_ace/pkg/logger"
	"github.com/gin-gonic/gin"
)

// Handler struct holds our dependencies
type Handler struct {
	DB       db.DBService
	Ethereum ethereum.EthereumService
}

// NewHandler creates a new Handler with the given dependencies
func NewHandler(db db.DBService, eth ethereum.EthereumService) *Handler {
	return &Handler{
		DB:       db,
		Ethereum: eth,
	}
}

// GetUserTasks handles the GET request for user tasks
func (h *Handler) GetUserTasks(c *gin.Context) {
	address := c.Param("address")

	tasks, err := h.DB.GetUserTasks(address)
	if err != nil {
		logger.Error("Failed to fetch user tasks for address %s: %v", address, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user tasks", "details": err.Error()})
		return
	}

	if tasks == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, tasks)
}

// GetUserPointsHistory handles the GET request for user points history
func (h *Handler) GetUserPointsHistory(c *gin.Context) {
	address := c.Param("address")

	pointsHistory, err := h.DB.GetUserPointsHistory(address)
	if err != nil {
		logger.Error("Failed to fetch user points history for address %s: %v", address, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user points history", "details": err.Error()})
		return
	}

	if pointsHistory == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, pointsHistory)
}

// GetEthereumPrice handles the GET request for current Ethereum price
func (h *Handler) GetEthereumPrice(c *gin.Context) {
	price, err := h.Ethereum.GetEthereumPrice()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Ethereum price"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"price": price})
}

// GetLeaderboard handles the GET request for the leaderboard
func (h *Handler) GetLeaderboard(c *gin.Context) {
	leaderboard, err := h.DB.GetLeaderboard(10) // Get top 10 for now
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch leaderboard"})
		return
	}

	campaign, err := h.DB.GetCampaignConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch campaign config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"leaderboard": leaderboard,
		"campaign":    campaign,
	})
}
