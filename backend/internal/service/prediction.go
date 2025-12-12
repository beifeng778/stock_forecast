package service

import (
	"fmt"
	"sync"

	"stock-forecast-backend/internal/client"
	"stock-forecast-backend/internal/langchain"
	"stock-forecast-backend/internal/model"
)

// PredictStocks 批量预测股票
func PredictStocks(codes []string, period string) ([]model.PredictResult, error) {
	results := make([]model.PredictResult, 0, len(codes))
	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, len(codes))

	for _, code := range codes {
		wg.Add(1)
		go func(stockCode string) {
			defer wg.Done()

			result, err := predictSingleStock(stockCode, period)
			if err != nil {
				errChan <- fmt.Errorf("预测 %s 失败: %v", stockCode, err)
				return
			}

			mu.Lock()
			results = append(results, *result)
			mu.Unlock()
		}(code)
	}

	wg.Wait()
	close(errChan)

	// 收集错误
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 && len(results) == 0 {
		return nil, errs[0]
	}

	return results, nil
}

// predictSingleStock 预测单只股票
func predictSingleStock(code, period string) (*model.PredictResult, error) {
	// 1. 获取股票名称
	stockName, err := client.GetStockName(code)
	if err != nil {
		stockName = "未知"
	}

	// 2. 获取技术指标
	indicators, err := client.GetIndicators(code)
	if err != nil {
		return nil, fmt.Errorf("获取技术指标失败: %v", err)
	}

	// 3. 获取ML预测结果
	mlResult, err := client.GetMLPrediction(code, period)
	if err != nil {
		return nil, fmt.Errorf("获取ML预测失败: %v", err)
	}

	// 4. 解析技术指标
	techIndicators := parseIndicators(indicators)

	// 5. 解析ML预测
	mlPredictions := parseMLPredictions(mlResult)

	// 6. 获取当前价格
	currentPrice := getFloat(indicators, "current_price")

	// 7. 生成技术信号
	signals := generateSignals(techIndicators)

	// 8. 获取股票新闻
	clientNews, _ := client.GetStockNews(code)
	news := make([]langchain.NewsItem, len(clientNews))
	for i, n := range clientNews {
		news[i] = langchain.NewsItem{Title: n.Title, Time: n.Time, Source: n.Source}
	}

	// 9. 使用LangChain进行综合分析（包含新闻）
	analysis, err := langchain.AnalyzeStock(code, stockName, techIndicators, mlPredictions, signals, news)
	if err != nil {
		analysis = "AI分析暂时不可用"
	}

	// 9. 综合判断趋势
	trend, trendCN, confidence := determineTrend(mlPredictions, signals)

	// 10. 计算支撑位和压力位
	supportLevel := getFloat(indicators, "support_level")
	resistanceLevel := getFloat(indicators, "resistance_level")

	// 11. 计算目标价位
	targetPrices := calculateTargetPrices(currentPrice, trend, confidence)

	return &model.PredictResult{
		StockCode:       code,
		StockName:       stockName,
		CurrentPrice:    currentPrice,
		Trend:           trend,
		TrendCN:         trendCN,
		Confidence:      confidence,
		PriceRange: model.PriceRange{
			Low:  supportLevel,
			High: resistanceLevel,
		},
		TargetPrices:    targetPrices,
		SupportLevel:    supportLevel,
		ResistanceLevel: resistanceLevel,
		Indicators:      techIndicators,
		Signals:         signals,
		Analysis:        analysis,
		MLPredictions:   mlPredictions,
	}, nil
}

// parseIndicators 解析技术指标
func parseIndicators(data map[string]interface{}) model.TechnicalIndicators {
	return model.TechnicalIndicators{
		MA5:    getFloat(data, "ma5"),
		MA10:   getFloat(data, "ma10"),
		MA20:   getFloat(data, "ma20"),
		MA60:   getFloat(data, "ma60"),
		MACD:   getFloat(data, "macd"),
		Signal: getFloat(data, "signal"),
		Hist:   getFloat(data, "hist"),
		RSI:    getFloat(data, "rsi"),
		KDJ_K:  getFloat(data, "kdj_k"),
		KDJ_D:  getFloat(data, "kdj_d"),
		KDJ_J:  getFloat(data, "kdj_j"),
		BOLL_U: getFloat(data, "boll_upper"),
		BOLL_M: getFloat(data, "boll_middle"),
		BOLL_L: getFloat(data, "boll_lower"),
	}
}

