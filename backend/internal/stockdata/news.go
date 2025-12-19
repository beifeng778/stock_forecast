package stockdata

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

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

func extractJSONPBody(b []byte) []byte {
	s := string(b)
	start := strings.Index(s, "(")
	end := strings.LastIndex(s, ")")
	if start < 0 || end < 0 || end <= start {
		return b
	}
	return []byte(s[start+1 : end])
}

func stripHTMLTags(s string) string {
	if strings.IndexByte(s, '<') < 0 {
		return s
	}
	return htmlTagRe.ReplaceAllString(s, "")
}

// GetStockMediaNews 获取东方财富个股媒体新闻（标题为主）
func GetStockMediaNews(code string, limit int) ([]NewsItem, error) {
	if limit <= 0 {
		limit = 10
	}

	cb := fmt.Sprintf("jQuery%d_%d", time.Now().UnixNano(), time.Now().UnixMilli())
	paramBody := map[string]any{
		"uid":           "",
		"keyword":       strings.TrimSpace(code),
		"type":          []string{"cmsArticleWebOld"},
		"client":        "web",
		"clientType":    "web",
		"clientVersion": "curr",
		"params": map[string]any{
			"cmsArticleWebOld": map[string]any{
				"searchScope": "default",
				"sort":        "default",
				"pageIndex":   1,
				"pageSize":    limit,
				"preTag":      "<em>",
				"postTag":     "</em>",
			},
		},
	}
	paramJSON, _ := json.Marshal(paramBody)

	u, _ := url.Parse("https://search-api-web.eastmoney.com/search/jsonp")
	q := u.Query()
	q.Set("cb", cb)
	q.Set("param", string(paramJSON))
	u.RawQuery = q.Encode()

	req, _ := http.NewRequest("GET", u.String(), nil)
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

	jsonBody := extractJSONPBody(body)
	var result struct {
		Result struct {
			CmsArticleWebOld []struct {
				Title     string `json:"title"`
				Date      string `json:"date"`
				MediaName string `json:"mediaName"`
			} `json:"cmsArticleWebOld"`
		} `json:"result"`
	}
	if err := json.Unmarshal(jsonBody, &result); err != nil {
		return nil, err
	}

	news := make([]NewsItem, 0, len(result.Result.CmsArticleWebOld))
	for _, item := range result.Result.CmsArticleWebOld {
		t := strings.TrimSpace(item.Date)
		if len(t) >= 10 {
			t = t[:10]
		}
		source := strings.TrimSpace(item.MediaName)
		if source == "" {
			source = "东方财富"
		}
		title := strings.TrimSpace(stripHTMLTags(item.Title))
		if title == "" {
			continue
		}
		news = append(news, NewsItem{Title: title, Time: t, Source: source})
	}
	return news, nil
}
