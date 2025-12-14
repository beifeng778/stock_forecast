import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import ReactECharts from "echarts-for-react";
import { Button, Empty, Select, Space, Spin, Tooltip, message } from "antd";
import { ReloadOutlined } from "@ant-design/icons";
import { useStockStore } from "../../store";
import { getKline } from "../../services/api";
import type { KlineData } from "../../types";
import "./index.css";

const REFRESH_SUCCESS_COOLDOWN = 5 * 60;
const REFRESH_FAIL_COOLDOWN = 2 * 60;

// 判断A股是否已收盘（15:00收盘）
const isMarketClosed = (): boolean => {
  const now = new Date();
  const closeTime = now.getHours() * 100 + now.getMinutes();
  return closeTime >= 1500;
};

const isTradingDay = (): boolean => {
  const now = new Date();
  return now.getDay() !== 0 && now.getDay() !== 6;
};

const getHHMM = (d: Date): number => d.getHours() * 100 + d.getMinutes();

// 开盘到收盘（包含午休）：09:30-15:00
const isOpenToClose = (): boolean => {
  if (!isTradingDay()) return false;
  const hhmm = getHHMM(new Date());
  return hhmm >= 930 && hhmm < 1500;
};

// A股交易时段：09:30-11:30、13:00-15:00
const isTradingTime = (): boolean => {
  if (!isTradingDay()) return false;
  const now = new Date();
  const hhmm = getHHMM(now);
  const morning = hhmm >= 930 && hhmm < 1130;
  const afternoon = hhmm >= 1300 && hhmm < 1500;
  return morning || afternoon;
};

