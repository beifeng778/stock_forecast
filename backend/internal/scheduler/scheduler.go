package scheduler

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"stock-forecast-backend/internal/mail"
	"stock-forecast-backend/internal/stockdata"
)

var (
	currentInviteCode string
	codeVersion       int64 // 验证码版本号，每次更新递增
	codeMutex         sync.RWMutex
)

// GenerateRandomCode 生成随机验证码
func GenerateRandomCode(length int) string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	code := make([]byte, length)
	for i := range code {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		code[i] = charset[n.Int64()]
	}
	return string(code)
}

// GetCurrentInviteCode 获取当前验证码
func GetCurrentInviteCode() string {
	codeMutex.RLock()
	defer codeMutex.RUnlock()
	return currentInviteCode
}

// SetInviteCode 设置验证码
func SetInviteCode(code string) {
	codeMutex.Lock()
	defer codeMutex.Unlock()
	currentInviteCode = code
	codeVersion = time.Now().Unix() // 更新版本号
}

// GetCodeVersion 获取当前验证码版本号
func GetCodeVersion() int64 {
	codeMutex.RLock()
	defer codeMutex.RUnlock()
	return codeVersion
}

// RotateInviteCode 轮换验证码并发送通知
func RotateInviteCode() {
	newCode := GenerateRandomCode(6)
	SetInviteCode(newCode)
	log.Printf("验证码已更新: %s", newCode)

	// 发送邮件通知
	if err := mail.SendInviteCodeNotification(newCode); err != nil {
		log.Printf("发送验证码通知邮件失败: %v", err)
	} else {
		log.Println("验证码通知邮件已发送")
	}
}

// StartScheduler 启动定时任务
func StartScheduler() {
	// 检查是否为本地模式（不发送邮件，使用固定验证码）
	localMode := os.Getenv("LOCAL_MODE") == "1"
	backdoorCode := os.Getenv("BACKDOOR_CODE")

	if localMode {
		if backdoorCode != "" {
			SetInviteCode(backdoorCode)
			log.Printf("本地模式：使用后门验证码 %s（不发送邮件）", backdoorCode)
		} else {
			// 本地模式但没有后门验证码，生成一个但不发邮件
			newCode := GenerateRandomCode(6)
			SetInviteCode(newCode)
			log.Printf("本地模式：生成验证码 %s（不发送邮件）", newCode)
		}
		return
	}

	// 获取轮换周期（小时），默认1小时
	rotateHours := 1
	if h := os.Getenv("CODE_ROTATE_HOURS"); h != "" {
		if parsed, err := strconv.Atoi(h); err == nil && parsed > 0 {
			rotateHours = parsed
		}
	}

	// 启动时立即生成新验证码并发送邮件
	RotateInviteCode()

	// 如果设置为0，则不自动轮换（但启动时仍会生成一次）
	if rotateHours == 0 {
		log.Println("验证码自动轮换已禁用（仅启动时生成一次）")
		return
	}

	log.Printf("验证码将每 %d 小时自动更新", rotateHours)

	// 启动定时任务
	go func() {
		ticker := time.NewTicker(time.Duration(rotateHours) * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			RotateInviteCode()
		}
	}()
}

// TestSendMail 测试邮件发送
func TestSendMail() error {
	to := os.Getenv("NOTIFY_EMAIL")
	if to == "" {
		return fmt.Errorf("未配置 NOTIFY_EMAIL")
	}
	return mail.SendMail(to, "【股票预测系统】邮件测试", "<h1>邮件发送测试成功！</h1>")
}

// StartStockCacheRefreshScheduler 启动股票缓存刷新定时任务
func StartStockCacheRefreshScheduler() {
	// 读取配置
	refreshTime := os.Getenv("STOCK_CACHE_REFRESH_TIME")
	if refreshTime == "" {
		refreshTime = "04:00"
	}

	retryCount := 3
	if v := os.Getenv("STOCK_CACHE_RETRY_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			retryCount = n
		}
	}

	retryInterval := 10 // 分钟
	if v := os.Getenv("STOCK_CACHE_RETRY_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			retryInterval = n
		}
	}

	// 启动时是否立即执行
	runOnStartup := os.Getenv("STOCK_CACHE_REFRESH_ON_STARTUP")
	shouldRunOnStartup := runOnStartup == "true" || runOnStartup == "1"

	// 解析刷新时间
	parts := strings.Split(refreshTime, ":")
	hour, minute := 4, 0
	if len(parts) == 2 {
		if h, err := strconv.Atoi(parts[0]); err == nil {
			hour = h
		}
		if m, err := strconv.Atoi(parts[1]); err == nil {
			minute = m
		}
	}

	log.Printf("股票缓存刷新任务已启动，刷新时间: %02d:%02d，重试次数: %d，重试间隔: %d分钟，启动时执行: %v",
		hour, minute, retryCount, retryInterval, shouldRunOnStartup)

	// 如果配置了启动时执行，立即执行一次
	if shouldRunOnStartup {
		go func() {
			log.Println("服务启动时执行股票缓存刷新...")
			refreshWithRetry(retryCount, retryInterval)
		}()
	}

	go func() {
		for {
			now := time.Now()
			// 计算下一个刷新时间
			nextRefresh := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
			if now.After(nextRefresh) {
				nextRefresh = nextRefresh.Add(24 * time.Hour)
			}

			duration := nextRefresh.Sub(now)
			log.Printf("股票缓存将在 %s 刷新（%v 后）", nextRefresh.Format("2006-01-02 15:04:05"), duration.Round(time.Minute))

			time.Sleep(duration)

			// 执行刷新（带重试）
			refreshWithRetry(retryCount, retryInterval)
		}
	}()
}

