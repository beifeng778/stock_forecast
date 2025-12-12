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

	// 同时从东方财富和新浪获取，合并去重
	stocks := fetchAndMergeStocks()
	if len(stocks) == 0 {
		return nil, fmt.Errorf("获取股票列表失败")
	}

	stockListMutex.Lock()
	stockListCache = stocks
	lastFetchTime = time.Now()
	stockListMutex.Unlock()

	return stocks, nil
}

// fetchAndMergeStocks 从多个数据源获取股票并合并去重
func fetchAndMergeStocks() []Stock {
	stockMap := make(map[string]Stock)

	// 从东方财富获取
	emStocks, err := fetchStockListFromEM()
	if err == nil {
		for _, s := range emStocks {
			stockMap[s.Code] = s
		}
		fmt.Printf("东方财富贡献 %d 只股票\n", len(emStocks))
	} else {
		fmt.Printf("东方财富获取失败: %v\n", err)
	}

	// 从新浪获取
	sinaStocks, err := fetchStockListFromSina()
	if err == nil {
		added := 0
		for _, s := range sinaStocks {
			if _, exists := stockMap[s.Code]; !exists {
				stockMap[s.Code] = s
				added++
			}
		}
		fmt.Printf("新浪贡献 %d 只新股票（总获取 %d 只）\n", added, len(sinaStocks))
	} else {
		fmt.Printf("新浪获取失败: %v\n", err)
	}

	// 转换为数组
	var result []Stock
	for _, s := range stockMap {
		result = append(result, s)
	}

	fmt.Printf("合并去重后总计: %d 只股票\n", len(result))
	return result
}

// fetchStockListFromEM 从东方财富获取股票列表
func fetchStockListFromEM() ([]Stock, error) {
	var allStocks []Stock

	// 获取沪市主板 (60开头)
	shStocks, err := fetchEMStocks("m:1+t:2,m:1+t:23")
	if err == nil {
		shCount := 0
		for _, s := range shStocks {
			if strings.HasPrefix(s.Code, "6") {
				s.Market = "SH"
				allStocks = append(allStocks, s)
				shCount++
			}
		}
		fmt.Printf("东方财富沪市: 获取 %d 只, 过滤后 %d 只\n", len(shStocks), shCount)
	} else {
		fmt.Printf("获取沪市股票失败: %v\n", err)
	}

	// 获取深市主板 (00开头)
	szStocks, err := fetchEMStocks("m:0+t:6,m:0+t:80")
	if err == nil {
		szCount := 0
		for _, s := range szStocks {
			if strings.HasPrefix(s.Code, "0") {
				s.Market = "SZ"
				allStocks = append(allStocks, s)
				szCount++
			}
		}
		fmt.Printf("东方财富深市: 获取 %d 只, 过滤后 %d 只\n", len(szStocks), szCount)
	} else {
		fmt.Printf("获取深市股票失败: %v\n", err)
	}

	fmt.Printf("东方财富总计: %d 只股票\n", len(allStocks))
	return allStocks, nil
}

// fetchStockListFromSina 从新浪获取股票列表（备用）
func fetchStockListFromSina() ([]Stock, error) {
	var allStocks []Stock

	// 沪市 - 约2000只，每页80只，需要约25页
	for page := 1; page <= 50; page++ {
		stocks, err := fetchSinaStocks("sh", page)
		if err != nil {
			fmt.Printf("新浪沪市第%d页获取失败: %v\n", page, err)
			break
		}
		if len(stocks) == 0 {
			break
		}
		allStocks = append(allStocks, stocks...)
	}
	fmt.Printf("新浪沪市获取到 %d 只股票\n", len(allStocks))

	shCount := len(allStocks)

	// 深市 - 约2500只，每页80只，需要约32页
	for page := 1; page <= 50; page++ {
		stocks, err := fetchSinaStocks("sz", page)
		if err != nil {
			fmt.Printf("新浪深市第%d页获取失败: %v\n", page, err)
			break
		}
		if len(stocks) == 0 {
			break
		}
		allStocks = append(allStocks, stocks...)
	}
	fmt.Printf("新浪深市获取到 %d 只股票\n", len(allStocks)-shCount)

	if len(allStocks) == 0 {
		return nil, fmt.Errorf("获取股票列表失败")
	}

	fmt.Printf("从新浪总共获取到 %d 只股票\n", len(allStocks))
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

	// diff 结构
	type DiffItem struct {
		F12 string `json:"f12"` // 代码
		F14 string `json:"f14"` // 名称
	}

	var diffList []DiffItem

	// 检查 diff 是否为空
	if len(result.Data.Diff) == 0 || string(result.Data.Diff) == "null" {
		fmt.Printf("东方财富返回空数据, fs=%s\n", fs)
		return nil, fmt.Errorf("东方财富返回空数据")
	}

	// 先尝试解析为数组
	if err := json.Unmarshal(result.Data.Diff, &diffList); err != nil {
		// 如果失败，尝试解析为对象（key-value 形式，如 {"0": {...}, "1": {...}}）
		var diffMap map[string]DiffItem
		if err2 := json.Unmarshal(result.Data.Diff, &diffMap); err2 != nil {
			fmt.Printf("解析diff失败(数组): %v, 解析diff失败(对象): %v\n", err, err2)
			return nil, err
		}
		// 从对象转换为数组
		for _, item := range diffMap {
			diffList = append(diffList, item)
		}
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

	fmt.Printf("股票总数: %d, 搜索关键词: %s\n", len(allStocks), keyword)

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
