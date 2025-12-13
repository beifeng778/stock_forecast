import React, { useState, useMemo, useRef, useEffect } from "react";
import {
  Form,
  Select,
  InputNumber,
  DatePicker,
  Button,
  Card,
  Statistic,
  Row,
  Col,
  Descriptions,
  message,
  Empty,
  Tabs,
} from "antd";
import {
  ArrowUpOutlined,
  ArrowDownOutlined,
  SmileOutlined,
  MehOutlined,
  FrownOutlined,
} from "@ant-design/icons";
import dayjs, { Dayjs } from "dayjs";
import { useStockStore } from "../../store";
import { simulateTrade, getKline } from "../../services/api";
import type { ScenarioResult, KlineData } from "../../types";
import "./index.css";

// 判断是否为工作日（周一到周五）
const isWorkday = (date: Dayjs): boolean => {
  const day = date.day();
  return day !== 0 && day !== 6; // 0是周日，6是周六
};

// 判断股票板块类型
const getStockBoard = (code: string): "科创板" | "创业板" | "主板" => {
  if (code.startsWith("688")) return "科创板";
  if (code.startsWith("300") || code.startsWith("301")) return "创业板";
  return "主板";
};

// 获取最小买入股数
const getMinQuantity = (code: string): number => {
  return getStockBoard(code) === "科创板" ? 200 : 100;
};

// 获取股数步进值
const getQuantityStep = (code: string): number => {
  return getStockBoard(code) === "科创板" ? 1 : 100;
};

// 验证股数是否合法
const validateQuantity = (code: string, quantity: number): string | null => {
  const board = getStockBoard(code);
  const minQty = getMinQuantity(code);

  if (quantity < minQty) {
    return `${board}最少买入${minQty}股`;
  }

  if (board === "科创板") {
    // 科创板：首次200股，之后可以1股递增
    if (quantity < 200) {
      return "科创板最少买入200股";
    }
  } else {
    // 主板/创业板：必须是100的整数倍
    if (quantity % 100 !== 0) {
      return "股数必须是100的整数倍";
    }
  }
  return null;
};

// 判断A股是否已收盘（15:00收盘）
const isMarketClosed = (): boolean => {
  const now = dayjs();
  const closeTime = now.hour() * 100 + now.minute();
  return closeTime >= 1500;
};

// 判断日期是否为未来（考虑收盘时间）
const isFutureDate = (date: Dayjs): boolean => {
  const today = dayjs().startOf("day");
  if (date.isAfter(today, "day")) return true;
  if (date.isSame(today, "day") && !isMarketClosed()) return true;
  return false;
};

// 获取从今天开始的未来N个工作日
const getNextWorkdays = (count: number): Dayjs[] => {
  const workdays: Dayjs[] = [];
  // 如果今天未收盘且是工作日，今天也算未来
  let current = dayjs();
  if (isMarketClosed() || !isWorkday(current)) {
    current = current.add(1, "day");
  }
  while (workdays.length < count) {
    if (isWorkday(current)) {
      workdays.push(current);
    }
    current = current.add(1, "day");
  }
  return workdays;
};

