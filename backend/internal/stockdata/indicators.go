package stockdata

import (
	"fmt"
	"log"
	"math"
)

// Indicators 技术指标
type Indicators struct {
	CurrentPrice    float64  `json:"current_price"`
	MA5             float64  `json:"ma5"`
	MA10            float64  `json:"ma10"`
	MA20            float64  `json:"ma20"`
	MA60            float64  `json:"ma60"`
	MACD            float64  `json:"macd"`
	Signal          float64  `json:"signal"`
	Hist            float64  `json:"hist"`
	RSI             float64  `json:"rsi"`
	KDJK            float64  `json:"kdj_k"`
	KDJD            float64  `json:"kdj_d"`
	KDJJ            float64  `json:"kdj_j"`
	BollUpper       float64  `json:"boll_upper"`
	BollMiddle      float64  `json:"boll_middle"`
	BollLower       float64  `json:"boll_lower"`
	SupportLevel    float64  `json:"support_level"`
	ResistanceLevel float64  `json:"resistance_level"`
	Signals         []Signal `json:"signals"`
	// 动量指标
	Change1D  float64 `json:"change_1d"`  // 1日涨跌幅
	Change5D  float64 `json:"change_5d"`  // 5日涨跌幅
	Change10D float64 `json:"change_10d"` // 10日涨跌幅
	MA5Slope  float64 `json:"ma5_slope"`  // MA5斜率
	// 成交量指标
	CurrentVolume   float64 `json:"current_volume"`    // 当前成交量
	VolumeMA5       float64 `json:"volume_ma5"`        // 5日成交量均线
	VolumeMA10      float64 `json:"volume_ma10"`       // 10日成交量均线
	VolumeRatio     float64 `json:"volume_ratio"`      // 量比（当日/5日均量）
	PriceVolumeDiv  string  `json:"price_volume_div"`  // 量价背离信号
	VolumeStrength  float64 `json:"volume_strength"`   // 成交量强度
	// 动态阈值
	RSIUpperThreshold float64 `json:"rsi_upper_threshold"` // RSI动态超买阈值
	RSILowerThreshold float64 `json:"rsi_lower_threshold"` // RSI动态超卖阈值
	// 市场环境
	MarketTrend    string  `json:"market_trend"`     // 市场趋势：bull/bear/sideways
	Volatility     float64 `json:"volatility"`       // 价格波动率
	TrendStrength  float64 `json:"trend_strength"`   // 趋势强度
	// 突破信号
	BollBreakout   string  `json:"boll_breakout"`    // 布林带突破信号
	VolumeBreakout bool    `json:"volume_breakout"`  // 成交量突破确认
	PriceAccel     float64 `json:"price_accel"`      // 价格加速度
	MomentumScore  float64 `json:"momentum_score"`   // 综合动量评分
	// 情绪指标
	Amplitude          float64 `json:"amplitude"`            // 振幅（当日）
	AvgAmplitude5D     float64 `json:"avg_amplitude_5d"`     // 5日平均振幅
	UpperShadowRatio   float64 `json:"upper_shadow_ratio"`   // 上影线比率
	LowerShadowRatio   float64 `json:"lower_shadow_ratio"`   // 下影线比率
	ContinuousDays     int     `json:"continuous_days"`      // 连续涨跌天数（正数涨，负数跌）
	SentimentStrength  float64 `json:"sentiment_strength"`   // 情绪强度（0-100）
	SentimentType      string  `json:"sentiment_type"`       // 情绪类型：bullish/bearish/neutral/panic/frenzy
	// 主力成本指标
	MainForceCost20    float64 `json:"main_force_cost_20"`   // 20日主力成本（VWAP）
	MainForceCost60    float64 `json:"main_force_cost_60"`   // 60日主力成本（VWAP）
	CostDeviation20    float64 `json:"cost_deviation_20"`    // 当前价与20日成本偏离度（%）
	CostDeviation60    float64 `json:"cost_deviation_60"`    // 当前价与60日成本偏离度（%）
	ChipConcentration  float64 `json:"chip_concentration"`   // 筹码集中度（0-1，越大越集中）
	MainForceProfit    float64 `json:"main_force_profit"`    // 主力浮盈（%，基于20日成本）
	// 大盘影响指标
	IndexCode          string  `json:"index_code"`           // 参考指数代码（000001.SH/399006.SZ）
	IndexChange        float64 `json:"index_change"`         // 大盘当日涨跌幅（%）
	IndexTrend         string  `json:"index_trend"`          // 大盘趋势：bull/bear/sideways
	RelativeStrength   float64 `json:"relative_strength"`    // 相对大盘强度（个股涨幅-大盘涨幅）
	Beta               float64 `json:"beta"`                 // Beta系数（个股与大盘的相关性）
	FollowIndex        bool    `json:"follow_index"`         // 是否跟随大盘（Beta>0.8）
}

// Signal 信号
type Signal struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Desc string `json:"desc"`
}

