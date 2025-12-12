package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"stock-forecast-backend/internal/model"
	"stock-forecast-backend/internal/service"
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

	results, err := service.PredictStocks(req.StockCodes, req.Period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, model.PredictResponse{
		Results: results,
	})
}
