package handler

import (
	"net/http"

	"stock-forecast-backend/internal/model"
	"stock-forecast-backend/internal/service"

	"github.com/gin-gonic/gin"
)

// Predict 股票预测
func Predict(c *gin.Context) {
	var req model.PredictRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误: " + err.Error(),
		})
		return
	}

	if len(req.StockCodes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请选择至少一只股票",
		})
		return
	}

	if req.Period == "" {
		req.Period = "daily"
	}

	status, created, err := service.CreatePredictTask(req.StockCodes, req.Period, req.RequestID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	type PredictTaskCreateResponse struct {
		service.PredictTaskStatus
		Created bool `json:"created"`
	}

	c.JSON(http.StatusOK, PredictTaskCreateResponse{PredictTaskStatus: status, Created: created})
}