// CalculateIndicators 计算技术指标
func CalculateIndicators(data []KlineData) (*Indicators, error) {
	if len(data) < 1 {
		return nil, fmt.Errorf("K线数据为空")
	}

	closes := make([]float64, len(data))
	highs := make([]float64, len(data))
	lows := make([]float64, len(data))
	volumes := make([]float64, len(data))
	for i, d := range data {
		closes[i] = d.Close
		highs[i] = d.High
		lows[i] = d.Low
		volumes[i] = d.Volume
	}

	ind := &Indicators{}

	// 当前价格
	ind.CurrentPrice = closes[len(closes)-1]

	n := len(closes)

	// 均线（根据数据量动态调整）
	ind.MA5 = calculateMA(closes, min(5, n))
	ind.MA10 = calculateMA(closes, min(10, n))
	ind.MA20 = calculateMA(closes, min(20, n))
	ind.MA60 = calculateMA(closes, min(60, n))

	// MACD
	ind.MACD, ind.Signal, ind.Hist = calculateMACD(closes)

	// RSI
	ind.RSI = calculateRSI(closes, min(14, n))

	// KDJ
	ind.KDJK, ind.KDJD, ind.KDJJ = calculateKDJ(highs, lows, closes)

	// 布林带
	ind.BollUpper, ind.BollMiddle, ind.BollLower = calculateBollinger(closes, min(20, n))

	// 支撑位和压力位
	lookback := min(20, n)
	ind.SupportLevel = minSlice(lows[n-lookback:])
	ind.ResistanceLevel = maxSlice(highs[n-lookback:])

	// 动量指标
	if n >= 2 {
		ind.Change1D = (closes[n-1] - closes[n-2]) / closes[n-2] * 100
	}
	if n >= 6 {
		ind.Change5D = (closes[n-1] - closes[n-6]) / closes[n-6] * 100
	}
	if n >= 11 {
		ind.Change10D = (closes[n-1] - closes[n-11]) / closes[n-11] * 100
	}
	// MA5斜率（最近3天MA5的变化率）
	if n >= 8 {
		ma5Today := calculateMA(closes, 5)
		ma5_3DaysAgo := calculateMA(closes[:n-3], 5)
		if ma5_3DaysAgo > 0 {
			ind.MA5Slope = (ma5Today - ma5_3DaysAgo) / ma5_3DaysAgo * 100
		}
	}

	// 成交量指标
	ind.CurrentVolume = volumes[n-1]
	ind.VolumeMA5 = calculateMA(volumes, min(5, n))
	ind.VolumeMA10 = calculateMA(volumes, min(10, n))

	// 量比（当日成交量 / 5日平均成交量）
	if ind.VolumeMA5 > 0 {
		ind.VolumeRatio = ind.CurrentVolume / ind.VolumeMA5
	} else {
		ind.VolumeRatio = 1.0
	}

	// 量价背离检测
	ind.PriceVolumeDiv = detectPriceVolumeDiv(closes, volumes)

	// 成交量强度（相对于历史波动的成交量水平）
	ind.VolumeStrength = calculateVolumeStrength(volumes)

	// 计算动态RSI阈值
	ind.RSIUpperThreshold, ind.RSILowerThreshold = calculateDynamicRSIThresholds(closes)

	// 计算市场环境
	ind.MarketTrend, ind.Volatility, ind.TrendStrength = analyzeMarketEnvironment(closes, volumes)

	// 计算突破信号
	ind.BollBreakout = detectBollingerBreakout(closes, ind.BollUpper, ind.BollLower, ind.CurrentPrice)
	ind.VolumeBreakout = detectVolumeBreakout(volumes, ind.VolumeRatio)
	ind.PriceAccel = calculatePriceAcceleration(closes)
	ind.MomentumScore = calculateMomentumScore(ind)

	// 计算情绪指标
	ind.Amplitude, ind.AvgAmplitude5D = calculateAmplitude(data)
	ind.UpperShadowRatio, ind.LowerShadowRatio = calculateShadowRatios(data)
	ind.ContinuousDays = calculateContinuousDays(closes)
	ind.SentimentStrength, ind.SentimentType = calculateSentiment(ind, data)

	// 计算主力成本指标
	ind.MainForceCost20, ind.MainForceCost60 = calculateMainForceCost(data)
	ind.CostDeviation20 = calculateCostDeviation(ind.CurrentPrice, ind.MainForceCost20)
	ind.CostDeviation60 = calculateCostDeviation(ind.CurrentPrice, ind.MainForceCost60)
	ind.ChipConcentration = calculateChipConcentration(data)
	ind.MainForceProfit = calculateCostDeviation(ind.CurrentPrice, ind.MainForceCost20)

	// 生成信号
	ind.Signals = generateSignals(ind)

	return ind, nil
}

// calculateMA 计算移动平均
func calculateMA(data []float64, period int) float64 {
	if len(data) < period {
		return 0
	}
	sum := 0.0
	for i := len(data) - period; i < len(data); i++ {
		sum += data[i]
	}
	return sum / float64(period)
}

// calculateEMA 计算指数移动平均
func calculateEMA(data []float64, period int) []float64 {
	if len(data) < period {
		return nil
	}

	ema := make([]float64, len(data))
	multiplier := 2.0 / float64(period+1)

	// 第一个EMA使用SMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += data[i]
	}
	ema[period-1] = sum / float64(period)

	// 后续使用EMA公式
	for i := period; i < len(data); i++ {
		ema[i] = (data[i]-ema[i-1])*multiplier + ema[i-1]
	}

	return ema
}

// calculateMACD 计算MACD
func calculateMACD(closes []float64) (macd, signal, hist float64) {
	if len(closes) < 26 {
		return 0, 0, 0
	}

	ema12 := calculateEMA(closes, 12)
	ema26 := calculateEMA(closes, 26)

	if ema12 == nil || ema26 == nil {
		return 0, 0, 0
	}

	// DIF = EMA12 - EMA26（只计算有效部分）
	validStart := 25 // EMA26需要26个数据点，索引从25开始
	dif := make([]float64, len(closes)-validStart)
	for i := validStart; i < len(closes); i++ {
		dif[i-validStart] = ema12[i] - ema26[i]
	}

	// DEA = EMA9(DIF)
	dea := calculateEMA(dif, 9)
	if dea == nil || len(dea) == 0 {
		return 0, 0, 0
	}

	// 取最后的值
	macd = dif[len(dif)-1]
	signal = dea[len(dea)-1]
	hist = macd - signal // 标准MACD柱状图不需要乘以2

	return macd, signal, hist
}

// calculateRSI 计算RSI（标准算法，使用EMA平滑）
func calculateRSI(closes []float64, period int) float64 {
	if len(closes) < period+1 {
		return 50
	}

	// 计算价格变化
	changes := make([]float64, len(closes)-1)
	for i := 1; i < len(closes); i++ {
		changes[i-1] = closes[i] - closes[i-1]
	}

	if len(changes) < period {
		return 50
	}

	// 分离涨跌
	gains := make([]float64, len(changes))
	losses := make([]float64, len(changes))
	for i, change := range changes {
		if change > 0 {
			gains[i] = change
			losses[i] = 0
		} else {
			gains[i] = 0
			losses[i] = -change
		}
	}

	// 计算初始平均涨跌幅（前period个数据的简单平均）
	var avgGain, avgLoss float64
	for i := 0; i < period; i++ {
		avgGain += gains[i]
		avgLoss += losses[i]
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	// 使用EMA方式计算后续的平均涨跌幅
	alpha := 1.0 / float64(period) // EMA平滑因子
	for i := period; i < len(gains); i++ {
		avgGain = alpha*gains[i] + (1-alpha)*avgGain
		avgLoss = alpha*losses[i] + (1-alpha)*avgLoss
	}

	if avgLoss == 0 {
		return 100
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))

	// 确保RSI在0-100范围内
	if rsi < 0 {
		rsi = 0
	} else if rsi > 100 {
		rsi = 100
	}

	return rsi
}

