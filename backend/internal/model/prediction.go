package model

// PredictRequest 预测请求
type PredictRequest struct {
	StockCodes []string `json:"stock_codes" binding:"required"`
	Period     string   `json:"period"` // daily, weekly, monthly
}

// DailyChange 每日涨跌幅
type DailyChange struct {
	Date   string  `json:"date"`   // 日期
	Change float64 `json:"change"` // 涨跌幅(%)
	Close  float64 `json:"close"`  // 收盘价
}

// PredictResult 预测结果
type PredictResult struct {
	StockCode       string              `json:"stock_code"`
	StockName       string              `json:"stock_name"`
	Sector          string              `json:"sector"`   // 板块
	Industry        string              `json:"industry"` // 主营业务行业
	CurrentPrice    float64             `json:"current_price"`
	Trend           string              `json:"trend"`            // up, down, sideways
	TrendCN         string              `json:"trend_cn"`         // 看涨, 看跌, 震荡
	Confidence      float64             `json:"confidence"`       // 0-100
	PriceRange      PriceRange          `json:"price_range"`      // 预测价格区间
	TargetPrices    TargetPrices        `json:"target_prices"`    // 目标价位
	SupportLevel    float64             `json:"support_level"`    // 支撑位
	ResistanceLevel float64             `json:"resistance_level"` // 压力位
	Indicators      TechnicalIndicators `json:"indicators"`       // 技术指标
	Signals         []Signal            `json:"signals"`          // 技术信号
	Analysis        string              `json:"analysis"`         // AI分析
	MLPredictions   MLPredictions       `json:"ml_predictions"`   // ML模型预测
	DailyChanges    []DailyChange       `json:"daily_changes"`    // 近期每日涨跌幅
}

// PriceRange 价格区间
type PriceRange struct {
	Low  float64 `json:"low"`
	High float64 `json:"high"`
}

// TargetPrices 目标价位
type TargetPrices struct {
	Short  float64 `json:"short"`  // 短期(5日)
	Medium float64 `json:"medium"` // 中期(20日)
	Long   float64 `json:"long"`   // 长期(60日)
}

// Signal 技术信号
type Signal struct {
	Name   string `json:"name"`    // 指标名称
	Type   string `json:"type"`    // bullish, bearish, neutral
	TypeCN string `json:"type_cn"` // 看涨, 看跌, 中性
	Desc   string `json:"desc"`    // 描述
}

// MLPredictions ML模型预测结果
type MLPredictions struct {
	LSTM    MLPrediction `json:"lstm"`
	Prophet MLPrediction `json:"prophet"`
	XGBoost MLPrediction `json:"xgboost"`
}

// MLPrediction 单个ML模型预测
type MLPrediction struct {
	Trend      string  `json:"trend"`
	Price      float64 `json:"price"`
	Confidence float64 `json:"confidence"`
}

// PredictResponse 预测响应
type PredictResponse struct {
	Results []PredictResult `json:"results"`
}
