package service

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"stock-forecast-backend/internal/langchain"
	"stock-forecast-backend/internal/model"
	"stock-forecast-backend/internal/stockdata"
)

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

func isTradingTimeNow() bool {
	now := time.Now()
	wd := now.Weekday()
	if wd == time.Saturday || wd == time.Sunday {
		return false
	}
	hhmm := now.Hour()*100 + now.Minute()
	morning := hhmm >= 930 && hhmm < 1130
	afternoon := hhmm >= 1300 && hhmm < 1500
	return morning || afternoon
}

// PredictStocks 批量预测股票（保持原始顺序）
func PredictStocks(codes []string, period string) ([]model.PredictResult, error) {
	results := make([]*model.PredictResult, len(codes))
	errors := make([]error, len(codes))
	var wg sync.WaitGroup

	for i, code := range codes {
		wg.Add(1)
		go func(idx int, stockCode string) {
			defer wg.Done()

			result, err := predictSingleStock(stockCode, period)
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
func predictSingleStock(code, period string) (*model.PredictResult, error) {
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
	if (sector == "" || industry == "") && stockName != "未知" {
		classification := langchain.GetStockClassification(code, stockName)
		if sector == "" {
			sector = classification.Sector
		}
		if industry == "" {
			industry = classification.Industry
		}
	}

	// 2. 获取技术指标
	isIntraday := isTradingTimeNow()
	kline, err := stockdata.GetKlineWithRefresh(code, "daily", isIntraday)
	if err != nil {
		return nil, fmt.Errorf("获取技术指标失败: %v", err)
	}
	if kline == nil || len(kline.Data) == 0 {
		return nil, fmt.Errorf("获取技术指标失败: 返回数据为空")
	}

	// 3. 转换技术指标
	indicators, err := stockdata.CalculateIndicators(kline.Data)
	if err != nil {
		return nil, fmt.Errorf("获取技术指标失败: %v", err)
	}
	if indicators == nil {
		return nil, fmt.Errorf("获取技术指标失败: 返回数据为空")
	}

	techIndicators := convertIndicators(indicators)

	// 4. 生成技术信号（从 stockdata 获取）
	signals := convertSignals(indicators.Signals)

	// 5. 获取股票新闻
	newsItems, _ := stockdata.GetStockNews(code, 5)
	news := make([]langchain.NewsItem, len(newsItems))
	for i, n := range newsItems {
		news[i] = langchain.NewsItem{Title: n.Title, Time: n.Time, Source: n.Source}
	}

	// 6. 分析新闻对股价的量化影响
	newsImpact := langchain.AnalyzeNewsImpact(code, stockName, news)

	// 7. 基于技术指标生成简化的ML预测（不再依赖Python服务）
	mlPredictions := generateMLPredictions(indicators)

	// 8. 使用LangChain进行综合分析（包含新闻）
	analysis, err := langchain.AnalyzeStock(code, stockName, techIndicators, mlPredictions, signals, news)
	if err != nil {
		analysis = "AI分析暂时不可用"
	}
	if isIntraday {
		analysis = "盘中未收盘（已实时刷新第三方日K）：\n\n" + analysis
	}

	// 8.1. 生成专门的消息面分析
	newsAnalysis := generateNewsAnalysis(newsImpact, news)

	// 9. 综合判断趋势（融入新闻影响）
	trend, trendCN, confidence := determineTrendWithNews(mlPredictions, signals, newsImpact)

	// 10. 计算目标价位（考虑新闻影响）
	targetPrices := calculateTargetPricesWithNews(indicators.CurrentPrice, trend, confidence, newsImpact)

	// 10. 获取近期每日涨跌幅
	dailyChanges := getDailyChangesFromKline(kline.Data, 10)

	now := time.Now()
	todayStr := now.Format("2006-01-02")
	last := kline.Data[len(kline.Data)-1]
	hasTodayData := last.Date == todayStr
	needPredictToday := isIntraday && !hasTodayData

	var aiToday *model.KlineData
	if isIntraday && hasTodayData {
		aiToday = generateTodayKline(code, stockName, kline.Data, targetPrices.Short, indicators.SupportLevel, indicators.ResistanceLevel, confidence, indicators.Volatility)
	}

	futureKlines := generateFutureKlines(code, stockName, kline.Data, isIntraday, targetPrices.Short, indicators.SupportLevel, indicators.ResistanceLevel, confidence, indicators.Volatility)

	return &model.PredictResult{
		StockCode:    code,
		StockName:    stockName,
		Sector:       sector,
		Industry:     industry,
		IsIntraday:   isIntraday,
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
	if len(history) < 2 {
		return nil
	}

	now := time.Now()
	todayStr := now.Format("2006-01-02")
	last := history[len(history)-1]
	if last.Date != todayStr {
		return nil
	}

	anchorClose := history[len(history)-2].Close
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

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
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

	limit := getDailyPriceLimitPercent(stockCode, stockName)
	clampKlineToLimit(anchorClose, limit, &open, &high, &low, &close)

	_ = supportLevel
	_ = resistanceLevel

	return &model.KlineData{
		Date:   todayStr,
		Open:   round2(open),
		Close:  round2(close),
		High:   round2(high),
		Low:    round2(low),
		Volume: last.Volume,
		Amount: last.Amount,
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

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	currentDate := now
	if !isIntraday || hasTodayData {
		currentDate = currentDate.AddDate(0, 0, 1)
	}

	prevClose := anchorClose
	result := make([]model.KlineData, 0, steps)
	limit := getDailyPriceLimitPercent(stockCode, stockName)
	for len(result) < steps {
		wd := currentDate.Weekday()
		if wd == time.Saturday || wd == time.Sunday {
			currentDate = currentDate.AddDate(0, 0, 1)
			continue
		}

		gap := rng.NormFloat64() * dailyStd * 0.15
		open := prevClose * (1 + gap)
		ret := drift + rng.NormFloat64()*dailyStd*0.5
		close := prevClose * (1 + ret)
		if close <= 0 {
			close = prevClose
		}

		rangeFactor := math.Abs(rng.NormFloat64()) * dailyStd * 0.8
		high := math.Max(open, close) * (1 + rangeFactor)
		low := math.Min(open, close) * (1 - rangeFactor)
		if low <= 0 {
			low = math.Min(open, close)
		}

		clampKlineToLimit(prevClose, limit, &open, &high, &low, &close)

		volume := avgVolume
		if volume > 0 {
			volume = volume * (0.7 + 0.6*rng.Float64())
		}
		amount := volume * close

		_ = supportLevel
		_ = resistanceLevel

		result = append(result, model.KlineData{
			Date:   currentDate.Format("2006-01-02"),
			Open:   round2(open),
			Close:  round2(close),
			High:   round2(high),
			Low:    round2(low),
			Volume: volume,
			Amount: amount,
		})

		prevClose = close
		currentDate = currentDate.AddDate(0, 0, 1)
	}

	return result
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
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
	newsAdjustment := 1.0 + newsImpact.PriceImpact

	// 重要性等级越高，影响越大
	impactMultiplier := 1.0 + float64(newsImpact.ImportanceLevel-2)*0.2 // 等级3=1.2x, 等级4=1.4x, 等级5=1.6x

	// 应用新闻影响调整
	if newsImpact.PriceImpact > 0 {
		// 利好新闻：上调目标价位
		return model.TargetPrices{
			Short:  basePrices.Short * (newsAdjustment * impactMultiplier),
			Medium: basePrices.Medium * (newsAdjustment * impactMultiplier),
			Long:   basePrices.Long * (newsAdjustment * impactMultiplier),
		}
	} else if newsImpact.PriceImpact < 0 {
		// 利空新闻：下调目标价位
		return model.TargetPrices{
			Short:  basePrices.Short * (newsAdjustment / impactMultiplier),
			Medium: basePrices.Medium * (newsAdjustment / impactMultiplier),
			Long:   basePrices.Long * (newsAdjustment / impactMultiplier),
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

		analysis += fmt.Sprintf("**影响评估**：%s消息，重要性等级%s，预期对股价产生%s影响（约%.1f%%）\n\n",
			sentiment, impactDesc, priceImpactDesc, priceImpactPercent)
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
			analysis += "- 重要利好消息可能推动股价上涨，建议关注放量突破\n"
			analysis += "- 注意消息兑现后的获利回吐风险"
		} else if newsImpact.SentimentScore < -0.5 {
			analysis += "- 重要利空消息可能施压股价，建议谨慎操作\n"
			analysis += "- 关注是否出现超跌反弹机会"
		} else {
			analysis += "- 消息面影响相对中性，以技术面分析为主"
		}
	} else {
		analysis += "\n**投资提示**：\n\n- 消息面影响有限，建议重点关注技术面信号。"
	}

	return analysis
}
