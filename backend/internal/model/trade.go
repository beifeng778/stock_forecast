package model

// TradeSimulateRequest 委托模拟请求
type TradeSimulateRequest struct {
	StockCode         string  `json:"stock_code" binding:"required"`
	BuyPrice          float64 `json:"buy_price" binding:"required"`
	BuyDate           string  `json:"buy_date" binding:"required"`
	ExpectedPrice     float64 `json:"expected_price" binding:"required"`      // 预期卖出价格（乐观情况）
	PredictedMidPrice float64 `json:"predicted_mid_price" binding:"required"` // 预测中位数价格（中性情况）
	PredictedLowPrice float64 `json:"predicted_low_price" binding:"required"` // 预测最低价格（悲观情况）
	Confidence        float64 `json:"confidence" binding:"required"`          // 预测置信度 (0-1)
	Trend             string  `json:"trend" binding:"required"`               // 预测趋势 (up/down/sideways)
	SellDate          string  `json:"sell_date" binding:"required"`
	Quantity          int     `json:"quantity" binding:"required"`
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
	BuyCost       float64        `json:"buy_cost"`       // 买入成本
	BuyFees       TradeFees      `json:"buy_fees"`       // 买入费用
	Optimistic    ScenarioResult `json:"optimistic"`     // 乐观情况（最高点）
	Neutral       ScenarioResult `json:"neutral"`        // 中性情况（中位数）
	Pessimistic   ScenarioResult `json:"pessimistic"`    // 悲观情况（最低点）
}
