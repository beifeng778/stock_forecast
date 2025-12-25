package service

import (
	"fmt"
	"hash/fnv"
	"log"
	"math"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"stock-forecast-backend/internal/holiday"
	"stock-forecast-backend/internal/langchain"
	"stock-forecast-backend/internal/model"
	"stock-forecast-backend/internal/stockdata"
)

func seedFromString(s string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return int64(h.Sum64())
}

func normalizeNewsTitleKey(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\t", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}

func mergeNewsForLLM(ann []langchain.NewsItem, media []langchain.NewsItem, totalLimit int) []langchain.NewsItem {
	if totalLimit <= 0 {
		totalLimit = 15
	}

	seen := make(map[string]struct{}, len(ann)+len(media))
	out := make([]langchain.NewsItem, 0, totalLimit)
	push := func(n langchain.NewsItem) {
		if len(out) >= totalLimit {
			return
		}
		key := normalizeNewsTitleKey(n.Title)
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, n)
	}

	for _, n := range ann {
		push(n)
	}
	for _, n := range media {
		push(n)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Time != out[j].Time {
			return out[i].Time > out[j].Time
		}
		return out[i].Title > out[j].Title
	})

	if len(out) > totalLimit {
		out = out[:totalLimit]
	}
	return out
}

func getDailyPriceLimitPercent(stockCode, stockName string) float64 {
	code := strings.TrimSpace(stockCode)
	name := strings.ToUpper(strings.TrimSpace(stockName))

	limit := 0.10
	if strings.HasPrefix(code, "300") || strings.HasPrefix(code, "301") || strings.HasPrefix(code, "688") {
		limit = 0.20
	} else if strings.HasPrefix(code, "8") || strings.HasPrefix(code, "4") {
		limit = 0.30
	}

	if limit == 0.10 {
		if strings.Contains(name, "ST") {
			limit = 0.05
		}
	}

	return limit
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampKlineToLimit(prevClose, limit float64, open, high, low, close *float64) {
	if prevClose <= 0 || limit <= 0 {
		return
	}

	lo := prevClose * (1 - limit)
	hi := prevClose * (1 + limit)

	*open = clamp(*open, lo, hi)
	*close = clamp(*close, lo, hi)

	minOC := math.Min(*open, *close)
	maxOC := math.Max(*open, *close)

	*high = clamp(*high, maxOC, hi)
	*low = clamp(*low, lo, minOC)

	if *high < *low {
		*high = maxOC
		*low = minOC
		*high = clamp(*high, *low, hi)
		*low = clamp(*low, lo, *high)
	}
}

func sanitizeLLMKlines(stockCode, stockName string, prevClose float64, in []model.KlineData) []model.KlineData {
	if len(in) == 0 {
		return in
	}

	// 第一步：过滤非交易日
	tradingDayKlines := make([]model.KlineData, 0, len(in))
	for _, k := range in {
		// 解析日期
		date, err := time.Parse("2006-01-02", k.Date)
		if err != nil {
			continue
		}
		// 只保留交易日
		if holiday.IsTradingDay(date) {
			tradingDayKlines = append(tradingDayKlines, k)
		}
	}

	// 第二步：对交易日K线进行涨跌停限制和数据清洗
	cap := getDailyPriceLimitPercent(stockCode, stockName)
	out := make([]model.KlineData, 0, len(tradingDayKlines))
	pc := prevClose
	for _, k := range tradingDayKlines {
		open := k.Open
		high := k.High
		low := k.Low
		close := k.Close
		if pc <= 0 {
			if close > 0 {
				pc = close
			} else {
				continue
			}
		}
		if open <= 0 {
			open = pc
		}
		if close <= 0 {
			close = pc
		}
		if high <= 0 {
			high = math.Max(open, close)
		}
		if low <= 0 {
			low = math.Min(open, close)
		}
		clampKlineToLimit(pc, cap, &open, &high, &low, &close)
		high = math.Max(high, math.Max(open, close))
		low = math.Min(low, math.Min(open, close))
		if high < low {
			high = math.Max(open, close)
			low = math.Min(open, close)
			clampKlineToLimit(pc, cap, &open, &high, &low, &close)
			high = math.Max(high, math.Max(open, close))
			low = math.Min(low, math.Min(open, close))
		}
		k.Open = round4(open)
		k.Close = round4(close)
		k.High = round4(high)
		k.Low = round4(low)
		out = append(out, k)
		pc = k.Close
	}
	return out
}

func sanitizeLLMToday(stockCode, stockName string, prevClose float64, in *model.KlineData) *model.KlineData {
	if in == nil {
		return nil
	}
	ks := sanitizeLLMKlines(stockCode, stockName, prevClose, []model.KlineData{*in})
	if len(ks) == 0 {
		return nil
	}
	out := ks[0]
	return &out
}

func isTradingTimeNow() bool {
	return holiday.IsTradingTimeNow()
}

func isTradingDayNow() bool {
	return holiday.IsTradingDayNow()
}

func isDailyKlineOHLCVComplete(d stockdata.KlineData) bool {
	if d.Open <= 0 || d.Close <= 0 || d.High <= 0 || d.Low <= 0 {
		return false
	}
	if d.Volume < 0 {
		return false
	}
	if d.High < d.Low {
		return false
	}
	return true
}

// parseTimeToHHMM 解析时间字符串（HH:MM）为整数（HHMM）
func parseTimeToHHMM(timeStr string) int {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return 1700 // 默认17:00
	}
	hour, err1 := strconv.Atoi(parts[0])
	minute, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 1700 // 默认17:00
	}
	return hour*100 + minute
}

// PredictStocks 批量预测股票（保持原始顺序）
func PredictStocks(codes []string, period string) ([]model.PredictResult, error) {
	results := make([]*model.PredictResult, len(codes))
	errors := make([]error, len(codes))
	var wg sync.WaitGroup
	isBatch := len(codes) > 1
	var sem chan struct{}
	if isBatch {
		sem = make(chan struct{}, 4)
	}

	for i, code := range codes {
		wg.Add(1)
		go func(idx int, stockCode string) {
			defer wg.Done()
			if sem != nil {
				sem <- struct{}{}
				defer func() { <-sem }()
			}

			result, err := predictSingleStock(stockCode, period, isBatch)
			if err != nil {
				errors[idx] = fmt.Errorf("预测 %s 失败: %v", stockCode, err)
				return
			}
			results[idx] = result
		}(i, code)
	}

	wg.Wait()

	// 按顺序收集成功的结果
	var finalResults []model.PredictResult
	var firstErr error
	for i := range codes {
		if results[i] != nil {
			finalResults = append(finalResults, *results[i])
		} else if errors[i] != nil && firstErr == nil {
			firstErr = errors[i]
		}
	}

	if len(finalResults) == 0 && firstErr != nil {
		return nil, firstErr
	}

	return finalResults, nil
}

