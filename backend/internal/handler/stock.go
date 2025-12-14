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

	stocks, fromCache, refreshFailed := stockdata.SearchStocksWithRefresh(keyword, refresh)

	// 刷新时第三方接口获取失败，返回错误让用户感知
	if refreshFailed {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "第三方数据接口异常，请稍后再试",
		})
		return
	}

	// 获取全量列表时，空结果算错误
	if keyword == "" && len(stocks) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "第三方数据接口异常，请稍后再试",
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
	refresh := c.Query("refresh") == "1"

	// 避免浏览器/代理缓存影响盘中刷新
	// refresh=1 时强制刷新语义更明确；同时对所有请求返回 no-store，防止拿到陈旧数据
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	kline, err := stockdata.GetKlineWithRefresh(code, period, refresh)
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