// calculateKDJ 计算KDJ（标准算法）
func calculateKDJ(highs, lows, closes []float64) (k, d, j float64) {
	period := 9
	if len(closes) < period {
		return 50, 50, 50
	}

	// 计算所有RSV值
	rsvs := make([]float64, 0)
	for i := period - 1; i < len(closes); i++ {
		// 取当前位置往前period个数据
		periodHighs := highs[i-period+1 : i+1]
		periodLows := lows[i-period+1 : i+1]

		highest := maxSlice(periodHighs)
		lowest := minSlice(periodLows)
		close := closes[i]

		rsv := 50.0
		if highest != lowest {
			rsv = (close - lowest) / (highest - lowest) * 100
		}
		rsvs = append(rsvs, rsv)
	}

	if len(rsvs) == 0 {
		return 50, 50, 50
	}

	// 计算K值（RSV的EMA，平滑因子1/3）
	k = rsvs[0] // 初始K值等于第一个RSV
	for i := 1; i < len(rsvs); i++ {
		k = (2.0/3.0)*k + (1.0/3.0)*rsvs[i]
	}

	// 计算D值（K值的EMA，平滑因子1/3）
	// 为了计算D值，我们需要维护K值序列
	ks := make([]float64, len(rsvs))
	ks[0] = rsvs[0]
	for i := 1; i < len(rsvs); i++ {
		ks[i] = (2.0/3.0)*ks[i-1] + (1.0/3.0)*rsvs[i]
	}

	d = ks[0] // 初始D值等于第一个K值
	for i := 1; i < len(ks); i++ {
		d = (2.0/3.0)*d + (1.0/3.0)*ks[i]
	}

	// 计算J值
	j = 3*k - 2*d

	// 确保KDJ值在合理范围内
	if k < 0 {
		k = 0
	} else if k > 100 {
		k = 100
	}

	if d < 0 {
		d = 0
	} else if d > 100 {
		d = 100
	}

	// J值可以超出0-100范围，这是正常的

	return k, d, j
}

// calculateBollinger 计算布林带
func calculateBollinger(closes []float64, period int) (upper, middle, lower float64) {
	if len(closes) < period {
		return 0, 0, 0
	}

	// 中轨 = MA20
	middle = calculateMA(closes, period)

	// 计算标准差
	sum := 0.0
	for i := len(closes) - period; i < len(closes); i++ {
		sum += math.Pow(closes[i]-middle, 2)
	}
	std := math.Sqrt(sum / float64(period))

	// 上轨 = 中轨 + 2*标准差
	upper = middle + 2*std
	// 下轨 = 中轨 - 2*标准差
	lower = middle - 2*std

	return upper, middle, lower
}

