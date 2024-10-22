package api

import (
	"github.com/SIMPLYBOYS/trading_ace/internal/errors"
	"github.com/SIMPLYBOYS/trading_ace/pkg/logger"
	"github.com/gin-gonic/gin"
)

func ErrorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			switch e := err.(type) {
			case *errors.DatabaseError:
				logger.Error("Database error: %v", e)
				c.JSON(500, gin.H{"error": "Internal server error"})
			case *errors.EthereumError:
				logger.Error("Ethereum error: %v", e)
				c.JSON(500, gin.H{"error": "Ethereum service unavailable"})
			case *errors.APIError:
				logger.Error("API error: %v", e)
				c.JSON(e.StatusCode, gin.H{"error": e.Message})
			default:
				logger.Error("Unexpected error: %v", e)
				c.JSON(500, gin.H{"error": "Internal server error"})
			}
			c.Abort()
		}
	}
}
