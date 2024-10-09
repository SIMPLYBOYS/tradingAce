package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

	r.GET("/user/:address/tasks", getUserTasks)
	r.GET("/user/:address/points", getUserPointsHistory)

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