// generateSignals 生成信号
func generateSignals(ind *Indicators) []Signal {
	var signals []Signal

	// MACD信号
	if ind.MACD > ind.Signal {
		signals = append(signals, Signal{Name: "MACD", Type: "bullish", Desc: "金叉"})
	} else {
		signals = append(signals, Signal{Name: "MACD", Type: "bearish", Desc: "死叉"})
	}

	// RSI信号（使用动态阈值）
	if ind.RSI > ind.RSIUpperThreshold {
		signals = append(signals, Signal{Name: "RSI", Type: "bearish", Desc: fmt.Sprintf("超买(%.1f)", ind.RSIUpperThreshold)})
	} else if ind.RSI < ind.RSILowerThreshold {
		signals = append(signals, Signal{Name: "RSI", Type: "bullish", Desc: fmt.Sprintf("超卖(%.1f)", ind.RSILowerThreshold)})
	} else {
		signals = append(signals, Signal{Name: "RSI", Type: "neutral", Desc: "中性"})
	}

	// KDJ信号
	if ind.KDJJ > 80 {
		signals = append(signals, Signal{Name: "KDJ", Type: "bearish", Desc: "超买"})
	} else if ind.KDJJ < 20 {
		signals = append(signals, Signal{Name: "KDJ", Type: "bullish", Desc: "超卖"})
	} else {
		signals = append(signals, Signal{Name: "KDJ", Type: "neutral", Desc: "中性"})
	}

	// 均线信号
	if ind.CurrentPrice > ind.MA5 && ind.MA5 > ind.MA20 {
		signals = append(signals, Signal{Name: "均线", Type: "bullish", Desc: "多头排列"})
	} else if ind.CurrentPrice < ind.MA5 && ind.MA5 < ind.MA20 {
		signals = append(signals, Signal{Name: "均线", Type: "bearish", Desc: "空头排列"})
	} else {
		signals = append(signals, Signal{Name: "均线", Type: "neutral", Desc: "交织"})
	}

	// 成交量信号
	if ind.VolumeRatio > 2.0 {
		signals = append(signals, Signal{Name: "成交量", Type: "bullish", Desc: "放量"})
	} else if ind.VolumeRatio < 0.5 {
		signals = append(signals, Signal{Name: "成交量", Type: "bearish", Desc: "缩量"})
	} else {
		signals = append(signals, Signal{Name: "成交量", Type: "neutral", Desc: "正常"})
	}

	// 量价背离信号
	switch ind.PriceVolumeDiv {
	case "bearish_divergence":
		signals = append(signals, Signal{Name: "量价", Type: "bearish", Desc: "顶背离"})
	case "bullish_divergence":
		signals = append(signals, Signal{Name: "量价", Type: "bullish", Desc: "底背离"})
	case "healthy_uptrend":
		signals = append(signals, Signal{Name: "量价", Type: "bullish", Desc: "量价齐升"})
	case "healthy_downtrend":
		signals = append(signals, Signal{Name: "量价", Type: "bearish", Desc: "量价齐跌"})
	default:
		signals = append(signals, Signal{Name: "量价", Type: "neutral", Desc: "中性"})
	}

	// 市场环境信号
	switch ind.MarketTrend {
	case "bull":
		signals = append(signals, Signal{Name: "市场", Type: "bullish", Desc: fmt.Sprintf("牛市(强度%.2f)", ind.TrendStrength)})
	case "bear":
		signals = append(signals, Signal{Name: "市场", Type: "bearish", Desc: fmt.Sprintf("熊市(强度%.2f)", ind.TrendStrength)})
	default:
		signals = append(signals, Signal{Name: "市场", Type: "neutral", Desc: "震荡市"})
	}

	// 波动率信号
	if ind.Volatility > 0.05 {
		signals = append(signals, Signal{Name: "波动", Type: "bearish", Desc: "高波动"})
	} else if ind.Volatility < 0.02 {
		signals = append(signals, Signal{Name: "波动", Type: "neutral", Desc: "低波动"})
	} else {
		signals = append(signals, Signal{Name: "波动", Type: "neutral", Desc: "正常波动"})
	}

	// 突破信号
	switch ind.BollBreakout {
	case "upper_breakout":
		signals = append(signals, Signal{Name: "突破", Type: "bullish", Desc: "布林上轨突破"})
	case "upper_touch":
		signals = append(signals, Signal{Name: "突破", Type: "bullish", Desc: "布林上轨触及"})
	case "lower_breakout":
		signals = append(signals, Signal{Name: "突破", Type: "bearish", Desc: "布林下轨突破"})
	case "lower_touch":
		signals = append(signals, Signal{Name: "突破", Type: "bearish", Desc: "布林下轨触及"})
	default:
		signals = append(signals, Signal{Name: "突破", Type: "neutral", Desc: "无突破"})
	}

	// 动量信号
	if ind.MomentumScore > 70 {
		signals = append(signals, Signal{Name: "动量", Type: "bullish", Desc: fmt.Sprintf("强势(%.0f分)", ind.MomentumScore)})
	} else if ind.MomentumScore > 50 {
		signals = append(signals, Signal{Name: "动量", Type: "bullish", Desc: fmt.Sprintf("偏强(%.0f分)", ind.MomentumScore)})
	} else if ind.MomentumScore < 30 {
		signals = append(signals, Signal{Name: "动量", Type: "bearish", Desc: fmt.Sprintf("弱势(%.0f分)", ind.MomentumScore)})
	} else {
		signals = append(signals, Signal{Name: "动量", Type: "neutral", Desc: fmt.Sprintf("中性(%.0f分)", ind.MomentumScore)})
	}

	// 情绪信号
	switch ind.SentimentType {
	case "panic":
		signals = append(signals, Signal{Name: "情绪", Type: "bullish", Desc: fmt.Sprintf("恐慌(强度%.0f)", ind.SentimentStrength)})
	case "frenzy":
		signals = append(signals, Signal{Name: "情绪", Type: "bearish", Desc: fmt.Sprintf("狂热(强度%.0f)", ind.SentimentStrength)})
	case "bullish":
		signals = append(signals, Signal{Name: "情绪", Type: "bullish", Desc: fmt.Sprintf("乐观(强度%.0f)", ind.SentimentStrength)})
	case "bearish":
		signals = append(signals, Signal{Name: "情绪", Type: "bearish", Desc: fmt.Sprintf("悲观(强度%.0f)", ind.SentimentStrength)})
	default:
		signals = append(signals, Signal{Name: "情绪", Type: "neutral", Desc: "平稳"})
	}

	// 主力成本信号
	if ind.CostDeviation20 > 20 {
		signals = append(signals, Signal{Name: "主力成本", Type: "bearish", Desc: fmt.Sprintf("获利盘较重(+%.1f%%)", ind.CostDeviation20)})
	} else if ind.CostDeviation20 > 10 {
		signals = append(signals, Signal{Name: "主力成本", Type: "neutral", Desc: fmt.Sprintf("主力浮盈(+%.1f%%)", ind.CostDeviation20)})
	} else if ind.CostDeviation20 > 0 {
		signals = append(signals, Signal{Name: "主力成本", Type: "bullish", Desc: fmt.Sprintf("主力小幅浮盈(+%.1f%%)", ind.CostDeviation20)})
	} else if ind.CostDeviation20 > -10 {
		signals = append(signals, Signal{Name: "主力成本", Type: "bullish", Desc: fmt.Sprintf("接近成本区(%.1f%%)", ind.CostDeviation20)})
	} else {
		signals = append(signals, Signal{Name: "主力成本", Type: "bullish", Desc: fmt.Sprintf("主力被套(%.1f%%)", ind.CostDeviation20)})
	}

	// 筹码集中度信号
	if ind.ChipConcentration > 0.7 {
		signals = append(signals, Signal{Name: "筹码", Type: "bullish", Desc: fmt.Sprintf("高度集中(%.2f)", ind.ChipConcentration)})
	} else if ind.ChipConcentration > 0.5 {
		signals = append(signals, Signal{Name: "筹码", Type: "neutral", Desc: fmt.Sprintf("中度集中(%.2f)", ind.ChipConcentration)})
	} else {
		signals = append(signals, Signal{Name: "筹码", Type: "neutral", Desc: fmt.Sprintf("较分散(%.2f)", ind.ChipConcentration)})
	}

	// 大盘影响信号
	log.Printf("[DEBUG][signals] IndexCode=%s, IndexChange=%.2f, IndexTrend=%s", ind.IndexCode, ind.IndexChange, ind.IndexTrend)
	if ind.IndexCode != "" {
		// 大盘趋势信号
		switch ind.IndexTrend {
		case "bull":
			signals = append(signals, Signal{Name: "大盘", Type: "bullish", Desc: fmt.Sprintf("牛市(+%.2f%%)", ind.IndexChange)})
		case "bear":
			signals = append(signals, Signal{Name: "大盘", Type: "bearish", Desc: fmt.Sprintf("熊市(%.2f%%)", ind.IndexChange)})
		default:
			if ind.IndexChange > 1 {
				signals = append(signals, Signal{Name: "大盘", Type: "bullish", Desc: fmt.Sprintf("震荡偏强(+%.2f%%)", ind.IndexChange)})
			} else if ind.IndexChange < -1 {
				signals = append(signals, Signal{Name: "大盘", Type: "bearish", Desc: fmt.Sprintf("震荡偏弱(%.2f%%)", ind.IndexChange)})
			} else {
				signals = append(signals, Signal{Name: "大盘", Type: "neutral", Desc: fmt.Sprintf("震荡(%.2f%%)", ind.IndexChange)})
			}
		}

		// 相对强度信号
		if ind.RelativeStrength > 2 {
			signals = append(signals, Signal{Name: "相对强度", Type: "bullish", Desc: fmt.Sprintf("强于大盘(+%.2f%%)", ind.RelativeStrength)})
		} else if ind.RelativeStrength < -2 {
			signals = append(signals, Signal{Name: "相对强度", Type: "bearish", Desc: fmt.Sprintf("弱于大盘(%.2f%%)", ind.RelativeStrength)})
		} else {
			signals = append(signals, Signal{Name: "相对强度", Type: "neutral", Desc: fmt.Sprintf("跟随大盘(%.2f%%)", ind.RelativeStrength)})
		}

		// Beta系数信号
		if ind.Beta > 1.2 {
			signals = append(signals, Signal{Name: "Beta", Type: "neutral", Desc: fmt.Sprintf("高波动(%.2f)", ind.Beta)})
		} else if ind.Beta < 0.8 {
			signals = append(signals, Signal{Name: "Beta", Type: "neutral", Desc: fmt.Sprintf("低波动(%.2f)", ind.Beta)})
		} else {
			signals = append(signals, Signal{Name: "Beta", Type: "neutral", Desc: fmt.Sprintf("跟随大盘(%.2f)", ind.Beta)})
		}
	}

	return signals
}

