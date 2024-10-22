// File: internal/api/router.go

package api

import (
	"github.com/SIMPLYBOYS/trading_ace/internal/db"
	"github.com/SIMPLYBOYS/trading_ace/internal/ethereum"
	"github.com/SIMPLYBOYS/trading_ace/internal/websocket"
	"github.com/gin-gonic/gin"
)

// SetupRouter initializes the Gin router and sets up the routes
func SetupRouter(dbService db.DBService, ethService ethereum.EthereumService, wsManager *websocket.WebSocketManager) *gin.Engine {
	r := gin.Default()
	r.Use(ErrorMiddleware())

	// Create a new Handler with the provided services
	h := NewHandler(dbService, ethService)

	// User-related routes
	r.GET("/user/:address/tasks", h.GetUserTasks)
	r.GET("/user/:address/points", h.GetUserPointsHistory)

	// Ethereum-related routes
	r.GET("/ethereum/price", h.GetEthereumPrice)

	// Leaderboard route
	r.GET("/leaderboard", h.GetLeaderboard)

	// WebSocket route
	r.GET("/ws", func(c *gin.Context) {
		wsManager.HandleWebSocket(c.Writer, c.Request)
	})

	return r
}
