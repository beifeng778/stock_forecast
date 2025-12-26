import React, { useCallback, useEffect, useRef, useState } from "react";
import { Card, Tag, Progress, Collapse, Empty } from "antd";
import {
  ArrowUpOutlined,
  ArrowDownOutlined,
  MinusOutlined,
} from "@ant-design/icons";
import ReactMarkdown from "react-markdown";
import { useStockStore } from "../../store";
import type { PredictResult, Signal } from "../../types";
import "./index.css";

// 趋势图标
const TrendIcon: React.FC<{ trend: string }> = ({ trend }) => {
  if (trend === "up") {
    return <ArrowUpOutlined style={{ color: "#f5222d" }} />;
  }
  if (trend === "down") {
    return <ArrowDownOutlined style={{ color: "#52c41a" }} />;
  }
  return <MinusOutlined style={{ color: "#faad14" }} />;
};

// 信号标签
const SignalTag: React.FC<{ signal: Signal }> = ({ signal }) => {
  const colorMap: Record<string, string> = {
    bullish: "red",
    bearish: "green",
    neutral: "default",
  };
  return (
    <Tag color={colorMap[signal.type]}>
      {signal.name}: {signal.desc}
    </Tag>
  );
};

// 鼠标拖拽滚动Hook
const useDragScroll = (uniqueId?: string) => {
  const [element, setElement] = useState<HTMLDivElement | null>(null);
  const isDragging = useRef(false);
  const activePointerId = useRef<number | null>(null);
  const startX = useRef(0);
  const scrollLeft = useRef(0);
  const hasMoved = useRef(false);

  const refCb = useCallback((node: HTMLDivElement | null) => {
    setElement(node);
  }, []);

  useEffect(() => {
    if (!element) {
      return;
    }

    // 确保元素有正确的样式
    element.style.cursor = "grab";
    element.style.userSelect = "none";
    element.style.touchAction = "pan-x pinch-zoom";
    element.style.overflowX = "auto";
    element.style.scrollbarWidth = "none";
    (element.style as any).webkitOverflowScrolling = "touch";

    const getMaxScroll = () => element.scrollWidth - element.clientWidth;

    const handlePointerDown = (e: PointerEvent) => {
      // 仅左键/触控笔/触摸触发
      if (e.pointerType === "mouse" && e.button !== 0) {
        return;
      }
      // 移动端触控使用原生惯性滚动，避免卡顿
      if (e.pointerType === "touch") {
        return;
      }

      console.log(
        `[${uniqueId}] PointerDown triggered, scrollWidth: ${
          element.scrollWidth
        }, clientWidth: ${element.clientWidth}, maxScroll: ${getMaxScroll()}`
      );

      isDragging.current = true;
      activePointerId.current = e.pointerId;
      hasMoved.current = false;
      startX.current = e.pageX;
      scrollLeft.current = element.scrollLeft;
      element.style.cursor = "grabbing";

      try {
        element.setPointerCapture(e.pointerId);
      } catch {
        // ignore
      }
      e.preventDefault();
      e.stopPropagation();
    };

    const handlePointerMove = (e: PointerEvent) => {
      if (!isDragging.current) return;
      if (activePointerId.current !== e.pointerId) return;

      const maxScroll = getMaxScroll();
      if (maxScroll <= 0) {
        e.preventDefault();
        return;
      }

      const x = e.pageX;
      const walk = (x - startX.current) * 2;
      const newScrollLeft = scrollLeft.current - walk;
      if (newScrollLeft !== element.scrollLeft) {
        hasMoved.current = true;
      }

      element.scrollLeft = Math.max(0, Math.min(newScrollLeft, maxScroll));
      e.preventDefault();
    };

    const endDrag = () => {
      if (!isDragging.current) return;
      isDragging.current = false;
      activePointerId.current = null;
      element.style.cursor = "grab";
    };

    const handlePointerUp = (e: PointerEvent) => {
      if (activePointerId.current !== e.pointerId) return;
      try {
        element.releasePointerCapture(e.pointerId);
      } catch {
        // ignore
      }
      endDrag();
    };

    const handlePointerCancel = (e: PointerEvent) => {
      if (activePointerId.current !== e.pointerId) return;
      endDrag();
    };

    const handleWindowBlur = () => {
      endDrag();
    };

    const handleSelectStart = (e: Event) => {
      if (isDragging.current) {
        e.preventDefault();
      }
    };

    const handleDragStart = (e: DragEvent) => {
      e.preventDefault();
    };

    const handleClick = (e: MouseEvent) => {
      if (hasMoved.current) {
        e.preventDefault();
        e.stopPropagation();
      }
    };

    const handleWheel = (e: WheelEvent) => {
      if (element.scrollWidth > element.clientWidth) {
        e.preventDefault();
        element.scrollLeft += e.deltaY;
      }
    };

    element.addEventListener("pointerdown", handlePointerDown);
    element.addEventListener("pointermove", handlePointerMove);
    element.addEventListener("pointerup", handlePointerUp);
    element.addEventListener("pointercancel", handlePointerCancel);
    element.addEventListener("selectstart", handleSelectStart);
    element.addEventListener("dragstart", handleDragStart);
    element.addEventListener("click", handleClick, true);
    element.addEventListener("wheel", handleWheel, { passive: false });
    window.addEventListener("blur", handleWindowBlur);

    return () => {
      element.removeEventListener("pointerdown", handlePointerDown);
      element.removeEventListener("pointermove", handlePointerMove);
      element.removeEventListener("pointerup", handlePointerUp);
      element.removeEventListener("pointercancel", handlePointerCancel);
      element.removeEventListener("selectstart", handleSelectStart);
      element.removeEventListener("dragstart", handleDragStart);
      element.removeEventListener("click", handleClick, true);
      element.removeEventListener("wheel", handleWheel);
      window.removeEventListener("blur", handleWindowBlur);
    };
  }, [uniqueId, element, refCb]);

  return refCb;
};

