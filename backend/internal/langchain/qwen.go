package langchain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"stock-forecast-backend/internal/model"
	"stock-forecast-backend/pkg/llmsamples"
)

var (
	llmBaseURL      string
	llmAuthToken    string
	llmModel        string
	llmSamplesPath  string
	llmSamplesTopK  int
	llmDebugSamples bool
)

var llmConfigOnce sync.Once

func ensureLLMConfig() {
	llmConfigOnce.Do(loadLLMConfig)
}

func loadLLMConfig() {
	llmBaseURL = strings.TrimSpace(os.Getenv("LLM_BASE_URL"))
	if llmBaseURL == "" {
		llmBaseURL = "https://api.openai.com"
	}
	llmAuthToken = strings.TrimSpace(os.Getenv("LLM_AUTH_TOKEN"))
	llmModel = os.Getenv("LLM_MODEL")
	if llmModel == "" {
		llmModel = "gpt-4o-mini"
	}
	llmDebugSamples = getEnvBool("LLM_DEBUG_SAMPLES", false)
	llmSamplesPath = os.Getenv("LLM_SAMPLES_PATH")
	if llmSamplesPath == "" {
		llmSamplesPath = "/app/rag"
	}
	llmSamplesPath = ResolveLLMSamplesPath(llmSamplesPath)
	llmSamplesTopK = 3
	if v := strings.TrimSpace(os.Getenv("LLM_SAMPLES_TOPK")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 20 {
			llmSamplesTopK = n
		}
	}
}

func getEnvBool(name string, defaultValue bool) bool {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return defaultValue
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return defaultValue
	}
}

func chatCompletionsURL() string {
	ensureLLMConfig()
	return strings.TrimRight(llmBaseURL, "/") + "/v1/chat/completions"
}

func callChatCompletions(messages []Message, timeout time.Duration) (string, error) {
	ensureLLMConfig()
	if strings.TrimSpace(llmAuthToken) == "" {
		return "", fmt.Errorf("LLM_AUTH_TOKEN 未配置")
	}

	req := OpenAIChatCompletionRequest{
		Model:    llmModel,
		Messages: messages,
	}
	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %v", err)
	}

	httpReq, err := http.NewRequest("POST", chatCompletionsURL(), bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+llmAuthToken)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	var out OpenAIChatCompletionResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}
	if out.Error != nil && strings.TrimSpace(out.Error.Message) != "" {
		return "", fmt.Errorf("LLM错误: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("LLM返回choices为空")
	}
	content := out.Choices[0].Message.Content
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("LLM返回空内容")
	}
	return content, nil
}

func ResolveLLMSamplesPath(p string) string {
	return llmsamples.ResolvePath(p)
}

type ohlcvLLMResponse struct {
	AIToday      *model.KlineData  `json:"ai_today"`
	FutureKlines []model.KlineData `json:"future_klines"`
	Confidence   float64           `json:"confidence"`
	Reasons      []string          `json:"reasons"`
}

type llmSample struct {
	ID       string `json:"id"`
	Features struct {
		RSI           float64 `json:"rsi"`
		Volatility    float64 `json:"volatility"`
		Change5D      float64 `json:"change_5d"`
		MA5Slope      float64 `json:"ma5_slope"`
		MomentumScore float64 `json:"momentum_score"`
	} `json:"features"`
	Outcome struct {
		Future1D float64 `json:"future_1d"`
		Future5D float64 `json:"future_5d"`
	} `json:"outcome"`
}

func loadTopLLMSamples(path string, ind model.TechnicalIndicators, topK int) []llmSample {
	loaded, err := llmsamples.QueryTopK(path, llmsamples.Indicators{
		RSI:           ind.RSI,
		Volatility:    ind.Volatility,
		Change5D:      ind.Change5D,
		MA5Slope:      ind.MA5Slope,
		MomentumScore: ind.MomentumScore,
	}, topK)
	if err != nil {
		if llmDebugSamples {
			fmt.Printf("[LLM][samples] QueryTopK failed: db=%s err=%v\n", path, err)
		}
		return nil
	}
	if len(loaded) == 0 {
		return nil
	}

	out := make([]llmSample, 0, len(loaded))
	for _, s := range loaded {
		var x llmSample
		x.ID = s.ID
		x.Features.RSI = s.RSI
		x.Features.Volatility = s.Volatility
		x.Features.Change5D = s.Change5D
		x.Features.MA5Slope = s.MA5Slope
		x.Features.MomentumScore = s.MomentumScore
		x.Outcome.Future1D = s.Future1D
		x.Outcome.Future5D = s.Future5D
		out = append(out, x)
	}
	return out
}