// minSlice 求最小值
func minSlice(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	min := data[0]
	for _, v := range data {
		if v < min {
			min = v
		}
	}
	return min
}

// maxSlice 求最大值
func maxSlice(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	max := data[0]
	for _, v := range data {
		if v > max {
			max = v
		}
	}
	return max
}

// GetIndicators 获取股票技术指标
func GetIndicators(code string) (*Indicators, error) {
	return GetIndicatorsWithRefresh(code, false)
}

func GetIndicatorsWithRefresh(code string, forceRefresh bool) (*Indicators, error) {
	kline, err := GetKlineWithRefresh(code, "daily", forceRefresh)
	if err != nil {
		return nil, err
	}

	return CalculateIndicators(kline.Data)
}

// detectPriceVolumeDiv 检测量价背离
func detectPriceVolumeDiv(prices, volumes []float64) string {
	if len(prices) < 5 || len(volumes) < 5 {
		return "neutral"
	}

	// 取最近5天的数据计算趋势
	recentPrices := prices[len(prices)-5:]
	recentVolumes := volumes[len(volumes)-5:]

	// 计算价格和成交量的斜率（简单线性回归）
	priceSlope := calculateSlope(recentPrices)
	volumeSlope := calculateSlope(recentVolumes)

	// 设定阈值来判断趋势
	priceThreshold := 0.01  // 价格趋势阈值（1%）
	volumeThreshold := 0.05 // 成交量趋势阈值（5%）

	// 价涨量跌 = 顶背离（看跌信号）
	if priceSlope > priceThreshold && volumeSlope < -volumeThreshold {
		return "bearish_divergence"
	}
	// 价跌量涨 = 底背离（看涨信号）
	if priceSlope < -priceThreshold && volumeSlope > volumeThreshold {
		return "bullish_divergence"
	}
	// 价涨量涨 = 健康上涨
	if priceSlope > priceThreshold && volumeSlope > volumeThreshold {
		return "healthy_uptrend"
	}
	// 价跌量跌 = 健康下跌
	if priceSlope < -priceThreshold && volumeSlope < -volumeThreshold {
		return "healthy_downtrend"
	}

	return "neutral"
}

