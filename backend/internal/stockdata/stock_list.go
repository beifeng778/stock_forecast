package stockdata

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Stock 股票信息
type Stock struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Market string `json:"market"`
}

var (
	stockListCache []Stock
	stockListMutex sync.RWMutex
	lastFetchTime  time.Time
	cacheDuration  = 24 * time.Hour
)

// HTTPClient HTTP客户端
var HTTPClient = &http.Client{
	Timeout: 30 * time.Second,
}

// GetStockList 获取A股股票列表
func GetStockList() ([]Stock, error) {
	stockListMutex.RLock()
	if len(stockListCache) > 0 && time.Since(lastFetchTime) < cacheDuration {
		defer stockListMutex.RUnlock()
		return stockListCache, nil
	}
	stockListMutex.RUnlock()

	// 从东方财富获取股票列表
	stocks, err := fetchStockListFromEM()
	if err != nil {
		return nil, err
	}

	stockListMutex.Lock()
	stockListCache = stocks
	lastFetchTime = time.Now()
	stockListMutex.Unlock()

	return stocks, nil
}

// fetchStockListFromEM 从东方财富获取股票列表
func fetchStockListFromEM() ([]Stock, error) {
	var allStocks []Stock

	// 获取沪市主板 (60开头)
	shStocks, err := fetchEMStocks("m:1+t:2,m:1+t:23")
	if err == nil {
		for _, s := range shStocks {
			if strings.HasPrefix(s.Code, "6") {
				s.Market = "SH"
				allStocks = append(allStocks, s)
			}
		}
	} else {
		fmt.Printf("获取沪市股票失败: %v\n", err)
	}

	// 获取深市主板 (00开头)
	szStocks, err := fetchEMStocks("m:0+t:6,m:0+t:80")
	if err == nil {
		for _, s := range szStocks {
			if strings.HasPrefix(s.Code, "0") {
				s.Market = "SZ"
				allStocks = append(allStocks, s)
			}
		}
	} else {
		fmt.Printf("获取深市股票失败: %v\n", err)
	}

	// 如果东方财富失败，尝试新浪接口
	if len(allStocks) == 0 {
		fmt.Println("东方财富接口失败，尝试新浪接口...")
		return fetchStockListFromSina()
	}

	return allStocks, nil
}

// fetchStockListFromSina 从新浪获取股票列表（备用）
func fetchStockListFromSina() ([]Stock, error) {
	var allStocks []Stock

	// 沪市
	for page := 1; page <= 20; page++ {
		stocks, err := fetchSinaStocks("sh", page)
		if err != nil || len(stocks) == 0 {
			break
		}
		allStocks = append(allStocks, stocks...)
	}

	// 深市
	for page := 1; page <= 20; page++ {
		stocks, err := fetchSinaStocks("sz", page)
		if err != nil || len(stocks) == 0 {
			break
		}
		allStocks = append(allStocks, stocks...)
	}

	if len(allStocks) == 0 {
		return nil, fmt.Errorf("获取股票列表失败")
	}

	fmt.Printf("从新浪获取到 %d 只股票\n", len(allStocks))
	return allStocks, nil
}

// fetchSinaStocks 从新浪获取股票
func fetchSinaStocks(market string, page int) ([]Stock, error) {
	url := fmt.Sprintf("https://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/Market_Center.getHQNodeData?page=%d&num=80&sort=symbol&asc=1&node=%s_a&symbol=&_s_r_a=auto", page, market)

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

	var items []struct {
		Symbol string `json:"symbol"`
		Name   string `json:"name"`
	}

	if err := json.Unmarshal(body, &items); err != nil {
		return nil, err
	}

	var stocks []Stock
	for _, item := range items {
		code := strings.TrimPrefix(item.Symbol, "sh")
		code = strings.TrimPrefix(code, "sz")

		// 只保留 6 和 0 开头的
		if strings.HasPrefix(code, "6") || strings.HasPrefix(code, "0") {
			mkt := "SZ"
			if strings.HasPrefix(code, "6") {
				mkt = "SH"
			}
			stocks = append(stocks, Stock{
				Code:   code,
				Name:   item.Name,
				Market: mkt,
			})
		}
	}

	return stocks, nil
}

// fetchEMStocks 从东方财富API获取股票
func fetchEMStocks(fs string) ([]Stock, error) {
	url := fmt.Sprintf("https://push2.eastmoney.com/api/qt/clist/get?pn=1&pz=5000&fs=%s&fields=f12,f14", fs)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://quote.eastmoney.com")

	resp, err := HTTPClient.Do(req)
	if err != nil {
		fmt.Printf("请求东方财富API失败: %v\n", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("读取响应失败: %v\n", err)
		return nil, err
	}

	// 东方财富返回的 diff 可能是数组或 null
	var result struct {
		Data struct {
			Diff json.RawMessage `json:"diff"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Printf("解析JSON失败: %v, body: %s\n", err, string(body[:min(200, len(body))]))
		return nil, err
	}

	// 尝试解析 diff 数组
	var diffList []struct {
		F12 string `json:"f12"` // 代码
		F14 string `json:"f14"` // 名称
	}

	if err := json.Unmarshal(result.Data.Diff, &diffList); err != nil {
		fmt.Printf("解析diff失败: %v\n", err)
		return nil, err
	}

	var stocks []Stock
	for _, item := range diffList {
		stocks = append(stocks, Stock{
			Code: item.F12,
			Name: item.F14,
		})
	}

	fmt.Printf("从东方财富获取到 %d 只股票\n", len(stocks))
	return stocks, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SearchStocks 搜索股票
func SearchStocks(keyword string) ([]Stock, error) {
	allStocks, err := GetStockList()
	if err != nil {
		return nil, err
	}

	if keyword == "" {
		return allStocks, nil
	}

	keyword = strings.ToUpper(keyword)
	var result []Stock
	for _, s := range allStocks {
		if strings.Contains(s.Code, keyword) || strings.Contains(strings.ToUpper(s.Name), keyword) {
			result = append(result, s)
			if len(result) >= 100 {
				break
			}
		}
	}

	return result, nil
}

// GetStockInfo 获取股票信息
func GetStockInfo(code string) (*Stock, error) {
	allStocks, err := GetStockList()
	if err != nil {
		return nil, err
	}

	for _, s := range allStocks {
		if s.Code == code {
			return &s, nil
		}
	}

	return nil, fmt.Errorf("股票不存在: %s", code)
}

// GetStockName 获取股票名称
func GetStockName(code string) (string, error) {
	info, err := GetStockInfo(code)
	if err != nil {
		return "", err
	}
	return info.Name, nil
}