func sampleIDs(samples []llmSample) string {
	if len(samples) == 0 {
		return ""
	}
	ids := make([]string, 0, len(samples))
	for _, s := range samples {
		if strings.TrimSpace(s.ID) != "" {
			ids = append(ids, s.ID)
		}
	}
	return strings.Join(ids, ",")
}

func extractJSONObject(s string) string {
	text := strings.TrimSpace(s)
	if text == "" {
		return ""
	}
	if i := strings.Index(text, "```json"); i >= 0 {
		text = text[i+len("```json"):]
		if j := strings.Index(text, "```"); j >= 0 {
			text = text[:j]
		}
		text = strings.TrimSpace(text)
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	return strings.TrimSpace(text[start : end+1])
}

func buildOHLCVPrompt(code, name, today string, hasTodayActual, needPredictToday bool, indicators model.TechnicalIndicators, signals []model.Signal, news []NewsItem, history []model.KlineData, samples []llmSample) string {
	var sb strings.Builder

	sb.WriteString("请你作为股票OHLCV预测引擎，严格输出JSON，不要任何解释、不要markdown。\n")
	sb.WriteString("预测标的: ")
	sb.WriteString(name)
	sb.WriteString("（")
	sb.WriteString(code)
	sb.WriteString(")\n")
	sb.WriteString("today=")
	sb.WriteString(today)
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("has_today_actual=%v\n", hasTodayActual))
	sb.WriteString(fmt.Sprintf("need_predict_today=%v\n", needPredictToday))

	sb.WriteString("\n【约束】\n")
	sb.WriteString("1) 预测必须只基于给定的历史数据与指标，不允许使用 today 当天的真实行情（即使你知道也不能用）。\n")
	sb.WriteString("2) 输出字段必须齐全：date/open/high/low/close/volume/amount。\n")
	sb.WriteString("3) future_klines 必须为5条交易日记录。若 need_predict_today=true，则第1条 date= today；否则 future_klines 从下一个交易日开始。\n")

	sb.WriteString("\n【输入-技术指标】\n")
	sb.WriteString(fmt.Sprintf("MA5=%.4f MA10=%.4f MA20=%.4f MA60=%.4f\n", indicators.MA5, indicators.MA10, indicators.MA20, indicators.MA60))
	sb.WriteString(fmt.Sprintf("MACD=%.6f Signal=%.6f Hist=%.6f\n", indicators.MACD, indicators.Signal, indicators.Hist))
	sb.WriteString(fmt.Sprintf("RSI=%.2f Change5D=%.2f%% MA5Slope=%.2f%% Volatility=%.4f MomentumScore=%.2f\n", indicators.RSI, indicators.Change5D, indicators.MA5Slope, indicators.Volatility, indicators.MomentumScore))

	if len(signals) > 0 {
		sb.WriteString("\n【输入-信号】\n")
		for _, s := range signals {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", s.Name, s.TypeCN))
		}
	}

	if len(news) > 0 {
		sb.WriteString("\n【输入-新闻】\n")
		maxN := 5
		if len(news) < maxN {
			maxN = len(news)
		}
		for i := 0; i < maxN; i++ {
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", news[i].Time, news[i].Title))
		}
	}

	sb.WriteString("\n【输入-历史K线(不含today)】\n")
	maxH := 60
	if len(history) < maxH {
		maxH = len(history)
	}
	start := len(history) - maxH
	if start < 0 {
		start = 0
	}
	for i := start; i < len(history); i++ {
		d := history[i]
		sb.WriteString(fmt.Sprintf("%s,%.4f,%.4f,%.4f,%.4f,%.0f,%.0f\n", d.Date, d.Open, d.High, d.Low, d.Close, d.Volume, d.Amount))
	}

	if len(samples) > 0 {
		sb.WriteString("\n【相似历史样本(用于蒸馏对齐)】\n")
		for _, s := range samples {
			sb.WriteString(fmt.Sprintf("sample_id=%s rsi=%.2f vol=%.4f chg5d=%.2f ma5slope=%.2f mom=%.2f -> future1d=%.2f%% future5d=%.2f%%\n", s.ID, s.Features.RSI, s.Features.Volatility, s.Features.Change5D, s.Features.MA5Slope, s.Features.MomentumScore, s.Outcome.Future1D, s.Outcome.Future5D))
		}
	}

	sb.WriteString("\n【输出JSON格式】\n")
	sb.WriteString("{\"ai_today\":{...}|null,\"future_klines\":[{...}],\"confidence\":0-100,\"reasons\":[...]}\n")

	return sb.String()
}

