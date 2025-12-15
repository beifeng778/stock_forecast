// 股票基本信息
export interface Stock {
  code: string;
  name: string;
  market: "SH" | "SZ";
  industry?: string;
}

// K线数据
export interface KlineData {
  date: string;
  open: number;
  close: number;
  high: number;
  low: number;
  volume: number;
  amount: number;
}

// K线响应
export interface KlineResponse {
  code: string;
  name: string;
  period: string;
  data: KlineData[];
}

// 技术指标
export interface TechnicalIndicators {
  ma5: number;
  ma10: number;
  ma20: number;
  ma60: number;
  macd: number;
  signal: number;
  hist: number;
  rsi: number;
  kdj_k: number;
  kdj_d: number;
  kdj_j: number;
  boll_upper: number;
  boll_middle: number;
  boll_lower: number;
}

// 技术信号
export interface Signal {
  name: string;
  type: "bullish" | "bearish" | "neutral";
  type_cn: string;
  desc: string;
}

// ML预测结果
export interface MLPrediction {
  trend: string;
  price: number;
  confidence: number;
}

// ML预测集合
export interface MLPredictions {
  lstm: MLPrediction;
  prophet: MLPrediction;
  xgboost: MLPrediction;
}

// 价格区间
export interface PriceRange {
  low: number;
  high: number;
}

// 目标价位
export interface TargetPrices {
  short: number;
  medium: number;
  long: number;
}

// 每日涨跌幅
export interface DailyChange {
  date: string;
  change: number;
  close: number;
}

// 预测结果
export interface PredictResult {
  stock_code: string;
  stock_name: string;
  sector?: string; // 板块
  industry?: string; // 主营业务行业
  is_intraday?: boolean;
  current_price: number;
  trend: string;
  trend_cn: string;
  confidence: number;
  price_range: PriceRange;
  target_prices: TargetPrices;
  future_klines?: KlineData[];
  ai_today?: KlineData;
  need_predict_today?: boolean;
  support_level: number;
  resistance_level: number;
  indicators: TechnicalIndicators;
  signals: Signal[];
  analysis: string;
  news_analysis?: string; // 消息面分析
  ml_predictions: MLPredictions;
  daily_changes?: DailyChange[]; // 近期每日涨跌幅
}

// 预测请求
export interface PredictRequest {
  stock_codes: string[];
  period: string;
}

// 预测响应
export interface PredictResponse {
  results: PredictResult[];
}

// 交易费用
export interface TradeFees {
  buy_commission: number;
  sell_commission: number;
  stamp_tax: number;
  transfer_fee: number;
  total_fees: number;
}

// 委托模拟请求
export interface TradeSimulateRequest {
  stock_code: string;
  buy_price: number;
  buy_date: string;
  expected_price: number; // 预期卖出价格
  predicted_high: number; // 预测当日最高价
  predicted_close: number; // 预测当日收盘价
  predicted_low: number; // 预测当日最低价
  confidence: number; // 预测置信度 (0-1)
  trend: string; // 预测趋势 (up/down/sideways)
  sell_date: string;
  quantity: number;
}

// 场景结果
export interface ScenarioResult {
  sell_price: number;
  sell_income: number;
  profit: number;
  profit_rate: string;
  probability: string; // 出现概率
  fees: TradeFees;
}

// 委托模拟响应
export interface TradeSimulateResponse {
  stock_code: string;
  stock_name: string;
  buy_price: number;
  expected_price: number;
  quantity: number;
  buy_cost: number;
  buy_fees: TradeFees;
  expected: ScenarioResult; // 符合预期
  conservative: ScenarioResult; // 保守（AI分析）
  moderate: ScenarioResult; // 中等（AI分析）
  aggressive: ScenarioResult; // 激进（AI分析）
}

// 周期类型
export type PeriodType = "daily" | "weekly" | "monthly";
