package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"stock-forecast-backend/internal/model"
	"stock-forecast-backend/internal/service"
)

// SimulateTrade 模拟委托交易
func SimulateTrade(c *gin.Context) {
	var req model.TradeSimulateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误: " + err.Error(),
		})
		return
	}

	result, err := service.SimulateTrade(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}