const TrendChart: React.FC = () => {
  const { selectedStocks, predictions, setPredictionKlines } = useStockStore();
  const [currentStock, setCurrentStock] = useState<string>("");
  const [klineData, setKlineData] = useState<KlineData[]>([]);
  const [stockName, setStockName] = useState<string>("");
  const [loading, setLoading] = useState(false);
  const [refreshCooldown, setRefreshCooldown] = useState(0);
  const cooldownTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const getCooldownStorageKey = useCallback((code: string) => {
    return `klineRefreshCooldownEnd:${code}`;
  }, []);

  const stopCooldownTimer = useCallback(() => {
    if (cooldownTimerRef.current) {
      clearInterval(cooldownTimerRef.current);
      cooldownTimerRef.current = null;
    }
  }, []);

  const startCooldown = useCallback(
    (seconds: number) => {
      if (!currentStock) return;

      stopCooldownTimer();
      setRefreshCooldown(seconds);

      const endTime = Date.now() + seconds * 1000;
      localStorage.setItem(
        getCooldownStorageKey(currentStock),
        String(endTime)
      );

      cooldownTimerRef.current = setInterval(() => {
        const remaining = Math.ceil((endTime - Date.now()) / 1000);
        if (remaining <= 0) {
          stopCooldownTimer();
          setRefreshCooldown(0);
          localStorage.removeItem(getCooldownStorageKey(currentStock));
          return;
        }
        setRefreshCooldown(remaining);
      }, 1000);
    },
    [currentStock, getCooldownStorageKey, stopCooldownTimer]
  );

  const canRefreshIntraday = Boolean(currentStock) && isTradingTime();

  // 切换股票时恢复该股票的冷却状态
  useEffect(() => {
    stopCooldownTimer();
    setRefreshCooldown(0);

    if (!currentStock) return;
    const savedEndTime = localStorage.getItem(
      getCooldownStorageKey(currentStock)
    );
    if (!savedEndTime) return;

    const remaining = Math.ceil((Number(savedEndTime) - Date.now()) / 1000);
    if (remaining > 0) {
      startCooldown(remaining);
    } else {
      localStorage.removeItem(getCooldownStorageKey(currentStock));
    }
  }, [currentStock, getCooldownStorageKey, startCooldown, stopCooldownTimer]);

  // 组件卸载时清理定时器
  useEffect(() => {
    return () => {
      stopCooldownTimer();
    };
  }, [stopCooldownTimer]);

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

  const loadData = useCallback(
    async (showToast: boolean, refresh: boolean): Promise<boolean> => {
      if (!currentStock) {
        setKlineData([]);
        return false;
      }

      setLoading(true);
      try {
        const kline = await getKline(currentStock, "daily", refresh);
        setKlineData(kline.data || []);
        setStockName(kline.name || "");
        if (showToast) {
          message.success("盘中数据已刷新");
        }
        return true;
      } catch (error) {
        console.error("加载K线数据失败:", error);
        setKlineData([]);
        if (showToast) {
          message.error("刷新失败，请稍后再试");
        }
        return false;
      } finally {
        setLoading(false);
      }
    },
    [currentStock]
  );

  // 加载K线数据
  useEffect(() => {
    loadData(false, false);
  }, [currentStock, loadData]);

  const formatCooldown = (seconds: number) => {
    const m = Math.floor(seconds / 60);
    const s = seconds % 60;
    return `${m}:${String(s).padStart(2, "0")}`;
  };

  const refreshDisabled = !canRefreshIntraday || loading || refreshCooldown > 0;
  const refreshButtonText =
    refreshCooldown > 0 ? formatCooldown(refreshCooldown) : "刷新盘中";
  const refreshTooltipTitle = !canRefreshIntraday
    ? "仅交易日 09:30-11:30 / 13:00-15:00 可刷新"
    : refreshCooldown > 0
    ? `冷却中 ${formatCooldown(refreshCooldown)}（失败后2分钟，成功后5分钟）`
    : "刷新盘中数据（会请求第三方接口）";

  const handleRefreshIntraday = async () => {
    if (refreshDisabled) return;
    const ok = await loadData(true, true);
    startCooldown(ok ? REFRESH_SUCCESS_COOLDOWN : REFRESH_FAIL_COOLDOWN);
  };

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
    const today = new Date();
    const todayStr = today.toISOString().split("T")[0];
    const lastDataDate = lastKline.date;
    const hasTodayData = lastDataDate === todayStr;
    const hhmmNow = getHHMM(today);
    const isTradingDayNow = today.getDay() !== 0 && today.getDay() !== 6;
    const isIntraday = isTradingDayNow && hhmmNow >= 930 && hhmmNow < 1500;
    const isAfterClose = isTradingDayNow && hhmmNow >= 1500;
    const baseCloseForPrediction = (() => {
      if (!hasTodayData) return lastKline?.close || 0;
      if (isIntraday && klineData.length >= 2) {
        return klineData[klineData.length - 2].close;
      }
      if (isAfterClose) {
        return lastKline?.close || 0;
      }
      return lastKline?.close || 0;
    })();

    const lastPrice = baseCloseForPrediction;
    const targetPrice = prediction.target_prices.short;
    const supportLevel = prediction.support_level;
    const resistanceLevel = prediction.resistance_level;
    const confidence = prediction.confidence;

    const dataForStats =
      hasTodayData && isIntraday && klineData.length >= 2
        ? klineData.slice(0, -1)
        : klineData;
    const recentData = dataForStats.slice(-20);
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
    const needPredictToday = isIntraday && !hasTodayData;

    // 从下一个需要预测的交易日开始生成（避免历史今日+预测今日重复）
    let currentDate = new Date(today);
    if (!isIntraday || hasTodayData) {
      currentDate.setDate(currentDate.getDate() + 1);
    }
    let prevClose = lastPrice;
    let dayIndex = 0;

    const totalDays = 5;
    // 生成随机波动因子，让曲线更自然
    const randomFactors: number[] = [];
    for (let i = 0; i < totalDays; i++) {
      randomFactors.push((Math.random() - 0.5) * 0.6);
    }
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

    let aiToday: PredictionKline | null = null;
    if (isIntraday && hasTodayData && klineData.length >= 2) {
      const basePriceForToday = klineData[klineData.length - 2].close;
      const progress = 1 / totalDays;
      const easedProgress =
        progress < 0.5
          ? 2 * progress * progress
          : 1 - Math.pow(-2 * progress + 2, 2) / 2;
      const close =
        basePriceForToday + (targetPrice - basePriceForToday) * easedProgress;

      const volatilityFactor = avgVolatility * (1.2 - confidence * 0.4);
      const open = basePriceForToday;
      const priceRangeSpread = resistanceLevel - supportLevel;
      const dayRange = Math.max(
        close * volatilityFactor,
        priceRangeSpread * 0.1
      );
      const isUp = close > open;
      let high: number;
      let low: number;
      if (isUp) {
        high = Math.max(open, close) + dayRange * 0.3;
        low = Math.min(open, close) - dayRange * 0.2;
      } else {
        high = Math.max(open, close) + dayRange * 0.2;
        low = Math.min(open, close) - dayRange * 0.3;
      }
      high = Math.min(Math.max(high, open, close), resistanceLevel);
      low = Math.max(Math.min(low, open, close), supportLevel);
      const volume = Math.round(avgVolume);

      aiToday = {
        date: todayStr,
        open: parseFloat(open.toFixed(2)),
        close: parseFloat(close.toFixed(2)),
        high: parseFloat(high.toFixed(2)),
        low: parseFloat(low.toFixed(2)),
        volume,
      };
    }

    return { dates, klines: predictionKlines, needPredictToday, aiToday };
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
      const now = new Date();
      const todayStr = now.toISOString().split("T")[0];
      const hhmm = getHHMM(now);
      const isTradingDayNow = now.getDay() !== 0 && now.getDay() !== 6;
      const isIntradayNow = isTradingDayNow && hhmm >= 930 && hhmm < 1500;
      const hasTodayData = klineData[klineData.length - 1]?.date === todayStr;

      const anchorIndex =
        hasTodayData && isIntradayNow && klineData.length >= 2
          ? klineData.length - 2
          : klineData.length - 1;
      const anchorPrice =
        klineData[anchorIndex]?.close ??
        klineData[klineData.length - 1]?.close ??
        0;

      // 构建预测收盘价数据（前面填充null，从最后一个历史数据点开始）
      const predictionPrices: (number | null)[] = [];
      for (let i = 0; i < anchorIndex; i++) {
        predictionPrices.push(null);
      }
      predictionPrices.push(anchorPrice);
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

            const today = new Date().toISOString().split("T")[0];
            const isToday = data.date === today;
            const showTodayUnclosedStyle = isToday && isOpenToClose();
            const isValidNumber = (v: unknown): v is number => {
              return typeof v === "number" && Number.isFinite(v) && v > 0;
            };
            const isValidVolume = (v: unknown): v is number => {
              return typeof v === "number" && Number.isFinite(v) && v >= 0;
            };
            const hasActualOpen = isToday && isValidNumber(data.open);
            const hasActualClose = isToday && isValidNumber(data.close);
            const hasActualHigh = isToday && isValidNumber(data.high);
            const hasActualLow = isToday && isValidNumber(data.low);
            const hasActualVolume = isToday && isValidVolume(data.volume);
            const hasTodayActual =
              hasActualOpen ||
              hasActualClose ||
              hasActualHigh ||
              hasActualLow ||
              hasActualVolume;

            const showActualOpen =
              isToday &&
              isTradingDay() &&
              getHHMM(new Date()) >= 930 &&
              hasActualOpen;
            const showActualAll =
              isToday && isTradingDay() && isMarketClosed() && hasTodayActual;

            const predToday =
              isToday && predictionData?.aiToday
                ? predictionData.aiToday
                : isToday &&
                  predictionData?.needPredictToday &&
                  predictionData.klines?.[0]?.date === today
                ? predictionData.klines[0]
                : null;

            const pred = predictions.find((p) => p.stock_code === currentStock);

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

            if (showTodayUnclosedStyle && predToday) {
              // 预测涨跌幅：以昨日收盘作为基准
              let aiChangePercent = 0;
              if (idx > 0 && klineData[idx - 1]?.close > 0) {
                aiChangePercent =
                  ((predToday.close - klineData[idx - 1].close) /
                    klineData[idx - 1].close) *
                  100;
              }
              const aiChangeColor =
                aiChangePercent >= 0 ? "#f5222d" : "#52c41a";
              const aiChangeSign = aiChangePercent >= 0 ? "+" : "";

              return `
                <div style="font-size:12px">
                  <div style="font-weight:bold;margin-bottom:6px;color:#f59e0b">AI预测（今日未收盘） ${
                    data.date
                  }</div>
                  <div>开盘: ${predToday.open.toFixed(2)}（预测值）</div>
                  ${
                    showActualOpen
                      ? `<div style="color:#94a3b8">开盘: ${data.open.toFixed(
                          2
                        )}（实际值）</div>`
                      : ""
                  }

                  <div>收盘: ${predToday.close.toFixed(2)}（预测值）</div>
                  ${
                    showActualAll && hasActualClose
                      ? `<div style="color:#94a3b8">收盘: ${data.close.toFixed(
                          2
                        )}（实际值）</div>`
                      : ""
                  }

                  <div>最高: ${predToday.high.toFixed(2)}（预测值）</div>
                  ${
                    showActualAll && hasActualHigh
                      ? `<div style="color:#94a3b8">最高: ${data.high.toFixed(
                          2
                        )}（实际值）</div>`
                      : ""
                  }

                  <div>最低: ${predToday.low.toFixed(2)}（预测值）</div>
                  ${
                    showActualAll && hasActualLow
                      ? `<div style="color:#94a3b8">最低: ${data.low.toFixed(
                          2
                        )}（实际值）</div>`
                      : ""
                  }

                  <div>成交量: ${(predToday.volume / 10000).toFixed(
                    0
                  )}万（预测值）</div>
                  ${
                    showActualAll && hasActualVolume
                      ? `<div style="color:#94a3b8">成交量: ${(
                          data.volume / 10000
                        ).toFixed(0)}万（实际值）</div>`
                      : ""
                  }

                  <div style="color:${aiChangeColor};font-weight:bold;margin-top:4px">AI预测涨跌幅: ${aiChangeSign}${aiChangePercent.toFixed(
                2
              )}%</div>
                  ${
                    pred
                      ? `<div style="margin-top:6px;padding-top:6px;border-top:1px solid rgba(255,255,255,0.2);color:#94a3b8;font-size:11px">目标价(5日): ${pred.target_prices.short.toFixed(
                          2
                        )} | 支撑: ${pred.support_level.toFixed(
                          2
                        )} | 压力: ${pred.resistance_level.toFixed(2)}</div>`
                      : ""
                  }
                </div>
              `;
            }

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
            const label =
              isToday && isOpenToClose() ? "AI预测（今日未收盘）" : "AI预测";
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
          <Tooltip title={refreshTooltipTitle}>
            <Button
              size="small"
              icon={<ReloadOutlined />}
              onClick={handleRefreshIntraday}
              disabled={refreshDisabled}
            >
              {refreshButtonText}
            </Button>
          </Tooltip>
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