// refreshWithRetry 带重试的刷新
func refreshWithRetry(maxRetry, intervalMinutes int) {
	for i := 0; i <= maxRetry; i++ {
		if i > 0 {
			log.Printf("第 %d 次重试刷新股票缓存...", i)
		} else {
			log.Println("开始刷新股票缓存...")
		}

		stocks, err := stockdata.RefreshStockCache()
		if err != nil {
			log.Printf("刷新股票缓存失败: %v", err)
			if i < maxRetry {
				log.Printf("将在 %d 分钟后重试", intervalMinutes)
				time.Sleep(time.Duration(intervalMinutes) * time.Minute)
			}
		} else {
			log.Println("股票缓存刷新完成")

			// 预热大盘指数数据
			log.Println("开始预热大盘指数数据...")
			indexCodes := []string{"000001.SH", "399006.SZ"} // 上证指数、创业板指
			for _, indexCode := range indexCodes {
				if _, err := stockdata.GetKlineWithRefresh(indexCode, "daily", true); err != nil {
					log.Printf("预热大盘指数 %s 失败: %v", indexCode, err)
				} else {
					log.Printf("预热大盘指数 %s 成功", indexCode)
				}
			}

			// 预热常用股票K线数据（包含换手率）
			log.Println("开始预热常用股票K线数据...")
			prewarmStocks := getPrewarmStockList(stocks)
			successCount := 0
			failCount := 0

			for i, stock := range prewarmStocks {
				// 每10只股票休息1秒，避免请求过快被风控
				if i > 0 && i%10 == 0 {
					time.Sleep(1 * time.Second)
				}

				if _, err := stockdata.GetKlineWithRefresh(stock.Code, "daily", true); err != nil {
					log.Printf("预热股票 %s(%s) 失败: %v", stock.Code, stock.Name, err)
					failCount++
				} else {
					successCount++
					if (i+1)%50 == 0 {
						log.Printf("已预热 %d/%d 只股票...", i+1, len(prewarmStocks))
					}
				}
			}

			log.Printf("股票K线预热完成，成功: %d，失败: %d", successCount, failCount)

			return
		}
	}
	log.Printf("股票缓存刷新失败，已重试 %d 次", maxRetry)
}

// StartPostMarketUpdateScheduler 启动收盘后增量更新定时任务
func StartPostMarketUpdateScheduler() {
	// 读取配置
	enabled := os.Getenv("POST_MARKET_UPDATE_ENABLED")
	if enabled == "false" || enabled == "0" {
		log.Println("收盘后增量更新任务已禁用")
		return
	}

	// 收盘后更新时间，默认16:30
	updateTimeStr := os.Getenv("POST_MARKET_UPDATE_TIME")
	if updateTimeStr == "" {
		updateTimeStr = "16:30"
	}

	// 解析更新时间
	parts := strings.Split(updateTimeStr, ":")
	updateHour, updateMinute := 16, 30
	if len(parts) == 2 {
		if h, err := strconv.Atoi(parts[0]); err == nil {
			updateHour = h
		}
		if m, err := strconv.Atoi(parts[1]); err == nil {
			updateMinute = m
		}
	}

	// 重试次数，默认3次
	retryCount := 3
	if v := os.Getenv("POST_MARKET_RETRY_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			retryCount = n
		}
	}

	// 重试间隔（分钟），默认10分钟
	retryInterval := 10
	if v := os.Getenv("POST_MARKET_RETRY_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			retryInterval = n
		}
	}

	// 启动时是否立即执行
	runOnStartup := os.Getenv("POST_MARKET_UPDATE_ON_STARTUP")
	shouldRunOnStartup := runOnStartup == "true" || runOnStartup == "1"

	log.Printf("收盘后增量更新任务已启动，更新时间: %02d:%02d，重试次数: %d，重试间隔: %d分钟，启动时执行: %v",
		updateHour, updateMinute, retryCount, retryInterval, shouldRunOnStartup)

	// 如果配置了启动时执行，立即执行一次
	if shouldRunOnStartup {
		go func() {
			log.Println("服务启动时执行收盘后增量更新...")
			executePostMarketUpdateWithRetry(retryCount, retryInterval)
		}()
	}

	go func() {
		for {
			now := time.Now()

			// 计算下次更新时间
			nextRun := time.Date(now.Year(), now.Month(), now.Day(),
				updateHour, updateMinute, 0, 0, now.Location())

			// 如果今天的更新时间已过，或者是周末，找下一个交易日
			if now.After(nextRun) || now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
				nextRun = nextRun.Add(24 * time.Hour)
				// 跳过周末
				for nextRun.Weekday() == time.Saturday || nextRun.Weekday() == time.Sunday {
					nextRun = nextRun.Add(24 * time.Hour)
				}
			}

			duration := nextRun.Sub(now)
			log.Printf("下次收盘后更新时间: %s（%v后）",
				nextRun.Format("2006-01-02 15:04:05"), duration.Round(time.Minute))
			time.Sleep(duration)

			// 执行收盘后更新（带重试）
			executePostMarketUpdateWithRetry(retryCount, retryInterval)
		}
	}()
}

