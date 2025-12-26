package model

// Stock 股票基本信息
type Stock struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Market string `json:"market"` // SH: 上海, SZ: 深圳
}

// KlineData K线数据
type KlineData struct {
	Date   string  `json:"date"`
	Open   float64 `json:"open"`
	Close  float64 `json:"close"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Volume float64 `json:"volume"`
	Amount float64 `json:"amount"`
}

// KlineResponse K线响应
type KlineResponse struct {
	Code   string      `json:"code"`
	Name   string      `json:"name"`
	Period string      `json:"period"`
	Data   []KlineData `json:"data"`
}

// TechnicalIndicators 技术指标
type TechnicalIndicators struct {
	MA5    float64 `json:"ma5"`
	MA10   float64 `json:"ma10"`
	MA20   float64 `json:"ma20"`
	MA60   float64 `json:"ma60"`
	MACD   float64 `json:"macd"`
	Signal float64 `json:"signal"`
	Hist   float64 `json:"hist"`
	RSI    float64 `json:"rsi"`
	KDJ_K  float64 `json:"kdj_k"`
	KDJ_D  float64 `json:"kdj_d"`
	KDJ_J  float64 `json:"kdj_j"`
	BOLL_U float64 `json:"boll_upper"`
	BOLL_M float64 `json:"boll_middle"`
	BOLL_L float64 `json:"boll_lower"`
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
