package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"stock-forecast-backend/internal/client"
)

// GetStocks 获取股票列表
func GetStocks(c *gin.Context) {
	keyword := c.Query("keyword")

	stocks, err := client.GetStocks(keyword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": stocks,
	})
}

// GetKline 获取K线数据
func GetKline(c *gin.Context) {
	code := c.Param("code")
	period := c.DefaultQuery("period", "daily")

	kline, err := client.GetKline(code, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, kline)
}
