import React, { useState, useEffect, useCallback } from 'react';
import { Select, Button, Tag, Space, message } from 'antd';
import { SearchOutlined, DeleteOutlined } from '@ant-design/icons';
import { useStockStore } from '../../store';
import { getStocks, predict } from '../../services/api';
import type { Stock } from '../../types';
import './index.css';

const StockSelector: React.FC = () => {
  const [stocks, setStocks] = useState<Stock[]>([]);
  const [searching, setSearching] = useState(false);

  const {
    selectedStocks,
    addStock,
    removeStock,
    clearStocks,
    period,
    setPredictions,
    setLoading,
    loading,
  } = useStockStore();

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
      console.error('搜索股票失败:', error);
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
      message.warning('请先选择股票');
      return;
    }

    setLoading(true);
    try {
      const result = await predict({
        stock_codes: selectedStocks.map((s) => s.code),
        period,
      });
      setPredictions(result.results);
      message.success('预测完成');
    } catch (error) {
      console.error('预测失败:', error);
      message.error('预测失败，请稍后重试');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="stock-selector">
      <div className="selector-header">
        <h3>股票选择</h3>
      </div>

      <div className="selector-search">
        <Select
          showSearch
          placeholder="输入股票代码或名称搜索"
          filterOption={false}
          onSearch={handleSearch}
          onSelect={handleSelect}
          loading={searching}
          notFoundContent={searching ? '搜索中...' : '未找到股票'}
          style={{ width: '100%' }}
          suffixIcon={<SearchOutlined />}
          value={undefined}
        >
          {stocks.map((stock) => (
            <Select.Option key={stock.code} value={stock.code}>
              {stock.code} - {stock.name}
              <Tag color={stock.market === 'SH' ? 'red' : 'blue'} style={{ marginLeft: 8 }}>
                {stock.market === 'SH' ? '沪' : '深'}
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
              <Tag color={stock.market === 'SH' ? 'red' : 'blue'}>
                {stock.market === 'SH' ? '沪' : '深'}
              </Tag>
              <DeleteOutlined
                className="delete-icon"
                onClick={() => removeStock(stock.code)}
              />
            </div>
          ))
        )}
      </div>

      <div className="selector-actions">
        <Space>
          <Button onClick={clearStocks} disabled={selectedStocks.length === 0}>
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
