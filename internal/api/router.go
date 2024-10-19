package api

import (
	"github.com/SIMPLYBOYS/trading_ace/internal/websocket"
	"github.com/gin-gonic/gin"
)

// SetupRouter initializes the Gin router and sets up the routes
func SetupRouter(wsManager *websocket.WebSocketManager) *gin.Engine {
	r := gin.Default()
	r.Use(ErrorMiddleware())

	// User-related routes
	r.GET("/user/:address/tasks", GetUserTasks)
	r.GET("/user/:address/points", GetUserPointsHistory)

	// Ethereum-related routes
	r.GET("/ethereum/price", GetEthereumPrice)

	// Leaderboard route
	r.GET("/leaderboard", GetLeaderboard)

	// WebSocket route
	r.GET("/ws", func(c *gin.Context) {
		wsManager.HandleWebSocket(c.Writer, c.Request)
	})

	return r
}
