package handler

import (
	"net/http"

	"stock-forecast-backend/internal/service"

	"github.com/gin-gonic/gin"
)

func GetPredictTask(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少task_id"})
		return
	}

	status, ok := service.GetPredictTaskStatus(taskID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在或已过期"})
		return
	}

	c.JSON(http.StatusOK, status)
}

func CancelPredictTask(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少task_id"})
		return
	}

	status, ok := service.CancelPredictTask(taskID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在或已过期"})
		return
	}

	c.JSON(http.StatusOK, status)
}