// predictSingleStock 预测单只股票
func predictSingleStock(code, period string, isBatch bool) (*model.PredictResult, error) {
	// 1. 获取股票信息（名称和行业）
	stockInfo, err := stockdata.GetStockInfo(code)
	stockName := "未知"
	sector := ""
	industry := ""
	if err == nil && stockInfo != nil {
		stockName = stockInfo.Name
		industry = stockInfo.Industry
	}
	// 如果缓存中没有行业信息，尝试从东方财富获取
	if industry == "" {
		industry = stockdata.GetStockIndustry(code)
	}
	// 如果东方财富也获取失败，使用LLM获取板块和行业
	if !isBatch && (sector == "" || industry == "") && stockName != "未知" {
		classification := langchain.GetStockClassification(code, stockName)
		if sector == "" {
			sector = classification.Sector
		}
		if industry == "" {
			industry = classification.Industry
		}
	}

	// 2. 获取技术指标
	isTradingTime := isTradingTimeNow()
	kline, err := stockdata.GetKlineWithRefresh(code, "daily", isTradingTime)
	if err != nil {
		return nil, fmt.Errorf("获取技术指标失败: %v", err)
	}
	if kline == nil || len(kline.Data) == 0 {
		return nil, fmt.Errorf("获取技术指标失败: 返回数据为空")
	}

	now := time.Now()
	todayStr := now.Format("2006-01-02")
	hhmm := now.Hour()*100 + now.Minute()
	last := kline.Data[len(kline.Data)-1]
	hasTodayData := last.Date == todayStr

	// 判断今天的K线是否完整（已收盘且数据已同步）
	// 1. 有今天的数据 && OHLCV数据完整 && 时间 >= FRONTEND_REFRESH_AVAILABLE_TIME
	// FRONTEND_REFRESH_AVAILABLE_TIME 表示数据已同步完成的时间
	refreshAvailableTime := os.Getenv("FRONTEND_REFRESH_AVAILABLE_TIME")
	if refreshAvailableTime == "" {
		refreshAvailableTime = "17:00"
	}
	refreshHHMM := parseTimeToHHMM(refreshAvailableTime)
	todayKlineComplete := hasTodayData && isDailyKlineOHLCVComplete(last) && hhmm >= refreshHHMM

	// 3. 转换技术指标（盘中/当日未收盘：仅用历史完整日K）
	klineForAnalysis := kline.Data
	if hasTodayData && !todayKlineComplete && len(klineForAnalysis) >= 2 {
		klineForAnalysis = klineForAnalysis[:len(klineForAnalysis)-1]
	}

	indicators, err := stockdata.CalculateIndicators(klineForAnalysis)
	if err != nil {
		return nil, fmt.Errorf("获取技术指标失败: %v", err)
	}
	if indicators == nil {
		return nil, fmt.Errorf("获取技术指标失败: 返回数据为空")
	}

	isIntradayCard := isTradingTimeNow() && !todayKlineComplete
	needPredictToday := isTradingDayNow() && !todayKlineComplete

	var indicatorsForTodayPred *stockdata.Indicators
	if hasTodayData && len(kline.Data) >= 2 {
		indicatorsForTodayPred, _ = stockdata.CalculateIndicators(kline.Data[:len(kline.Data)-1])
	}

	techIndicators := convertIndicators(indicators)

	// 4. 生成技术信号（从 stockdata 获取）
	signals := convertSignals(indicators.Signals)
	signalsForTodayPred := signals

	// 5-10. 批量预测加速：跳过所有 LLM 调用，仅使用本地逻辑
	var news []langchain.NewsItem
	var newsImpact langchain.NewsImpact
	mlPredictions := generateMLPredictions(indicators)
	analysis := ""
	newsAnalysis := ""
	if isBatch {
		analysis = "批量模式为加速已跳过详细AI分析，点进单股查看"
	} else {
		annItems, _ := stockdata.GetStockNews(code, 5)
		mediaItems, _ := stockdata.GetStockMediaNews(code, 10)

		annNews := make([]langchain.NewsItem, 0, len(annItems))
		for _, n := range annItems {
			annNews = append(annNews, langchain.NewsItem{Title: n.Title, Time: n.Time, Source: "公告"})
		}
		mediaNews := make([]langchain.NewsItem, 0, len(mediaItems))
		for _, n := range mediaItems {
			mediaNews = append(mediaNews, langchain.NewsItem{Title: n.Title, Time: n.Time, Source: "新闻"})
		}
		news = mergeNewsForLLM(annNews, mediaNews, 15)

		newsImpact = langchain.AnalyzeNewsImpact(code, stockName, news)
		analysis, err = langchain.AnalyzeStock(code, stockName, techIndicators, mlPredictions, signals, news)
		if err != nil {
			analysis = "AI分析暂时不可用"
		}
		newsAnalysis = generateNewsAnalysis(newsImpact, news)
	}
	if needPredictToday && isTradingTimeNow() {
		analysis = "盘中（当日K线未就绪，分析仅使用历史完整日K）：\n\n" + analysis
	} else if hasTodayData && !todayKlineComplete {
		analysis = "当日K线数据不完整（分析仅使用历史完整日K）：\n\n" + analysis
	}

	trend, trendCN, confidence := determineTrendWithNews(mlPredictions, signals, newsImpact)
	targetPrices := calculateTargetPricesWithNews(indicators.CurrentPrice, trend, confidence, newsImpact)

	// 10. 获取近期每日涨跌幅（同样只用完整日K）
	dailyChanges := getDailyChangesFromKline(klineForAnalysis, 10)

	confidenceForTodayPred := confidence
	targetPricesForTodayPred := targetPrices
	volatilityForTodayPred := indicators.Volatility
	supportForTodayPred := indicators.SupportLevel
	resistanceForTodayPred := indicators.ResistanceLevel
	if indicatorsForTodayPred != nil {
		signalsForTodayPred = convertSignals(indicatorsForTodayPred.Signals)
		mlPredForTodayPred := generateMLPredictions(indicatorsForTodayPred)
		trendForTodayPred, _, confForTodayPred := determineTrendWithNews(mlPredForTodayPred, signalsForTodayPred, newsImpact)
		confidenceForTodayPred = confForTodayPred
		targetPricesForTodayPred = calculateTargetPricesWithNews(indicatorsForTodayPred.CurrentPrice, trendForTodayPred, confidenceForTodayPred, newsImpact)
		volatilityForTodayPred = indicatorsForTodayPred.Volatility
		supportForTodayPred = indicatorsForTodayPred.SupportLevel
		resistanceForTodayPred = indicatorsForTodayPred.ResistanceLevel
	}

	var aiToday *model.KlineData
	if needPredictToday {
		aiToday = generateTodayKline(code, stockName, kline.Data, targetPricesForTodayPred.Short, supportForTodayPred, resistanceForTodayPred, confidenceForTodayPred, volatilityForTodayPred)
	} else if hasTodayData {
		aiToday = generateTodayKline(code, stockName, kline.Data, targetPricesForTodayPred.Short, supportForTodayPred, resistanceForTodayPred, confidenceForTodayPred, volatilityForTodayPred)
	}

	// 统一设计：三种场景（盘前/盘中/盘后）都预测今天
	// - aiToday: 今天的预测（用于卡片显示和对比）
	// - future_klines: 包含今天的5天预测（今天,明天,后天,第4天,第5天）
	// 总共5个预测节点
	futureKlines := generateFutureKlines(code, stockName, kline.Data, true, targetPrices.Short, indicators.SupportLevel, indicators.ResistanceLevel, confidence, indicators.Volatility)

	// 11. 使用LLM生成结构化OHLCV预测（ai_today + future_klines），失败则回退本地预测
	if isBatch {
		llmIndicators := techIndicators
		llmSignals := signals
		if indicatorsForTodayPred != nil && (!todayKlineComplete || needPredictToday) {
			llmIndicators = convertIndicators(indicatorsForTodayPred)
			llmSignals = signalsForTodayPred
		}

		llmHistorySrc := kline.Data
		if len(llmHistorySrc) > 0 && llmHistorySrc[len(llmHistorySrc)-1].Date == todayStr && !todayKlineComplete {
			llmHistorySrc = llmHistorySrc[:len(llmHistorySrc)-1]
		}
		llmHistory := make([]model.KlineData, 0, len(llmHistorySrc))
		for _, d := range llmHistorySrc {
			llmHistory = append(llmHistory, model.KlineData{
				Date:   d.Date,
				Open:   d.Open,
				Close:  d.Close,
				High:   d.High,
				Low:    d.Low,
				Volume: d.Volume,
				Amount: d.Amount,
			})
		}

		hasTodayActual := todayKlineComplete
		llmAiToday, llmFutureKlines, llmErr := langchain.PredictOHLCVWithOptions(
			code,
			stockName,
			todayStr,
			hasTodayActual,
			needPredictToday,
			llmIndicators,
			llmSignals,
			nil,
			llmHistory,
			langchain.OHLCVOptions{AllowRetry: false, Timeout: 12 * time.Second, RetryTimeout: 0},
		)
		if llmErr == nil && len(llmFutureKlines) > 0 {
			prevCloseForLLM := 0.0
			if len(llmHistory) > 0 {
				prevCloseForLLM = llmHistory[len(llmHistory)-1].Close
			}
			llmAiToday = sanitizeLLMToday(code, stockName, prevCloseForLLM, llmAiToday)
			llmFutureKlines = sanitizeLLMKlines(code, stockName, prevCloseForLLM, llmFutureKlines)

			// 强制修正LLM返回的日期，使用本地生成的正确日期
			// 因为LLM可能不遵守Prompt中的日期列表
			if len(llmFutureKlines) == len(futureKlines) {
				for i := range llmFutureKlines {
					llmFutureKlines[i].Date = futureKlines[i].Date
				}
			}

			if llmAiToday != nil {
				aiToday = llmAiToday
			}
			futureKlines = llmFutureKlines
			if llmFutureKlines[len(llmFutureKlines)-1].Close > 0 {
				llmTargetPrice := llmFutureKlines[len(llmFutureKlines)-1].Close
				targetPrices.Short = llmTargetPrice

				// 根据LLM目标价调整趋势判断，确保一致性
				priceChange := (llmTargetPrice - indicators.CurrentPrice) / indicators.CurrentPrice
				if priceChange > 0.01 { // 涨幅超过1%
					trend = "up"
					trendCN = "看涨"
				} else if priceChange < -0.01 { // 跌幅超过1%
					trend = "down"
					trendCN = "看跌"
				} else {
					trend = "sideways"
					trendCN = "震荡"
				}
			}
		} else if llmErr != nil {
			log.Printf("[ERROR][LLM] PredictOHLCV失败(%s): %v", code, llmErr)
		}
	} else {
		llmIndicators := techIndicators
		llmSignals := signals
		if indicatorsForTodayPred != nil && (!todayKlineComplete || needPredictToday) {
			llmIndicators = convertIndicators(indicatorsForTodayPred)
			llmSignals = signalsForTodayPred
		}

		llmHistorySrc := kline.Data
		if len(llmHistorySrc) > 0 && llmHistorySrc[len(llmHistorySrc)-1].Date == todayStr && !todayKlineComplete {
			llmHistorySrc = llmHistorySrc[:len(llmHistorySrc)-1]
		}
		llmHistory := make([]model.KlineData, 0, len(llmHistorySrc))
		for _, d := range llmHistorySrc {
			llmHistory = append(llmHistory, model.KlineData{
				Date:   d.Date,
				Open:   d.Open,
				Close:  d.Close,
				High:   d.High,
				Low:    d.Low,
				Volume: d.Volume,
				Amount: d.Amount,
			})
		}

		hasTodayActual := todayKlineComplete
		llmAiToday, llmFutureKlines, llmErr := langchain.PredictOHLCV(code, stockName, todayStr, hasTodayActual, needPredictToday, llmIndicators, llmSignals, news, llmHistory)
		if llmErr == nil && len(llmFutureKlines) > 0 {
			prevCloseForLLM := 0.0
			if len(llmHistory) > 0 {
				prevCloseForLLM = llmHistory[len(llmHistory)-1].Close
			}
			llmAiToday = sanitizeLLMToday(code, stockName, prevCloseForLLM, llmAiToday)
			llmFutureKlines = sanitizeLLMKlines(code, stockName, prevCloseForLLM, llmFutureKlines)

			// 强制修正LLM返回的日期，使用本地生成的正确日期
			// 因为LLM可能不遵守Prompt中的日期列表
			if len(llmFutureKlines) == len(futureKlines) {
				for i := range llmFutureKlines {
					llmFutureKlines[i].Date = futureKlines[i].Date
				}
			}

			if llmAiToday != nil {
				aiToday = llmAiToday
			}
			futureKlines = llmFutureKlines
			if llmFutureKlines[len(llmFutureKlines)-1].Close > 0 {
				llmTargetPrice := llmFutureKlines[len(llmFutureKlines)-1].Close
				targetPrices.Short = llmTargetPrice

				// 根据LLM目标价调整趋势判断，确保一致性
				priceChange := (llmTargetPrice - indicators.CurrentPrice) / indicators.CurrentPrice
				if priceChange > 0.01 { // 涨幅超过1%
					trend = "up"
					trendCN = "看涨"
				} else if priceChange < -0.01 { // 跌幅超过1%
					trend = "down"
					trendCN = "看跌"
				} else {
					trend = "sideways"
					trendCN = "震荡"
				}
			}
		} else if llmErr != nil {
			log.Printf("[ERROR][LLM] PredictOHLCV失败(%s): %v", code, llmErr)
		}
	}

	return &model.PredictResult{
		StockCode:    code,
		StockName:    stockName,
		Sector:       sector,
		Industry:     industry,
		IsIntraday:   isIntradayCard,
		CurrentPrice: indicators.CurrentPrice,
		Trend:        trend,
		TrendCN:      trendCN,
		Confidence:   confidence,
		PriceRange: model.PriceRange{
			Low:  indicators.SupportLevel,
			High: indicators.ResistanceLevel,
		},
		TargetPrices:     targetPrices,
		FutureKlines:     futureKlines,
		AIToday:          aiToday,
		NeedPredictToday: needPredictToday,
		SupportLevel:     indicators.SupportLevel,
		ResistanceLevel:  indicators.ResistanceLevel,
		Indicators:       techIndicators,
		Signals:          signals,
		Analysis:         analysis,
		NewsAnalysis:     newsAnalysis,
		MLPredictions:    mlPredictions,
		DailyChanges:     dailyChanges,
	}, nil
}

