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
import { getKline, getSystemConfig } from "../../services/api";
import type { KlineData } from "../../types";
import "./index.css";

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
  const [refreshAvailableTime, setRefreshAvailableTime] =
    useState<string>("17:00");
  const chartRef = useRef<any>(null);

  // 获取系统配置
  useEffect(() => {
    getSystemConfig()
      .then((config) => {
        if (config.refresh_available_time) {
          setRefreshAvailableTime(config.refresh_available_time);
        }
      })
      .catch((err) => {
        console.error("获取系统配置失败:", err);
      });
  }, []);

  // 判断当前时间是否在刷新可用时间之后
  const isRefreshTimeAvailable = useCallback((): boolean => {
    if (!isTradingDay()) return true; // 非交易日始终可用

    const now = new Date();
    const [hour, minute] = refreshAvailableTime.split(":").map(Number);
    const availableTime = hour * 100 + minute;
    const currentTime = getHHMM(now);

    return currentTime >= availableTime;
  }, [refreshAvailableTime]);

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
    async (showToast: boolean): Promise<boolean> => {
      if (!currentStock) {
        setKlineData([]);
        return false;
      }

      setLoading(true);
      try {
        const kline = await getKline(currentStock, "daily");
        setKlineData(kline.data || []);
        setStockName(kline.name || "");
        if (showToast) {
          message.success("数据已刷新");
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
    loadData(false);
  }, [currentStock, loadData]);

  // 添加全局点击事件来处理tooltip失焦
  useEffect(() => {
    const handleGlobalClick = (event: Event) => {
      const chartInstance = chartRef.current?.getEchartsInstance();
      if (chartInstance) {
        const chartContainer = chartRef.current?.ele;
        // 如果点击的不是图表区域，隐藏tooltip
        if (chartContainer && !chartContainer.contains(event.target as Node)) {
          chartInstance.dispatchAction({
            type: "hideTip",
          });
        }
      }
    };

    document.addEventListener("click", handleGlobalClick);
    document.addEventListener("touchstart", handleGlobalClick);

    return () => {
      document.removeEventListener("click", handleGlobalClick);
      document.removeEventListener("touchstart", handleGlobalClick);
    };
  }, []);

  // 只在交易日显示刷新按钮
  const shouldShowRefreshButton = isTradingDay() || isTradingTime();
  const canRefresh = Boolean(currentStock) && isRefreshTimeAvailable();
  const refreshDisabled = !canRefresh || loading;
  const refreshTooltipTitle = !canRefresh
    ? `当日数据将在 ${refreshAvailableTime} 后可刷新（需等待第三方接口更新后，后台同步数据）`
    : "刷新当日K线数据（从缓存获取）";

  const handleRefresh = async () => {
    if (refreshDisabled) return;
    await loadData(true);
  };

  // 处理移动端点击置灰按钮的提示
  const handleDisabledRefreshClick = () => {
    if (refreshDisabled) {
      message.info({
        content: refreshTooltipTitle,
        duration: 3,
        style: {
          marginTop: "10vh",
        },
      });
    }
  };

  const predictionData = useMemo(() => {
    const prediction = predictions.find((p) => p.stock_code === currentStock);
    if (
      !prediction ||
      !prediction.future_klines ||
      prediction.future_klines.length === 0
    ) {
      return null;
    }

    const dates = prediction.future_klines.map((k) => k.date);
    return {
      dates,
      klines: prediction.future_klines,
      needPredictToday: !!prediction.need_predict_today,
      aiToday: prediction.ai_today || null,
    };
  }, [currentStock, predictions]);

  // 当预测数据生成后保存到store
  useEffect(() => {
    if (predictionData && currentStock) {
      setPredictionKlines(currentStock, predictionData.klines);
    }
  }, [predictionData, currentStock, setPredictionKlines]);

  // 生成图表配置 - 使用useMemo避免频繁重新生成导致tooltip闪烁
  const chartOption = useMemo(() => {
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
        smooth: false,
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
        hideDelay: 200, // 减少延迟时间，让tooltip更快响应
        showDelay: 0, // 鼠标移入后立即显示
        enterable: false, // 禁止鼠标进入tooltip区域，让它纯粹跟随鼠标
        confine: true, // 限制tooltip在图表区域内
        transitionDuration: 0.1, // 减少过渡时间，让移动更流畅
        alwaysShowContent: false, // 确保不会一直显示
        triggerOn: "mousemove|click", // 支持鼠标移动和点击触发，移动端更友好
        appendToBody: false, // 不添加到body，避免定位问题
        renderMode: "html", // 使用HTML渲染模式，提升性能
        position: function (
          point: number[],
          _params: any,
          _dom: HTMLElement,
          _rect: any,
          size: any
        ) {
          // 简化定位逻辑，让tooltip流畅跟随鼠标
          const [x, y] = point;
          const tooltipWidth = size.contentSize[0];
          const tooltipHeight = size.contentSize[1];
          const chartWidth = size.viewSize[0];
          const chartHeight = size.viewSize[1];

          // 基础偏移量
          const offsetX = 10;
          const offsetY = -10;

          let posX = x + offsetX;
          let posY = y + offsetY;

          // 简单的边界检查
          if (posX + tooltipWidth > chartWidth - 10) {
            posX = x - tooltipWidth - offsetX;
          }

          if (posY < 10) {
            posY = y + 20;
          } else if (posY + tooltipHeight > chartHeight - 10) {
            posY = y - tooltipHeight - 20;
          }

          return [posX, posY];
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
  }, [klineData, predictionData, currentStock, stockName, predictions]);

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
          {shouldShowRefreshButton && (
            <>
              {/* 检测是否为移动端 */}
              {/Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(
                navigator.userAgent
              ) ? (
                // 移动端：不显示 Tooltip，点击时显示 message
                <Button
                  size="small"
                  icon={<ReloadOutlined />}
                  onClick={
                    refreshDisabled ? handleDisabledRefreshClick : handleRefresh
                  }
                  disabled={false}
                  style={
                    refreshDisabled
                      ? {
                          opacity: 0.6,
                          cursor: "not-allowed",
                          pointerEvents: "auto",
                        }
                      : {}
                  }
                >
                  刷新
                </Button>
              ) : (
                // PC端：显示 Tooltip，按钮置灰时禁用
                <Tooltip title={refreshTooltipTitle}>
                  <Button
                    size="small"
                    icon={<ReloadOutlined />}
                    onClick={refreshDisabled ? undefined : handleRefresh}
                    disabled={refreshDisabled}
                  >
                    刷新
                  </Button>
                </Tooltip>
              )}
            </>
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
            ref={chartRef}
            option={chartOption}
            style={{ height: "100%", width: "100%" }}
            notMerge={true}
            shouldSetOption={(prevProps: any, nextProps: any) => {
              // 只有在关键数据变化时才重新设置option，避免倒计时更新导致的闪烁
              return (
                prevProps.option !== nextProps.option ||
                JSON.stringify(prevProps.option) !==
                  JSON.stringify(nextProps.option)
              );
            }}
          />
        )}
      </div>
    </div>
  );
};

export default TrendChart;
