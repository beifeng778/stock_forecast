package config

import (
	"os"
	"strconv"
	"time"
)

// SchedulerConfig 定时任务配置
type SchedulerConfig struct {
	// 夜间全量更新配置
	NightlyUpdate struct {
		Enabled  bool   `json:"enabled"`
		Schedule string `json:"schedule"` // cron表达式，默认 "0 2 * * *" (每天凌晨2点)
	} `json:"nightly_update"`

	// 收盘后增量更新配置
	PostMarketUpdate struct {
		Enabled  bool          `json:"enabled"`
		Schedule string        `json:"schedule"` // cron表达式，默认 "0 16 * * 1-5" (交易日下午4点)
		Delay    time.Duration `json:"delay"`    // 收盘后延迟时间，默认1小时
	} `json:"post_market_update"`

	// 前端刷新冷却配置
	RefreshCooldown struct {
		Enabled         bool          `json:"enabled"`
		CooldownMinutes int           `json:"cooldown_minutes"` // 刷新冷却时间（分钟），默认5分钟
		MaxRequestsPerHour int        `json:"max_requests_per_hour"` // 每小时最大请求次数，默认12次
	} `json:"refresh_cooldown"`
}

// GetSchedulerConfig 获取定时任务配置
func GetSchedulerConfig() *SchedulerConfig {
	config := &SchedulerConfig{}

	// 夜间全量更新配置
	config.NightlyUpdate.Enabled = getEnvBool("NIGHTLY_UPDATE_ENABLED", true)
	config.NightlyUpdate.Schedule = getEnvString("NIGHTLY_UPDATE_SCHEDULE", "0 2 * * *")

	// 收盘后增量更新配置
	config.PostMarketUpdate.Enabled = getEnvBool("POST_MARKET_UPDATE_ENABLED", true)
	config.PostMarketUpdate.Schedule = getEnvString("POST_MARKET_UPDATE_SCHEDULE", "0 16 * * 1-5")
	config.PostMarketUpdate.Delay = getEnvDuration("POST_MARKET_UPDATE_DELAY", 1*time.Hour)

	// 前端刷新冷却配置
	config.RefreshCooldown.Enabled = getEnvBool("REFRESH_COOLDOWN_ENABLED", true)
	config.RefreshCooldown.CooldownMinutes = getEnvInt("REFRESH_COOLDOWN_MINUTES", 5)
	config.RefreshCooldown.MaxRequestsPerHour = getEnvInt("MAX_REQUESTS_PER_HOUR", 12)

	return config
}

// 辅助函数
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}