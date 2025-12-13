package service

import (
	"fmt"
	"sync"

	"stock-forecast-backend/internal/langchain"
	"stock-forecast-backend/internal/model"
	"stock-forecast-backend/internal/stockdata"
)

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
	// 1. 获取股票名称
	stockName, err := stockdata.GetStockName(code)
	if err != nil {
		stockName = "未知"
	}

	// 2. 获取技术指标
	indicators, err := stockdata.GetIndicators(code)
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

	// 8. 综合判断趋势
	trend, trendCN, confidence := determineTrend(mlPredictions, signals)

	// 9. 计算目标价位
	targetPrices := calculateTargetPrices(indicators.CurrentPrice, trend, confidence)

	return &model.PredictResult{
		StockCode:    code,
		StockName:    stockName,
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

// generateMLPredictions 基于技术指标生成ML预测（简化版）
func generateMLPredictions(ind *stockdata.Indicators) model.MLPredictions {
	// 基于均线判断趋势
	maTrend := "sideways"
	maConfidence := 0.5
	if ind.CurrentPrice > ind.MA5 && ind.MA5 > ind.MA20 {
		maTrend = "up"
		maConfidence = 0.7
	} else if ind.CurrentPrice < ind.MA5 && ind.MA5 < ind.MA20 {
		maTrend = "down"
		maConfidence = 0.7
	}

	// 基于MACD判断趋势
	macdTrend := "sideways"
	macdConfidence := 0.5
	if ind.MACD > ind.Signal && ind.Hist > 0 {
		macdTrend = "up"
		macdConfidence = 0.65
	} else if ind.MACD < ind.Signal && ind.Hist < 0 {
		macdTrend = "down"
		macdConfidence = 0.65
	}

	// 基于RSI判断趋势
	rsiTrend := "sideways"
	rsiConfidence := 0.5
	if ind.RSI < 30 {
		rsiTrend = "up" // 超卖，可能反弹
		rsiConfidence = 0.6
	} else if ind.RSI > 70 {
		rsiTrend = "down" // 超买，可能回调
		rsiConfidence = 0.6
	}

	// 计算预测价格
	maPrice := ind.CurrentPrice
	if maTrend == "up" {
		maPrice = ind.CurrentPrice * 1.02
	} else if maTrend == "down" {
		maPrice = ind.CurrentPrice * 0.98
	}

	macdPrice := ind.CurrentPrice
	if macdTrend == "up" {
		macdPrice = ind.CurrentPrice * 1.015
	} else if macdTrend == "down" {
		macdPrice = ind.CurrentPrice * 0.985
	}

	rsiPrice := ind.CurrentPrice
	if rsiTrend == "up" {
		rsiPrice = ind.CurrentPrice * 1.025
	} else if rsiTrend == "down" {
		rsiPrice = ind.CurrentPrice * 0.975
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

// determineTrend 综合判断趋势
func determineTrend(ml model.MLPredictions, signals []model.Signal) (string, string, float64) {
	bullishCount := 0
	bearishCount := 0
	totalConfidence := 0.0

	// 统计ML模型投票
	models := []model.MLPrediction{ml.LSTM, ml.Prophet, ml.XGBoost}
	for _, m := range models {
		if m.Trend == "up" {
			bullishCount++
		} else if m.Trend == "down" {
			bearishCount++
		}
		totalConfidence += m.Confidence
	}

	// 统计技术信号投票
	for _, s := range signals {
		if s.Type == "bullish" {
			bullishCount++
		} else if s.Type == "bearish" {
			bearishCount++
		}
	}

	// 计算平均置信度
	avgConfidence := totalConfidence / 3

	// 判断趋势
	if bullishCount > bearishCount+1 {
		return "up", "看涨", avgConfidence
	} else if bearishCount > bullishCount+1 {
		return "down", "看跌", avgConfidence
	}
	return "sideways", "震荡", avgConfidence * 0.8
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
