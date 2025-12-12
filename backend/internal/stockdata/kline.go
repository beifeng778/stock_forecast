package stockdata

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
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

// GetKline 获取K线数据
func GetKline(code, period string) (*KlineResponse, error) {
	// 先尝试新浪接口
	data, err := getKlineFromSina(code, period)
	if err == nil && len(data) > 0 {
		name, _ := GetStockName(code)
		return &KlineResponse{
			Code:   code,
			Name:   name,
			Period: period,
			Data:   data,
		}, nil
	}

	// 新浪失败，尝试东方财富
	data, err = getKlineFromEM(code, period)
	if err == nil && len(data) > 0 {
		name, _ := GetStockName(code)
		return &KlineResponse{
			Code:   code,
			Name:   name,
			Period: period,
			Data:   data,
		}, nil
	}

	return nil, fmt.Errorf("获取K线数据失败")
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