func generateTodayKline(stockCode, stockName string, history []stockdata.KlineData, targetPrice float64, supportLevel float64, resistanceLevel float64, confidence float64, volatility float64) *model.KlineData {
	if len(history) < 1 {
		return nil
	}

	now := time.Now()
	todayStr := now.Format("2006-01-02")
	last := history[len(history)-1]
	if last.Date == todayStr && len(history) < 2 {
		return nil
	}

	todayDate, err1 := time.Parse("2006-01-02", todayStr)
	lastDate, err2 := time.Parse("2006-01-02", last.Date)
	if err1 != nil || err2 != nil {
		return nil
	}
	if lastDate.After(todayDate) {
		return nil
	}
	if todayDate.Sub(lastDate) > 7*24*time.Hour {
		return nil
	}

	prevDay := last
	anchorClose := last.Close
	if last.Date == todayStr {
		prevDay = history[len(history)-2]
		anchorClose = prevDay.Close
	}
	if anchorClose <= 0 {
		return nil
	}

	steps := 5
	drift := 0.0
	if targetPrice > 0 {
		drift = (targetPrice/anchorClose - 1) / float64(steps)
	}

	baseVol := volatility
	if baseVol <= 0 {
		baseVol = 0.03
	}
	dailyStd := baseVol * (1.2 - confidence*0.4)
	if dailyStd < 0.01 {
		dailyStd = 0.01
	}

	_ = supportLevel
	_ = resistanceLevel

	simulations := 50
	baseSeed := seedFromString(stockCode + "|" + todayStr + "|today")

	sumOpen := 0.0
	sumClose := 0.0
	sumHigh := 0.0
	sumLow := 0.0
	limit := getDailyPriceLimitPercent(stockCode, stockName)
	for i := 0; i < simulations; i++ {
		rng := rand.New(rand.NewSource(baseSeed + int64(i)*10007))

		open := anchorClose
		ret := drift + rng.NormFloat64()*dailyStd*0.5
		close := anchorClose * (1 + ret)
		if close <= 0 {
			close = anchorClose
		}

		rangeFactor := math.Abs(rng.NormFloat64()) * dailyStd * 0.8
		high := math.Max(open, close) * (1 + rangeFactor)
		low := math.Min(open, close) * (1 - rangeFactor)
		if low <= 0 {
			low = math.Min(open, close)
		}

		clampKlineToLimit(anchorClose, limit, &open, &high, &low, &close)

		sumOpen += open
		sumClose += close
		sumHigh += high
		sumLow += low
	}

	open := sumOpen / float64(simulations)
	close := sumClose / float64(simulations)
	high := sumHigh / float64(simulations)
	low := sumLow / float64(simulations)

	high = math.Max(high, math.Max(open, close))
	low = math.Min(low, math.Min(open, close))
	clampKlineToLimit(anchorClose, limit, &open, &high, &low, &close)

	volume := prevDay.Volume
	amount := prevDay.Amount

	return &model.KlineData{
		Date:   todayStr,
		Open:   round4(open),
		Close:  round4(close),
		High:   round4(high),
		Low:    round4(low),
		Volume: volume,
		Amount: amount,
	}
}

