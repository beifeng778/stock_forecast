import React, { useState, useEffect, useCallback } from "react";
import { Select, Button, Tag, Space, message, Spin, Modal } from "antd";
import {
  SearchOutlined,
  DeleteOutlined,
  ReloadOutlined,
} from "@ant-design/icons";
import { useStockStore } from "../../store";
import { getStocks, predict } from "../../services/api";
import type { Stock } from "../../types";
import "./index.css";

const StockSelector: React.FC = () => {
  const [stocks, setStocks] = useState<Stock[]>([]);
  const [allStocks, setAllStocks] = useState<Stock[]>([]);
  const [searching, setSearching] = useState(false);
  const [initialLoading, setInitialLoading] = useState(true);

  const {
    selectedStocks,
    addStock,
    removeStock,
    period,
    setPredictions,
    setLoading,
    loading,
    clearPredictions,
    clearTradeData,
    clearAllData,
  } = useStockStore();

  // 首次加载股票列表
  const loadStockList = useCallback(async () => {
    setInitialLoading(true);
    try {
      const result = await getStocks("");
      setAllStocks(result || []);
    } catch (error) {
      console.error("加载股票列表失败:", error);
      message.error("加载股票列表失败");
      setAllStocks([]);
    } finally {
      setInitialLoading(false);
    }
  }, []);

  // 组件挂载时加载股票列表
  useEffect(() => {
    loadStockList();
  }, [loadStockList]);

  // 搜索股票
  const handleSearch = useCallback(async (keyword: string) => {
    if (!keyword || keyword.length < 1) {
      setStocks([]);
      return;
    }

    setSearching(true);
    try {
      const result = await getStocks(keyword);
      setStocks(result || []);
    } catch (error) {
      console.error("搜索股票失败:", error);
      setStocks([]);
    } finally {
      setSearching(false);
    }
  }, []);

  // 选择股票
  const handleSelect = (value: string) => {
    const stock = stocks.find((s) => s.code === value);
    if (stock) {
      addStock(stock);
    }
  };

  // 开始预测
  const handlePredict = async () => {
    if (selectedStocks.length === 0) {
      message.warning("请先选择股票");
      return;
    }

    // 清空之前的预测结果和交易数据
    clearPredictions();
    clearTradeData();

    setLoading(true);
    try {
      const result = await predict({
        stock_codes: selectedStocks.map((s) => s.code),
        period,
      });
      setPredictions(result.results);
      message.success("预测完成");
    } catch (error) {
      console.error("预测失败:", error);
      message.error("预测失败，请稍后重试");
    } finally {
      setLoading(false);
    }
  };

  // 移除股票时清空交易数据
  const handleRemoveStock = (code: string) => {
    removeStock(code);
    clearTradeData();
  };

  // 清空所有股票（二次确认）
  const handleClearAll = () => {
    Modal.confirm({
      title: "确认清空",
      content:
        "确定要清空所有股票和相关数据吗？这将清除选中的股票、预测结果和盈亏分析数据。",
      okText: "确定",
      cancelText: "取消",
      okButtonProps: { danger: true },
      onOk: () => {
        clearAllData();
        message.success("已清空所有数据");
      },
    });
  };

  return (
    <div className="stock-selector">
      <div className="selector-header">
        <h3>股票选择</h3>
        <Space>
          {!initialLoading && (
            <span className="stock-count">共 {allStocks.length} 只股票</span>
          )}
          <Button
            className="refresh-btn"
            icon={<ReloadOutlined spin={initialLoading} />}
            onClick={loadStockList}
            disabled={initialLoading}
            title="刷新股票列表"
          />
        </Space>
      </div>

      {initialLoading ? (
        <div className="loading-container">
          <Spin />
          <span className="loading-text">正在加载股票列表...</span>
        </div>
      ) : (
        <>
          <div className="selector-search">
            <Select
              showSearch
              placeholder="输入股票代码或名称搜索"
              filterOption={false}
              onSearch={handleSearch}
              onSelect={handleSelect}
              loading={searching}
              notFoundContent={searching ? "搜索中..." : "未找到股票"}
              style={{ width: "100%" }}
              suffixIcon={<SearchOutlined />}
              value={undefined}
            >
              {stocks.map((stock) => (
                <Select.Option key={stock.code} value={stock.code}>
                  {stock.code} - {stock.name}
                  <Tag
                    color={stock.market === "SH" ? "red" : "blue"}
                    style={{ marginLeft: 8 }}
                  >
                    {stock.market === "SH" ? "沪" : "深"}
                  </Tag>
                </Select.Option>
              ))}
            </Select>
          </div>

          <div className="selected-stocks">
            {selectedStocks.length === 0 ? (
              <div className="empty-tip">请搜索并选择股票</div>
            ) : (
              selectedStocks.map((stock) => (
                <div key={stock.code} className="stock-item">
                  <span className="stock-code">{stock.code}</span>
                  <span className="stock-name">{stock.name}</span>
                  <Tag color={stock.market === "SH" ? "red" : "blue"}>
                    {stock.market === "SH" ? "沪" : "深"}
                  </Tag>
                  <DeleteOutlined
                    className="delete-icon"
                    onClick={() => handleRemoveStock(stock.code)}
                  />
                </div>
              ))
            )}
          </div>
        </>
      )}

      <div className="selector-actions">
        <Space>
          <Button
            onClick={handleClearAll}
            disabled={selectedStocks.length === 0}
          >
            清空
          </Button>
          <Button
            type="primary"
            onClick={handlePredict}
            loading={loading}
            disabled={selectedStocks.length === 0}
          >
            开始预测
          </Button>
        </Space>
      </div>
    </div>
  );
};

export default StockSelector;
