package main

import (
	"bufio"
	"log"
	"os"
	"strings"
	"time"

	"stock-forecast-backend/internal/cache"
	"stock-forecast-backend/internal/handler"
	"stock-forecast-backend/internal/scheduler"
	"stock-forecast-backend/internal/stockdata"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func loadEnvFile(filename string) bool {
	file, err := os.Open(filename)
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			os.Setenv(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}
	return true
}

func init() {
	// 先加载 .env 文件
	if loadEnvFile(".env") {
		log.Println("已加载 .env 文件")
	} else {
		log.Println("未找到 .env 文件，使用系统环境变量")
	}

	// 再加载 .env.local 文件（会覆盖 .env 中的配置）
	if loadEnvFile(".env.local") {
		log.Println("已加载 .env.local 文件（本地配置）")
	}
}

func main() {
	// 初始化Redis
	if err := cache.InitRedis(); err != nil {
		log.Printf("警告: %v，将使用内存缓存", err)
	} else {
		stockdata.SetCacheProvider(redisCacheAdapter{})
	}

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
		// 认证相关（不需要token）
		api.POST("/auth/verify", handler.VerifyInviteCode)
		api.GET("/auth/check", handler.CheckToken)

		// 系统配置（不需要token）
		api.GET("/config", handler.GetConfig)

		// 需要认证的路由
		protected := api.Group("")
		protected.Use(handler.AuthMiddleware())
		{
			// 股票相关
			protected.GET("/stocks", handler.GetStocks)
			protected.GET("/stocks/:code/kline", handler.GetKline)
			protected.GET("/stocks/:code/indicators", handler.GetIndicators)
			protected.GET("/stocks/:code/news", handler.GetNews)

			// 预测相关
			protected.POST("/predict", handler.Predict)
			protected.GET("/predict/:task_id", handler.GetPredictTask)
			protected.DELETE("/predict/:task_id", handler.CancelPredictTask)

			// 委托模拟
			protected.POST("/trade/simulate", handler.SimulateTrade)
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// 启动定时任务（验证码轮换）
	scheduler.StartScheduler()

	// 启动股票缓存刷新定时任务（每天凌晨4点）
	scheduler.StartStockCacheRefreshScheduler()

	// 启动收盘后增量更新定时任务（交易日收盘后）
	scheduler.StartPostMarketUpdateScheduler()

	log.Printf("服务启动在端口 %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("启动服务失败: %v", err)
	}
}

type redisCacheAdapter struct{}

func (redisCacheAdapter) Get(key string, dest any) error {
	return cache.Get(key, dest)
}

func (redisCacheAdapter) Set(key string, value any, expiration time.Duration) error {
	return cache.Set(key, value, expiration)
}
