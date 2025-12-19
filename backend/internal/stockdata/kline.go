package stockdata

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// KlineData K线数据
type KlineData struct {
	Date   string  `json:"date"`
	Open   float64 `json:"open"`
	Close  float64 `json:"close"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Volume float64 `json:"volume"`
	Amount float64 `json:"amount"`
}

// KlineResponse K线响应
type KlineResponse struct {
	Code   string      `json:"code"`
	Name   string      `json:"name"`
	Period string      `json:"period"`
	Data   []KlineData `json:"data"`
}

type klineCacheItem struct {
	Value     *KlineResponse
	ExpiresAt time.Time
}

var (
	klineCacheMu sync.RWMutex
	klineCache   = map[string]klineCacheItem{}
)

func getKlineCacheKey(code, period string) string {
	return fmt.Sprintf("kline:%s:%s", code, period)
}

func getKlineCacheTTL(period string) time.Duration {
	// 日/周/月K默认短缓存，既降低第三方压力，也避免盘中数据过旧。
	// 盘中需要实时可通过 refresh=1 强制刷新。
	switch period {
	case "daily":
		return 5 * time.Minute
	case "weekly", "monthly":
		return 30 * time.Minute
	default:
		return 5 * time.Minute
	}
}

// GetKline 获取K线数据（默认允许缓存）
func GetKline(code, period string) (*KlineResponse, error) {
	return GetKlineWithRefresh(code, period, false)
}

// GetKlineWithRefresh 获取K线数据，forceRefresh=true 时绕过缓存直连第三方
func GetKlineWithRefresh(code, period string, forceRefresh bool) (*KlineResponse, error) {
	key := getKlineCacheKey(code, period)

	if !forceRefresh {
		// 1) 内存缓存
		klineCacheMu.RLock()
		item, ok := klineCache[key]
		klineCacheMu.RUnlock()
		if ok && item.Value != nil && time.Now().Before(item.ExpiresAt) {
			return item.Value, nil
		}

		// 2) Redis缓存（可用则读取）
		var cached KlineResponse
		if err := getCacheProvider().Get(key, &cached); err == nil && len(cached.Data) > 0 {
			cached.Data = normalizeKlineData(cached.Data)
			resp := &cached
			klineCacheMu.Lock()
			klineCache[key] = klineCacheItem{Value: resp, ExpiresAt: time.Now().Add(getKlineCacheTTL(period))}
			klineCacheMu.Unlock()
			return resp, nil
		}
	}

	// 先尝试新浪接口
	data, err := getKlineFromSina(code, period)
	if err == nil && len(data) > 0 {
		data = normalizeKlineData(data)
		name, _ := GetStockName(code)
		resp := &KlineResponse{
			Code:   code,
			Name:   name,
			Period: period,
			Data:   data,
		}

		ttl := getKlineCacheTTL(period)
		klineCacheMu.Lock()
		klineCache[key] = klineCacheItem{Value: resp, ExpiresAt: time.Now().Add(ttl)}
		klineCacheMu.Unlock()
		_ = getCacheProvider().Set(key, resp, ttl)

		return resp, nil
	}

	// 新浪失败，尝试东方财富
	data, err = getKlineFromEM(code, period)
	if err == nil && len(data) > 0 {
		data = normalizeKlineData(data)
		name, _ := GetStockName(code)
		resp := &KlineResponse{
			Code:   code,
			Name:   name,
			Period: period,
			Data:   data,
		}

		ttl := getKlineCacheTTL(period)
		klineCacheMu.Lock()
		klineCache[key] = klineCacheItem{Value: resp, ExpiresAt: time.Now().Add(ttl)}
		klineCacheMu.Unlock()
		_ = getCacheProvider().Set(key, resp, ttl)

		return resp, nil
	}

	return nil, fmt.Errorf("获取K线数据失败")
}

func normalizeKlineData(in []KlineData) []KlineData {
	if len(in) == 0 {
		return in
	}

	data := make([]KlineData, 0, len(in))
	for _, d := range in {
		d.Date = normalizeKlineDate(d.Date)
		if strings.TrimSpace(d.Date) == "" {
			continue
		}
		data = append(data, d)
	}
	if len(data) == 0 {
		return data
	}

	sort.Slice(data, func(i, j int) bool { return data[i].Date < data[j].Date })

	out := make([]KlineData, 0, len(data))
	for _, d := range data {
		if len(out) == 0 {
			out = append(out, d)
			continue
		}
		if out[len(out)-1].Date == d.Date {
			out[len(out)-1] = d
			continue
		}
		out = append(out, d)
	}
	return out
}

func normalizeKlineDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// 常见带时间格式："YYYY-MM-DD HH:MM:SS" / "YYYY/MM/DD HH:MM:SS"
	if len(s) >= 10 {
		prefix := s[:10]
		if t, err := time.Parse("2006-01-02", prefix); err == nil {
			return t.Format("2006-01-02")
		}
		if t, err := time.Parse("2006/01/02", prefix); err == nil {
			return t.Format("2006-01-02")
		}
	}

	// 纯数字格式："YYYYMMDD"
	if len(s) == 8 {
		if t, err := time.Parse("20060102", s); err == nil {
			return t.Format("2006-01-02")
		}
	}

	// 兜底：尽量把分隔符统一
	s = strings.ReplaceAll(s, "/", "-")
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// getKlineFromSina 从新浪获取K线
func getKlineFromSina(code, period string) ([]KlineData, error) {
	// 确定市场前缀
	var symbol string
	if strings.HasPrefix(code, "6") {
		symbol = "sh" + code
	} else {
		symbol = "sz" + code
	}

	// 周期映射
	scaleMap := map[string]string{
		"daily":   "240",
		"weekly":  "1680",
		"monthly": "7200",
	}
	scale := scaleMap[period]
	if scale == "" {
		scale = "240"
	}

	url := fmt.Sprintf("https://quotes.sina.cn/cn/api/jsonp_v2.php/var__%s_%s/CN_MarketDataService.getKLineData?symbol=%s&scale=%s&ma=no&datalen=250",
		symbol, scale, symbol, scale)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://finance.sina.com.cn")

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 解析JSONP响应
	text := string(body)
	start := strings.Index(text, "(")
	end := strings.LastIndex(text, ")")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("响应格式错误")
	}

	jsonStr := text[start+1 : end]

	var rawData []struct {
		Day    string `json:"day"`
		Open   string `json:"open"`
		Close  string `json:"close"`
		High   string `json:"high"`
		Low    string `json:"low"`
		Volume string `json:"volume"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &rawData); err != nil {
		return nil, err
	}

	var result []KlineData
	for _, item := range rawData {
		open, _ := strconv.ParseFloat(item.Open, 64)
		close, _ := strconv.ParseFloat(item.Close, 64)
		high, _ := strconv.ParseFloat(item.High, 64)
		low, _ := strconv.ParseFloat(item.Low, 64)
		volume, _ := strconv.ParseFloat(item.Volume, 64)

		result = append(result, KlineData{
			Date:   item.Day,
			Open:   open,
			Close:  close,
			High:   high,
			Low:    low,
			Volume: volume,
			Amount: 0,
		})
	}

	return result, nil
}

