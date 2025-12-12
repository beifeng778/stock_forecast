package model

// TradeSimulateRequest 委托模拟请求
type TradeSimulateRequest struct {
	StockCode      string  `json:"stock_code" binding:"required"`
	BuyPrice       float64 `json:"buy_price" binding:"required"`
	BuyDate        string  `json:"buy_date" binding:"required"`
	ExpectedPrice  float64 `json:"expected_price" binding:"required"`   // 预期卖出价格
	PredictedHigh  float64 `json:"predicted_high" binding:"required"`   // 预测当日最高价
	PredictedClose float64 `json:"predicted_close" binding:"required"`  // 预测当日收盘价
	PredictedLow   float64 `json:"predicted_low" binding:"required"`    // 预测当日最低价
	Confidence     float64 `json:"confidence" binding:"required"`       // 预测置信度 (0-1)
	Trend          string  `json:"trend" binding:"required"`            // 预测趋势 (up/down/sideways)
	SellDate       string  `json:"sell_date" binding:"required"`
	Quantity       int     `json:"quantity" binding:"required"`
}

// TradeFees 交易费用
type TradeFees struct {
	BuyCommission  float64 `json:"buy_commission"`  // 买入佣金
	SellCommission float64 `json:"sell_commission"` // 卖出佣金
	StampTax       float64 `json:"stamp_tax"`       // 印花税
	TransferFee    float64 `json:"transfer_fee"`    // 过户费
	TotalFees      float64 `json:"total_fees"`      // 总费用
}

// ScenarioResult 单个场景结果
type ScenarioResult struct {
	SellPrice   float64   `json:"sell_price"`   // 卖出价格
	SellIncome  float64   `json:"sell_income"`  // 卖出收入
	Profit      float64   `json:"profit"`       // 盈亏金额
	ProfitRate  string    `json:"profit_rate"`  // 盈亏比例
	Probability string    `json:"probability"`  // 出现概率
	Fees        TradeFees `json:"fees"`         // 费用明细
}

// TradeSimulateResponse 委托模拟响应
type TradeSimulateResponse struct {
	StockCode     string         `json:"stock_code"`
	StockName     string         `json:"stock_name"`
	BuyPrice      float64        `json:"buy_price"`
	ExpectedPrice float64        `json:"expected_price"` // 预期卖出价格
	Quantity      int            `json:"quantity"`
	BuyCost       float64        `json:"buy_cost"`  // 买入成本
	BuyFees       TradeFees      `json:"buy_fees"`  // 买入费用
	Expected      ScenarioResult `json:"expected"`  // 符合预期
	DayHigh       ScenarioResult `json:"day_high"`  // 当日最高价
	DayClose      ScenarioResult `json:"day_close"` // 当日收盘价
	DayLow        ScenarioResult `json:"day_low"`   // 当日最低价
}
