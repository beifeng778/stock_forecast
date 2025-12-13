import React, { useEffect, useState, useMemo } from "react";
import ReactECharts from "echarts-for-react";
import { Spin, Empty, Select, Space } from "antd";
import { useStockStore } from "../../store";
import { getKline } from "../../services/api";
import type { KlineData } from "../../types";
import "./index.css";

// 判断A股是否已收盘（15:00收盘）
const isMarketClosed = (): boolean => {
  const now = new Date();
  const closeTime = now.getHours() * 100 + now.getMinutes();
  return closeTime >= 1500;
};

const TrendChart: React.FC = () => {
  const { selectedStocks, predictions, setPredictionKlines } = useStockStore();
  const [currentStock, setCurrentStock] = useState<string>("");
  const [klineData, setKlineData] = useState<KlineData[]>([]);
  const [stockName, setStockName] = useState<string>("");
  const [loading, setLoading] = useState(false);

  // 当选中股票变化时，自动选择第一只
  useEffect(() => {
    if (selectedStocks.length > 0) {
      const exists = selectedStocks.some((s) => s.code === currentStock);
      if (!currentStock || !exists) {
        setCurrentStock(selectedStocks[0].code);
      }
    } else {
      setCurrentStock("");
      setKlineData([]);
      setStockName("");
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
        const kline = await getKline(currentStock, "daily");
        setKlineData(kline.data || []);
        setStockName(kline.name || "");
      } catch (error) {
        console.error("加载K线数据失败:", error);
        setKlineData([]);
      } finally {
        setLoading(false);
      }
    };

    loadData();
  }, [currentStock]);

  // 预测K线数据类型
  interface PredictionKline {
    date: string;
    open: number;
    close: number;
    high: number;
    low: number;
    volume: number;
  }

  // 生成未来5日预测数据（完整K线格式）- 使用useMemo避免重复计算
  const predictionData = useMemo(() => {
    const prediction = predictions.find((p) => p.stock_code === currentStock);
    if (!prediction || klineData.length === 0) return null;

    const lastKline = klineData[klineData.length - 1];
    const lastPrice = lastKline?.close || 0;
    const targetPrice = prediction.target_prices.short;
    const supportLevel = prediction.support_level;
    const resistanceLevel = prediction.resistance_level;
    const confidence = prediction.confidence;

    const recentData = klineData.slice(-20);
    let avgVolatility = 0;
    let avgVolume = 0;
    recentData.forEach((d) => {
      avgVolatility += (d.high - d.low) / d.close;
      avgVolume += d.volume;
    });
    avgVolatility = avgVolatility / recentData.length;
    avgVolume = avgVolume / recentData.length;

    const predictionKlines: PredictionKline[] = [];
    const dates: string[] = [];

    // 判断今天是否需要预测（未收盘且是工作日且数据中没有今天）
    const today = new Date();
    const todayStr = today.toISOString().split("T")[0];
    const lastDataDate = klineData[klineData.length - 1].date;
    const isWeekday = today.getDay() !== 0 && today.getDay() !== 6;
    const needPredictToday =
      !isMarketClosed() && isWeekday && lastDataDate !== todayStr;

    // 从明天开始计算未来5个工作日（如果今天未收盘，从今天开始）
    let currentDate = new Date(today);
    if (isMarketClosed() || !isWeekday) {
      currentDate.setDate(currentDate.getDate() + 1);
    }
    let prevClose = lastPrice;
    let dayIndex = 0;

    const totalDays = 5;
    // 生成随机波动因子，让曲线更自然
    const randomFactors: number[] = [];
    for (let i = 0; i < totalDays; i++) {
      // 生成-0.3到0.3之间的随机波动
      randomFactors.push((Math.random() - 0.5) * 0.6);
    }
    // 确保最后一天接近目标价
    randomFactors[totalDays - 1] = 0;

    while (predictionKlines.length < totalDays) {
      // 跳过周末
      while (currentDate.getDay() === 0 || currentDate.getDay() === 6) {
        currentDate.setDate(currentDate.getDate() + 1);
      }
      const dateStr = currentDate.toISOString().split("T")[0];
      dates.push(dateStr);
      dayIndex++;

      // 使用缓动函数让曲线更自然（ease-in-out效果）
      const progress = dayIndex / totalDays;
      const easedProgress =
        progress < 0.5
          ? 2 * progress * progress
          : 1 - Math.pow(-2 * progress + 2, 2) / 2;

      // 基础目标价 + 随机波动
      const baseTarget = lastPrice + (targetPrice - lastPrice) * easedProgress;
      const randomWave =
        (targetPrice - lastPrice) * randomFactors[dayIndex - 1];
      const targetClose = baseTarget + randomWave;

      const volatilityFactor = avgVolatility * (1.2 - confidence * 0.4);
      const gapFactor = (Math.random() - 0.5) * 0.015;
      const open = prevClose * (1 + gapFactor);
      const close = targetClose;

      const priceRangeSpread = resistanceLevel - supportLevel;
      const dayRange = Math.max(
        close * volatilityFactor,
        priceRangeSpread * 0.1
      );

      const isUp = close > open;
      let high, low;
      if (isUp) {
        high = Math.max(open, close) + dayRange * 0.3;
        low = Math.min(open, close) - dayRange * 0.2;
      } else {
        high = Math.max(open, close) + dayRange * 0.2;
        low = Math.min(open, close) - dayRange * 0.3;
      }

      high = Math.min(Math.max(high, open, close), resistanceLevel);
      low = Math.max(Math.min(low, open, close), supportLevel);

      const volumeFactor = 0.8 + Math.random() * 0.4;
      const volume = Math.round(avgVolume * volumeFactor);

      // 标记是否为今日预测
      const isToday = dateStr === todayStr;

      predictionKlines.push({
        date: dateStr,
        open: parseFloat(open.toFixed(2)),
        close: parseFloat(close.toFixed(2)),
        high: parseFloat(high.toFixed(2)),
        low: parseFloat(low.toFixed(2)),
        volume,
      });

      prevClose = close;
      currentDate.setDate(currentDate.getDate() + 1);
    }

    return { dates, klines: predictionKlines, needPredictToday };
  }, [currentStock, klineData, predictions]);

  // 当预测数据生成后保存到store
  useEffect(() => {
    if (predictionData && currentStock) {
      setPredictionKlines(currentStock, predictionData.klines);
    }
  }, [predictionData, currentStock, setPredictionKlines]);

  // 生成图表配置
  const getOption = () => {
    if (klineData.length === 0) {
      return {};
    }

    const dates = klineData.map((d) => d.date);
    const prices = klineData.map((d) => d.close);
    const allDates = predictionData
      ? [...dates, ...predictionData.dates]
      : dates;

    // 计算默认显示范围：最近5个工作日 + 预测5天
    const totalDays = allDates.length;
    const displayDays = predictionData ? 10 : 5; // 有预测时显示10天，否则显示5天
    const startPercent = Math.max(
      0,
      ((totalDays - displayDays) / totalDays) * 100
    );

    const series: any[] = [
      {
        name: "历史价格",
        type: "line",
        data: prices,
        smooth: true,
        symbol: "none",
        lineStyle: {
          width: 2,
          color: "#6366f1",
        },
        areaStyle: {
          color: {
            type: "linear",
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: "rgba(99, 102, 241, 0.4)" },
              { offset: 1, color: "rgba(99, 102, 241, 0.05)" },
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
      predictionData.klines.forEach((k) => predictionPrices.push(k.close));

      series.push({
        name: "AI预测(5日)",
        type: "line",
        data: predictionPrices,
        smooth: true,
        symbol: "circle",
        symbolSize: 6,
        lineStyle: {
          width: 2,
          color: "#f59e0b",
          type: [5, 5],
        },
        itemStyle: {
          color: "#f59e0b",
        },
      });
    }

    // 检测是否为移动端
    const isMobile = window.innerWidth <= 768;

    return {
      title: {
        text: `${currentStock} ${stockName}`,
        left: "center",
        top: 5,
        textStyle: {
          fontSize: isMobile ? 12 : 14,
          fontWeight: "normal",
          color: "#e2e8f0",
        },
      },
      legend: {
        show: predictionData !== null,
        top: 25,
        left: "center",
        textStyle: {
          color: "#e2e8f0",
          fontSize: isMobile ? 10 : 11,
        },
        itemWidth: 25,
        itemHeight: 14,
        itemGap: 15,
      },
      tooltip: {
        trigger: "axis",
        backgroundColor: "rgba(30, 41, 59, 0.95)",
        borderColor: "rgba(99, 102, 241, 0.3)",
        textStyle: {
          color: "#e2e8f0",
        },
        formatter: (params: any) => {
          const idx = params[0]?.dataIndex;
          if (idx < klineData.length) {
            const data = klineData[idx];
            if (!data) return "";

            // 计算历史数据涨跌幅
            let histChangePercent = 0;
            if (idx > 0 && klineData[idx - 1]?.close > 0) {
              histChangePercent =
                ((data.close - klineData[idx - 1].close) /
                  klineData[idx - 1].close) *
                100;
            }
            const histChangeColor =
              histChangePercent >= 0 ? "#f5222d" : "#52c41a";
            const histChangeSign = histChangePercent >= 0 ? "+" : "";

            return `
              <div style="font-size:12px">
                <div style="font-weight:bold;margin-bottom:4px">${
                  data.date
                }</div>
                <div>开盘: ${data.open?.toFixed(2) || "-"}</div>
                <div>收盘: ${data.close?.toFixed(2) || "-"}</div>
                <div>最高: ${data.high?.toFixed(2) || "-"}</div>
                <div>最低: ${data.low?.toFixed(2) || "-"}</div>
                <div>成交量: ${
                  data.volume ? (data.volume / 10000).toFixed(0) + "万" : "-"
                }</div>
                ${
                  idx > 0
                    ? `<div style="color:${histChangeColor};font-weight:bold">涨跌幅: ${histChangeSign}${histChangePercent.toFixed(
                        2
                      )}%</div>`
                    : ""
                }
              </div>
            `;
          } else {
            // 预测数据
            const predIdx = idx - klineData.length;
            const predKline = predictionData?.klines[predIdx];
            if (!predKline) return "";
            const pred = predictions.find((p) => p.stock_code === currentStock);
            const today = new Date().toISOString().split("T")[0];
            const isToday = predKline.date === today;
            const label = isToday ? "AI预测（今日未收盘）" : "AI预测";
            const suffix = isToday ? "（预测值）" : "";

            // 计算涨跌幅
            let changePercent = 0;
            let prevClose = 0;
            if (predIdx === 0 && klineData.length > 0) {
              // 第一个预测点，用最后一个历史数据的收盘价
              prevClose = klineData[klineData.length - 1].close;
            } else if (predIdx > 0 && predictionData?.klines[predIdx - 1]) {
              prevClose = predictionData.klines[predIdx - 1].close;
            }
            if (prevClose > 0) {
              changePercent = ((predKline.close - prevClose) / prevClose) * 100;
            }
            const changeColor = changePercent >= 0 ? "#f5222d" : "#52c41a";
            const changeSign = changePercent >= 0 ? "+" : "";

            return `
              <div style="font-size:12px">
                <div style="font-weight:bold;margin-bottom:4px;color:#f59e0b">${label} ${
              predKline.date
            }</div>
                <div>开盘: ${predKline.open.toFixed(2)}${suffix}</div>
                <div>收盘: ${predKline.close.toFixed(2)}${suffix}</div>
                <div>最高: ${predKline.high.toFixed(2)}${suffix}</div>
                <div>最低: ${predKline.low.toFixed(2)}${suffix}</div>
                <div>成交量: ${(predKline.volume / 10000).toFixed(
                  0
                )}万${suffix}</div>
                <div style="color:${changeColor};font-weight:bold">AI预测涨跌幅: ${changeSign}${changePercent.toFixed(
              2
            )}%</div>
                ${
                  pred
                    ? `<div style="margin-top:4px;padding-top:4px;border-top:1px solid rgba(255,255,255,0.2);color:#94a3b8;font-size:11px">目标价(5日): ${pred.target_prices.short.toFixed(
                        2
                      )} | 支撑: ${pred.support_level.toFixed(
                        2
                      )} | 压力: ${pred.resistance_level.toFixed(2)}</div>`
                    : ""
                }
              </div>
            `;
          }
        },
      },
      grid: {
        left: "3%",
        right: "4%",
        bottom: "15%",
        top: 50,
        containLabel: true,
      },
      xAxis: {
        type: "category",
        data: allDates,
        axisLabel: {
          rotate: 45,
          fontSize: 10,
          color: "#e2e8f0",
        },
        axisLine: {
          lineStyle: {
            color: "rgba(148, 163, 184, 0.3)",
          },
        },
      },
      yAxis: {
        type: "value",
        scale: true,
        axisLabel: {
          formatter: "{value}",
          color: "#e2e8f0",
        },
        axisLine: {
          lineStyle: {
            color: "rgba(148, 163, 184, 0.3)",
          },
        },
        splitLine: {
          lineStyle: {
            type: "dashed",
            color: "rgba(148, 163, 184, 0.15)",
          },
        },
      },
      dataZoom: [
        {
          type: "inside",
          start: startPercent,
          end: 100,
        },
        {
          type: "slider",
          start: startPercent,
          end: 100,
          height: 20,
          bottom: 5,
          textStyle: {
            color: "#e2e8f0",
          },
          borderColor: "rgba(99, 102, 241, 0.3)",
          fillerColor: "rgba(99, 102, 241, 0.2)",
          showDataShadow: false,
          brushSelect: false,
        },
      ],
      series,
    };
  };

  const stockOptions = selectedStocks.map((s) => ({
    value: s.code,
    label: `${s.code} ${s.name}`,
  }));

  const selectValue = stockOptions.find((o) => o.value === currentStock)
    ? currentStock
    : undefined;

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
      </div>

      <div
        className={`chart-content ${klineData.length > 0 ? "has-chart" : ""}`}
      >
        {loading ? (
          <div className="chart-loading">
            <Spin tip="加载中..." />
          </div>
        ) : klineData.length === 0 ? (
          <Empty description="请选择股票查看趋势" />
        ) : (
          <ReactECharts
            option={getOption()}
            style={{ height: "100%", width: "100%" }}
            notMerge={true}
          />
        )}
      </div>
    </div>
  );
};

export default TrendChart;
