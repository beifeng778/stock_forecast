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

	// 基于预测价格计算三种场景的卖出价格
	// 乐观：用户预期卖出价格
	optimisticPrice := req.ExpectedPrice
	// 中性：预测中位数价格
	neutralPrice := req.PredictedMidPrice
	// 悲观：预测最低价格
	pessimisticPrice := req.PredictedLowPrice

	// 根据预测置信度和趋势计算三种场景的概率
	optimisticProb, neutralProb, pessimisticProb := calculateProbabilities(req.Confidence, req.Trend, req.BuyPrice, req.ExpectedPrice)

	// 计算三种场景
	optimistic := calculateScenarioWithProb(optimisticPrice, float64(req.Quantity), buyCost, isSH, optimisticProb)
	neutral := calculateScenarioWithProb(neutralPrice, float64(req.Quantity), buyCost, isSH, neutralProb)
	pessimistic := calculateScenarioWithProb(pessimisticPrice, float64(req.Quantity), buyCost, isSH, pessimisticProb)

	return &model.TradeSimulateResponse{
		StockCode:     req.StockCode,
		StockName:     stockName,
		BuyPrice:      req.BuyPrice,
		ExpectedPrice: req.ExpectedPrice,
		Quantity:      req.Quantity,
		BuyCost:       math.Round(buyCost*100) / 100,
		BuyFees:       buyFees,
		Optimistic:    optimistic,
		Neutral:       neutral,
		Pessimistic:   pessimistic,
	}, nil
}

// calculateProbabilities 根据置信度和趋势计算三种场景的概率
// 概率计算逻辑：
// - 如果预测趋势是上涨(up)，乐观概率较高
// - 如果预测趋势是下跌(down)，悲观概率较高
// - 如果预测趋势是震荡(sideways)，中性概率较高
// - 置信度越高，主要趋势的概率越高
func calculateProbabilities(confidence float64, trend string, buyPrice, expectedPrice float64) (optimistic, neutral, pessimistic float64) {
	// 基础概率分布
	baseOptimistic := 0.25
	baseNeutral := 0.50
	basePessimistic := 0.25

	// 根据趋势调整概率
	switch trend {
	case "up":
		// 上涨趋势：乐观概率增加
		optimistic = baseOptimistic + 0.25*confidence
		neutral = baseNeutral - 0.10*confidence
		pessimistic = basePessimistic - 0.15*confidence
	case "down":
		// 下跌趋势：悲观概率增加
		optimistic = baseOptimistic - 0.15*confidence
		neutral = baseNeutral - 0.10*confidence
		pessimistic = basePessimistic + 0.25*confidence
	default: // sideways
		// 震荡趋势：中性概率增加
		optimistic = baseOptimistic - 0.05*confidence
		neutral = baseNeutral + 0.10*confidence
		pessimistic = basePessimistic - 0.05*confidence
	}

	// 根据预期价格与买入价格的差距调整
	// 如果预期涨幅过大，降低乐观概率
	expectedReturn := (expectedPrice - buyPrice) / buyPrice
	if expectedReturn > 0.10 { // 预期涨幅超过10%
		adjustment := math.Min(expectedReturn-0.10, 0.15) // 最多调整15%
		optimistic -= adjustment
		pessimistic += adjustment * 0.5
		neutral += adjustment * 0.5
	}

	// 确保概率在合理范围内
	optimistic = math.Max(0.05, math.Min(0.60, optimistic))
	neutral = math.Max(0.20, math.Min(0.70, neutral))
	pessimistic = math.Max(0.05, math.Min(0.60, pessimistic))

	// 归一化，确保总和为1
	total := optimistic + neutral + pessimistic
	optimistic = optimistic / total
	neutral = neutral / total
	pessimistic = pessimistic / total

	return optimistic, neutral, pessimistic
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