const TradeSimulator: React.FC = () => {
  const [form] = Form.useForm();
  const {
    predictions,
    predictedCodes,
    predictionKlines,
    tradeFormData,
    setTradeFormData,
    tradeResult,
    setTradeResult,
    tradeHasFutureDate,
    setTradeHasFutureDate,
  } = useStockStore();
  const [loading, setLoading] = useState(false);
  const [buyDateOpen, setBuyDateOpen] = useState(false);
  const [sellDateOpen, setSellDateOpen] = useState(false);

  // 历史K线数据缓存
  const historyKlinesRef = useRef<Record<string, KlineData[]>>({});

  // 恢复保存的表单数据
  useEffect(() => {
    // 只有当保存的股票代码在当前预测列表中时才恢复股票选择
    const stockCodeValid =
      tradeFormData.stock_code &&
      predictions.some((p) => p.stock_code === tradeFormData.stock_code);

    // 恢复所有保存的表单数据
    const fieldsToRestore: Record<string, any> = {};

    if (stockCodeValid) {
      fieldsToRestore.stock_code = tradeFormData.stock_code;
    }
    if (tradeFormData.buy_price !== undefined) {
      fieldsToRestore.buy_price = tradeFormData.buy_price;
    }
    if (tradeFormData.expected_price !== undefined) {
      fieldsToRestore.expected_price = tradeFormData.expected_price;
    }

    if (Object.keys(fieldsToRestore).length > 0) {
      form.setFieldsValue(fieldsToRestore);
    }
  }, [predictions]);

  // 计算未来5个工作日的最后一天
  const allowedWorkdays = useMemo(() => getNextWorkdays(5), []);
  const maxSellDate = allowedWorkdays[allowedWorkdays.length - 1];

  // 禁用日期：过去到未来5个工作日，周末不可选
  const disabledDate = (current: Dayjs): boolean => {
    if (!current) return false;
    if (current.isAfter(maxSellDate, "day")) return true;
    return !isWorkday(current);
  };

  // 获取可选股票列表（过滤掉名称为"未知"的股票）
  const stockOptions = predictions
    .filter((p) => p.stock_name && p.stock_name !== "未知")
    .map((p) => ({
      value: p.stock_code,
      label: `${p.stock_code} ${p.stock_name}`,
    }));

  // 计算盈亏
  const handleCalculate = async (values: any) => {
    // 校验买入日期和卖出日期不能同一天
    if (values.buy_date.isSame(values.sell_date, "day")) {
      message.error("买入日期和卖出日期不能是同一天");
      return;
    }

    const prediction = predictions.find(
      (p) => p.stock_code === values.stock_code
    );
    if (!prediction) {
      message.error("未找到该股票的预测结果");
      return;
    }

    const sellDateStr = values.sell_date.format("YYYY-MM-DD");
    const klines = predictionKlines[values.stock_code] || [];
    const targetKline = klines.find((k) => k.date === sellDateStr);

    // 判断是否包含未来日期
    const buyIsFuture = isFutureDate(values.buy_date);
    const sellIsFuture = isFutureDate(values.sell_date);
    setTradeHasFutureDate(buyIsFuture || sellIsFuture);

    // 从预测K线获取对应日期的价格
    let predictedHigh: number;
    let predictedClose: number;
    let predictedLow: number;

    if (targetKline) {
      predictedHigh = targetKline.high;
      predictedClose = targetKline.close;
      predictedLow = targetKline.low;
    } else {
      // 如果没有找到对应日期的K线，使用默认值
      predictedHigh = prediction.resistance_level;
      predictedClose = prediction.target_prices.short;
      predictedLow = prediction.support_level;
    }

    setLoading(true);
    try {
      const response = await simulateTrade({
        stock_code: values.stock_code,
        buy_price: values.buy_price,
        buy_date: values.buy_date.format("YYYY-MM-DD"),
        expected_price: values.expected_price,
        predicted_high: predictedHigh,
        predicted_close: predictedClose,
        predicted_low: predictedLow,
        confidence: prediction.confidence,
        trend: prediction.trend,
        sell_date: sellDateStr,
        quantity: values.quantity,
      });
      setTradeResult(response);
    } catch (error) {
      console.error("计算失败:", error);
      message.error("计算失败，请稍后重试");
    } finally {
      setLoading(false);
    }
  };

  // 加载历史K线数据
  const loadHistoryKlines = async (code: string) => {
    if (historyKlinesRef.current[code]) {
      return historyKlinesRef.current[code];
    }
    try {
      const response = await getKline(code, "daily");
      historyKlinesRef.current[code] = response.data || [];
      return historyKlinesRef.current[code];
    } catch (error) {
      console.error("加载历史K线失败:", error);
      return [];
    }
  };

  // 根据日期获取K线价格（同时支持历史和预测K线）
  const getPriceByDate = (
    code: string,
    dateStr: string,
    priceType: "open" | "close"
  ): number | null => {
    // 先从预测K线中查找
    const predKlines = predictionKlines[code] || [];
    const predKline = predKlines.find((k) => k.date === dateStr);
    if (predKline) {
      return priceType === "open" ? predKline.open : predKline.close;
    }

    // 再从历史K线中查找
    const histKlines = historyKlinesRef.current[code] || [];
    const histKline = histKlines.find((k) => k.date === dateStr);
    if (histKline) {
      return priceType === "open" ? histKline.open : histKline.close;
    }

    return null;
  };

  // 选择股票时自动填充价格
  const handleStockChange = async (code: string) => {
    const prediction = predictions.find((p) => p.stock_code === code);
    if (prediction) {
      // 先加载历史K线数据
      await loadHistoryKlines(code);

      const buyDate = form.getFieldValue("buy_date");
      const sellDate = form.getFieldValue("sell_date");

      let buyPrice = prediction.current_price;
      let expectedPrice = prediction.target_prices.short;

      // 如果已选择买入日期，使用该日期的开盘价
      if (buyDate) {
        const dateStr = buyDate.format("YYYY-MM-DD");
        const price = getPriceByDate(code, dateStr, "open");
        if (price !== null) buyPrice = price;
      }

      // 如果已选择卖出日期，使用该日期的收盘价
      if (sellDate) {
        const dateStr = sellDate.format("YYYY-MM-DD");
        const price = getPriceByDate(code, dateStr, "close");
        if (price !== null) expectedPrice = price;
      }

      // 根据板块设置默认股数
      const currentQty = form.getFieldValue("quantity");
      const minQty = getMinQuantity(code);
      const step = getQuantityStep(code);

      // 如果当前股数不符合新板块规则，重置为最小值
      let newQty = currentQty;
      if (currentQty < minQty) {
        newQty = minQty;
      } else if (step === 100 && currentQty % 100 !== 0) {
        // 主板/创业板需要100的整数倍
        newQty = Math.ceil(currentQty / 100) * 100;
      }

      form.setFieldsValue({
        buy_price: buyPrice,
        expected_price: expectedPrice,
        quantity: newQty,
      });
    }
  };

  // 选择买入日期时自动设置买入价格为开盘价
  const handleBuyDateChange = async (date: Dayjs | null) => {
    if (!date) return;
    const code = form.getFieldValue("stock_code");
    if (!code) return;

    // 确保历史K线已加载
    await loadHistoryKlines(code);

    const dateStr = date.format("YYYY-MM-DD");
    const buyPrice = getPriceByDate(code, dateStr, "open");
    if (buyPrice !== null) {
      form.setFieldsValue({ buy_price: buyPrice });
    }
  };

  // 选择卖出日期时自动设置预期卖出价格为收盘价
  const handleSellDateChange = async (date: Dayjs | null) => {
    if (!date) return;
    const code = form.getFieldValue("stock_code");
    if (!code) return;

    // 确保历史K线已加载
    await loadHistoryKlines(code);

    const dateStr = date.format("YYYY-MM-DD");
    const expectedPrice = getPriceByDate(code, dateStr, "close");
    if (expectedPrice !== null) {
      form.setFieldsValue({ expected_price: expectedPrice });
    }
  };

  // 渲染场景结果
  const renderScenario = (
    scenario: ScenarioResult,
    label: string,
    icon: React.ReactNode,
    color: string,
    showProbability: boolean = true
  ) => (
    <Card size="small" className="scenario-card" style={{ borderColor: color }}>
      <div className="scenario-header" style={{ color }}>
        {icon}
        <span>{label}</span>
        {showProbability && (
          <span style={{ marginLeft: "auto", fontSize: 12, opacity: 0.8 }}>
            概率: {scenario.probability}
          </span>
        )}
      </div>
      <Row gutter={8}>
        <Col span={8}>
          <Statistic
            title="卖出价格"
            value={scenario.sell_price}
            precision={2}
            suffix="元"
            valueStyle={{ fontSize: 16 }}
          />
        </Col>
        <Col span={8}>
          <Statistic
            title="卖出收入"
            value={scenario.sell_income}
            precision={2}
            suffix="元"
            valueStyle={{ fontSize: 16 }}
          />
        </Col>
        <Col span={8}>
          <Statistic
            title="盈亏"
            value={scenario.profit}
            precision={2}
            valueStyle={{
              fontSize: 16,
              color: scenario.profit >= 0 ? "#cf1322" : "#3f8600",
            }}
            prefix={
              scenario.profit >= 0 ? <ArrowUpOutlined /> : <ArrowDownOutlined />
            }
            suffix={`(${scenario.profit_rate})`}
          />
        </Col>
      </Row>
      <div className="scenario-fees">
        卖出费用: 佣金 {scenario.fees.sell_commission.toFixed(2)}元 | 印花税{" "}
        {scenario.fees.stamp_tax.toFixed(2)}元 | 过户费{" "}
        {scenario.fees.transfer_fee.toFixed(2)}元
      </div>
    </Card>
  );

  if (predictedCodes.length === 0) {
    return (
      <div className="trade-simulator">
        <div className="simulator-header">
          <h3>委托盈亏模拟</h3>
        </div>
        <Empty description="请先进行股票预测，然后才能模拟委托" />
      </div>
    );
  }

  return (
    <div className="trade-simulator">
      <div className="simulator-header">
        <h3>委托盈亏模拟</h3>
        <span className="tip">仅可选择已预测的股票</span>
      </div>

      <div className="simulator-content">
        <Form
          form={form}
          layout="vertical"
          onFinish={handleCalculate}
          onValuesChange={(_, allValues) => {
            // 保存表单数据到 store（日期转为字符串）
            setTradeFormData({
              stock_code: allValues.stock_code,
              buy_price: allValues.buy_price,
              buy_date: allValues.buy_date?.format("YYYY-MM-DD"),
              expected_price: allValues.expected_price,
              sell_date: allValues.sell_date?.format("YYYY-MM-DD"),
              quantity: allValues.quantity,
            });
          }}
          initialValues={{
            quantity: tradeFormData.quantity || 100,
            buy_date: tradeFormData.buy_date
              ? dayjs(tradeFormData.buy_date)
              : allowedWorkdays[0],
            sell_date: tradeFormData.sell_date
              ? dayjs(tradeFormData.sell_date)
              : allowedWorkdays[0],
          }}
        >
          <Form.Item
            name="stock_code"
            label="股票"
            rules={[{ required: true, message: "请选择股票" }]}
          >
            <Select
              placeholder="选择已预测的股票"
              options={stockOptions}
              onChange={handleStockChange}
            />
          </Form.Item>

          <Row gutter={16}>
            <Col xs={12} sm={8}>
              <Form.Item
                name="buy_price"
                label="买入价格"
                rules={[{ required: true, message: "请输入买入价格" }]}
              >
                <InputNumber
                  style={{ width: "100%" }}
                  min={0.01}
                  step={0.01}
                  precision={2}
                  placeholder="买入价格"
                />
              </Form.Item>
            </Col>
            <Col xs={12} sm={8}>
              <Form.Item
                name="buy_date"
                label="买入日期"
                rules={[{ required: true, message: "请选择买入日期" }]}
                tooltip="最远可选择未来5个工作日"
              >
                <DatePicker
                  style={{ width: "100%" }}
                  disabledDate={disabledDate}
                  placeholder="选择工作日"
                  onChange={(date) => {
                    handleBuyDateChange(date);
                    setBuyDateOpen(false);
                  }}
                  inputReadOnly
                  open={buyDateOpen}
                  onOpenChange={setBuyDateOpen}
                  showToday={false}
                  renderExtraFooter={() => (
                    <div
                      style={{
                        display: "flex",
                        justifyContent: "space-between",
                        padding: "8px 12px",
                      }}
                    >
                      <Button
                        size="small"
                        onClick={() => {
                          const today = dayjs();
                          if (!disabledDate(today)) {
                            handleBuyDateChange(today);
                            form.setFieldValue("buy_date", today);
                          }
                          setBuyDateOpen(false);
                        }}
                      >
                        今天
                      </Button>
                      <Button
                        size="small"
                        onClick={() => setBuyDateOpen(false)}
                      >
                        关闭
                      </Button>
                    </div>
                  )}
                />
              </Form.Item>
            </Col>
            <Col xs={12} sm={8}>
              <Form.Item
                name="quantity"
                label="数量(股)"
                rules={[
                  { required: true, message: "请输入数量" },
                  {
                    validator: (_, value) => {
                      const code = form.getFieldValue("stock_code");
                      if (!code) return Promise.resolve();
                      const error = validateQuantity(code, value);
                      if (error) return Promise.reject(error);
                      return Promise.resolve();
                    },
                  },
                ]}
              >
                <InputNumber
                  style={{ width: "100%" }}
                  min={getMinQuantity(form.getFieldValue("stock_code") || "")}
                  step={getQuantityStep(form.getFieldValue("stock_code") || "")}
                  placeholder="数量"
                />
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col xs={12} sm={8}>
              <Form.Item
                name="expected_price"
                label="预期卖出价格"
                rules={[{ required: true, message: "请输入预期卖出价格" }]}
              >
                <InputNumber
                  style={{ width: "100%" }}
                  min={0.01}
                  step={0.01}
                  precision={2}
                  placeholder="预期卖出价格"
                />
              </Form.Item>
            </Col>
            <Col xs={12} sm={8}>
              <Form.Item
                name="sell_date"
                label="卖出日期"
                rules={[{ required: true, message: "请选择卖出日期" }]}
                tooltip="最远可选择未来5个工作日"
              >
                <DatePicker
                  style={{ width: "100%" }}
                  disabledDate={disabledDate}
                  placeholder="选择工作日"
                  onChange={(date) => {
                    handleSellDateChange(date);
                    setSellDateOpen(false);
                  }}
                  inputReadOnly
                  open={sellDateOpen}
                  onOpenChange={setSellDateOpen}
                  showToday={false}
                  renderExtraFooter={() => (
                    <div
                      style={{
                        display: "flex",
                        justifyContent: "space-between",
                        padding: "8px 12px",
                      }}
                    >
                      <Button
                        size="small"
                        onClick={() => {
                          const today = dayjs();
                          if (!disabledDate(today)) {
                            handleSellDateChange(today);
                            form.setFieldValue("sell_date", today);
                          }
                          setSellDateOpen(false);
                        }}
                      >
                        今天
                      </Button>
                      <Button
                        size="small"
                        onClick={() => setSellDateOpen(false)}
                      >
                        关闭
                      </Button>
                    </div>
                  )}
                />
              </Form.Item>
            </Col>
            <Col xs={24} sm={8}>
              <Form.Item label=" " className="submit-btn-item">
                <Button
                  type="primary"
                  htmlType="submit"
                  loading={loading}
                  block
                >
                  计算盈亏
                </Button>
              </Form.Item>
            </Col>
          </Row>
        </Form>

        {tradeResult && (
          <div className="result-section">
            <Card className="result-card">
              <Row gutter={[24, 16]} justify="space-around">
                <Col xs={24} sm={8}>
                  <div className="stat-item">
                    <div className="stat-label">买入成本</div>
                    <div className="stat-value">
                      {tradeResult.buy_cost.toLocaleString("zh-CN", {
                        minimumFractionDigits: 2,
                        maximumFractionDigits: 2,
                      })}
                      <span className="stat-unit">元</span>
                    </div>
                  </div>
                </Col>
                <Col xs={12} sm={8}>
                  <div className="stat-item">
                    <div className="stat-label">预期卖出价格</div>
                    <div className="stat-value">
                      {tradeResult.expected_price.toFixed(2)}
                      <span className="stat-unit">元/股</span>
                    </div>
                  </div>
                </Col>
                <Col xs={12} sm={8}>
                  <div className="stat-item">
                    <div className="stat-label">买入费用</div>
                    <div className="stat-value">
                      {tradeResult.buy_fees.total_fees.toFixed(2)}
                      <span className="stat-unit">元</span>
                    </div>
                  </div>
                </Col>
              </Row>
              <div className="fees-detail">
                买入费用明细: 佣金{" "}
                {tradeResult.buy_fees.buy_commission.toFixed(2)}元 | 过户费{" "}
                {tradeResult.buy_fees.transfer_fee.toFixed(2)}元
              </div>
            </Card>

            {tradeHasFutureDate ? (
              <>
                <div className="scenarios-title">盈亏分析</div>
                {/* 符合预期独占一行 */}
                <div className="scenarios-row-single">
                  {renderScenario(
                    tradeResult.expected,
                    "符合预期",
                    <SmileOutlined />,
                    "#10b981"
                  )}
                </div>
                {/* AI分析三种场景占一行 */}
                <div className="scenarios-row-triple">
                  {renderScenario(
                    tradeResult.conservative,
                    "保守",
                    <FrownOutlined />,
                    "#ef4444"
                  )}
                  {renderScenario(
                    tradeResult.moderate,
                    "中等",
                    <MehOutlined />,
                    "#6366f1"
                  )}
                  {renderScenario(
                    tradeResult.aggressive,
                    "激进",
                    <ArrowUpOutlined />,
                    "#f59e0b"
                  )}
                </div>
              </>
            ) : (
              <>
                <div className="scenarios-title">盈亏分析</div>
                <div className="scenarios-row-single">
                  {renderScenario(
                    tradeResult.expected,
                    "预期卖出",
                    <SmileOutlined />,
                    "#10b981",
                    false
                  )}
                </div>
              </>
            )}

            <div style={{ marginTop: 12, fontSize: 12, color: "#94a3b8" }}>
              费率说明: 佣金0.025%(最低5元) | 印花税0.05%(仅卖出) |
              过户费0.001%(仅沪市)
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default TradeSimulator;