func PredictOHLCV(code, name, today string, hasTodayActual, needPredictToday bool, indicators model.TechnicalIndicators, signals []model.Signal, news []NewsItem, history []model.KlineData) (*model.KlineData, []model.KlineData, error) {
	ensureLLMConfig()
	topSamples := loadTopLLMSamples(llmSamplesPath, indicators, llmSamplesTopK)
	if llmDebugSamples {
		fmt.Printf("[LLM][samples] code=%s db=%s topk=%d hit=%d ids=%s\n", code, llmSamplesPath, llmSamplesTopK, len(topSamples), sampleIDs(topSamples))
	}

	prompt := buildOHLCVPrompt(code, name, today, hasTodayActual, needPredictToday, indicators, signals, news, history, topSamples)

	result, err := callChatCompletions([]Message{
		{
			Role:    "system",
			Content: "你是一个只输出严格JSON的预测服务。输出必须是可被json解析的对象，禁止输出markdown/解释/额外文本。",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}, 60*time.Second)
	if err != nil {
		return nil, nil, err
	}
	jsonText := extractJSONObject(result)
	if jsonText == "" {
		return nil, nil, fmt.Errorf("LLM未返回可解析JSON")
	}

	var out ohlcvLLMResponse
	if err := json.Unmarshal([]byte(jsonText), &out); err != nil {
		return nil, nil, fmt.Errorf("解析LLM JSON失败: %v", err)
	}
	if len(out.FutureKlines) == 0 {
		return out.AIToday, nil, fmt.Errorf("LLM返回future_klines为空")
	}
	return out.AIToday, out.FutureKlines, nil
}

// Message 消息
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIChatCompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type OpenAIChatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// NewsItem 新闻条目
type NewsItem struct {
	Title  string `json:"title"`
	Time   string `json:"time"`
	Source string `json:"source"`
}

// NewsImpact 新闻影响评估
type NewsImpact struct {
	SentimentScore  float64 `json:"sentiment_score"`  // 情感评分 -1到+1
	ImportanceLevel int     `json:"importance_level"` // 重要性等级 1-5
	PriceImpact     float64 `json:"price_impact"`     // 预期价格影响 -0.2到+0.2
}

// AnalyzeStock 使用通义千问分析股票
func AnalyzeStock(code, name string, indicators model.TechnicalIndicators, ml model.MLPredictions, signals []model.Signal, news []NewsItem) (string, error) {
	ensureLLMConfig()
	if strings.TrimSpace(llmAuthToken) == "" {
		fmt.Println("[LLM] LLM_AUTH_TOKEN 未配置，使用备用分析")
		return generateFallbackAnalysis(code, name, indicators, ml, signals), nil
	}
	fmt.Printf("[LLM] 使用模型: %s, base_url=%s, 新闻数量: %d\n", llmModel, llmBaseURL, len(news))

	prompt := buildAnalysisPrompt(code, name, indicators, ml, signals, news)

	result, err := callChatCompletions([]Message{
		{
			Role:    "system",
			Content: "你是一位资深的量化分析师和技术分析专家，具备深厚的A股市场经验。你擅长：1)多维度技术指标解读 2)成交量与价格关系分析 3)市场环境感知 4)风险控制。请基于提供的全面技术数据，从技术面、资金面、市场环境三个维度给出专业分析，重点关注关键信号的确认与背离，提供具体的操作建议和风险提示。",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}, 30*time.Second)
	if err != nil {
		fmt.Printf("[LLM] 请求失败: %v\n", err)
		return generateFallbackAnalysis(code, name, indicators, ml, signals), nil
	}

	return result, nil
}

// buildAnalysisPrompt 构建分析提示词
func buildAnalysisPrompt(code, name string, indicators model.TechnicalIndicators, ml model.MLPredictions, signals []model.Signal, news []NewsItem) string {
	// 分类整理信号
	var technicalSignals, volumeSignals, marketSignals []string

	for _, s := range signals {
		switch s.Name {
		case "MACD", "RSI", "KDJ", "均线":
			technicalSignals = append(technicalSignals, fmt.Sprintf("%s: %s", s.Name, s.TypeCN))
		case "成交量", "量价":
			volumeSignals = append(volumeSignals, fmt.Sprintf("%s: %s", s.Name, s.TypeCN))
		case "市场", "波动":
			marketSignals = append(marketSignals, fmt.Sprintf("%s: %s", s.Name, s.TypeCN))
		}
	}

	newsStr := ""
	if len(news) > 0 {
		newsStr = "\n\n最新公告/新闻（请重点分析这些消息对股价的潜在影响）：\n"
		for i, n := range news {
			newsStr += fmt.Sprintf("%d. [%s] %s\n", i+1, n.Time, n.Title)
		}
	}

	return fmt.Sprintf(`请深度分析股票 %s（%s）的投资价值和风险：

【技术面分析】
基础指标：
• 均线系统：MA5=%.2f, MA10=%.2f, MA20=%.2f, MA60=%.2f
• MACD系统：DIF=%.4f, DEA=%.4f, 柱状图=%.4f
• RSI指标：%.2f (动态阈值: 超买>%.1f, 超卖<%.1f)
• KDJ指标：K=%.2f, D=%.2f, J=%.2f
• 布林带：上轨=%.2f, 中轨=%.2f, 下轨=%.2f

成交量分析：
• 当前成交量：%.0f万手, 5日均量：%.0f万手
• 量比：%.2f (当日/5日均量), 成交量强度：%.2f
• 近期涨跌幅：1日=%.2f%%, 5日=%.2f%%, 10日=%.2f%%
• MA5斜率：%.2f%% (反映短期动量)

市场环境：
• 市场趋势：%s, 趋势强度：%.2f
• 价格波动率：%.2f%% (%s)

【信号汇总】
• 技术信号：%s
• 成交量信号：%s
• 市场环境：%s

【AI模型预测】
• LSTM模型(均线+动量)：%s, 置信度%.1f%%, 目标价%.2f
• Prophet模型(MACD)：%s, 置信度%.1f%%, 目标价%.2f
• XGBoost模型(RSI+动量)：%s, 置信度%.1f%%, 目标价%.2f%s

【分析要求】
请从以下维度给出专业分析，使用清晰的段落结构：

**技术面强弱**：结合均线排列、MACD金叉死叉、RSI超买超卖状态

**成交量配合**：分析量价关系，是否存在背离信号

**市场环境影响**：当前市场趋势对个股的影响

**风险提示**：关键支撑阻力位，止损建议

**操作策略**：具体的买卖时机和仓位建议

请使用markdown格式，控制在350字以内，重点突出关键信息。`,
		name, code,
		// 基础技术指标
		indicators.MA5, indicators.MA10, indicators.MA20, indicators.MA60,
		indicators.MACD, indicators.Signal, indicators.Hist,
		indicators.RSI, indicators.RSIUpperThreshold, indicators.RSILowerThreshold,
		indicators.KDJ_K, indicators.KDJ_D, indicators.KDJ_J,
		indicators.BOLL_U, indicators.BOLL_M, indicators.BOLL_L,
		// 成交量分析
		indicators.CurrentVolume/10000, indicators.VolumeMA5/10000, // 转换为万手
		indicators.VolumeRatio, indicators.VolumeStrength,
		indicators.Change1D, indicators.Change5D, indicators.Change10D,
		indicators.MA5Slope,
		// 市场环境
		getMarketTrendCN(indicators.MarketTrend), indicators.TrendStrength,
		indicators.Volatility*100, getVolatilityDesc(indicators.Volatility),
		// 信号汇总
		joinStrings(technicalSignals), joinStrings(volumeSignals), joinStrings(marketSignals),
		// ML预测
		getTrendCN(ml.LSTM.Trend), ml.LSTM.Confidence*100, ml.LSTM.Price,
		getTrendCN(ml.Prophet.Trend), ml.Prophet.Confidence*100, ml.Prophet.Price,
		getTrendCN(ml.XGBoost.Trend), ml.XGBoost.Confidence*100, ml.XGBoost.Price,
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

// getMarketTrendCN 获取市场趋势中文描述
func getMarketTrendCN(trend string) string {
	switch trend {
	case "bull":
		return "牛市"
	case "bear":
		return "熊市"
	default:
		return "震荡市"
	}
}

// getTrendCN 获取趋势中文描述
func getTrendCN(trend string) string {
	switch trend {
	case "up":
		return "看涨"
	case "down":
		return "看跌"
	default:
		return "震荡"
	}
}

// getVolatilityDesc 获取波动率描述
func getVolatilityDesc(volatility float64) string {
	if volatility > 0.05 {
		return "高波动"
	} else if volatility < 0.02 {
		return "低波动"
	}
	return "正常波动"
}

// joinStrings 连接字符串数组
func joinStrings(strs []string) string {
	if len(strs) == 0 {
		return "无"
	}
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}

// StockClassification 股票分类信息
type StockClassification struct {
	Sector   string `json:"sector"`   // 板块
	Industry string `json:"industry"` // 主营业务行业
}

// GetStockClassification 使用LLM获取股票板块和行业
func GetStockClassification(code, name string) StockClassification {
	ensureLLMConfig()
	if strings.TrimSpace(llmAuthToken) == "" {
		return StockClassification{}
	}

	result, err := callChatCompletions([]Message{
		{
			Role: "system",
			Content: `你是一个股票数据助手。请返回股票所属的行业板块，格式为JSON：{"sector":"行业板块"}

行业板块必须从以下列表中选择一个最匹配的：
电网设备、半导体、证券、电力、零售、贵金属、房地产、通信设备、军工电子、白酒、银行、军工装备、电池、影视院线、铜、金属新材料、通用设备、光伏设备、化学原料、软件开发、服装家纺、医药、汽车、食品饮料、家电、旅游酒店、传媒、农业、煤炭、钢铁、航空机场、物流、保险、互联网、游戏、教育、环保、建筑、水泥建材、造纸印刷

注意：
- 黄金、白银等贵金属相关股票应归类为"贵金属"
- 服装、纺织、鞋帽、户外用品等归类为"服装家纺"
- 只能从上述列表中选择，不要使用其他分类名称
- 只返回JSON，不要有其他内容。`,
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("股票%s（代码%s）在同花顺中属于哪个行业板块？", name, code),
		},
	}, 10*time.Second)
	if err != nil {
		return StockClassification{}
	}

	// 解析JSON结果
	var classification StockClassification
	text := result
	if jsonText := extractJSONObject(result); jsonText != "" {
		text = jsonText
	}
	if err := json.Unmarshal([]byte(text), &classification); err != nil {
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

// AnalyzeNewsImpact 分析新闻对股价的影响
func AnalyzeNewsImpact(code, name string, news []NewsItem) NewsImpact {
	ensureLLMConfig()
	if strings.TrimSpace(llmAuthToken) == "" || len(news) == 0 {
		return NewsImpact{SentimentScore: 0, ImportanceLevel: 1, PriceImpact: 0}
	}

	// 构建新闻分析提示词
	newsStr := ""
	for i, n := range news {
		newsStr += fmt.Sprintf("%d. [%s] %s\n", i+1, n.Time, n.Title)
	}

	prompt := fmt.Sprintf(`请分析以下新闻对股票 %s（%s）的影响：

%s

请从以下维度评估：
1. 情感倾向：利好(+1)、中性(0)、利空(-1)
2. 重要性等级：1(一般) 2(较重要) 3(重要) 4(很重要) 5(极重要)
3. 预期价格影响：-20%%到+20%%的范围

请返回JSON格式：
{
  "sentiment_score": 0.5,
  "importance_level": 3,
  "price_impact": 0.08
}

注意：
- AI芯片、半导体相关利好消息权重加大
- 业绩预告、重大合同、技术突破等为重要消息
- 只返回JSON，不要其他内容`, name, code, newsStr)

	result, err := callChatCompletions([]Message{
		{
			Role:    "system",
			Content: "你是专业的股票新闻分析师，擅长评估消息面对股价的影响。请客观分析新闻内容，给出量化评估。",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}, 15*time.Second)
	if err != nil {
		return NewsImpact{SentimentScore: 0, ImportanceLevel: 1, PriceImpact: 0}
	}

	// 解析JSON结果
	var impact NewsImpact
	text := result
	if jsonText := extractJSONObject(result); jsonText != "" {
		text = jsonText
	}
	if err := json.Unmarshal([]byte(text), &impact); err != nil {
		// 如果解析失败，尝试提取JSON部分
		if start := findJSONStart(result); start >= 0 {
			if end := findJSONEnd(result, start); end > start {
				json.Unmarshal([]byte(result[start:end+1]), &impact)
			}
		}
	}

	// 确保数值在合理范围内
	if impact.SentimentScore > 1 {
		impact.SentimentScore = 1
	} else if impact.SentimentScore < -1 {
		impact.SentimentScore = -1
	}

	if impact.ImportanceLevel > 5 {
		impact.ImportanceLevel = 5
	} else if impact.ImportanceLevel < 1 {
		impact.ImportanceLevel = 1
	}

	if impact.PriceImpact > 0.2 {
		impact.PriceImpact = 0.2
	} else if impact.PriceImpact < -0.2 {
		impact.PriceImpact = -0.2
	}

	return impact
}
