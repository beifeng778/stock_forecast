package stockdata

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"stock-forecast-backend/internal/cache"
	"strings"
	"sync"
	"time"
)

// Stock 股票信息
type Stock struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Market   string `json:"market"`
	Industry string `json:"industry"`
}

const (
	stockListCacheKey = "stock:list"
	cacheDuration     = 24 * time.Hour
)

var (
	stockListCache []Stock
	stockListMutex sync.RWMutex
	lastFetchTime  time.Time
)

// HTTPClient HTTP客户端
var HTTPClient = &http.Client{
	Timeout: 30 * time.Second,
}

// GetStockList 获取A股股票列表
func GetStockList() ([]Stock, error) {
	return GetStockListWithRefresh(false)
}

// RefreshStockCache 刷新股票缓存（全量替换）
func RefreshStockCache() ([]Stock, error) {
	// 从数据源获取新数据
	newStocks := fetchAndMergeStocks()

	if len(newStocks) == 0 {
		return nil, fmt.Errorf("获取股票列表失败")
	}

	// 全量替换到Redis
	if err := cache.Set(stockListCacheKey, newStocks, cacheDuration); err != nil {
		return nil, fmt.Errorf("保存到Redis失败: %v", err)
	}

	fmt.Printf("股票缓存全量刷新完成: %d 只股票\n", len(newStocks))
	return newStocks, nil
}

// GetStockListWithRefresh 获取A股股票列表，支持强制刷新
func GetStockListWithRefresh(forceRefresh bool) ([]Stock, error) {
	stocks, _ := GetStockListWithRefresh2(forceRefresh)
	if len(stocks) == 0 {
		return nil, fmt.Errorf("获取股票列表失败")
	}
	return stocks, nil
}