func getDailyChangesFromKline(data []stockdata.KlineData, days int) []model.DailyChange {
	n := len(data)
	if n < 2 {
		return nil
	}

	start := n - days
	if start < 1 {
		start = 1
	}

	var changes []model.DailyChange
	for i := start; i < n; i++ {
		prevClose := data[i-1].Close
		change := 0.0
		if prevClose > 0 {
			change = (data[i].Close - prevClose) / prevClose * 100
		}
		changes = append(changes, model.DailyChange{
			Date:   data[i].Date,
			Change: change,
			Close:  data[i].Close,
		})
	}

	return changes
}

func generateFutureKlines(stockCode, stockName string, history []stockdata.KlineData, isIntraday bool, targetPrice float64, supportLevel float64, resistanceLevel float64, confidence float64, volatility float64) []model.KlineData {
	if len(history) == 0 {
		return nil
	}

	now := time.Now()
	todayStr := now.Format("2006-01-02")
	last := history[len(history)-1]
	hasTodayData := last.Date == todayStr

	anchorClose := last.Close
	if hasTodayData && isIntraday && len(history) >= 2 {
		anchorClose = history[len(history)-2].Close
	}
	if anchorClose <= 0 {
		return nil
	}

	recent := history
	if len(recent) > 20 {
		recent = recent[len(recent)-20:]
	}
	avgVolume := 0.0
	for _, d := range recent {
		avgVolume += d.Volume
	}
	if len(recent) > 0 {
		avgVolume = avgVolume / float64(len(recent))
	}

	steps := 5
	drift := 0.0
	if targetPrice > 0 {
		drift = (targetPrice/anchorClose - 1) / float64(steps)
	}

	baseVol := volatility
	if baseVol <= 0 {
		baseVol = 0.03
	}
	dailyStd := baseVol * (1.2 - confidence*0.4)
	if dailyStd < 0.01 {
		dailyStd = 0.01
	}

	_ = supportLevel
	_ = resistanceLevel

	currentDate := now
	// 统一设计：future_klines 包含今天的5天预测
	// isIntraday 参数为 true 时，从今天开始
	if !isIntraday {
		currentDate = currentDate.AddDate(0, 0, 1)
	}

	dates := make([]time.Time, 0, steps)
	for len(dates) < steps {
		// 使用节假日判断模块，确保只选择交易日
		if holiday.IsTradingDay(currentDate) {
			dates = append(dates, currentDate)
		}
		currentDate = currentDate.AddDate(0, 0, 1)
	}

	limit := getDailyPriceLimitPercent(stockCode, stockName)
	baseSeed := seedFromString(stockCode + "|" + todayStr + "|futureS")
	rng := rand.New(rand.NewSource(baseSeed))

	target := targetPrice
	if target <= 0 {
		target = anchorClose
	}

	result := make([]model.KlineData, 0, steps)
	prevClose := anchorClose
	for d := 0; d < steps; d++ {
		t := float64(d+1) / float64(steps)
		expectedClose := anchorClose + (target-anchorClose)*t
		if expectedClose <= 0 {
			expectedClose = prevClose
		}

		gap := rng.NormFloat64() * dailyStd * 0.08
		open := prevClose * (1 + gap)
		close := expectedClose

		rangeFactor := math.Abs(rng.NormFloat64()) * dailyStd * 0.6
		high := math.Max(open, close) * (1 + rangeFactor)
		low := math.Min(open, close) * (1 - rangeFactor)
		if low <= 0 {
			low = math.Min(open, close)
		}

		clampKlineToLimit(prevClose, limit, &open, &high, &low, &close)
		high = math.Max(high, math.Max(open, close))
		low = math.Min(low, math.Min(open, close))

		volume := avgVolume
		if volume > 0 {
			volume = volume * (0.85 + 0.3*rng.Float64())
		}
		amount := volume * close

		result = append(result, model.KlineData{
			Date:   dates[d].Format("2006-01-02"),
			Open:   round4(open),
			Close:  round4(close),
			High:   round4(high),
			Low:    round4(low),
			Volume: volume,
			Amount: amount,
		})

		prevClose = close
	}

	_ = drift

	return result
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

// convertIndicators 转换技术指标
func convertIndicators(ind *stockdata.Indicators) model.TechnicalIndicators {
	return model.TechnicalIndicators{
		MA5:    ind.MA5,
		MA10:   ind.MA10,
		MA20:   ind.MA20,
		MA60:   ind.MA60,
		MACD:   ind.MACD,
		Signal: ind.Signal,
		Hist:   ind.Hist,
		RSI:    ind.RSI,
		KDJ_K:  ind.KDJK,
		KDJ_D:  ind.KDJD,
		KDJ_J:  ind.KDJJ,
		BOLL_U: ind.BollUpper,
		BOLL_M: ind.BollMiddle,
		BOLL_L: ind.BollLower,
	}
}

// convertSignals 转换信号
func convertSignals(signals []stockdata.Signal) []model.Signal {
	result := make([]model.Signal, len(signals))
	for i, s := range signals {
		result[i] = model.Signal{
			Name:   s.Name,
			Type:   s.Type,
			TypeCN: s.Desc,
			Desc:   s.Desc,
		}
	}
	return result
}

// generateMLPredictions 基于技术指标生成ML预测（改进版）
func generateMLPredictions(ind *stockdata.Indicators) model.MLPredictions {
	// 1. 基于均线+动量判断趋势（LSTM模型）
	maTrend := "sideways"
	maConfidence := 0.5

	// 均线多头排列 + 价格在均线上方
	maAligned := ind.MA5 > ind.MA10 && ind.MA10 > ind.MA20
	maBearish := ind.MA5 < ind.MA10 && ind.MA10 < ind.MA20
	priceAboveMA := ind.CurrentPrice > ind.MA5
	priceBelowMA := ind.CurrentPrice < ind.MA5

	// 考虑动量：近期涨幅和MA5斜率
	strongMomentum := ind.Change5D > 5 || ind.MA5Slope > 1 // 5日涨幅>5% 或 MA5斜率>1%
	weakMomentum := ind.Change5D < -5 || ind.MA5Slope < -1

	// 超强动量检测（针对摩尔线程这种强势股）
	superStrongMomentum := ind.Change5D > 15 || ind.MomentumScore > 80
	superWeakMomentum := ind.Change5D < -15 || ind.MomentumScore < 20

	if superStrongMomentum || (maAligned && priceAboveMA && strongMomentum) {
		maTrend = "up"
		if superStrongMomentum {
			maConfidence = 0.95 // 超强势股，高置信度
		} else {
			maConfidence = 0.85 // 强势股
		}
	} else if (maAligned && priceAboveMA) || strongMomentum {
		maTrend = "up"
		maConfidence = 0.7
	} else if superWeakMomentum || (maBearish && priceBelowMA && weakMomentum) {
		maTrend = "down"
		if superWeakMomentum {
			maConfidence = 0.95
		} else {
			maConfidence = 0.85
		}
	} else if (maBearish && priceBelowMA) || weakMomentum {
		maTrend = "down"
		maConfidence = 0.7
	}

	// 2. 基于MACD判断趋势（Prophet模型）
	macdTrend := "sideways"
	macdConfidence := 0.5

	// MACD金叉且柱状图放大
	if ind.MACD > ind.Signal && ind.Hist > 0 {
		macdTrend = "up"
		macdConfidence = 0.65
		if ind.Hist > 0.1 { // 柱状图较大
			macdConfidence = 0.75
		}
	} else if ind.MACD < ind.Signal && ind.Hist < 0 {
		macdTrend = "down"
		macdConfidence = 0.65
		if ind.Hist < -0.1 {
			macdConfidence = 0.75
		}
	}

	// 3. 基于RSI+动量判断趋势（XGBoost模型）
	rsiTrend := "sideways"
	rsiConfidence := 0.5

	// 改进RSI逻辑：强势股RSI高但持续上涨应该看涨
	if ind.RSI < 30 {
		rsiTrend = "up" // 超卖反弹
		rsiConfidence = 0.65
	} else if ind.RSI > 70 {
		// RSI超买，但如果动量强劲，仍然看涨
		if ind.Change1D > 3 || ind.Change5D > 8 {
			rsiTrend = "up" // 强势股，继续看涨
			rsiConfidence = 0.6
		} else {
			rsiTrend = "down" // 普通超买回调
			rsiConfidence = 0.55
		}
	} else if ind.RSI > 50 && ind.Change5D > 3 {
		rsiTrend = "up"
		rsiConfidence = 0.6
	} else if ind.RSI < 50 && ind.Change5D < -3 {
		rsiTrend = "down"
		rsiConfidence = 0.6
	}

	// 计算预测价格（动态调整预测幅度）
	// 基础预测系数
	baseFactor := 0.05 // 提高基础预测幅度从2%到5%

	// 根据动量评分调整预测幅度
	momentumMultiplier := 1.0
	if ind.MomentumScore > 80 {
		momentumMultiplier = 3.0 // 超强势股，预测幅度放大3倍
	} else if ind.MomentumScore > 70 {
		momentumMultiplier = 2.5 // 强势股，预测幅度放大2.5倍
	} else if ind.MomentumScore > 60 {
		momentumMultiplier = 2.0 // 偏强势股，预测幅度放大2倍
	} else if ind.MomentumScore > 50 {
		momentumMultiplier = 1.5 // 略强势股，预测幅度放大1.5倍
	}

	// 根据波动率调整预测幅度
	volatilityMultiplier := 1.0
	if ind.Volatility > 0.08 {
		volatilityMultiplier = 2.0 // 高波动股票，预测幅度放大
	} else if ind.Volatility > 0.05 {
		volatilityMultiplier = 1.5
	}

	// 综合调整系数
	adjustedFactor := baseFactor * momentumMultiplier * volatilityMultiplier

	maPrice := ind.CurrentPrice
	if maTrend == "up" {
		maPrice = ind.CurrentPrice * (1 + adjustedFactor*maConfidence)
	} else if maTrend == "down" {
		maPrice = ind.CurrentPrice * (1 - adjustedFactor*maConfidence)
	}

	macdPrice := ind.CurrentPrice
	if macdTrend == "up" {
		macdPrice = ind.CurrentPrice * (1 + adjustedFactor*0.8*macdConfidence)
	} else if macdTrend == "down" {
		macdPrice = ind.CurrentPrice * (1 - adjustedFactor*0.8*macdConfidence)
	}

	rsiPrice := ind.CurrentPrice
	if rsiTrend == "up" {
		rsiPrice = ind.CurrentPrice * (1 + adjustedFactor*1.2*rsiConfidence)
	} else if rsiTrend == "down" {
		rsiPrice = ind.CurrentPrice * (1 - adjustedFactor*1.2*rsiConfidence)
	}

	return model.MLPredictions{
		LSTM: model.MLPrediction{
			Trend:      maTrend,
			Price:      maPrice,
			Confidence: maConfidence,
		},
		Prophet: model.MLPrediction{
			Trend:      macdTrend,
			Price:      macdPrice,
			Confidence: macdConfidence,
		},
		XGBoost: model.MLPrediction{
			Trend:      rsiTrend,
			Price:      rsiPrice,
			Confidence: rsiConfidence,
		},
	}
}

// determineTrendWithNews 综合判断趋势（融入新闻影响和市场环境感知）
func determineTrendWithNews(ml model.MLPredictions, signals []model.Signal, newsImpact langchain.NewsImpact) (string, string, float64) {
	// 先调用原有的趋势判断逻辑
	baseTrend, baseTrendCN, baseConfidence := determineTrend(ml, signals)

	// 如果没有新闻影响，直接返回基础判断
	if newsImpact.ImportanceLevel <= 1 && newsImpact.SentimentScore == 0 {
		return baseTrend, baseTrendCN, baseConfidence
	}

	// 计算新闻影响权重（基于重要性等级）
	newsWeight := float64(newsImpact.ImportanceLevel) * 0.15 // 最高权重0.75

	// 根据新闻情感调整趋势和置信度
	adjustedConfidence := baseConfidence
	finalTrend := baseTrend
	finalTrendCN := baseTrendCN

	// 强烈利好新闻可能改变趋势判断
	if newsImpact.SentimentScore > 0.6 && newsImpact.ImportanceLevel >= 4 {
		if baseTrend == "down" {
			// 强利好可能扭转看跌趋势为震荡
			finalTrend = "sideways"
			finalTrendCN = "震荡"
			adjustedConfidence = baseConfidence * 0.7 // 降低原趋势置信度
		} else if baseTrend == "sideways" {
			// 强利好可能将震荡转为看涨
			finalTrend = "up"
			finalTrendCN = "看涨"
			adjustedConfidence = baseConfidence + newsWeight
		} else {
			// 强化看涨趋势
			adjustedConfidence = baseConfidence + newsWeight
		}
	} else if newsImpact.SentimentScore < -0.6 && newsImpact.ImportanceLevel >= 4 {
		// 强烈利空新闻
		if baseTrend == "up" {
			finalTrend = "sideways"
			finalTrendCN = "震荡"
			adjustedConfidence = baseConfidence * 0.7
		} else if baseTrend == "sideways" {
			finalTrend = "down"
			finalTrendCN = "看跌"
			adjustedConfidence = baseConfidence + newsWeight
		} else {
			adjustedConfidence = baseConfidence + newsWeight
		}
	} else {
		// 一般性新闻影响：调整置信度
		sentimentAdjustment := newsImpact.SentimentScore * newsWeight
		if (baseTrend == "up" && newsImpact.SentimentScore > 0) ||
			(baseTrend == "down" && newsImpact.SentimentScore < 0) {
			// 新闻与技术面同向，增强置信度
			adjustedConfidence += math.Abs(sentimentAdjustment)
		} else if (baseTrend == "up" && newsImpact.SentimentScore < 0) ||
			(baseTrend == "down" && newsImpact.SentimentScore > 0) {
			// 新闻与技术面反向，降低置信度
			adjustedConfidence -= math.Abs(sentimentAdjustment)
		}
	}

	// 确保置信度在合理范围内
	if adjustedConfidence > 1.0 {
		adjustedConfidence = 1.0
	} else if adjustedConfidence < 0.1 {
		adjustedConfidence = 0.1
	}

	return finalTrend, finalTrendCN, adjustedConfidence
}

// determineTrend 综合判断趋势（融入市场环境感知）
func determineTrend(ml model.MLPredictions, signals []model.Signal) (string, string, float64) {
	bullishCount := 0
	bearishCount := 0
	totalConfidence := 0.0
	weightedBullish := 0.0
	weightedBearish := 0.0

	// 提取市场环境信息
	var marketTrend string
	var volatility float64

	for _, s := range signals {
		if s.Name == "市场" {
			if s.Type == "bullish" {
				marketTrend = "bull"
			} else if s.Type == "bearish" {
				marketTrend = "bear"
			} else {
				marketTrend = "sideways"
			}
		}
		if s.Name == "波动" && s.Desc == "高波动" {
			volatility = 0.06 // 高波动
		} else if s.Name == "波动" && s.Desc == "低波动" {
			volatility = 0.015 // 低波动
		} else {
			volatility = 0.03 // 正常波动
		}
	}

	// 统计ML模型投票（带权重）
	models := []model.MLPrediction{ml.LSTM, ml.Prophet, ml.XGBoost}
	for _, m := range models {
		if m.Trend == "up" {
			bullishCount++
			weightedBullish += m.Confidence
		} else if m.Trend == "down" {
			bearishCount++
			weightedBearish += m.Confidence
		}
		totalConfidence += m.Confidence
	}

	// 统计技术信号投票（根据信号类型调整权重）
	for _, s := range signals {
		weight := 0.5 // 默认权重

		// 突破信号权重最高（新增）
		if s.Name == "突破" && (s.Desc == "布林上轨突破" || s.Desc == "布林下轨突破") {
			weight = 1.0 // 突破信号权重最高
		} else if s.Name == "突破" && (s.Desc == "布林上轨触及" || s.Desc == "布林下轨触及") {
			weight = 0.8
		}
		// 动量信号权重很高（新增）
		if s.Name == "动量" && (s.Desc == "强势(80分)" || s.Desc == "强势(90分)" || s.Desc == "强势(100分)") {
			weight = 0.9 // 强势动量权重很高
		} else if s.Name == "动量" && s.Type == "bullish" {
			weight = 0.7
		}
		// 量价背离信号权重更高
		if s.Name == "量价" && (s.Desc == "顶背离" || s.Desc == "底背离") {
			weight = 0.8
		}
		// 市场环境信号权重较高
		if s.Name == "市场" {
			weight = 0.7
		}

		if s.Type == "bullish" {
			bullishCount++
			weightedBullish += weight
		} else if s.Type == "bearish" {
			bearishCount++
			weightedBearish += weight
		}
	}

	// 计算基础置信度
	avgConfidence := totalConfidence / 3

	// 根据市场环境调整置信度
	adjustedConfidence := adjustConfidenceByMarket(avgConfidence, marketTrend, volatility)

	// 判断趋势（考虑市场环境）
	var finalTrend string
	var finalTrendCN string

	if bullishCount > bearishCount || weightedBullish > weightedBearish+0.3 {
		finalTrend = "up"
		finalTrendCN = "看涨"

		// 熊市中的看涨信号需要更强的确认
		if marketTrend == "bear" {
			adjustedConfidence *= 0.7
			if adjustedConfidence < 0.6 {
				finalTrend = "sideways"
				finalTrendCN = "震荡"
			}
		}
	} else if bearishCount > bullishCount || weightedBearish > weightedBullish+0.3 {
		finalTrend = "down"
		finalTrendCN = "看跌"

		// 牛市中的看跌信号需要更强的确认
		if marketTrend == "bull" {
			adjustedConfidence *= 0.7
			if adjustedConfidence < 0.6 {
				finalTrend = "sideways"
				finalTrendCN = "震荡"
			}
		}
	} else {
		finalTrend = "sideways"
		finalTrendCN = "震荡"
		adjustedConfidence *= 0.8
	}

	return finalTrend, finalTrendCN, adjustedConfidence
}

// adjustConfidenceByMarket 根据市场环境调整置信度（优化版）
func adjustConfidenceByMarket(baseConfidence float64, marketTrend string, volatility float64) float64 {
	adjusted := baseConfidence

	// 重新设计波动率对置信度的影响
	if volatility > 0.1 {
		// 极高波动：强势股可能是突破行情，提高置信度
		if baseConfidence > 0.8 {
			adjusted *= 1.1 // 强势股在高波动中提高置信度
		} else {
			adjusted *= 0.9 // 弱势股在高波动中降低置信度
		}
	} else if volatility > 0.05 {
		// 高波动：根据基础置信度调整
		if baseConfidence > 0.7 {
			adjusted *= 1.05 // 较强势股略微提高置信度
		} else {
			adjusted *= 0.95 // 较弱势股略微降低置信度
		}
	}

	// 市场环境影响
	if marketTrend == "bull" && baseConfidence > 0.7 {
		adjusted *= 1.1 // 牛市中的强势信号加强
	} else if marketTrend == "bear" && baseConfidence < 0.4 {
		adjusted *= 1.1 // 熊市中的弱势信号加强
	} else if marketTrend == "sideways" {
		adjusted *= 0.9 // 震荡市中降低置信度
	}

	// 确保置信度在合理范围内
	if adjusted > 1.0 {
		adjusted = 1.0
	} else if adjusted < 0.1 {
		adjusted = 0.1
	}

	return adjusted
}

// getDailyChanges 获取近期每日涨跌幅
func getDailyChanges(code string, days int) []model.DailyChange {
	kline, err := stockdata.GetKline(code, "daily")
	if err != nil || kline == nil || len(kline.Data) == 0 {
		return nil
	}

	return getDailyChangesFromKline(kline.Data, days)
}

// calculateTargetPricesWithNews 计算目标价位（考虑新闻影响）
func calculateTargetPricesWithNews(currentPrice float64, trend string, confidence float64, newsImpact langchain.NewsImpact) model.TargetPrices {
	// 先计算基础目标价位
	basePrices := calculateTargetPrices(currentPrice, trend, confidence)

	// 如果没有重要新闻影响，直接返回基础价位
	if newsImpact.ImportanceLevel <= 2 || math.Abs(newsImpact.PriceImpact) < 0.02 {
		return basePrices
	}

	// 根据新闻预期价格影响调整目标价位
	// 重要性等级越高，影响越大
	impactMultiplier := 1.0 + float64(newsImpact.ImportanceLevel-2)*0.2 // 等级3=1.2x, 等级4=1.4x, 等级5=1.6x

	// 应用新闻影响调整
	if newsImpact.PriceImpact > 0 {
		// 利好新闻：上调目标价位
		adjustmentFactor := newsImpact.PriceImpact * impactMultiplier
		return model.TargetPrices{
			Short:  basePrices.Short * (1 + adjustmentFactor),
			Medium: basePrices.Medium * (1 + adjustmentFactor),
			Long:   basePrices.Long * (1 + adjustmentFactor),
		}
	} else if newsImpact.PriceImpact < 0 {
		// 利空新闻：下调目标价位（修复：使用减法而不是除法）
		adjustmentFactor := math.Abs(newsImpact.PriceImpact) * impactMultiplier
		return model.TargetPrices{
			Short:  basePrices.Short * (1 - adjustmentFactor),
			Medium: basePrices.Medium * (1 - adjustmentFactor),
			Long:   basePrices.Long * (1 - adjustmentFactor),
		}
	}

	return basePrices
}

// calculateTargetPrices 计算目标价位
func calculateTargetPrices(currentPrice float64, trend string, confidence float64) model.TargetPrices {
	factor := confidence
	if factor < 0.3 {
		factor = 0.3
	}
	if trend == "up" {
		return model.TargetPrices{
			Short:  currentPrice * (1 + 0.03*factor),
			Medium: currentPrice * (1 + 0.08*factor),
			Long:   currentPrice * (1 + 0.15*factor),
		}
	} else if trend == "down" {
		return model.TargetPrices{
			Short:  currentPrice * (1 - 0.03*factor),
			Medium: currentPrice * (1 - 0.08*factor),
			Long:   currentPrice * (1 - 0.15*factor),
		}
	}
	return model.TargetPrices{
		Short:  currentPrice * (1 + 0.01*factor),
		Medium: currentPrice,
		Long:   currentPrice * (1 - 0.01*factor),
	}
}

// generateNewsAnalysis 生成消息面分析
func generateNewsAnalysis(newsImpact langchain.NewsImpact, news []langchain.NewsItem) string {
	if len(news) == 0 {
		return "**消息面分析**\n\n暂无重要公告或新闻消息。"
	}

	// 构建消息面分析
	analysis := "**消息面分析**\n\n"

	// 新闻影响评估
	if newsImpact.ImportanceLevel > 1 {
		var sentiment string
		var impactDesc string

		// 情感倾向描述
		if newsImpact.SentimentScore > 0.6 {
			sentiment = "**利好**"
		} else if newsImpact.SentimentScore > 0.2 {
			sentiment = "**偏利好**"
		} else if newsImpact.SentimentScore < -0.6 {
			sentiment = "**利空**"
		} else if newsImpact.SentimentScore < -0.2 {
			sentiment = "**偏利空**"
		} else {
			sentiment = "**中性**"
		}

		// 重要性等级描述
		switch newsImpact.ImportanceLevel {
		case 5:
			impactDesc = "极重要"
		case 4:
			impactDesc = "很重要"
		case 3:
			impactDesc = "重要"
		case 2:
			impactDesc = "较重要"
		default:
			impactDesc = "一般"
		}

		// 预期价格影响
		priceImpactPercent := newsImpact.PriceImpact * 100
		var priceImpactDesc string
		if math.Abs(priceImpactPercent) >= 5 {
			priceImpactDesc = "显著"
		} else if math.Abs(priceImpactPercent) >= 2 {
			priceImpactDesc = "中等"
		} else {
			priceImpactDesc = "轻微"
		}

		analysis += fmt.Sprintf("**影响评估**：%s消息，重要性等级%s，预期对股价产生%s影响。\n\n- 综合判断：该消息面将可能对股价产生%s驱动，请结合技术信号确认节奏。\n\n",
			sentiment, impactDesc, priceImpactDesc,
			func() string {
				if newsImpact.PriceImpact > 0 {
					return "上行"
				} else if newsImpact.PriceImpact < 0 {
					return "下行"
				}
				return "有限"
			}())
	}

	// 最新消息列表
	analysis += "\n**最新消息**：\n\n"
	for i, n := range news {
		if i >= 3 { // 只显示前3条
			break
		}
		analysis += fmt.Sprintf("- %s %s\n", n.Time, n.Title)
	}

	// 投资建议
	if newsImpact.ImportanceLevel >= 3 {
		analysis += "\n**投资提示**：\n"
		if newsImpact.SentimentScore > 0.5 {
			analysis += "- 重要利好消息可能推动股价上涨，建议关注放量突破。\n"
			analysis += "- 注意消息兑现后的获利回吐风险。"
		} else if newsImpact.SentimentScore < -0.5 {
			analysis += "- 重要利空消息可能施压股价，建议谨慎操作。\n"
			analysis += "- 关注是否出现超跌反弹机会。"
		} else {
			analysis += "- 消息面影响相对中性，以技术面分析为主。"
		}
	} else {
		priceImpactAbs := math.Abs(newsImpact.PriceImpact)
		analysis += "\n**投资提示**：\n"
		if priceImpactAbs >= 0.03 {
			if newsImpact.SentimentScore > 0.2 {
				analysis += "- 消息面对股价可能产生一定驱动，可结合技术信号择机参与。\n"
			} else if newsImpact.SentimentScore < -0.2 {
				analysis += "- 消息面或对股价造成阶段性压力，关注量价是否出现企稳迹象。\n"
			} else {
				analysis += "- 消息面影响为中性，建议结合技术面信号确认方向。\n"
			}
		} else {
			analysis += "- 消息面影响有限，建议重点关注技术面信号。"
		}
	}

	return analysis
}
