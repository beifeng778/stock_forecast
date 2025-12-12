import React, { useState, useMemo } from 'react';
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
import { simulateTrade } from '../../services/api';
import type { TradeSimulateResponse, ScenarioResult } from '../../types';
import './index.css';

// 判断是否为工作日（周一到周五）
const isWorkday = (date: Dayjs): boolean => {
  const day = date.day();
  return day !== 0 && day !== 6; // 0是周日，6是周六
};

// 获取从今天开始的未来N个工作日
const getNextWorkdays = (count: number): Dayjs[] => {
  const workdays: Dayjs[] = [];
  let current = dayjs().add(1, 'day'); // 从明天开始
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
  const { predictions, predictedCodes } = useStockStore();
  const [result, setResult] = useState<TradeSimulateResponse | null>(null);
  const [loading, setLoading] = useState(false);

  // 计算未来5个工作日的最后一天
  const allowedWorkdays = useMemo(() => getNextWorkdays(5), []);
  const maxSellDate = allowedWorkdays[allowedWorkdays.length - 1];

  // 禁用非工作日和超出范围的日期
  const disabledDate = (current: Dayjs): boolean => {
    if (!current) return false;
    const today = dayjs().startOf('day');
    // 禁用今天及之前的日期
    if (current.isBefore(today) || current.isSame(today, 'day')) return true;
    // 禁用超过最大卖出日期的日期
    if (current.isAfter(maxSellDate, 'day')) return true;
    // 禁用周末
    return !isWorkday(current);
  };

  // 获取可选股票列表
  const stockOptions = predictions.map((p) => ({
    value: p.stock_code,
    label: `${p.stock_code} ${p.stock_name}`,
  }));

  // 计算盈亏
  const handleCalculate = async (values: any) => {
    // 获取选中股票的预测结果
    const prediction = predictions.find((p) => p.stock_code === values.stock_code);
    if (!prediction) {
      message.error('未找到该股票的预测结果');
      return;
    }

    setLoading(true);
    try {
      // 中性情况：使用 target_prices.short（5日目标价），与预测结果卡片一致
      const predictedMidPrice = prediction.target_prices.short;
      // 悲观情况：使用支撑位
      const predictedLowPrice = prediction.support_level;

      const response = await simulateTrade({
        stock_code: values.stock_code,
        buy_price: values.buy_price,
        buy_date: values.buy_date.format('YYYY-MM-DD'),
        expected_price: values.expected_price,
        predicted_mid_price: predictedMidPrice,
        predicted_low_price: predictedLowPrice,
        confidence: prediction.confidence,
        trend: prediction.trend,
        sell_date: values.sell_date.format('YYYY-MM-DD'),
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

  // 选择股票时自动填充当前价格
  const handleStockChange = (code: string) => {
    const prediction = predictions.find((p) => p.stock_code === code);
    if (prediction) {
      form.setFieldsValue({
        buy_price: prediction.current_price,
        expected_price: prediction.target_prices.short,
      });
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
            buy_date: dayjs(),
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
              >
                <DatePicker style={{ width: '100%' }} />
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
                tooltip="仅可选择未来5个工作日内"
              >
                <DatePicker
                  style={{ width: '100%' }}
                  disabledDate={disabledDate}
                  placeholder="选择工作日"
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

            <div className="scenarios-title">三种场景分析</div>
            <div className="scenarios-grid">
              {renderScenario(
                result.optimistic,
                '乐观情况 (预期价格)',
                <SmileOutlined />,
                '#10b981'
              )}
              {renderScenario(
                result.neutral,
                '中性情况 (目标价5日)',
                <MehOutlined />,
                '#6366f1'
              )}
              {renderScenario(
                result.pessimistic,
                '悲观情况 (支撑位)',
                <FrownOutlined />,
                '#ef4444'
              )}
            </div>

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
