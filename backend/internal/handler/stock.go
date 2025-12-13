package handler

import (
	"net/http"

	"stock-forecast-backend/internal/stockdata"

	"github.com/gin-gonic/gin"
)

// GetStocks 获取股票列表
func GetStocks(c *gin.Context) {
	keyword := c.Query("keyword")
	refresh := c.Query("refresh") == "1"

	stocks, fromCache := stockdata.SearchStocksWithRefresh(keyword, refresh)
	if stocks == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取股票列表失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      stocks,
		"fromCache": fromCache,
	})
}

// GetKline 获取K线数据
func GetKline(c *gin.Context) {
	code := c.Param("code")
	period := c.DefaultQuery("period", "daily")

	kline, err := stockdata.GetKline(code, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, kline)
}

// GetIndicators 获取技术指标
func GetIndicators(c *gin.Context) {
	code := c.Param("code")

	indicators, err := stockdata.GetIndicators(code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, indicators)
}

// GetNews 获取股票新闻
func GetNews(c *gin.Context) {
	code := c.Param("code")

	news, err := stockdata.GetStockNews(code, 5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": news,
	})
}