// getKlineFromEM 从东方财富获取K线
func getKlineFromEM(code, period string) ([]KlineData, error) {
	// 确定市场代码
	var secid string
	if strings.HasPrefix(code, "6") {
		secid = "1." + code
	} else {
		secid = "0." + code
	}

	// 周期映射
	kltMap := map[string]string{
		"daily":   "101",
		"weekly":  "102",
		"monthly": "103",
	}
	klt := kltMap[period]
	if klt == "" {
		klt = "101"
	}

	url := fmt.Sprintf("https://push2his.eastmoney.com/api/qt/stock/kline/get?secid=%s&fields1=f1,f2,f3,f4,f5,f6&fields2=f51,f52,f53,f54,f55,f56,f57&klt=%s&fqt=1&end=20500101&lmt=250",
		secid, klt)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://quote.eastmoney.com")

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var emResp struct {
		Data struct {
			Klines []string `json:"klines"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &emResp); err != nil {
		return nil, err
	}

	var result []KlineData
	for _, line := range emResp.Data.Klines {
		parts := strings.Split(line, ",")
		if len(parts) < 7 {
			continue
		}

		open, _ := strconv.ParseFloat(parts[1], 64)
		close, _ := strconv.ParseFloat(parts[2], 64)
		high, _ := strconv.ParseFloat(parts[3], 64)
		low, _ := strconv.ParseFloat(parts[4], 64)
		volume, _ := strconv.ParseFloat(parts[5], 64)
		amount, _ := strconv.ParseFloat(parts[6], 64)

		result = append(result, KlineData{
			Date:   parts[0],
			Open:   open,
			Close:  close,
			High:   high,
			Low:    low,
			Volume: volume,
			Amount: amount,
		})
	}

	return result, nil
}
