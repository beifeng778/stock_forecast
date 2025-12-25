package holiday

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

var (
	cacheMu    sync.RWMutex
	cache      = make(map[string]bool)
	cacheTime  = make(map[string]time.Time)
	cacheTTL   = 24 * time.Hour
	apiTimeout = 3 * time.Second

	// 自定义节假日配置（可选，从环境变量或配置文件加载）
	customHolidays = make(map[string]bool)
)

// LoadCustomHolidays 从JSON文件加载自定义节假日配置
// 文件格式：{"holidays": ["2025-01-01", "2025-01-28", ...]}
func LoadCustomHolidays(filePath string) error {
	if filePath == "" {
		return nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在不算错误
		}
		return fmt.Errorf("读取节假日配置文件失败: %v", err)
	}

	var config struct {
		Holidays []string `json:"holidays"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析节假日配置文件失败: %v", err)
	}

	cacheMu.Lock()
	defer cacheMu.Unlock()

	for _, date := range config.Holidays {
		customHolidays[date] = true
	}

	log.Printf("[INFO][Holiday] 加载自定义节假日配置: %d个节假日", len(config.Holidays))
	return nil
}

// IsTradingDay 判断是否为A股交易日
// A股交易规则：周一到周五交易，周六周日不交易（即使是调休补班日），法定节假日不交易
// 优先级：周末判断 > 自定义配置 > API
func IsTradingDay(date time.Time) bool {
	dateStr := date.Format("2006-01-02")

	// 1. 首先检查是否为周末（A股周六周日永远不交易）
	wd := date.Weekday()
	if wd == time.Saturday || wd == time.Sunday {
		return false
	}

	// 2. 检查缓存
	cacheMu.RLock()
	if result, ok := cache[dateStr]; ok {
		if t, ok := cacheTime[dateStr]; ok && time.Since(t) < cacheTTL {
			cacheMu.RUnlock()
			return result
		}
	}
	cacheMu.RUnlock()

	// 3. 检查自定义节假日配置
	cacheMu.RLock()
	isCustomHoliday := customHolidays[dateStr]
	cacheMu.RUnlock()

	if isCustomHoliday {
		result := false
		updateCache(dateStr, result)
		return result
	}

	// 4. 尝试从API获取
	if result, ok := checkFromAPI(dateStr); ok {
		updateCache(dateStr, result)
		return result
	}

	// 5. API失败，回退到默认逻辑：周一到周五是交易日
	result := true
	updateCache(dateStr, result)
	return result
}

func updateCache(dateStr string, result bool) {
	cacheMu.Lock()
	cache[dateStr] = result
	cacheTime[dateStr] = time.Now()
	cacheMu.Unlock()
}

// checkFromAPI 从API检查是否为交易日
// 使用免费的节假日API：http://timor.tech/api/holiday/info/{date}
func checkFromAPI(dateStr string) (bool, bool) {
	url := fmt.Sprintf("http://timor.tech/api/holiday/info/%s", dateStr)

	client := &http.Client{Timeout: apiTimeout}
	resp, err := client.Get(url)
	if err != nil {
		// API失败不打印日志，避免刷屏
		return false, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, false
	}

	var result struct {
		Code int `json:"code"`
		Type struct {
			Type int    `json:"type"` // 0工作日 1周末 2节假日 3调休
			Name string `json:"name"`
		} `json:"type"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return false, false
	}

	if result.Code != 0 {
		return false, false
	}

	// type: 0工作日 1周末 2节假日 3调休（上班）
	isTrading := result.Type.Type == 0 || result.Type.Type == 3
	return isTrading, true
}

// IsTradingDayNow 判断当前是否为交易日
func IsTradingDayNow() bool {
	return IsTradingDay(time.Now())
}

// IsTradingTimeNow 判断当前是否为交易时段（09:30-11:30, 13:00-15:00）
func IsTradingTimeNow() bool {
	now := time.Now()
	if !IsTradingDay(now) {
		return false
	}
	hhmm := now.Hour()*100 + now.Minute()
	morning := hhmm >= 930 && hhmm < 1130
	afternoon := hhmm >= 1300 && hhmm < 1500
	return morning || afternoon
}
