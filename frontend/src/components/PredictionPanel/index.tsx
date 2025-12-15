import React, { useEffect, useRef } from "react";
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
      {signal.name}: {signal.type_cn}
    </Tag>
  );
};

// 鼠标拖拽滚动Hook
const useDragScroll = (uniqueId?: string) => {
  const elementRef = useRef<HTMLDivElement>(null);
  const isDragging = useRef(false);
  const startX = useRef(0);
  const scrollLeft = useRef(0);
  const hasMoved = useRef(false);

  useEffect(() => {
    const element = elementRef.current;
    if (!element) {
      return;
    }

    // 确保元素有正确的样式
    element.style.cursor = 'grab';
    element.style.userSelect = 'none';
    element.style.overflowX = 'auto';
    element.style.scrollbarWidth = 'none';

    const handleMouseDown = (e: MouseEvent) => {
      console.log(`[${uniqueId}] MouseDown triggered, scrollWidth: ${element.scrollWidth}, clientWidth: ${element.clientWidth}`);

      // 检查是否可以滚动
      if (element.scrollWidth <= element.clientWidth) {
        console.log(`[${uniqueId}] No scroll needed, skipping drag`);
        return;
      }

      console.log(`[${uniqueId}] Starting drag`);
      // 直接开始拖拽，不做复杂的事件检查
      isDragging.current = true;
      hasMoved.current = false;
      startX.current = e.pageX;
      scrollLeft.current = element.scrollLeft;
      element.style.cursor = 'grabbing';
      e.preventDefault();
      e.stopPropagation();
    };

    const handleMouseMove = (e: MouseEvent) => {
      if (!isDragging.current) return;

      e.preventDefault();
      hasMoved.current = true;

      const x = e.pageX;
      const walk = (x - startX.current) * 2; // 增加滚动速度
      const newScrollLeft = scrollLeft.current - walk;

      // 确保滚动值在有效范围内
      const maxScroll = element.scrollWidth - element.clientWidth;
      element.scrollLeft = Math.max(0, Math.min(newScrollLeft, maxScroll));
    };

    const handleMouseUp = () => {
      if (isDragging.current) {
        isDragging.current = false;
        element.style.cursor = 'grab';
      }
    };

    const handleMouseLeave = () => {
      if (isDragging.current) {
        isDragging.current = false;
        element.style.cursor = 'grab';
      }
    };

    // 阻止拖拽时的文本选择
    const handleSelectStart = (e: Event) => {
      if (isDragging.current) {
        e.preventDefault();
      }
    };

    // 阻止拖拽时的默认拖拽行为
    const handleDragStart = (e: DragEvent) => {
      e.preventDefault();
    };

    // 点击事件处理，防止拖拽后触发点击
    const handleClick = (e: MouseEvent) => {
      if (hasMoved.current) {
        e.preventDefault();
        e.stopPropagation();
      }
    };

    // 添加滚轮支持
    const handleWheel = (e: WheelEvent) => {
      if (element.scrollWidth > element.clientWidth) {
        e.preventDefault();
        element.scrollLeft += e.deltaY;
      }
    };

    element.addEventListener('mousedown', handleMouseDown);
    document.addEventListener('mousemove', handleMouseMove);
    document.addEventListener('mouseup', handleMouseUp);
    element.addEventListener('mouseleave', handleMouseLeave);
    element.addEventListener('selectstart', handleSelectStart);
    element.addEventListener('dragstart', handleDragStart);
    element.addEventListener('click', handleClick, true);
    element.addEventListener('wheel', handleWheel, { passive: false });

    return () => {
      element.removeEventListener('mousedown', handleMouseDown);
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleMouseUp);
      element.removeEventListener('mouseleave', handleMouseLeave);
      element.removeEventListener('selectstart', handleSelectStart);
      element.removeEventListener('dragstart', handleDragStart);
      element.removeEventListener('click', handleClick, true);
      element.removeEventListener('wheel', handleWheel);
    };
  }, [uniqueId]);

  return elementRef;
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
            <span className="label">目标价(5日)</span>
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
              key: '1',
              label: '详细分析',
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
                            {(result.ml_predictions.lstm.confidence * 100).toFixed(
                              0
                            )}
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
                      </tbody>
                    </table>
                  </div>

                  <div className="target-prices">
                    <h4>目标价位</h4>
                    <div className="target-grid">
                      <div>短期(5日): {result.target_prices.short.toFixed(2)}</div>
                      <div>
                        中期(20日): {result.target_prices.medium.toFixed(2)}
                      </div>
                      <div>长期(60日): {result.target_prices.long.toFixed(2)}</div>
                    </div>
                  </div>

                  {result.daily_changes &&
                    result.daily_changes.length > 0 &&
                    (() => {
                      // 过滤掉当天未收盘的数据，只取已收盘的最近5天
                      const today = new Date().toISOString().split("T")[0];
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
                                <span className="date">{item.date.slice(5)}</span>
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
              )
            }
          ]}
        />
      </div>
    </Card>
  );
};

const PredictionPanel: React.FC = () => {
  const { predictions, loading } = useStockStore();

  return (
    <div className="prediction-panel">
      <div className="panel-header">
        <h3>预测结果</h3>
      </div>

      <div className="panel-content">
        {predictions.length === 0 ? (
          <Empty
            description={
              loading ? "预测中..." : '选择股票后点击"开始预测"查看结果'
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
