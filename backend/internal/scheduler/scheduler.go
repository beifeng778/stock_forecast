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

		if _, err := stockdata.RefreshStockCache(); err != nil {
			log.Printf("刷新股票缓存失败: %v", err)
			if i < maxRetry {
				log.Printf("将在 %d 分钟后重试", intervalMinutes)
				time.Sleep(time.Duration(intervalMinutes) * time.Minute)
			}
		} else {
			log.Println("股票缓存刷新完成")
			return
		}
	}
	log.Printf("股票缓存刷新失败，已重试 %d 次", maxRetry)
}