// calculateSlope 计算数据的斜率（简单线性回归）
func calculateSlope(data []float64) float64 {
	n := len(data)
	if n < 2 {
		return 0
	}

	// 计算x和y的平均值
	var sumX, sumY, sumXY, sumX2 float64
	for i, y := range data {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	avgY := sumY / float64(n)

	// 计算斜率 slope = (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	numerator := float64(n)*sumXY - sumX*sumY
	denominator := float64(n)*sumX2 - sumX*sumX

	if denominator == 0 {
		return 0
	}

	slope := numerator / denominator

	// 将斜率标准化为相对变化率
	if avgY != 0 {
		return slope / avgY
	}

	return 0
}

// calculateVolumeStrength 计算成交量强度
func calculateVolumeStrength(volumes []float64) float64 {
	if len(volumes) < 10 {
		return 1.0
	}

	// 取最近10天的成交量
	recent := volumes[len(volumes)-10:]
	currentVolume := volumes[len(volumes)-1]

	// 计算平均成交量和标准差
	var sum, sumSquares float64
	for _, v := range recent {
		sum += v
		sumSquares += v * v
	}

	mean := sum / float64(len(recent))
	variance := (sumSquares / float64(len(recent))) - (mean * mean)
	stdDev := math.Sqrt(variance)

	if stdDev == 0 {
		return 1.0
	}

	// 计算Z-score，表示当前成交量相对于历史的强度
	zScore := (currentVolume - mean) / stdDev

	// 将Z-score转换为0-3的强度值
	strength := math.Max(0, math.Min(3, 1+zScore*0.5))

	return strength
}

// calculateDynamicRSIThresholds 计算动态RSI阈值
func calculateDynamicRSIThresholds(closes []float64) (upper, lower float64) {
	if len(closes) < 30 {
		return 70, 30 // 默认阈值
	}

	// 计算最近30天的RSI值
	rsiValues := make([]float64, 0)
	for i := 14; i < len(closes) && i < 30; i++ {
		rsi := calculateRSI(closes[:i+1], 14)
		rsiValues = append(rsiValues, rsi)
	}

	if len(rsiValues) < 10 {
		return 70, 30
	}

	// 计算RSI的统计特征
	var sum, sumSquares float64
	for _, rsi := range rsiValues {
		sum += rsi
		sumSquares += rsi * rsi
	}

	mean := sum / float64(len(rsiValues))
	variance := (sumSquares / float64(len(rsiValues))) - (mean * mean)
	stdDev := math.Sqrt(variance)

	// 根据历史RSI分布动态调整阈值
	// 使用1.5倍标准差作为阈值调整
	upper = math.Min(85, mean+1.5*stdDev)
	lower = math.Max(15, mean-1.5*stdDev)

	// 确保阈值在合理范围内
	if upper < 60 {
		upper = 70
	}
	if lower > 40 {
		lower = 30
	}

	return upper, lower
}

// analyzeMarketEnvironment 分析市场环境
func analyzeMarketEnvironment(closes, volumes []float64) (trend string, volatility, trendStrength float64) {
	if len(closes) < 20 {
		return "sideways", 0.1, 0.5
	}

	// 计算价格波动率（最近20天的标准差）
	recent := closes[len(closes)-20:]
	var sum, sumSquares float64
	for _, price := range recent {
		sum += price
		sumSquares += price * price
	}
	mean := sum / float64(len(recent))
	variance := (sumSquares / float64(len(recent))) - (mean * mean)
	volatility = math.Sqrt(variance) / mean // 相对波动率

	// 计算趋势方向和强度
	// 使用线性回归分析最近20天的价格趋势
	slope := calculateSlope(recent)

	// 计算趋势强度（R²相关系数）
	trendStrength = calculateTrendStrength(recent)

	// 根据斜率和强度判断市场趋势
	slopeThreshold := 0.02 // 2%的趋势阈值
	strengthThreshold := 0.3 // 趋势强度阈值

	if math.Abs(slope) > slopeThreshold && trendStrength > strengthThreshold {
		if slope > 0 {
			trend = "bull"
		} else {
			trend = "bear"
		}
	} else {
		trend = "sideways"
	}

	return trend, volatility, trendStrength
}

// calculateTrendStrength 计算趋势强度（R²相关系数）
func calculateTrendStrength(data []float64) float64 {
	n := len(data)
	if n < 3 {
		return 0
	}

	// 计算线性回归的R²
	var sumX, sumY, sumXY, sumX2, sumY2 float64
	for i, y := range data {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
		sumY2 += y * y
	}

	// 计算相关系数
	numerator := float64(n)*sumXY - sumX*sumY
	denominatorX := float64(n)*sumX2 - sumX*sumX
	denominatorY := float64(n)*sumY2 - sumY*sumY

	if denominatorX <= 0 || denominatorY <= 0 {
		return 0
	}

	correlation := numerator / math.Sqrt(denominatorX*denominatorY)

	// R² = correlation²
	rSquared := correlation * correlation

	return rSquared
}

// detectBollingerBreakout 检测布林带突破
func detectBollingerBreakout(closes []float64, bollUpper, bollLower, currentPrice float64) string {
	if len(closes) < 3 {
		return "none"
	}

	// 检查当前价格是否突破布林带
	if currentPrice > bollUpper {
		// 检查是否是有效突破（连续2天在上轨上方）
		if len(closes) >= 2 && closes[len(closes)-2] > bollUpper {
			return "upper_breakout"
		}
		return "upper_touch"
	} else if currentPrice < bollLower {
		// 检查是否是有效突破（连续2天在下轨下方）
		if len(closes) >= 2 && closes[len(closes)-2] < bollLower {
			return "lower_breakout"
		}
		return "lower_touch"
	}

	return "none"
}

// detectVolumeBreakout 检测成交量突破
func detectVolumeBreakout(volumes []float64, volumeRatio float64) bool {
	if len(volumes) < 5 {
		return false
	}

	// 成交量突破条件：
	// 1. 量比 > 2.0（当日成交量是5日均量的2倍以上）
	// 2. 连续放量（最近3天成交量递增）
	if volumeRatio > 2.0 {
		recent := volumes[len(volumes)-3:]
		if len(recent) >= 3 {
			// 检查是否连续放量
			if recent[2] > recent[1] && recent[1] > recent[0] {
				return true
			}
		}
		return true // 单日放量也算突破
	}

	return false
}

// calculatePriceAcceleration 计算价格加速度
func calculatePriceAcceleration(closes []float64) float64 {
	if len(closes) < 5 {
		return 0
	}

	// 计算最近5天的价格变化率的变化率（二阶导数）
	recent := closes[len(closes)-5:]

	// 计算每日涨跌幅
	changes := make([]float64, len(recent)-1)
	for i := 1; i < len(recent); i++ {
		changes[i-1] = (recent[i] - recent[i-1]) / recent[i-1] * 100
	}

	if len(changes) < 3 {
		return 0
	}

	// 计算涨跌幅的变化率（加速度）
	var acceleration float64
	for i := 1; i < len(changes); i++ {
		acceleration += changes[i] - changes[i-1]
	}

	return acceleration / float64(len(changes)-1)
}

// calculateMomentumScore 计算综合动量评分
func calculateMomentumScore(ind *Indicators) float64 {
	score := 0.0

	// 1. 价格动量（30%权重）
	if ind.Change5D > 10 {
		score += 30 // 5日涨幅超过10%
	} else if ind.Change5D > 5 {
		score += 20
	} else if ind.Change5D > 0 {
		score += 10
	}

	// 2. 成交量动量（25%权重）
	if ind.VolumeBreakout {
		score += 25
	} else if ind.VolumeRatio > 1.5 {
		score += 15
	} else if ind.VolumeRatio > 1.0 {
		score += 10
	}

	// 3. 技术指标动量（25%权重）
	if ind.RSI > 50 && ind.MACD > ind.Signal {
		score += 25
	} else if ind.RSI > 50 || ind.MACD > ind.Signal {
		score += 15
	}

	// 4. 突破信号（20%权重）
	switch ind.BollBreakout {
	case "upper_breakout":
		score += 20
	case "upper_touch":
		score += 10
	case "lower_breakout":
		score -= 20
	case "lower_touch":
		score -= 10
	}

	// 5. 价格加速度加成
	if ind.PriceAccel > 2 {
		score += 10 // 加速上涨
	} else if ind.PriceAccel < -2 {
		score -= 10 // 加速下跌
	}

	// 确保评分在0-100范围内
	if score < 0 {
		score = 0
	} else if score > 100 {
		score = 100
	}

	return score
}

// calculateAmplitude 计算振幅
func calculateAmplitude(data []KlineData) (current, avg5d float64) {
	if len(data) == 0 {
		return 0, 0
	}

	// 当日振幅 = (最高价 - 最低价) / 昨收 * 100
	n := len(data)
	lastDay := data[n-1]
	if n >= 2 {
		prevClose := data[n-2].Close
		if prevClose > 0 {
			current = (lastDay.High - lastDay.Low) / prevClose * 100
		}
	}

	// 5日平均振幅
	if n >= 5 {
		sum := 0.0
		for i := n - 5; i < n; i++ {
			if i > 0 && data[i-1].Close > 0 {
				amp := (data[i].High - data[i].Low) / data[i-1].Close * 100
				sum += amp
			}
		}
		avg5d = sum / 5.0
	}

	return current, avg5d
}

// calculateShadowRatios 计算上下影线比率
func calculateShadowRatios(data []KlineData) (upper, lower float64) {
	if len(data) == 0 {
		return 0, 0
	}

	lastDay := data[len(data)-1]
	bodyHigh := math.Max(lastDay.Open, lastDay.Close)
	bodyLow := math.Min(lastDay.Open, lastDay.Close)
	totalRange := lastDay.High - lastDay.Low

	if totalRange > 0 {
		// 上影线比率 = 上影线长度 / 总振幅
		upper = (lastDay.High - bodyHigh) / totalRange
		// 下影线比率 = 下影线长度 / 总振幅
		lower = (bodyLow - lastDay.Low) / totalRange
	}

	return upper, lower
}

// calculateContinuousDays 计算连续涨跌天数
func calculateContinuousDays(closes []float64) int {
	if len(closes) < 2 {
		return 0
	}

	n := len(closes)
	count := 0
	lastChange := closes[n-1] - closes[n-2]

	// 从最后一天往前遍历
	for i := n - 1; i > 0; i-- {
		change := closes[i] - closes[i-1]
		// 判断涨跌方向是否一致
		if (lastChange > 0 && change > 0) || (lastChange < 0 && change < 0) {
			if lastChange > 0 {
				count++
			} else {
				count--
			}
		} else {
			break
		}
	}

	return count
}

// calculateSentiment 计算情绪指标
func calculateSentiment(ind *Indicators, data []KlineData) (strength float64, sentimentType string) {
	if len(data) == 0 {
		return 50, "neutral"
	}

	score := 0.0

	// 1. 振幅因子（30%权重）- 振幅越大情绪越强
	if ind.Amplitude > ind.AvgAmplitude5D*2 {
		score += 30 // 异常放大
	} else if ind.Amplitude > ind.AvgAmplitude5D*1.5 {
		score += 20
	} else if ind.Amplitude > ind.AvgAmplitude5D {
		score += 10
	}

	// 2. 连续涨跌因子（25%权重）
	absDays := math.Abs(float64(ind.ContinuousDays))
	if absDays >= 5 {
		score += 25 // 连续5天以上，情绪极端
	} else if absDays >= 3 {
		score += 15
	} else if absDays >= 2 {
		score += 10
	}

	// 3. 影线因子（20%权重）- 长上影线或长下影线表示情绪分歧
	maxShadow := math.Max(ind.UpperShadowRatio, ind.LowerShadowRatio)
	if maxShadow > 0.4 {
		score += 20 // 长影线，情绪分歧大
	} else if maxShadow > 0.3 {
		score += 10
	}

	// 4. 成交量因子（25%权重）
	if ind.VolumeRatio > 3.0 {
		score += 25 // 巨量，情绪极端
	} else if ind.VolumeRatio > 2.0 {
		score += 15
	} else if ind.VolumeRatio > 1.5 {
		score += 10
	}

	strength = math.Min(100, score)

	// 判断情绪类型
	lastDay := data[len(data)-1]
	changePercent := (lastDay.Close - lastDay.Open) / lastDay.Open * 100

	// 恐慌：大跌+巨量+长下影线
	if changePercent < -5 && ind.VolumeRatio > 2.0 && ind.LowerShadowRatio > 0.3 {
		sentimentType = "panic"
	} else if changePercent > 7 && ind.VolumeRatio > 2.5 && ind.Amplitude > ind.AvgAmplitude5D*1.5 {
		// 狂热：大涨+巨量+大振幅
		sentimentType = "frenzy"
	} else if ind.ContinuousDays >= 3 || changePercent > 3 {
		sentimentType = "bullish"
	} else if ind.ContinuousDays <= -3 || changePercent < -3 {
		sentimentType = "bearish"
	} else {
		sentimentType = "neutral"
	}

	return strength, sentimentType
}

// calculateMainForceCost 计算主力成本（历史低价法）
// 理论依据：主力建仓在底部区域，成本接近历史低价
// cost20 ≈ 过去90天最低价 × 1.15（短期主力成本，假设在底部上方15%建仓）
// cost60 ≈ 过去180天最低价 × 1.18（中长期主力成本，假设在底部上方18%建仓）
func calculateMainForceCost(data []KlineData) (cost20, cost60 float64) {
	n := len(data)
	if n == 0 {
		return 0, 0
	}

	// 短期主力成本：过去90天最低价 × 1.15
	period20 := min(90, n)
	if period20 >= 20 {
		lows := make([]float64, period20)
		for i := 0; i < period20; i++ {
			lows[i] = data[n-period20+i].Low
		}
		minLow := minSlice(lows)
		cost20 = minLow * 1.15 // 假设主力在底部上方15%建仓
	} else {
		// 数据太少，使用当前价的80%作为估计
		cost20 = data[n-1].Close * 0.8
	}

	// 中长期主力成本：过去180天最低价 × 1.18
	period60 := min(180, n)
	if period60 >= 60 {
		lows := make([]float64, period60)
		for i := 0; i < period60; i++ {
			lows[i] = data[n-period60+i].Low
		}
		minLow := minSlice(lows)
		cost60 = minLow * 1.18 // 假设主力在底部上方18%建仓
	} else if period60 >= 20 {
		// 数据不足180天，使用短期成本
		cost60 = cost20
	} else {
		// 数据太少，使用当前价的75%作为估计
		cost60 = data[n-1].Close * 0.75
	}

	return cost20, cost60
}

// calculateCostDeviation 计算成本偏离度
func calculateCostDeviation(currentPrice, cost float64) float64 {
	if cost == 0 {
		return 0
	}
	return (currentPrice - cost) / cost * 100
}

// calculateChipConcentration 计算筹码集中度
func calculateChipConcentration(data []KlineData) float64 {
	n := len(data)
	if n < 20 {
		return 0.5 // 默认中等集中度
	}

	// 使用最近20天的价格分布计算集中度
	period := min(60, n)
	prices := make([]float64, period)
	volumes := make([]float64, period)

	for i := 0; i < period; i++ {
		idx := n - period + i
		// 使用典型价格
		prices[i] = (data[idx].High + data[idx].Low + data[idx].Close) / 3
		volumes[i] = data[idx].Volume
	}

	// 计算成交量加权的价格标准差
	var sumPV, sumV float64
	for i := 0; i < period; i++ {
		sumPV += prices[i] * volumes[i]
		sumV += volumes[i]
	}
	avgPrice := sumPV / sumV

	var sumSquares float64
	for i := 0; i < period; i++ {
		diff := prices[i] - avgPrice
		sumSquares += diff * diff * volumes[i]
	}
	stdDev := math.Sqrt(sumSquares / sumV)

	// 标准差越小，筹码越集中
	// 将标准差转换为0-1的集中度（标准差相对于均价的比例）
	relativeStdDev := stdDev / avgPrice
	concentration := 1.0 / (1.0 + relativeStdDev*10) // 归一化到0-1

	return concentration
}

// CalculateIndicatorsWithIndex 计算包含大盘影响的技术指标
func CalculateIndicatorsWithIndex(stockCode string, data []KlineData) (*Indicators, error) {
	// 先计算基础指标
	ind, err := CalculateIndicators(data)
	if err != nil {
		return nil, err
	}

	// 获取大盘数据
	indexKline, err := GetIndexKline(stockCode, "daily")
	if err != nil {
		log.Printf("获取大盘数据失败 (股票: %s): %v", stockCode, err)
		return ind, nil
	}
	if indexKline == nil || len(indexKline.Data) == 0 {
		log.Printf("大盘数据为空 (股票: %s)", stockCode)
		return ind, nil
	}

	// 计算大盘影响指标
	calculateIndexInfluence(ind, data, indexKline.Data, indexKline.Code)
	log.Printf("大盘影响计算完成 (股票: %s, 指数: %s, 涨跌: %.2f%%)", stockCode, ind.IndexCode, ind.IndexChange)

	// 重新生成信号（包含大盘影响信号）
	ind.Signals = generateSignals(ind)

	return ind, nil
}

// calculateIndexInfluence 计算大盘影响指标
func calculateIndexInfluence(ind *Indicators, stockData []KlineData, indexData []KlineData, indexCode string) {
	if len(stockData) == 0 || len(indexData) == 0 {
		return
	}

	ind.IndexCode = indexCode

	// 1. 计算大盘当日涨跌幅
	if len(indexData) >= 2 {
		lastIndex := indexData[len(indexData)-1]
		prevIndex := indexData[len(indexData)-2]
		if prevIndex.Close > 0 {
			ind.IndexChange = (lastIndex.Close - prevIndex.Close) / prevIndex.Close * 100
		}
	}

	// 2. 计算大盘趋势（使用MA5和MA20）
	if len(indexData) >= 20 {
		indexCloses := make([]float64, len(indexData))
		for i, d := range indexData {
			indexCloses[i] = d.Close
		}
		indexMA5 := calculateMA(indexCloses, 5)
		indexMA20 := calculateMA(indexCloses, 20)

		if indexMA5 > indexMA20*1.02 {
			ind.IndexTrend = "bull" // 牛市
		} else if indexMA5 < indexMA20*0.98 {
			ind.IndexTrend = "bear" // 熊市
		} else {
			ind.IndexTrend = "sideways" // 震荡
		}
	}

	// 3. 计算相对强度（个股涨幅 - 大盘涨幅）
	if len(stockData) >= 2 && ind.IndexChange != 0 {
		lastStock := stockData[len(stockData)-1]
		prevStock := stockData[len(stockData)-2]
		if prevStock.Close > 0 {
			stockChange := (lastStock.Close - prevStock.Close) / prevStock.Close * 100
			ind.RelativeStrength = stockChange - ind.IndexChange
		}
	}

	// 4. 计算Beta系数（个股与大盘的相关性）
	ind.Beta = calculateBeta(stockData, indexData)
	ind.FollowIndex = ind.Beta > 0.8
}

// calculateBeta 计算Beta系数
func calculateBeta(stockData []KlineData, indexData []KlineData) float64 {
	// 使用最近20天的数据计算Beta
	period := min(20, min(len(stockData), len(indexData)))
	if period < 10 {
		return 1.0 // 数据不足，返回默认值
	}

	// 计算收益率
	stockReturns := make([]float64, period-1)
	indexReturns := make([]float64, period-1)

	for i := 1; i < period; i++ {
		stockIdx := len(stockData) - period + i
		indexIdx := len(indexData) - period + i

		if stockData[stockIdx-1].Close > 0 {
			stockReturns[i-1] = (stockData[stockIdx].Close - stockData[stockIdx-1].Close) / stockData[stockIdx-1].Close
		}
		if indexData[indexIdx-1].Close > 0 {
			indexReturns[i-1] = (indexData[indexIdx].Close - indexData[indexIdx-1].Close) / indexData[indexIdx-1].Close
		}
	}

	// 计算协方差和方差
	var stockMean, indexMean float64
	for i := 0; i < len(stockReturns); i++ {
		stockMean += stockReturns[i]
		indexMean += indexReturns[i]
	}
	stockMean /= float64(len(stockReturns))
	indexMean /= float64(len(indexReturns))

	var covariance, indexVariance float64
	for i := 0; i < len(stockReturns); i++ {
		stockDiff := stockReturns[i] - stockMean
		indexDiff := indexReturns[i] - indexMean
		covariance += stockDiff * indexDiff
		indexVariance += indexDiff * indexDiff
	}

	if indexVariance == 0 {
		return 1.0
	}

	beta := covariance / indexVariance
	return beta
}
