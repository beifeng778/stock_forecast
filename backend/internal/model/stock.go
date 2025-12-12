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
}
