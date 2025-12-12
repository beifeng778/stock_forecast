import React, { useEffect, useState } from 'react';
import ReactECharts from 'echarts-for-react';
import { Radio, Spin, Empty, Select, Space } from 'antd';
import { useStockStore } from '../../store';
import { getKline } from '../../services/api';
import type { KlineData, PeriodType } from '../../types';
import './index.css';

const TrendChart: React.FC = () => {
  const { selectedStocks, period, setPeriod, predictions } = useStockStore();
  const [currentStock, setCurrentStock] = useState<string>('');
  const [klineData, setKlineData] = useState<KlineData[]>([]);
  const [stockName, setStockName] = useState<string>('');
  const [loading, setLoading] = useState(false);

  // 当选中股票变化时，自动选择第一只
  useEffect(() => {
    if (selectedStocks.length > 0) {
      const exists = selectedStocks.some(s => s.code === currentStock);
      if (!currentStock || !exists) {
        setCurrentStock(selectedStocks[0].code);
      }
    } else {
      setCurrentStock('');
      setKlineData([]);
      setStockName('');
    }
  }, [selectedStocks, currentStock]);

  // 加载K线数据
  useEffect(() => {
    const loadData = async () => {
      if (!currentStock) {
        setKlineData([]);
        return;
      }

      setLoading(true);
      try {
        const kline = await getKline(currentStock, period);
        setKlineData(kline.data || []);
        setStockName(kline.name || '');
      } catch (error) {
        console.error('加载K线数据失败:', error);
        setKlineData([]);
      } finally {
        setLoading(false);
      }
    };

    loadData();
  }, [currentStock, period]);

  // 预测K线数据类型
  interface PredictionKline {
    date: string;
    open: number;
    close: number;
    high: number;
    low: number;
    volume: number;
  }

  // 生成未来5日预测数据（完整K线格式）
  // 使用 target_prices.short 作为5日目标价，与预测结果卡片保持一致
  const generatePredictionData = () => {
    const prediction = predictions.find(p => p.stock_code === currentStock);
    if (!prediction || klineData.length === 0) return null;

    const lastKline = klineData[klineData.length - 1];
    const lastPrice = lastKline?.close || 0;
    // 使用 target_prices.short 作为5日目标价格，与预测结果卡片一致
    const targetPrice = prediction.target_prices.short;
    // 支撑位和压力位用于计算高低价范围
    const supportLevel = prediction.support_level;
    const resistanceLevel = prediction.resistance_level;
    const confidence = prediction.confidence;

    // 计算历史波动率（用于生成合理的高低价）
    const recentData = klineData.slice(-20);
    let avgVolatility = 0;
    let avgVolume = 0;
    recentData.forEach(d => {
      avgVolatility += (d.high - d.low) / d.close;
      avgVolume += d.volume;
    });
    avgVolatility = avgVolatility / recentData.length;
    avgVolume = avgVolume / recentData.length;

    const predictionKlines: PredictionKline[] = [];
    const dates: string[] = [];

    // 生成未来5个工作日的日期和预测K线数据
    const lastDate = new Date(klineData[klineData.length - 1].date);
    let currentDate = new Date(lastDate);
    let prevClose = lastPrice;

    for (let i = 1; i <= 5; i++) {
      currentDate.setDate(currentDate.getDate() + 1);
      // 跳过周末
      while (currentDate.getDay() === 0 || currentDate.getDay() === 6) {
        currentDate.setDate(currentDate.getDate() + 1);
      }
      const dateStr = currentDate.toISOString().split('T')[0];
      dates.push(dateStr);

      // 线性插值计算目标收盘价
      const targetClose = lastPrice + (targetPrice - lastPrice) * (i / 5);

      // 根据置信度调整波动幅度（置信度越高，波动越小）
      const volatilityFactor = avgVolatility * (1.2 - confidence * 0.4);

      // 生成开盘价（基于前一天收盘价，有小幅跳空）
      const gapFactor = (Math.random() - 0.5) * 0.01; // ±0.5% 跳空
      const open = prevClose * (1 + gapFactor);

      // 收盘价
      const close = targetClose;

      // 计算日内波动范围，基于支撑位和压力位
      const priceRangeSpread = resistanceLevel - supportLevel;
      const dayRange = Math.max(close * volatilityFactor, priceRangeSpread * 0.1);

      // 根据趋势方向调整高低价
      const isUp = close > open;
      let high, low;
      if (isUp) {
        high = Math.max(open, close) + dayRange * 0.3;
        low = Math.min(open, close) - dayRange * 0.2;
      } else {
        high = Math.max(open, close) + dayRange * 0.2;
        low = Math.min(open, close) - dayRange * 0.3;
      }

      // 确保 high/low 在支撑位和压力位范围内波动
      high = Math.min(Math.max(high, open, close), resistanceLevel);
      low = Math.max(Math.min(low, open, close), supportLevel);

      // 预测成交量（基于历史平均，有一定波动）
      const volumeFactor = 0.8 + Math.random() * 0.4; // 80%-120%
      const volume = Math.round(avgVolume * volumeFactor);

      predictionKlines.push({
        date: dateStr,
        open: parseFloat(open.toFixed(2)),
        close: parseFloat(close.toFixed(2)),
        high: parseFloat(high.toFixed(2)),
        low: parseFloat(low.toFixed(2)),
        volume,
      });

      prevClose = close;
    }

    return { dates, klines: predictionKlines };
  };

  // 生成图表配置
  const getOption = () => {
    if (klineData.length === 0) {
      return {};
    }

    const dates = klineData.map(d => d.date);
    const prices = klineData.map(d => d.close);

    // 获取预测数据
    const predictionData = generatePredictionData();
    const allDates = predictionData ? [...dates, ...predictionData.dates] : dates;

    // 计算默认显示范围：最近5个工作日 + 预测5天
    const totalDays = allDates.length;
    const displayDays = predictionData ? 10 : 5; // 有预测时显示10天，否则显示5天
    const startPercent = Math.max(0, ((totalDays - displayDays) / totalDays) * 100);

    const series: any[] = [
      {
        name: '历史价格',
        type: 'line',
        data: prices,
        smooth: true,
        symbol: 'none',
        lineStyle: {
          width: 2,
          color: '#6366f1',
        },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(99, 102, 241, 0.4)' },
              { offset: 1, color: 'rgba(99, 102, 241, 0.05)' },
            ],
          },
        },
      },
    ];

    // 添加预测线
    if (predictionData) {
      // 构建预测收盘价数据（前面填充null，从最后一个历史数据点开始）
      const predictionPrices: (number | null)[] = [];
      for (let i = 0; i < klineData.length - 1; i++) {
        predictionPrices.push(null);
      }
      // 最后一个历史数据点作为预测起点
      predictionPrices.push(klineData[klineData.length - 1]?.close || 0);
      // 添加预测数据
      predictionData.klines.forEach(k => predictionPrices.push(k.close));

      series.push({
        name: 'AI预测(5日)',
        type: 'line',
        data: predictionPrices,
        smooth: true,
        symbol: 'circle',
        symbolSize: 6,
        lineStyle: {
          width: 2,
          color: '#f59e0b',
          type: 'dashed',
        },
        itemStyle: {
          color: '#f59e0b',
        },
      });
    }

    return {
      title: {
        text: `${currentStock} ${stockName}`,
        left: 'center',
        top: 5,
        textStyle: {
          fontSize: 14,
          fontWeight: 'normal',
          color: '#e2e8f0',
        },
      },
      legend: {
        show: predictionData !== null,
        top: 5,
        right: 10,
        textStyle: {
          color: '#e2e8f0',
          fontSize: 11,
        },
      },
      tooltip: {
        trigger: 'axis',
        backgroundColor: 'rgba(30, 41, 59, 0.95)',
        borderColor: 'rgba(99, 102, 241, 0.3)',
        textStyle: {
          color: '#e2e8f0',
        },
        formatter: (params: any) => {
          const idx = params[0]?.dataIndex;
          if (idx < klineData.length) {
            const data = klineData[idx];
            if (!data) return '';
            return `
              <div style="font-size:12px">
                <div style="font-weight:bold;margin-bottom:4px">${data.date}</div>
                <div>开盘: ${data.open?.toFixed(2) || '-'}</div>
                <div>收盘: ${data.close?.toFixed(2) || '-'}</div>
                <div>最高: ${data.high?.toFixed(2) || '-'}</div>
                <div>最低: ${data.low?.toFixed(2) || '-'}</div>
                <div>成交量: ${data.volume ? (data.volume / 10000).toFixed(0) + '万' : '-'}</div>
              </div>
            `;
          } else {
            // 预测数据
            const predIdx = idx - klineData.length;
            const predKline = predictionData?.klines[predIdx];
            if (!predKline) return '';
            const pred = predictions.find(p => p.stock_code === currentStock);
            return `
              <div style="font-size:12px">
                <div style="font-weight:bold;margin-bottom:4px;color:#f59e0b">AI预测 ${predKline.date}</div>
                <div>开盘: ${predKline.open.toFixed(2)}</div>
                <div>收盘: ${predKline.close.toFixed(2)}</div>
                <div>最高: ${predKline.high.toFixed(2)}</div>
                <div>最低: ${predKline.low.toFixed(2)}</div>
                <div>成交量: ${(predKline.volume / 10000).toFixed(0)}万</div>
                ${pred ? `<div style="margin-top:4px;padding-top:4px;border-top:1px solid rgba(255,255,255,0.2);color:#94a3b8;font-size:11px">目标价(5日): ${pred.target_prices.short.toFixed(2)} | 支撑: ${pred.support_level.toFixed(2)} | 压力: ${pred.resistance_level.toFixed(2)}</div>` : ''}
              </div>
            `;
          }
        },
      },
      grid: {
        left: '3%',
        right: '4%',
        bottom: '15%',
        top: 45,
        containLabel: true,
      },
      xAxis: {
        type: 'category',
        data: allDates,
        axisLabel: {
          rotate: 45,
          fontSize: 10,
          color: '#e2e8f0',
        },
        axisLine: {
          lineStyle: {
            color: 'rgba(148, 163, 184, 0.3)',
          },
        },
      },
      yAxis: {
        type: 'value',
        scale: true,
        axisLabel: {
          formatter: '{value}',
          color: '#e2e8f0',
        },
        axisLine: {
          lineStyle: {
            color: 'rgba(148, 163, 184, 0.3)',
          },
        },
        splitLine: {
          lineStyle: {
            type: 'dashed',
            color: 'rgba(148, 163, 184, 0.15)',
          },
        },
      },
      dataZoom: [
        {
          type: 'inside',
          start: startPercent,
          end: 100,
        },
        {
          type: 'slider',
          start: startPercent,
          end: 100,
          height: 20,
          bottom: 5,
          textStyle: {
            color: '#e2e8f0',
          },
          borderColor: 'rgba(99, 102, 241, 0.3)',
          fillerColor: 'rgba(99, 102, 241, 0.2)',
        },
      ],
      series,
    };
  };

  const periodOptions = [
    { label: '日', value: 'daily' },
    { label: '周', value: 'weekly' },
    { label: '月', value: 'monthly' },
  ];

  const stockOptions = selectedStocks.map(s => ({
    value: s.code,
    label: `${s.code} ${s.name}`,
  }));

  const selectValue = stockOptions.find(o => o.value === currentStock) ? currentStock : undefined;

  return (
    <div className="trend-chart">
      <div className="chart-header">
        <Space>
          <h3>趋势图表</h3>
          {selectedStocks.length > 0 && (
            <Select
              value={selectValue}
              onChange={setCurrentStock}
              options={stockOptions}
              style={{ width: 180 }}
              size="small"
              placeholder="选择股票"
            />
          )}
        </Space>
        <Radio.Group
          options={periodOptions}
          value={period}
          onChange={(e) => setPeriod(e.target.value as PeriodType)}
          optionType="button"
          buttonStyle="solid"
          size="small"
        />
      </div>

      <div className="chart-content">
        {loading ? (
          <div className="chart-loading">
            <Spin tip="加载中..." />
          </div>
        ) : klineData.length === 0 ? (
          <Empty description="请选择股票查看趋势" />
        ) : (
          <ReactECharts
            option={getOption()}
            style={{ height: '100%', width: '100%' }}
            notMerge={true}
          />
        )}
      </div>
    </div>
  );
};

export default TrendChart;