// 单个预测结果卡片
const PredictionCard: React.FC<{ result: PredictResult }> = ({ result }) => {
  const signalsRef = useDragScroll(`signals-${result.stock_code}`);
  const dailyChangesRef = useDragScroll(`daily-${result.stock_code}`);
  const priceChange = result.target_prices.short - result.current_price;
  const priceChangePercent = (
    (priceChange / result.current_price) *
    100
  ).toFixed(2);

  return (
    <Card className="prediction-card" size="small">
      <div className="card-header">
        <div className="stock-info">
          <span className="stock-code">{result.stock_code}</span>
          <span className="stock-name">{result.stock_name}</span>
          {result.sector && (
            <Tag
              className="sector-tag"
              color={
                result.trend === "up"
                  ? "red"
                  : result.trend === "down"
                  ? "green"
                  : "gold"
              }
            >
              {result.sector}
            </Tag>
          )}
        </div>
        <div className="trend-info">
          <TrendIcon trend={result.trend} />
          <span className={`trend-text trend-${result.trend}`}>
            {result.trend_cn}
          </span>
        </div>
      </div>

      <div className="card-body">
        <div className="price-section">
          <div className="price-item">
            <span className="label">当前价</span>
            <span className="value">{result.current_price.toFixed(2)}</span>
          </div>
          <div className="price-item">
            <span className="label">目标价(第5日收盘)</span>
            <span className={`value ${priceChange >= 0 ? "up" : "down"}`}>
              {result.target_prices.short.toFixed(2)}
              <small>
                ({priceChange >= 0 ? "+" : ""}
                {priceChangePercent}%)
              </small>
            </span>
          </div>
        </div>

        <div className="level-section">
          <div className="level-item">
            <span className="label">支撑位</span>
            <span className="value support">
              {result.support_level.toFixed(2)}
            </span>
          </div>
          <div className="level-item">
            <span className="label">压力位</span>
            <span className="value resistance">
              {result.resistance_level.toFixed(2)}
            </span>
          </div>
        </div>

        <div className="confidence-section">
          <span className="label">置信度</span>
          <Progress
            percent={Math.round(result.confidence * 100)}
            size="small"
            status={
              result.confidence > 0.7
                ? "success"
                : result.confidence > 0.5
                ? "normal"
                : "exception"
            }
          />
        </div>

        <div className="signals-section" ref={signalsRef}>
          {result.signals.map((signal, index) => (
            <SignalTag key={index} signal={signal} />
          ))}
        </div>

        <Collapse
          ghost
          size="small"
          items={[
            {
              key: "1",
              label: "详细分析",
              children: (
                <div className="analysis-content">
                  <div className="markdown-content">
                    <ReactMarkdown>{result.analysis}</ReactMarkdown>
                  </div>

                  {result.news_analysis && (
                    <div className="news-analysis-content">
                      <ReactMarkdown>{result.news_analysis}</ReactMarkdown>
                    </div>
                  )}

                  <div className="ml-predictions-table-wrapper">
                    <table className="ml-predictions-table">
                      <tbody>
                        <tr>
                          <td className="label">LSTM预测</td>
                          <td className="value">
                            {result.ml_predictions.lstm.trend === "up"
                              ? "看涨"
                              : result.ml_predictions.lstm.trend === "down"
                              ? "看跌"
                              : "震荡"}
                            (
                            {(
                              result.ml_predictions.lstm.confidence * 100
                            ).toFixed(0)}
                            %)
                          </td>
                        </tr>
                        <tr>
                          <td className="label">Prophet预测</td>
                          <td className="value">
                            {result.ml_predictions.prophet.trend === "up"
                              ? "看涨"
                              : result.ml_predictions.prophet.trend === "down"
                              ? "看跌"
                              : "震荡"}
                            (
                            {(
                              result.ml_predictions.prophet.confidence * 100
                            ).toFixed(0)}
                            %)
                          </td>
                        </tr>
                        <tr>
                          <td className="label">XGBoost预测</td>
                          <td className="value">
                            {result.ml_predictions.xgboost.trend === "up"
                              ? "看涨"
                              : result.ml_predictions.xgboost.trend === "down"
                              ? "看跌"
                              : "震荡"}
                            (
                            {(
                              result.ml_predictions.xgboost.confidence * 100
                            ).toFixed(0)}
                            %)
                          </td>
                        </tr>
                        <tr>
                          <td className="label">RSI</td>
                          <td className="value">
                            {result.indicators.rsi.toFixed(2)}
                          </td>
                        </tr>
                        {result.indicators.current_turnover !== undefined && result.indicators.current_turnover > 0 && (
                          <>
                            <tr>
                              <td className="label">换手率</td>
                              <td className="value">
                                {result.indicators.current_turnover.toFixed(2)}%
                                {result.indicators.turnover_level && (
                                  <span style={{ marginLeft: '4px', fontSize: '11px', opacity: 0.8 }}>
                                    ({result.indicators.turnover_level})
                                  </span>
                                )}
                              </td>
                            </tr>
                            {result.indicators.avg_turnover_5d !== undefined && result.indicators.avg_turnover_5d > 0 && (
                              <tr>
                                <td className="label">5日均换手</td>
                                <td className="value">
                                  {result.indicators.avg_turnover_5d.toFixed(2)}%
                                </td>
                              </tr>
                            )}
                          </>
                        )}
                      </tbody>
                    </table>
                  </div>

                  <div className="target-prices">
                    <h4>目标价位</h4>
                    <div className="target-grid">
                      <div>
                        <span className="target-label">短期(5日):</span>
                        <span className="target-value">{result.target_prices.short.toFixed(2)}</span>
                      </div>
                      <div>
                        <span className="target-label">中期(20日):</span>
                        <span className="target-value">{result.target_prices.medium.toFixed(2)}</span>
                      </div>
                      <div>
                        <span className="target-label">长期(60日):</span>
                        <span className="target-value">{result.target_prices.long.toFixed(2)}</span>
                      </div>
                    </div>
                  </div>

                  {result.daily_changes &&
                    result.daily_changes.length > 0 &&
                    (() => {
                      // 过滤掉当天未收盘的数据，只取已收盘的最近5天
                      // 使用本地时间而不是UTC时间
                      const now = new Date();
                      const today = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`;
                      const closedDays = result.daily_changes.filter(
                        (item) => item.date < today
                      );
                      const last5Days = closedDays.slice(-5);
                      if (last5Days.length === 0) return null;
                      return (
                        <div className="daily-changes">
                          <h4>近期涨跌幅</h4>
                          <div
                            className="daily-changes-list"
                            ref={dailyChangesRef}
                          >
                            {last5Days.map((item, index) => (
                              <div key={index} className="daily-change-item">
                                <span className="date">
                                  {item.date.slice(5)}
                                </span>
                                <span
                                  className={`change ${
                                    item.change >= 0 ? "up" : "down"
                                  }`}
                                >
                                  {item.change >= 0 ? "+" : ""}
                                  {item.change.toFixed(2)}%
                                </span>
                              </div>
                            ))}
                          </div>
                        </div>
                      );
                    })()}
                </div>
              ),
            },
          ]}
        />
      </div>
    </Card>
  );
};

// 平滑进度Hook
const useSmoothProgress = (
  targetPercent: number,
  isActive: boolean,
  persistedProgress: number,
  onProgressChange: (progress: number) => void
) => {
  const [smoothPercent, setSmoothPercent] = useState(persistedProgress);
  const animationRef = useRef<number | null>(null);
  const lastTargetRef = useRef(targetPercent);
  const lastUpdateTimeRef = useRef(Date.now());
  const isInitializedRef = useRef(false);

  useEffect(() => {
    if (!isActive) {
      setSmoothPercent(0);
      lastTargetRef.current = 0;
      lastUpdateTimeRef.current = Date.now();
      isInitializedRef.current = false;
      if (animationRef.current) {
        cancelAnimationFrame(animationRef.current);
        animationRef.current = null;
      }
      onProgressChange(0);
      return;
    }

    // 首次激活时，从持久化的进度开始（支持刷新恢复）
    if (!isInitializedRef.current) {
      isInitializedRef.current = true;
      setSmoothPercent(persistedProgress);
      lastTargetRef.current = targetPercent;
      lastUpdateTimeRef.current = Date.now();
    }

    const animate = () => {
      const now = Date.now();
      const elapsed = now - lastUpdateTimeRef.current;

      setSmoothPercent((current) => {
        // 如果目标进度更新了（后端返回新进度）
        if (targetPercent > lastTargetRef.current) {
          lastTargetRef.current = targetPercent;
          lastUpdateTimeRef.current = now;
          // 快速追上新的目标进度
          onProgressChange(targetPercent);
          return targetPercent;
        }

        // 如果当前进度已经达到或超过目标进度
        if (current >= targetPercent) {
          // 在两次后端更新之间，缓慢模拟进度增长
          // 每秒增长约0.5%，避免进度条停滞
          const simulatedGrowth = (elapsed / 1000) * 0.5;
          const nextPercent = Math.min(current + simulatedGrowth, 99.5);

          // 如果接近100%，停止模拟增长
          if (nextPercent >= 99.5) {
            return current;
          }

          lastUpdateTimeRef.current = now;
          onProgressChange(nextPercent);
          return nextPercent;
        }

        // 平滑追赶目标进度（使用缓动函数）
        const diff = targetPercent - current;
        const step = diff * 0.1; // 每帧追赶10%的差距
        const nextPercent = current + Math.max(step, 0.1);
        const finalPercent = Math.min(nextPercent, targetPercent);

        onProgressChange(finalPercent);
        return finalPercent;
      });

      animationRef.current = requestAnimationFrame(animate);
    };

    animationRef.current = requestAnimationFrame(animate);

    return () => {
      if (animationRef.current) {
        cancelAnimationFrame(animationRef.current);
      }
    };
  }, [targetPercent, isActive, onProgressChange]);

  return smoothPercent;
};

const PredictionPanel: React.FC = () => {
  const {
    predictions,
    loading,
    predictInProgress,
    predictMeta,
    predictProgress,
    selectedStocks,
    smoothProgress,
    setSmoothProgress,
  } = useStockStore();

  const targetPercent =
    predictInProgress && predictProgress && predictProgress.total > 0
      ? Math.min(
          100,
          Math.max(0, (predictProgress.done / predictProgress.total) * 100)
        )
      : 0;

  // 使用平滑进度
  const progressPercent = useSmoothProgress(
    targetPercent,
    predictInProgress,
    smoothProgress,
    setSmoothProgress
  );

  const progressText =
    predictInProgress && predictMeta
      ? `预测中：${predictProgress?.done ?? 0}/${predictProgress?.total ?? 0}` +
        (predictProgress?.current_code
          ? (() => {
              const code = predictProgress.current_code;
              const name = selectedStocks.find((s) => s.code === code)?.name;
              return `，当前：${code}${name ? " " + name : ""}`;
            })()
          : "")
      : "";

  const showProgress = predictInProgress || loading;

  return (
    <div className="prediction-panel">
      <div className="panel-header">
        <h3>预测结果</h3>
      </div>

      <div className="panel-content">
        {predictions.length === 0 ? (
          <Empty
            description={
              showProgress ? (
                <div style={{ minWidth: 260 }}>
                  <div style={{ marginBottom: 8 }}>
                    {progressText || "预测中..."}
                  </div>
                  <Progress percent={Math.round(progressPercent)} />
                </div>
              ) : (
                '选择股票后点击"开始预测"查看结果'
              )
            }
          />
        ) : (
          <div className="predictions-grid">
            {predictions.map((result) => (
              <PredictionCard key={result.stock_code} result={result} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
};

export default PredictionPanel;
