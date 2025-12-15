package handler

import (
	"net/http"
	"os"

	"stock-forecast-backend/internal/stockdata"

	"github.com/gin-gonic/gin"
)

// GetStocks 获取股票列表（始终从缓存获取）
func GetStocks(c *gin.Context) {
	keyword := c.Query("keyword")

	// 始终从缓存获取，不再支持前端直接刷新第三方接口
	stocks, _ := stockdata.SearchStocks(keyword)

	// 获取全量列表时，空结果算错误
	if keyword == "" && len(stocks) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "股票列表为空，请等待后端定时任务更新",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      stocks,
		"fromCache": true,
	})
}

// GetKline 获取K线数据
func GetKline(c *gin.Context) {
	code := c.Param("code")
	period := c.DefaultQuery("period", "daily")

	// 避免浏览器/代理缓存
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	// 始终从缓存获取数据，不再支持前端直接刷新第三方接口
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

// GetConfig 获取系统配置（前端需要的配置项）
func GetConfig(c *gin.Context) {
	// 前端刷新按钮可用时间，默认17:00
	refreshAvailableTime := os.Getenv("FRONTEND_REFRESH_AVAILABLE_TIME")
	if refreshAvailableTime == "" {
		refreshAvailableTime = "17:00"
	}

	c.JSON(http.StatusOK, gin.H{
		"refresh_available_time": refreshAvailableTime,
	})
}
