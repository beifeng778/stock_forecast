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
	Parameters struct {
		ResultFormat string `json:"result_format"`
	} `json:"parameters"`
}

// Message 消息
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// QwenResponse 通义千问响应
type QwenResponse struct {
	Output struct {
		Text string `json:"text"`
	} `json:"output"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// AnalyzeStock 使用通义千问分析股票
func AnalyzeStock(code, name string, indicators model.TechnicalIndicators, ml model.MLPredictions, signals []model.Signal) (string, error) {
	if dashscopeAPIKey == "" {
		return generateFallbackAnalysis(code, name, indicators, ml, signals), nil
	}

	prompt := buildAnalysisPrompt(code, name, indicators, ml, signals)

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
	req.Parameters.ResultFormat = "text"

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

	var qwenResp QwenResponse
	if err := json.Unmarshal(body, &qwenResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}

	if qwenResp.Output.Text == "" {
		return generateFallbackAnalysis(code, name, indicators, ml, signals), nil
	}

	return qwenResp.Output.Text, nil
}

// buildAnalysisPrompt 构建分析提示词
func buildAnalysisPrompt(code, name string, indicators model.TechnicalIndicators, ml model.MLPredictions, signals []model.Signal) string {
	signalStr := ""
	for _, s := range signals {
		signalStr += fmt.Sprintf("%s: %s; ", s.Name, s.TypeCN)
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
- XGBoost：趋势=%s, 置信度=%.1f%%

请给出综合分析和操作建议。`,
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
