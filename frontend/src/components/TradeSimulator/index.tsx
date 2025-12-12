import React, { useState, useMemo, useRef } from 'react';
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
} from 'antd';
import { ArrowUpOutlined, ArrowDownOutlined, SmileOutlined, MehOutlined, FrownOutlined } from '@ant-design/icons';
import dayjs, { Dayjs } from 'dayjs';
import { useStockStore } from '../../store';
import { simulateTrade, getKline } from '../../services/api';
import type { TradeSimulateResponse, ScenarioResult, KlineData } from '../../types';
import './index.css';

// 判断是否为工作日（周一到周五）
const isWorkday = (date: Dayjs): boolean => {
  const day = date.day();
  return day !== 0 && day !== 6; // 0是周日，6是周六
};

// 判断A股是否已收盘（15:00收盘）
const isMarketClosed = (): boolean => {
  const now = dayjs();
  const closeTime = now.hour() * 100 + now.minute();
  return closeTime >= 1500;
};

// 判断日期是否为未来（考虑收盘时间）
const isFutureDate = (date: Dayjs): boolean => {
  const today = dayjs().startOf('day');
  if (date.isAfter(today, 'day')) return true;
  if (date.isSame(today, 'day') && !isMarketClosed()) return true;
  return false;
};

// 获取从今天开始的未来N个工作日
const getNextWorkdays = (count: number): Dayjs[] => {
  const workdays: Dayjs[] = [];
  // 如果今天未收盘且是工作日，今天也算未来
  let current = dayjs();
  if (isMarketClosed() || !isWorkday(current)) {
    current = current.add(1, 'day');
  }
  while (workdays.length < count) {
    if (isWorkday(current)) {
      workdays.push(current);
    }
    current = current.add(1, 'day');
  }
  return workdays;
};

