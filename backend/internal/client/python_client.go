package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

var pythonServiceURL string

func init() {
	pythonServiceURL = os.Getenv("PYTHON_SERVICE_URL")
	if pythonServiceURL == "" {
		pythonServiceURL = "http://localhost:5000"
	}
}

// HTTPClient HTTP客户端
var HTTPClient = &http.Client{
	Timeout: 60 * time.Second,
}

// GetStocks 从Python服务获取股票列表
func GetStocks(keyword string) ([]map[string]interface{}, error) {
	reqURL := fmt.Sprintf("%s/api/stocks?keyword=%s", pythonServiceURL, url.QueryEscape(keyword))
	resp, err := HTTPClient.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("请求Python服务失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("��取响应失败: %v", err)
	}

	var result struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	return result.Data, nil
}

// GetKline 从Python服务获取K线数据
func GetKline(code, period string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/stocks/%s/kline?period=%s", pythonServiceURL, code, period)
	resp, err := HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求Python服务失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	return result, nil
}

// GetIndicators 从Python服务获取技术指标
func GetIndicators(code string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/stocks/%s/indicators", pythonServiceURL, code)
	resp, err := HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求Python服务失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	return result, nil
}

// MLPredictRequest ML预测请求
type MLPredictRequest struct {
	StockCode string `json:"stock_code"`
	Period    string `json:"period"`
}

// GetMLPrediction 从Python服务获取ML预测
func GetMLPrediction(code, period string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/ml/predict?code=%s&period=%s", pythonServiceURL, code, period)
	resp, err := HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求Python服务失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	return result, nil
}

// NewsItem 新闻条目
type NewsItem struct {
	Title  string `json:"title"`
	Time   string `json:"time"`
	Source string `json:"source"`
}

// GetStockNews 获取股票新闻
func GetStockNews(code string) ([]NewsItem, error) {
	url := fmt.Sprintf("%s/api/stocks/%s/news", pythonServiceURL, code)
	resp, err := HTTPClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []NewsItem `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// GetStockName 获取股票名称
func GetStockName(code string) (string, error) {
	url := fmt.Sprintf("%s/api/stocks/%s/info", pythonServiceURL, code)
	resp, err := HTTPClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("请求Python服务失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	var result struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}

	return result.Name, nil
}
