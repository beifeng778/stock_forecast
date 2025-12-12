package main

import (
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"stock-forecast-backend/internal/handler"
)

func main() {
	r := gin.Default()

	// 配置 CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	// 注册路由
	api := r.Group("/api")
	{
		// 股票相关
		api.GET("/stocks", handler.GetStocks)
		api.GET("/stocks/:code/kline", handler.GetKline)

		// 预测相关
		api.POST("/predict", handler.Predict)

		// 委托模拟
		api.POST("/trade/simulate", handler.SimulateTrade)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("服务启动在端口 %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("启动服务失败: %v", err)
	}
}
