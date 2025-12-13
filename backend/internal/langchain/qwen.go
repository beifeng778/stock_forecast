package langchain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"stock-forecast-backend/internal/model"
)

var (
	dashscopeAPIKey string
	llmModel        string
)

func init() {
	dashscopeAPIKey = os.Getenv("DASHSCOPE_API_KEY")
	llmModel = os.Getenv("LLM_MODEL")
	if llmModel == "" {
		llmModel = "qwen-plus"
	}
}

// QwenRequest 通义千问请求
type QwenRequest struct {
	Model string `json:"model"`
	Input struct {
		Messages []Message `json:"messages"`
	} `json:"input"`
}

// Message 消息
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// QwenResponse 通义千问响应
type QwenResponse struct {
	Output struct {
		Text    string `json:"text"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	} `json:"output"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// NewsItem 新闻条目
type NewsItem struct {
	Title  string `json:"title"`
	Time   string `json:"time"`
	Source string `json:"source"`
}

// AnalyzeStock 使用通义千问分析股票
func AnalyzeStock(code, name string, indicators model.TechnicalIndicators, ml model.MLPredictions, signals []model.Signal, news []NewsItem) (string, error) {
	if dashscopeAPIKey == "" {
		fmt.Println("[LLM] DASHSCOPE_API_KEY 未配置，使用备用分析")
		return generateFallbackAnalysis(code, name, indicators, ml, signals), nil
	}
	fmt.Printf("[LLM] 使用模型: %s, 新闻数量: %d\n", llmModel, len(news))

	prompt := buildAnalysisPrompt(code, name, indicators, ml, signals, news)

	req := QwenRequest{
		Model: llmModel,
	}
	req.Input.Messages = []Message{
		{
			Role:    "system",
			Content: "你是一位专业的股票分析师，擅长技术分析和趋势预测。请根据提供的技术指标和模型预测结果，给出专业、客观的分析建议。回答要简洁明了，控制在200字以内。",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}
	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %v", err)
	}

	httpReq, err := http.NewRequest("POST", "https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+dashscopeAPIKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	fmt.Printf("[LLM] API响应状态: %d, 响应内容: %s\n", resp.StatusCode, string(body)[:min(500, len(body))])

	var qwenResp QwenResponse
	if err := json.Unmarshal(body, &qwenResp); err != nil {
		fmt.Printf("[LLM] 解析响应失败: %v\n", err)
		return generateFallbackAnalysis(code, name, indicators, ml, signals), nil
	}

	// 优先从 choices 获取结果（qwen3 格式），否则从 text 获取（旧格式）
	result := qwenResp.Output.Text
	if result == "" && len(qwenResp.Output.Choices) > 0 {
		result = qwenResp.Output.Choices[0].Message.Content
	}

	if result == "" {
		fmt.Println("[LLM] API返回空结果，使用备用分析")
		return generateFallbackAnalysis(code, name, indicators, ml, signals), nil
	}

	return result, nil
}

// buildAnalysisPrompt 构建分析提示词
func buildAnalysisPrompt(code, name string, indicators model.TechnicalIndicators, ml model.MLPredictions, signals []model.Signal, news []NewsItem) string {
	signalStr := ""
	for _, s := range signals {
		signalStr += fmt.Sprintf("%s: %s; ", s.Name, s.TypeCN)
	}

	newsStr := ""
	if len(news) > 0 {
		newsStr = "\n\n最新公告/新闻（请分析这些公告对股价的潜在影响）：\n"
		for i, n := range news {
			newsStr += fmt.Sprintf("%d. [%s] %s\n", i+1, n.Time, n.Title)
		}
	}

	return fmt.Sprintf(`请分析股票 %s（%s）的走势：

技术指标：
- 均线：MA5=%.2f, MA10=%.2f, MA20=%.2f, MA60=%.2f
- MACD：DIF=%.4f, DEA=%.4f, MACD柱=%.4f
- RSI：%.2f
- KDJ：K=%.2f, D=%.2f, J=%.2f
- 布林带：上轨=%.2f, 中轨=%.2f, 下轨=%.2f

技术信号：%s

ML模型预测：
- LSTM：趋势=%s, 置信度=%.1f%%
- Prophet：趋势=%s, 置信度=%.1f%%
- XGBoost：趋势=%s, 置信度=%.1f%%%s

请结合技术面和消息面给出综合分析和操作建议。`,
		name, code,
		indicators.MA5, indicators.MA10, indicators.MA20, indicators.MA60,
		indicators.MACD, indicators.Signal, indicators.Hist,
		indicators.RSI,
		indicators.KDJ_K, indicators.KDJ_D, indicators.KDJ_J,
		indicators.BOLL_U, indicators.BOLL_M, indicators.BOLL_L,
		signalStr,
		ml.LSTM.Trend, ml.LSTM.Confidence*100,
		ml.Prophet.Trend, ml.Prophet.Confidence*100,
		ml.XGBoost.Trend, ml.XGBoost.Confidence*100,
		newsStr,
	)
}

// generateFallbackAnalysis 生成备用分析（当API不可用时）
func generateFallbackAnalysis(code, name string, indicators model.TechnicalIndicators, ml model.MLPredictions, signals []model.Signal) string {
	bullish := 0
	bearish := 0

	for _, s := range signals {
		if s.Type == "bullish" {
			bullish++
		} else if s.Type == "bearish" {
			bearish++
		}
	}

	if ml.LSTM.Trend == "up" {
		bullish++
	} else if ml.LSTM.Trend == "down" {
		bearish++
	}

	var trend string
	if bullish > bearish {
		trend = "偏多"
	} else if bearish > bullish {
		trend = "偏空"
	} else {
		trend = "震荡"
	}

	return fmt.Sprintf("根据技术指标和模型预测，%s（%s）当前走势%s。MACD指标显示%s信号，RSI值为%.1f处于%s区间。建议结合市场整体环境和个人风险偏好做出投资决策。",
		name, code, trend,
		getMACD信号(indicators.MACD, indicators.Signal),
		indicators.RSI,
		getRSI区间(indicators.RSI),
	)
}

func getMACD信号(macd, signal float64) string {
	if macd > signal {
		return "金叉"
	}
	return "死叉"
}

func getRSI区间(rsi float64) string {
	if rsi > 70 {
		return "超买"
	} else if rsi < 30 {
		return "超卖"
	}
	return "正常"
}

// StockClassification 股票分类信息
type StockClassification struct {
	Sector   string `json:"sector"`   // 板块
	Industry string `json:"industry"` // 主营业务行业
}

// GetStockClassification 使用LLM获取股票板块和行业
func GetStockClassification(code, name string) StockClassification {
	if dashscopeAPIKey == "" {
		return StockClassification{}
	}

	req := QwenRequest{
		Model: llmModel,
	}
	req.Input.Messages = []Message{
		{
			Role: "system",
			Content: `你是一个股票数据助手。请返回股票所属的行业板块，格式为JSON：{"sector":"行业板块"}

行业板块必须从以下列表中选择一个最匹配的：
电网设备、半导体、证券、电力、零售、贵金属、房地产、通信设备、军工电子、白酒、银行、军工装备、电池、影视院线、铜、金属新材料、通用设备、光伏设备、化学原料、软件开发

注意：
- 黄金、白银等贵金属相关股票应归类为"贵金属"
- 只能从上述列表中选择，不要使用其他分类名称
- 只返回JSON，不要有其他内容。`,
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("股票%s（代码%s）在同花顺中属于哪个行业板块？", name, code),
		},
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return StockClassification{}
	}

	httpReq, err := http.NewRequest("POST", "https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation", bytes.NewBuffer(jsonData))
	if err != nil {
		return StockClassification{}
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+dashscopeAPIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return StockClassification{}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return StockClassification{}
	}

	var qwenResp QwenResponse
	if err := json.Unmarshal(body, &qwenResp); err != nil {
		return StockClassification{}
	}

	result := qwenResp.Output.Text
	if result == "" && len(qwenResp.Output.Choices) > 0 {
		result = qwenResp.Output.Choices[0].Message.Content
	}

	// 解析JSON结果
	var classification StockClassification
	if err := json.Unmarshal([]byte(result), &classification); err != nil {
		// 如果解析失败，尝试提取JSON部分
		if start := findJSONStart(result); start >= 0 {
			if end := findJSONEnd(result, start); end > start {
				json.Unmarshal([]byte(result[start:end+1]), &classification)
			}
		}
	}

	return classification
}

func findJSONStart(s string) int {
	for i, c := range s {
		if c == '{' {
			return i
		}
	}
	return -1
}

func findJSONEnd(s string, start int) int {
	depth := 0
	for i := start; i < len(s); i++ {
		if s[i] == '{' {
			depth++
		} else if s[i] == '}' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
