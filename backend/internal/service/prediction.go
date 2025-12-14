package service

import (
	"fmt"
	"sync"
	"time"

	"stock-forecast-backend/internal/langchain"
	"stock-forecast-backend/internal/model"
	"stock-forecast-backend/internal/stockdata"
)

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
	indicators, err := stockdata.GetIndicatorsWithRefresh(code, isIntraday)
	if err != nil {
		return nil, fmt.Errorf("获取技术指标失败: %v", err)
	}
	if indicators == nil {
		return nil, fmt.Errorf("获取技术指标失败: 返回数据为空")
	}

	// 3. 转换技术指标
	techIndicators := convertIndicators(indicators)

	// 4. 生成技术信号（从 stockdata 获取）
	signals := convertSignals(indicators.Signals)

	// 5. 获取股票新闻
	newsItems, _ := stockdata.GetStockNews(code, 5)
	news := make([]langchain.NewsItem, len(newsItems))
	for i, n := range newsItems {
		news[i] = langchain.NewsItem{Title: n.Title, Time: n.Time, Source: n.Source}
	}

	// 6. 基于技术指标生成简化的ML预测（不再依赖Python服务）
	mlPredictions := generateMLPredictions(indicators)

	// 7. 使用LangChain进行综合分析（包含新闻）
	analysis, err := langchain.AnalyzeStock(code, stockName, techIndicators, mlPredictions, signals, news)
	if err != nil {
		analysis = "AI分析暂时不可用"
	}
	if isIntraday {
		analysis = "盘中未收盘（已实时刷新第三方日K）：\n\n" + analysis
	}

	// 8. 综合判断趋势
	trend, trendCN, confidence := determineTrend(mlPredictions, signals)

	// 9. 计算目标价位
	targetPrices := calculateTargetPrices(indicators.CurrentPrice, trend, confidence)

	// 10. 获取近期每日涨跌幅
	dailyChanges := getDailyChanges(code, 10)

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
		TargetPrices:    targetPrices,
		SupportLevel:    indicators.SupportLevel,
		ResistanceLevel: indicators.ResistanceLevel,
		Indicators:      techIndicators,
		Signals:         signals,
		Analysis:        analysis,
		MLPredictions:   mlPredictions,
		DailyChanges:    dailyChanges,
	}, nil
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

	if (maAligned && priceAboveMA) || strongMomentum {
		maTrend = "up"
		maConfidence = 0.7
		if strongMomentum && maAligned {
			maConfidence = 0.85 // 强势股
		}
	} else if (maBearish && priceBelowMA) || weakMomentum {
		maTrend = "down"
		maConfidence = 0.7
		if weakMomentum && maBearish {
			maConfidence = 0.85
		}
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

	// 计算预测价格
	maPrice := ind.CurrentPrice
	if maTrend == "up" {
		maPrice = ind.CurrentPrice * (1 + 0.02*maConfidence)
	} else if maTrend == "down" {
		maPrice = ind.CurrentPrice * (1 - 0.02*maConfidence)
	}

	macdPrice := ind.CurrentPrice
	if macdTrend == "up" {
		macdPrice = ind.CurrentPrice * (1 + 0.015*macdConfidence)
	} else if macdTrend == "down" {
		macdPrice = ind.CurrentPrice * (1 - 0.015*macdConfidence)
	}

	rsiPrice := ind.CurrentPrice
	if rsiTrend == "up" {
		rsiPrice = ind.CurrentPrice * (1 + 0.025*rsiConfidence)
	} else if rsiTrend == "down" {
		rsiPrice = ind.CurrentPrice * (1 - 0.025*rsiConfidence)
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

// determineTrend 综合判断趋势（改进版）
func determineTrend(ml model.MLPredictions, signals []model.Signal) (string, string, float64) {
	bullishCount := 0
	bearishCount := 0
	totalConfidence := 0.0
	weightedBullish := 0.0
	weightedBearish := 0.0

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

	// 统计技术信号投票（权重0.5）
	for _, s := range signals {
		if s.Type == "bullish" {
			bullishCount++
			weightedBullish += 0.5
		} else if s.Type == "bearish" {
			bearishCount++
			weightedBearish += 0.5
		}
	}

	// 计算平均置信度
	avgConfidence := totalConfidence / 3

	// 判断趋势（放宽条件：差1票即可，或加权分数差距明显）
	if bullishCount > bearishCount || weightedBullish > weightedBearish+0.3 {
		return "up", "看涨", avgConfidence
	} else if bearishCount > bullishCount || weightedBearish > weightedBullish+0.3 {
		return "down", "看跌", avgConfidence
	}
	return "sideways", "震荡", avgConfidence * 0.8
}

// getDailyChanges 获取近期每日涨跌幅
func getDailyChanges(code string, days int) []model.DailyChange {
	kline, err := stockdata.GetKline(code, "daily")
	if err != nil || kline == nil || len(kline.Data) == 0 {
		return nil
	}

	data := kline.Data
	n := len(data)
	if n < 2 {
		return nil
	}

	// 取最近days天的数据
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