const TradeSimulator: React.FC = () => {
  const [form] = Form.useForm();
  const { predictions, predictedCodes, predictionKlines } = useStockStore();
  const [result, setResult] = useState<TradeSimulateResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [hasFutureDate, setHasFutureDate] = useState(false); // 是否包含未来日期

  // 历史K线数据缓存
  const historyKlinesRef = useRef<Record<string, KlineData[]>>({});

  // 计算未来5个工作日的最后一天
  const allowedWorkdays = useMemo(() => getNextWorkdays(5), []);
  const maxSellDate = allowedWorkdays[allowedWorkdays.length - 1];

  // 禁用日期：过去到未来5个工作日，周末不可选
  const disabledDate = (current: Dayjs): boolean => {
    if (!current) return false;
    if (current.isAfter(maxSellDate, 'day')) return true;
    return !isWorkday(current);
  };

  // 获取可选股票列表
  const stockOptions = predictions.map((p) => ({
    value: p.stock_code,
    label: `${p.stock_code} ${p.stock_name}`,
  }));

  // 计算盈亏
  const handleCalculate = async (values: any) => {
    const prediction = predictions.find((p) => p.stock_code === values.stock_code);
    if (!prediction) {
      message.error('未找到该股票的预测结果');
      return;
    }

    const sellDateStr = values.sell_date.format('YYYY-MM-DD');
    const klines = predictionKlines[values.stock_code] || [];
    const targetKline = klines.find((k) => k.date === sellDateStr);

    // 判断是否包含未来日期
    const buyIsFuture = isFutureDate(values.buy_date);
    const sellIsFuture = isFutureDate(values.sell_date);
    setHasFutureDate(buyIsFuture || sellIsFuture);

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
        buy_date: values.buy_date.format('YYYY-MM-DD'),
        expected_price: values.expected_price,
        predicted_high: predictedHigh,
        predicted_close: predictedClose,
        predicted_low: predictedLow,
        confidence: prediction.confidence,
        trend: prediction.trend,
        sell_date: sellDateStr,
        quantity: values.quantity,
      });
      setResult(response);
    } catch (error) {
      console.error('计算失败:', error);
      message.error('计算失败，请稍后重试');
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
      const response = await getKline(code, 'daily');
      historyKlinesRef.current[code] = response.data || [];
      return historyKlinesRef.current[code];
    } catch (error) {
      console.error('加载历史K线失败:', error);
      return [];
    }
  };

  // 根据日期获取K线价格（同时支持历史和预测K线）
  const getPriceByDate = (code: string, dateStr: string, priceType: 'open' | 'close'): number | null => {
    // 先从预测K线中查找
    const predKlines = predictionKlines[code] || [];
    const predKline = predKlines.find((k) => k.date === dateStr);
    if (predKline) {
      return priceType === 'open' ? predKline.open : predKline.close;
    }

    // 再从历史K线中查找
    const histKlines = historyKlinesRef.current[code] || [];
    const histKline = histKlines.find((k) => k.date === dateStr);
    if (histKline) {
      return priceType === 'open' ? histKline.open : histKline.close;
    }

    return null;
  };

  // 选择股票时自动填充价格
  const handleStockChange = async (code: string) => {
    const prediction = predictions.find((p) => p.stock_code === code);
    if (prediction) {
      // 先加载历史K线数据
      await loadHistoryKlines(code);

      const buyDate = form.getFieldValue('buy_date');
      const sellDate = form.getFieldValue('sell_date');

      let buyPrice = prediction.current_price;
      let expectedPrice = prediction.target_prices.short;

      // 如果已选择买入日期，使用该日期的开盘价
      if (buyDate) {
        const dateStr = buyDate.format('YYYY-MM-DD');
        const price = getPriceByDate(code, dateStr, 'open');
        if (price !== null) buyPrice = price;
      }

      // 如果已选择卖出日期，使用该日期的收盘价
      if (sellDate) {
        const dateStr = sellDate.format('YYYY-MM-DD');
        const price = getPriceByDate(code, dateStr, 'close');
        if (price !== null) expectedPrice = price;
      }

      form.setFieldsValue({
        buy_price: buyPrice,
        expected_price: expectedPrice,
      });
    }
  };

  // 选择买入日期时自动设置买入价格为开盘价
  const handleBuyDateChange = async (date: Dayjs | null) => {
    if (!date) return;
    const code = form.getFieldValue('stock_code');
    if (!code) return;

    // 确保历史K线已加载
    await loadHistoryKlines(code);

    const dateStr = date.format('YYYY-MM-DD');
    const buyPrice = getPriceByDate(code, dateStr, 'open');
    if (buyPrice !== null) {
      form.setFieldsValue({ buy_price: buyPrice });
    }
  };

  // 选择卖出日期时自动设置预期卖出价格为收盘价
  const handleSellDateChange = async (date: Dayjs | null) => {
    if (!date) return;
    const code = form.getFieldValue('stock_code');
    if (!code) return;

    // 确保历史K线已加载
    await loadHistoryKlines(code);

    const dateStr = date.format('YYYY-MM-DD');
    const expectedPrice = getPriceByDate(code, dateStr, 'close');
    if (expectedPrice !== null) {
      form.setFieldsValue({ expected_price: expectedPrice });
    }
  };

  // 渲染场景结果
  const renderScenario = (scenario: ScenarioResult, label: string, icon: React.ReactNode, color: string) => (
    <Card size="small" className="scenario-card" style={{ borderColor: color }}>
      <div className="scenario-header" style={{ color }}>
        {icon}
        <span>{label}</span>
        <span style={{ marginLeft: 'auto', fontSize: 12, opacity: 0.8 }}>概率: {scenario.probability}</span>
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
              color: scenario.profit >= 0 ? '#cf1322' : '#3f8600',
            }}
            prefix={scenario.profit >= 0 ? <ArrowUpOutlined /> : <ArrowDownOutlined />}
            suffix={`(${scenario.profit_rate})`}
          />
        </Col>
      </Row>
      <div className="scenario-fees">
        卖出费用: 佣金 {scenario.fees.sell_commission.toFixed(2)}元 |
        印花税 {scenario.fees.stamp_tax.toFixed(2)}元 |
        过户费 {scenario.fees.transfer_fee.toFixed(2)}元
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
          initialValues={{
            quantity: 100,
            buy_date: allowedWorkdays[0], // 默认第一个工作日
            sell_date: allowedWorkdays[0], // 默认第一个工作日
          }}
        >
          <Form.Item
            name="stock_code"
            label="股票"
            rules={[{ required: true, message: '请选择股票' }]}
          >
            <Select
              placeholder="选择已预测的股票"
              options={stockOptions}
              onChange={handleStockChange}
            />
          </Form.Item>

          <Row gutter={16}>
            <Col span={8}>
              <Form.Item
                name="buy_price"
                label="买入价格"
                rules={[{ required: true, message: '请输入买入价格' }]}
              >
                <InputNumber
                  style={{ width: '100%' }}
                  min={0.01}
                  step={0.01}
                  precision={2}
                  placeholder="买入价格"
                />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item
                name="buy_date"
                label="买入日期"
                rules={[{ required: true, message: '请选择买入日期' }]}
                tooltip="最远可选择未来5个工作日"
              >
                <DatePicker
                  style={{ width: '100%' }}
                  disabledDate={disabledDate}
                  placeholder="选择工作日"
                  onChange={handleBuyDateChange}
                />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item
                name="quantity"
                label="数量(股)"
                rules={[{ required: true, message: '请输入数量' }]}
              >
                <InputNumber
                  style={{ width: '100%' }}
                  min={100}
                  step={100}
                  placeholder="数量"
                />
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col span={8}>
              <Form.Item
                name="expected_price"
                label="预期卖出价格"
                rules={[{ required: true, message: '请输入预期卖出价格' }]}
              >
                <InputNumber
                  style={{ width: '100%' }}
                  min={0.01}
                  step={0.01}
                  precision={2}
                  placeholder="预期卖出价格"
                />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item
                name="sell_date"
                label="卖出日期"
                rules={[{ required: true, message: '请选择卖出日期' }]}
                tooltip="最远可选择未来5个工作日"
              >
                <DatePicker
                  style={{ width: '100%' }}
                  disabledDate={disabledDate}
                  placeholder="选择工作日"
                  onChange={handleSellDateChange}
                />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item label=" ">
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

        {result && (
          <div className="result-section">
            <Card className="result-card" size="small">
              <Row gutter={16}>
                <Col span={8}>
                  <Statistic
                    title="买入成本"
                    value={result.buy_cost}
                    precision={2}
                    suffix="元"
                  />
                </Col>
                <Col span={8}>
                  <Statistic
                    title="预期卖出价格"
                    value={result.expected_price}
                    precision={2}
                    suffix="元/股"
                  />
                </Col>
                <Col span={8}>
                  <Statistic
                    title="买入费用"
                    value={result.buy_fees.total_fees}
                    precision={2}
                    suffix="元"
                  />
                </Col>
              </Row>
              <div style={{ marginTop: 12, fontSize: 12, color: '#e2e8f0' }}>
                买入费用明细: 佣金 {result.buy_fees.buy_commission.toFixed(2)}元 |
                过户费 {result.buy_fees.transfer_fee.toFixed(2)}元
              </div>
            </Card>

            {hasFutureDate ? (
              <>
                <div className="scenarios-title">盈亏分析</div>
                {/* 符合预期独占一行 */}
                <div className="scenarios-row-single">
                  {renderScenario(
                    result.expected,
                    '符合预期',
                    <SmileOutlined />,
                    '#10b981'
                  )}
                </div>
                {/* AI分析三种场景占一行 */}
                <div className="scenarios-row-triple">
                  {renderScenario(
                    result.conservative,
                    '保守',
                    <FrownOutlined />,
                    '#ef4444'
                  )}
                  {renderScenario(
                    result.moderate,
                    '中等',
                    <MehOutlined />,
                    '#6366f1'
                  )}
                  {renderScenario(
                    result.aggressive,
                    '激进',
                    <ArrowUpOutlined />,
                    '#f59e0b'
                  )}
                </div>
              </>
            ) : (
              <>
                <div className="scenarios-title">盈亏分析</div>
                <div className="scenarios-row-single">
                  {renderScenario(
                    result.expected,
                    '预期卖出',
                    <SmileOutlined />,
                    '#10b981'
                  )}
                </div>
              </>
            )}

            <div style={{ marginTop: 12, fontSize: 12, color: '#94a3b8' }}>
              费率说明: 佣金0.025%(最低5元) | 印花税0.05%(仅卖出) | 过户费0.001%(仅沪市)
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default TradeSimulator;
