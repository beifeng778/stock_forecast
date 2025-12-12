package service

import (
	"fmt"
	"math"
	"strings"

	"stock-forecast-backend/internal/client"
	"stock-forecast-backend/internal/model"
)

const (
	// 佣金费率 0.025%，最低5元
	CommissionRate = 0.00025
	MinCommission  = 5.0

	// 印花税 0.05%（仅卖出）
	StampTaxRate = 0.0005

	// 过户费 0.001%（仅沪市）
	TransferFeeRate = 0.00001
)

// SimulateTrade 模拟委托交易计算
func SimulateTrade(req *model.TradeSimulateRequest) (*model.TradeSimulateResponse, error) {
	// 获取股票名称
	stockName, err := client.GetStockName(req.StockCode)
	if err != nil {
		stockName = "未知股票"
	}

	isSH := strings.HasPrefix(req.StockCode, "6")

	// 计算买入金额和费用
	buyAmount := req.BuyPrice * float64(req.Quantity)
	buyCommission := buyAmount * CommissionRate
	if buyCommission < MinCommission {
		buyCommission = MinCommission
	}
	buyTransferFee := 0.0
	if isSH {
		buyTransferFee = buyAmount * TransferFeeRate
	}
	buyCost := buyAmount + buyCommission + buyTransferFee

	buyFees := model.TradeFees{
		BuyCommission:  math.Round(buyCommission*100) / 100,
		SellCommission: 0,
		StampTax:       0,
		TransferFee:    math.Round(buyTransferFee*100) / 100,
		TotalFees:      math.Round((buyCommission+buyTransferFee)*100) / 100,
	}

	// 计算四种场景的概率
	expectedProb, highProb, closeProb, lowProb := calculateProbabilities(req.Confidence, req.Trend, req.BuyPrice, req.ExpectedPrice)

	// 计算四种场景
	expected := calculateScenarioWithProb(req.ExpectedPrice, float64(req.Quantity), buyCost, isSH, expectedProb)
	dayHigh := calculateScenarioWithProb(req.PredictedHigh, float64(req.Quantity), buyCost, isSH, highProb)
	dayClose := calculateScenarioWithProb(req.PredictedClose, float64(req.Quantity), buyCost, isSH, closeProb)
	dayLow := calculateScenarioWithProb(req.PredictedLow, float64(req.Quantity), buyCost, isSH, lowProb)

	return &model.TradeSimulateResponse{
		StockCode:     req.StockCode,
		StockName:     stockName,
		BuyPrice:      req.BuyPrice,
		ExpectedPrice: req.ExpectedPrice,
		Quantity:      req.Quantity,
		BuyCost:       math.Round(buyCost*100) / 100,
		BuyFees:       buyFees,
		Expected:      expected,
		DayHigh:       dayHigh,
		DayClose:      dayClose,
		DayLow:        dayLow,
	}, nil
}

// calculateProbabilities 根据置信度和趋势计算四种场景的概率
// 返回：符合预期、最高价、收盘价、最低价的概率
func calculateProbabilities(confidence float64, trend string, buyPrice, expectedPrice float64) (expected, high, close, low float64) {
	// 基础概率：符合预期15%，最高20%，收盘45%，最低20%
	expected = 0.15
	high = 0.20
	close = 0.45
	low = 0.20

	// 根据趋势调整
	switch trend {
	case "up":
		expected += 0.05 * confidence
		high += 0.05 * confidence
		close -= 0.05 * confidence
		low -= 0.05 * confidence
	case "down":
		expected -= 0.05 * confidence
		high -= 0.05 * confidence
		close -= 0.05 * confidence
		low += 0.15 * confidence
	}

	// 根据预期涨幅调整
	expectedReturn := (expectedPrice - buyPrice) / buyPrice
	if expectedReturn > 0.10 {
		adjustment := math.Min(expectedReturn-0.10, 0.10)
		expected -= adjustment
		close += adjustment
	}

	// 确保概率在合理范围内
	expected = math.Max(0.05, math.Min(0.30, expected))
	high = math.Max(0.10, math.Min(0.30, high))
	close = math.Max(0.30, math.Min(0.60, close))
	low = math.Max(0.10, math.Min(0.30, low))

	// 归一化
	total := expected + high + close + low
	return expected / total, high / total, close / total, low / total
}

// calculateScenarioWithProb 计算单个场景的盈亏（带概率）
func calculateScenarioWithProb(sellPrice float64, quantity float64, buyCost float64, isSH bool, probability float64) model.ScenarioResult {
	sellAmount := sellPrice * quantity

	// 卖出佣金
	sellCommission := sellAmount * CommissionRate
	if sellCommission < MinCommission {
		sellCommission = MinCommission
	}

	// 印花税
	stampTax := sellAmount * StampTaxRate

	// 过户费
	sellTransferFee := 0.0
	if isSH {
		sellTransferFee = sellAmount * TransferFeeRate
	}

	// 卖出收入
	sellIncome := sellAmount - sellCommission - stampTax - sellTransferFee

	// 盈亏
	profit := sellIncome - buyCost
	profitRate := (profit / buyCost) * 100

	totalFees := sellCommission + stampTax + sellTransferFee

	return model.ScenarioResult{
		SellPrice:   math.Round(sellPrice*100) / 100,
		SellIncome:  math.Round(sellIncome*100) / 100,
		Profit:      math.Round(profit*100) / 100,
		ProfitRate:  fmt.Sprintf("%.2f%%", profitRate),
		Probability: fmt.Sprintf("%.0f%%", probability*100),
		Fees: model.TradeFees{
			BuyCommission:  0,
			SellCommission: math.Round(sellCommission*100) / 100,
			StampTax:       math.Round(stampTax*100) / 100,
			TransferFee:    math.Round(sellTransferFee*100) / 100,
			TotalFees:      math.Round(totalFees*100) / 100,
		},
	}
}