// parseMLPredictions 解析ML预测结果
func parseMLPredictions(data map[string]interface{}) model.MLPredictions {
	return model.MLPredictions{
		LSTM: model.MLPrediction{
			Trend:      getString(data, "lstm_trend"),
			Price:      getFloat(data, "lstm_price"),
			Confidence: getFloat(data, "lstm_confidence"),
		},
		Prophet: model.MLPrediction{
			Trend:      getString(data, "prophet_trend"),
			Price:      getFloat(data, "prophet_price"),
			Confidence: getFloat(data, "prophet_confidence"),
		},
		XGBoost: model.MLPrediction{
			Trend:      getString(data, "xgboost_trend"),
			Price:      getFloat(data, "xgboost_price"),
			Confidence: getFloat(data, "xgboost_confidence"),
		},
	}
}

// generateSignals 生成技术信号
func generateSignals(ind model.TechnicalIndicators) []model.Signal {
	signals := make([]model.Signal, 0)

	// MACD信号
	if ind.MACD > ind.Signal {
		signals = append(signals, model.Signal{
			Name:   "MACD",
			Type:   "bullish",
			TypeCN: "金叉",
			Desc:   "MACD线上穿信号线",
		})
	} else {
		signals = append(signals, model.Signal{
			Name:   "MACD",
			Type:   "bearish",
			TypeCN: "死叉",
			Desc:   "MACD线下穿信号线",
		})
	}

	// RSI信号
	if ind.RSI > 70 {
		signals = append(signals, model.Signal{
			Name:   "RSI",
			Type:   "bearish",
			TypeCN: "超买",
			Desc:   "RSI超过70，可能回调",
		})
	} else if ind.RSI < 30 {
		signals = append(signals, model.Signal{
			Name:   "RSI",
			Type:   "bullish",
			TypeCN: "超卖",
			Desc:   "RSI低于30，可能反弹",
		})
	} else {
		signals = append(signals, model.Signal{
			Name:   "RSI",
			Type:   "neutral",
			TypeCN: "中性",
			Desc:   "RSI处于正常区间",
		})
	}

	// KDJ信号
	if ind.KDJ_K > ind.KDJ_D && ind.KDJ_J > 80 {
		signals = append(signals, model.Signal{
			Name:   "KDJ",
			Type:   "bearish",
			TypeCN: "超买",
			Desc:   "KDJ处于超买区域",
		})
	} else if ind.KDJ_K < ind.KDJ_D && ind.KDJ_J < 20 {
		signals = append(signals, model.Signal{
			Name:   "KDJ",
			Type:   "bullish",
			TypeCN: "超卖",
			Desc:   "KDJ处于超卖区域",
		})
	} else {
		signals = append(signals, model.Signal{
			Name:   "KDJ",
			Type:   "neutral",
			TypeCN: "中性",
			Desc:   "KDJ处于正常区间",
		})
	}

	return signals
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
	// confidence 已经是 0-1 之间的小数，直接使用
	factor := confidence
	if factor < 0.3 {
		factor = 0.3 // 最小因子，确保有一定的价格变化
	}
	if trend == "up" {
		return model.TargetPrices{
			Short:  currentPrice * (1 + 0.03*factor),  // 短期涨幅 ~1-3%
			Medium: currentPrice * (1 + 0.08*factor),  // 中期涨幅 ~2.4-8%
			Long:   currentPrice * (1 + 0.15*factor),  // 长期涨幅 ~4.5-15%
		}
	} else if trend == "down" {
		return model.TargetPrices{
			Short:  currentPrice * (1 - 0.03*factor),  // 短期跌幅 ~1-3%
			Medium: currentPrice * (1 - 0.08*factor),  // 中期跌幅 ~2.4-8%
			Long:   currentPrice * (1 - 0.15*factor),  // 长期跌幅 ~4.5-15%
		}
	}
	// 震荡行情，小幅波动
	return model.TargetPrices{
		Short:  currentPrice * (1 + 0.01*factor),
		Medium: currentPrice,
		Long:   currentPrice * (1 - 0.01*factor),
	}
}

// getFloat 安全获取float值
func getFloat(data map[string]interface{}, key string) float64 {
	if v, ok := data[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case int:
			return float64(val)
		}
	}
	return 0
}

// getString 安全获取string值
func getString(data map[string]interface{}, key string) string {
	if v, ok := data[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
