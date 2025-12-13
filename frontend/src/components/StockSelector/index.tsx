import React, { useState, useEffect, useCallback, useRef } from "react";
import {
  Select,
  Button,
  Tag,
  Space,
  message,
  Spin,
  Modal,
  Tooltip,
} from "antd";
import {
  SearchOutlined,
  DeleteOutlined,
  ReloadOutlined,
} from "@ant-design/icons";
import { useStockStore } from "../../store";
import { getStocks, predict, validateBeforeAction } from "../../services/api";
import type { Stock } from "../../types";
import "./index.css";

const REFRESH_COOLDOWN = 5 * 60; // 5分钟冷却时间（秒）

const StockSelector: React.FC = () => {
  const [stocks, setStocks] = useState<Stock[]>([]);
  const [allStocks, setAllStocks] = useState<Stock[]>([]);
  const [searching, setSearching] = useState(false);
  const [initialLoading, setInitialLoading] = useState(true);
  const [cooldown, setCooldown] = useState(0); // 刷新冷却倒计时
  const cooldownTimer = useRef<ReturnType<typeof setInterval> | null>(null);
  const stockListRef = useRef<HTMLDivElement>(null);
  const isComposing = useRef(false); // 输入法组合状态
  const searchTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [searchValue, setSearchValue] = useState<string>("");
  const [dropdownOpen, setDropdownOpen] = useState(false);

  // 下拉框打开时阻止页面滚动
  useEffect(() => {
    if (!dropdownOpen) return;

    const handleTouchMove = (e: TouchEvent) => {
      const dropdown = document.querySelector(".stock-search-dropdown");
      if (!dropdown) return;

      // 如果触摸点不在下拉框内，阻止滚动
      if (!dropdown.contains(e.target as Node)) {
        e.preventDefault();
      }
    };

    document.addEventListener("touchmove", handleTouchMove, { passive: false });

    return () => {
      document.removeEventListener("touchmove", handleTouchMove);
    };
  }, [dropdownOpen]);

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
    updateStockNames,
  } = useStockStore();

  // 开始冷却倒计时
  const startCooldown = useCallback((seconds: number = REFRESH_COOLDOWN) => {
    setCooldown(seconds);
    // 保存冷却结束时间到localStorage
    const endTime = Date.now() + seconds * 1000;
    localStorage.setItem("stockRefreshCooldownEnd", String(endTime));

    if (cooldownTimer.current) {
      clearInterval(cooldownTimer.current);
    }
    cooldownTimer.current = setInterval(() => {
      setCooldown((prev) => {
        if (prev <= 1) {
          if (cooldownTimer.current) {
            clearInterval(cooldownTimer.current);
            cooldownTimer.current = null;
          }
          localStorage.removeItem("stockRefreshCooldownEnd");
          return 0;
        }
        return prev - 1;
      });
    }, 1000);
  }, []);

  // 页面加载时恢复冷却状态
  useEffect(() => {
    const savedEndTime = localStorage.getItem("stockRefreshCooldownEnd");
    if (savedEndTime) {
      const remaining = Math.ceil((Number(savedEndTime) - Date.now()) / 1000);
      if (remaining > 0) {
        startCooldown(remaining);
      } else {
        localStorage.removeItem("stockRefreshCooldownEnd");
      }
    }
  }, [startCooldown]);

  // 清理定时器
  useEffect(() => {
    return () => {
      if (cooldownTimer.current) {
        clearInterval(cooldownTimer.current);
      }
    };
  }, []);

  // 加载股票列表（支持强制刷新）
  const loadStockList = useCallback(
    async (forceRefresh = false) => {
      setInitialLoading(true);
      try {
        const result = await getStocks("", forceRefresh);
        setAllStocks(result || []);
        // 同步更新已选择股票和预测结果中的名称
        if (result && result.length > 0) {
          const stockMap = new Map(result.map((s) => [s.code, s]));
          updateStockNames(stockMap);
        }
        if (forceRefresh) {
          message.success("股票列表已增量更新");
          startCooldown(); // 刷新后开始冷却
        }
      } catch (error: unknown) {
        // 401错误会触发页面刷新，不需要显示错误提示和倒计时
        const axiosError = error as { response?: { status?: number } };
        if (axiosError.response?.status === 401) {
          return;
        }
        console.error("加载股票列表失败:", error);
        message.error("第三方数据接口异常，请稍后再试");
        // 失败时启动2分钟倒计时
        startCooldown(120);
      } finally {
        setInitialLoading(false);
      }
    },
    [startCooldown, updateStockNames]
  );

  // 组件挂载时加载股票列表
  useEffect(() => {
    loadStockList();
  }, [loadStockList]);

  // 实际执行搜索
  const doSearch = useCallback(async (keyword: string) => {
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

  // 搜索股票（带防抖，处理输入法组合状态）
  const handleSearch = useCallback(
    (keyword: string) => {
      // 清除之前的定时器
      if (searchTimer.current) {
        clearTimeout(searchTimer.current);
      }

      // 如果正在输入法组合中，不触发搜索
      if (isComposing.current) {
        return;
      }

      // 防抖300ms
      searchTimer.current = setTimeout(() => {
        doSearch(keyword);
      }, 300);
    },
    [doSearch]
  );

  // 选择股票
  const handleSelect = (value: string) => {
    const stock = stocks.find((s) => s.code === value);
    if (stock) {
      addStock(stock);
    }
    // 选择后清空搜索结果和搜索框
    setStocks([]);
    setSearchValue("");
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

  // 清空所有股票（二次确认，需要验证token）
  const handleClearAll = async () => {
    // 清空操作没有调用后端接口，需要先验证token
    const valid = await validateBeforeAction();
    if (!valid) return;

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
        <Space size="middle">
          {!initialLoading && (
            <span className="stock-count">共 {allStocks.length} 只股票</span>
          )}
          <Tooltip title="数据来自第三方接口，可能存在延迟">
            <Button
              className="refresh-btn"
              icon={<ReloadOutlined spin={initialLoading} />}
              onClick={() => loadStockList(true)}
              disabled={initialLoading || loading || cooldown > 0}
            >
              {cooldown > 0
                ? `${Math.floor(cooldown / 60)}:${String(
                    cooldown % 60
                  ).padStart(2, "0")}`
                : null}
            </Button>
          </Tooltip>
        </Space>
      </div>

      {initialLoading ? (
        <div className="loading-container">
          <Spin />
          <span className="loading-text">正在加载股票列表...</span>
        </div>
      ) : (
        <>
          <div
            className="selector-search"
            onCompositionStart={() => {
              isComposing.current = true;
            }}
            onCompositionEnd={(e) => {
              isComposing.current = false;
              // 组合结束后立即触发搜索
              const target = e.target as HTMLInputElement;
              if (target.value) {
                doSearch(target.value);
              }
            }}
          >
            <Select
              showSearch
              placeholder="输入股票代码或名称搜索"
              filterOption={false}
              onSearch={(val) => {
                setSearchValue(val);
                handleSearch(val);
                // 有输入时打开下拉框
                if (val) {
                  setDropdownOpen(true);
                }
              }}
              onSelect={(val) => {
                if (val) {
                  handleSelect(val as string);
                }
                // 选择后关闭下拉框
                setDropdownOpen(false);
              }}
              loading={searching}
              notFoundContent={searching ? "搜索中..." : "未找到股票"}
              style={{ width: "100%" }}
              suffixIcon={<SearchOutlined />}
              value={undefined}
              searchValue={searchValue}
              popupClassName="stock-search-dropdown"
              autoClearSearchValue={false}
              open={dropdownOpen}
              onDropdownVisibleChange={(open) => {
                // 只有在有搜索结果时才允许打开
                if (open && stocks.length === 0 && !searchValue) {
                  return;
                }
                setDropdownOpen(open);
              }}
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

          <div className="selected-stocks" ref={stockListRef}>
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
            disabled={selectedStocks.length === 0 || initialLoading || loading}
          >
            清空
          </Button>
          <Button
            type="primary"
            onClick={handlePredict}
            loading={loading}
            disabled={
              selectedStocks.length === 0 ||
              initialLoading ||
              allStocks.length === 0
            }
          >
            开始预测
          </Button>
        </Space>
      </div>
    </div>
  );
};

export default StockSelector;
