package stockdata

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// NewsItem 新闻条目
type NewsItem struct {
	Title  string `json:"title"`
	Time   string `json:"time"`
	Source string `json:"source"`
}

// GetStockNews 获取股票新闻/公告
func GetStockNews(code string, limit int) ([]NewsItem, error) {
	if limit <= 0 {
		limit = 5
	}

	url := fmt.Sprintf("https://np-anotice-stock.eastmoney.com/api/security/ann?sr=-1&page_size=%d&page_index=1&ann_type=A&stock_list=%s&f_node=0&s_node=0",
		limit, code)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data struct {
			List []struct {
				Title      string `json:"title"`
				NoticeDate string `json:"notice_date"`
			} `json:"list"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var news []NewsItem
	for _, item := range result.Data.List {
		time := item.NoticeDate
		if len(time) > 10 {
			time = time[:10]
		}
		news = append(news, NewsItem{
			Title:  item.Title,
			Time:   time,
			Source: "东方财富",
		})
	}

	return news, nil
}
