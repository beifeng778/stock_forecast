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

	// 根据趋势和置信度计算AI分析的三种价格
	conservativePrice, moderatePrice, aggressivePrice := calculateAIPrices(req.BuyPrice, req.PredictedLow, req.PredictedClose, req.PredictedHigh, req.Confidence, req.Trend)

	// 计算四种场景的概率（符合预期、保守、中等、激进）
	// 如果预期价格接近某个场景价格，则合并概率
	expectedProb, conservativeProb, moderateProb, aggressiveProb := calculateProbabilitiesWithMerge(
		req.Confidence, req.Trend, req.ExpectedPrice,
		conservativePrice, moderatePrice, aggressivePrice,
	)

	// 计算四种场景
	expected := calculateScenarioWithProb(req.ExpectedPrice, float64(req.Quantity), buyCost, isSH, expectedProb)
	conservative := calculateScenarioWithProb(conservativePrice, float64(req.Quantity), buyCost, isSH, conservativeProb)
	moderate := calculateScenarioWithProb(moderatePrice, float64(req.Quantity), buyCost, isSH, moderateProb)
	aggressive := calculateScenarioWithProb(aggressivePrice, float64(req.Quantity), buyCost, isSH, aggressiveProb)

	return &model.TradeSimulateResponse{
		StockCode:    req.StockCode,
		StockName:    stockName,
		BuyPrice:     req.BuyPrice,
		ExpectedPrice: req.ExpectedPrice,
		Quantity:     req.Quantity,
		BuyCost:      math.Round(buyCost*100) / 100,
		BuyFees:      buyFees,
		Expected:     expected,
		Conservative: conservative,
		Moderate:     moderate,
		Aggressive:   aggressive,
	}, nil
}

// calculateProbabilitiesWithMerge 根据置信度和趋势计算四种场景的概率
// 保守/中等/激进三个场景概率之和为100%
// 符合预期的概率根据预期价格在价格区间中的位置插值计算
func calculateProbabilitiesWithMerge(confidence float64, trend string, expectedPrice, conservativePrice, moderatePrice, aggressivePrice float64) (expected, conservative, moderate, aggressive float64) {
	// 基础概率：保守25%，中等45%，激进30%
	conservative = 0.25
	moderate = 0.45
	aggressive = 0.30

	// 根据趋势调整
	switch trend {
	case "up":
		aggressive += 0.05 * confidence
		conservative -= 0.05 * confidence
	case "down":
		aggressive -= 0.10 * confidence
		conservative += 0.10 * confidence
	}

	// 确保概率在合理范围内
	conservative = math.Max(0.15, math.Min(0.35, conservative))
	moderate = math.Max(0.35, math.Min(0.55, moderate))
	aggressive = math.Max(0.15, math.Min(0.35, aggressive))

	// 归一化，确保三个场景概率之和为100%
	total := conservative + moderate + aggressive
	conservative = conservative / total
	moderate = moderate / total
	aggressive = aggressive / total

	// 计算符合预期的概率：根据预期价格在区间中的位置插值
	// 价格从低到高：保守 < 中等 < 激进
	if expectedPrice <= conservativePrice {
		// 低于保守价格，概率更低
		expected = conservative * (expectedPrice / conservativePrice)
	} else if expectedPrice <= moderatePrice {
		// 在保守和中等之间，线性插值
		ratio := (expectedPrice - conservativePrice) / (moderatePrice - conservativePrice)
		expected = conservative + ratio*(moderate-conservative)
	} else if expectedPrice <= aggressivePrice {
		// 在中等和激进之间，线性插值
		ratio := (expectedPrice - moderatePrice) / (aggressivePrice - moderatePrice)
		expected = moderate + ratio*(aggressive-moderate)
	} else {
		// 高于激进价格，概率递减
		expected = aggressive * (aggressivePrice / expectedPrice)
	}

	// 确保概率在合理范围内
	expected = math.Max(0.05, math.Min(0.95, expected))

	return expected, conservative, moderate, aggressive
}

// calculateAIPrices 根据AI分析计算保守/中等/激进三种价格
func calculateAIPrices(buyPrice, predictedLow, predictedClose, predictedHigh, confidence float64, trend string) (conservative, moderate, aggressive float64) {
	// 保守：接近最低价，风险最小
	conservative = predictedLow + (predictedClose-predictedLow)*0.2

	// 中等：接近收盘价
	moderate = predictedClose

	// 激进：接近最高价，收益最大但风险也大
	aggressive = predictedClose + (predictedHigh-predictedClose)*0.8

	// 根据趋势微调
	switch trend {
	case "up":
		conservative *= (1 + 0.01*confidence)
		moderate *= (1 + 0.02*confidence)
		aggressive *= (1 + 0.03*confidence)
	case "down":
		conservative *= (1 - 0.02*confidence)
		moderate *= (1 - 0.01*confidence)
		aggressive *= (1 - 0.005*confidence)
	}

	return math.Round(conservative*100) / 100, math.Round(moderate*100) / 100, math.Round(aggressive*100) / 100
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
		Probability: fmt.Sprintf("%.2f%%", probability*100),
		Fees: model.TradeFees{
			BuyCommission:  0,
			SellCommission: math.Round(sellCommission*100) / 100,
			StampTax:       math.Round(stampTax*100) / 100,
			TransferFee:    math.Round(sellTransferFee*100) / 100,
			TotalFees:      math.Round(totalFees*100) / 100,
		},
	}
}