// GetStockListWithRefresh2 获取A股股票列表，返回是否来自缓存
func GetStockListWithRefresh2(forceRefresh bool) ([]Stock, bool) {
	// 1. 尝试从Redis获取缓存
	if !forceRefresh {
		var cachedStocks []Stock
		if err := cache.Get(stockListCacheKey, &cachedStocks); err == nil && len(cachedStocks) > 0 {
			fmt.Printf("从Redis缓存获取 %d 只股票\n", len(cachedStocks))
			return cachedStocks, true
		}
	}

	// 2. 获取现有缓存用于增量更新
	var existingStocks []Stock
	if forceRefresh {
		cache.Get(stockListCacheKey, &existingStocks)
	}

	// 3. 从数据源获取新数据
	newStocks := fetchAndMergeStocks()

	if len(newStocks) == 0 {
		// 获取失败，返回缓存数据（如果有）
		if len(existingStocks) > 0 {
			return existingStocks, true
		}
		return nil, false
	}

	// 4. 增量合并
	var finalStocks []Stock
	if len(existingStocks) > 0 {
		stockMap := make(map[string]Stock)
		for _, s := range existingStocks {
			stockMap[s.Code] = s
		}
		newCount := 0
		for _, s := range newStocks {
			if _, exists := stockMap[s.Code]; !exists {
				newCount++
			}
			stockMap[s.Code] = s
		}
		finalStocks = make([]Stock, 0, len(stockMap))
		for _, s := range stockMap {
			finalStocks = append(finalStocks, s)
		}
		fmt.Printf("增量更新: 新增 %d 只股票, 总计 %d 只\n", newCount, len(finalStocks))
	} else {
		finalStocks = newStocks
		fmt.Printf("全量更新: 获取 %d 只股票\n", len(finalStocks))
	}

	// 5. 保存到Redis
	if err := cache.Set(stockListCacheKey, finalStocks, cacheDuration); err != nil {
		fmt.Printf("保存到Redis失败: %v\n", err)
	}

	return finalStocks, false
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
		fmt.Printf("东方财富沪市主板: 获取 %d 只, 过滤后 %d 只\n", len(shStocks), shCount)
	} else {
		fmt.Printf("获取沪市主板股票失败: %v\n", err)
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
		fmt.Printf("东方财富深市主板: 获取 %d 只, 过滤后 %d 只\n", len(szStocks), szCount)
	} else {
		fmt.Printf("获取深市主板股票失败: %v\n", err)
	}

	// 获取创业板 (300开头)
	cyStocks, err := fetchEMStocks("m:0+t:80")
	if err == nil {
		cyCount := 0
		for _, s := range cyStocks {
			if strings.HasPrefix(s.Code, "300") || strings.HasPrefix(s.Code, "301") {
				s.Market = "SZ"
				allStocks = append(allStocks, s)
				cyCount++
			}
		}
		fmt.Printf("东方财富创业板: 获取 %d 只, 过滤后 %d 只\n", len(cyStocks), cyCount)
	} else {
		fmt.Printf("获取创业板股票失败: %v\n", err)
	}

	// 获取科创板 (688开头)
	kcStocks, err := fetchEMStocks("m:1+t:23")
	if err == nil {
		kcCount := 0
		for _, s := range kcStocks {
			if strings.HasPrefix(s.Code, "688") {
				s.Market = "SH"
				allStocks = append(allStocks, s)
				kcCount++
			}
		}
		fmt.Printf("东方财富科创板: 获取 %d 只, 过滤后 %d 只\n", len(kcStocks), kcCount)
	} else {
		fmt.Printf("获取科创板股票失败: %v\n", err)
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
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://vip.stock.finance.sina.com.cn/")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

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
		// 打印返回内容用于调试
		fmt.Printf("新浪API返回内容(前200字符): %s\n", string(body[:min(200, len(body))]))
		return nil, fmt.Errorf("%v: %s", err, string(body[:min(100, len(body))]))
	}

	var stocks []Stock
	for _, item := range items {
		code := strings.TrimPrefix(item.Symbol, "sh")
		code = strings.TrimPrefix(code, "sz")

		// 保留主板(6/0开头)、创业板(300/301开头)、科创板(688开头)
		if strings.HasPrefix(code, "6") || strings.HasPrefix(code, "0") || strings.HasPrefix(code, "3") {
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
func fetchEMStocks(fs string) ([]Stock, error) {
	var allStocks []Stock
	pageSize := 100 // 东方财富API每页最多返回100条

	// 分页获取，每页100条，最多获取50页（5000只股票）
	for page := 1; page <= 50; page++ {
		var stocks []Stock
		var err error

		// 重试3次
		for retry := 0; retry < 3; retry++ {
			stocks, err = fetchEMStocksPage(fs, page, pageSize)
			if err == nil {
				break
			}
			time.Sleep(500 * time.Millisecond) // 重试前等待500ms
		}

		if err != nil {
			fmt.Printf("东方财富第%d页获取失败: %v\n", page, err)
			break
		}
		if len(stocks) == 0 {
			break
		}
		allStocks = append(allStocks, stocks...)
		if len(stocks) < pageSize {
			break // 不足一页，说明已经获取完毕
		}

		// 每页请求间隔50ms，避免被限流
		time.Sleep(50 * time.Millisecond)
	}

	fmt.Printf("从东方财富获取到 %d 只股票\n", len(allStocks))
	return allStocks, nil
}

// fetchEMStocksPage 从东方财富API获取单页股票
func fetchEMStocksPage(fs string, page, pageSize int) ([]Stock, error) {
	url := fmt.Sprintf("https://push2.eastmoney.com/api/qt/clist/get?pn=%d&pz=%d&fs=%s&fields=f12,f14,f100", page, pageSize, fs)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
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
		F12  string `json:"f12"`  // 代码
		F14  string `json:"f14"`  // 名称
		F100 string `json:"f100"` // 行业
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
			Code:     item.F12,
			Name:     item.F14,
			Industry: item.F100,
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
	stocks, _, _ := SearchStocksWithRefresh(keyword, false)
	return stocks, nil
}

// SearchStocksResult 搜索结果
type SearchStocksResult struct {
	Stocks    []Stock `json:"stocks"`
	FromCache bool    `json:"fromCache"`
	Total     int     `json:"total"`
}

// SearchStocksWithRefresh 搜索股票，支持强制刷新，返回是否来自缓存
// 第三个返回值表示刷新是否失败（用于前端显示错误提示）
func SearchStocksWithRefresh(keyword string, forceRefresh bool) ([]Stock, bool, bool) {
	allStocks, fromCache := GetStockListWithRefresh2(forceRefresh)

	// 如果是刷新操作但返回的是缓存数据，说明第三方接口获取失败
	refreshFailed := forceRefresh && fromCache

	if len(allStocks) == 0 {
		return nil, false, true
	}

	fmt.Printf("股票总数: %d, 搜索关键词: %s\n", len(allStocks), keyword)

	if keyword == "" {
		return allStocks, fromCache, refreshFailed
	}

	keyword = strings.ToUpper(keyword)

	// 分类匹配结果：精确匹配 > 前缀匹配 > 包含匹配
	var exactMatch []Stock     // 代码或名称完全匹配
	var nameExactMatch []Stock // 名称完全等于关键词
	var prefixMatch []Stock    // 代码或名称前缀匹配
	var containMatch []Stock   // 代码或名称包含匹配

	for _, s := range allStocks {
		upperName := strings.ToUpper(s.Name)

		// 精确匹配（代码完全匹配）
		if s.Code == keyword {
			exactMatch = append(exactMatch, s)
		} else if upperName == keyword {
			// 名称完全匹配
			nameExactMatch = append(nameExactMatch, s)
		} else if strings.HasPrefix(s.Code, keyword) || strings.HasPrefix(upperName, keyword) {
			// 前缀匹配
			prefixMatch = append(prefixMatch, s)
		} else if strings.Contains(s.Code, keyword) || strings.Contains(upperName, keyword) {
			// 包含匹配
			containMatch = append(containMatch, s)
		}
	}

	// 合并结果，按优先级排序
	var result []Stock
	result = append(result, exactMatch...)
	result = append(result, nameExactMatch...)
	result = append(result, prefixMatch...)
	result = append(result, containMatch...)

	// 限制返回数量
	if len(result) > 100 {
		result = result[:100]
	}

	return result, fromCache, false
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

// GetStockIndustry 获取股票行业（从东方财富单独获取）
func GetStockIndustry(code string) string {
	// 先从缓存中查找
	info, err := GetStockInfo(code)
	if err == nil && info.Industry != "" {
		return info.Industry
	}

	// 缓存中没有行业信息，从东方财富单独获取
	market := "0" // 深市
	if strings.HasPrefix(code, "6") {
		market = "1" // 沪市
	}

	url := fmt.Sprintf("https://push2.eastmoney.com/api/qt/stock/get?secid=%s.%s&fields=f100", market, code)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://quote.eastmoney.com")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	var result struct {
		Data struct {
			F100 string `json:"f100"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return ""
	}

	return result.Data.F100
}