// executePostMarketUpdateWithRetry 执行收盘后增量更新（带重试）
func executePostMarketUpdateWithRetry(maxRetry, intervalMinutes int) {
	for i := 0; i <= maxRetry; i++ {
		if i > 0 {
			log.Printf("第 %d 次重试收盘后增量更新...", i)
		} else {
			log.Println("开始执行收盘后增量更新任务...")
		}

		if err := executePostMarketUpdate(); err != nil {
			log.Printf("收盘后增量更新失败: %v", err)
			if i < maxRetry {
				log.Printf("将在 %d 分钟后重试", intervalMinutes)
				time.Sleep(time.Duration(intervalMinutes) * time.Minute)
			}
		} else {
			log.Println("收盘后增量更新完成")
			return
		}
	}
	log.Printf("收盘后增量更新失败，已重试 %d 次", maxRetry)
}

// executePostMarketUpdate 执行收盘后增量更新
func executePostMarketUpdate() error {
	start := time.Now()
	log.Println("开始执行收盘后全量数据刷新...")

	// 1. 刷新股票列表缓存
	stocks, err := stockdata.RefreshStockCache()
	if err != nil {
		return fmt.Errorf("收盘后全量刷新失败: %v", err)
	}

	// 2. 预热大盘指数数据
	log.Println("开始预热大盘指数数据...")
	indexCodes := []string{"000001.SH", "399006.SZ"} // 上证指数、创业板指
	for _, indexCode := range indexCodes {
		if _, err := stockdata.GetKlineWithRefresh(indexCode, "daily", true); err != nil {
			log.Printf("预热大盘指数 %s 失败: %v", indexCode, err)
		} else {
			log.Printf("预热大盘指数 %s 成功", indexCode)
		}
	}

	// 3. 预热常用股票K线数据（包含换手率）
	log.Println("开始预热常用股票K线数据...")
	prewarmStocks := getPrewarmStockList(stocks)
	successCount := 0
	failCount := 0

	for i, stock := range prewarmStocks {
		// 每10只股票休息1秒，避免请求过快被风控
		if i > 0 && i%10 == 0 {
			time.Sleep(1 * time.Second)
		}

		if _, err := stockdata.GetKlineWithRefresh(stock.Code, "daily", true); err != nil {
			log.Printf("预热股票 %s(%s) 失败: %v", stock.Code, stock.Name, err)
			failCount++
		} else {
			successCount++
			if (i+1)%50 == 0 {
				log.Printf("已预热 %d/%d 只股票...", i+1, len(prewarmStocks))
			}
		}
	}

	log.Printf("股票K线预热完成，成功: %d，失败: %d", successCount, failCount)

	duration := time.Since(start)
	log.Printf("收盘后全量刷新完成，耗时: %v，更新股票数量: %d", duration, len(stocks))

	return nil
}

// getPrewarmStockList 获取需要预热的股票列表
func getPrewarmStockList(allStocks []stockdata.Stock) []stockdata.Stock {
	// 读取环境变量配置
	prewarmMode := os.Getenv("PREWARM_MODE")
	if prewarmMode == "" {
		prewarmMode = "none" // 默认不预热
	}

	switch prewarmMode {
	case "all":
		// 预热所有股票（慎用，可能需要很长时间）
		log.Printf("预热模式: 全部股票 (%d只)", len(allStocks))
		return allStocks

	case "major":
		// 预��主要指数成分股（沪深300、创业板50等）
		// 这里简化处理：预热前500只股票（按代码排序）
		limit := 500
		if len(allStocks) < limit {
			limit = len(allStocks)
		}
		log.Printf("预热模式: 主要股票 (前%d只)", limit)
		return allStocks[:limit]

	case "custom":
		// 从环境变量读取自定义股票列表
		customCodes := os.Getenv("PREWARM_STOCK_CODES")
		if customCodes == "" {
			log.Println("预热模式: 自定义，但未配置股票列表，跳过预热")
			return []stockdata.Stock{}
		}

		codes := strings.Split(customCodes, ",")
		var result []stockdata.Stock
		stockMap := make(map[string]stockdata.Stock)
		for _, s := range allStocks {
			stockMap[s.Code] = s
		}

		for _, code := range codes {
			code = strings.TrimSpace(code)
			if stock, ok := stockMap[code]; ok {
				result = append(result, stock)
			}
		}
		log.Printf("预热模式: 自定义 (%d只)", len(result))
		return result

	default:
		// 不预热
		log.Println("预热模式: 关闭")
		return []stockdata.Stock{}
	}
}
