package stockdata

import (
	"fmt"
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
	for i, d := range data {
		closes[i] = d.Close
		highs[i] = d.High
		lows[i] = d.Low
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

	// DIF = EMA12 - EMA26
	dif := make([]float64, len(closes))
	for i := 25; i < len(closes); i++ {
		dif[i] = ema12[i] - ema26[i]
	}

	// DEA = EMA9(DIF)
	dea := calculateEMA(dif[25:], 9)
	if dea == nil || len(dea) == 0 {
		return 0, 0, 0
	}

	macd = dif[len(dif)-1]
	signal = dea[len(dea)-1]
	hist = (macd - signal) * 2

	return macd, signal, hist
}

// calculateRSI 计算RSI
func calculateRSI(closes []float64, period int) float64 {
	if len(closes) < period+1 {
		return 50
	}

	gains := 0.0
	losses := 0.0

	for i := len(closes) - period; i < len(closes); i++ {
		change := closes[i] - closes[i-1]
		if change > 0 {
			gains += change
		} else {
			losses -= change
		}
	}

	if losses == 0 {
		return 100
	}

	rs := gains / losses
	return 100 - (100 / (1 + rs))
}

// calculateKDJ 计算KDJ
func calculateKDJ(highs, lows, closes []float64) (k, d, j float64) {
	period := 9
	if len(closes) < period {
		return 50, 50, 50
	}

	// 计算RSV
	highest := maxSlice(highs[len(highs)-period:])
	lowest := minSlice(lows[len(lows)-period:])
	close := closes[len(closes)-1]

	rsv := 50.0
	if highest != lowest {
		rsv = (close - lowest) / (highest - lowest) * 100
	}

	// 简化计算：K = RSV, D = MA3(K), J = 3K - 2D
	k = rsv
	d = rsv // 简化
	j = 3*k - 2*d

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

	// RSI信号
	if ind.RSI > 70 {
		signals = append(signals, Signal{Name: "RSI", Type: "bearish", Desc: "超买"})
	} else if ind.RSI < 30 {
		signals = append(signals, Signal{Name: "RSI", Type: "bullish", Desc: "超卖"})
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
	kline, err := GetKline(code, "daily")
	if err != nil {
		return nil, err
	}

	return CalculateIndicators(kline.Data)
}
