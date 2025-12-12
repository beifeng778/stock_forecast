import { create } from "zustand";
import { persist } from "zustand/middleware";
import type {
  Stock,
  PredictResult,
  PeriodType,
  TradeSimulateResponse,
} from "../types";

// 预测K线数据类型
export interface PredictionKline {
  date: string;
  open: number;
  close: number;
  high: number;
  low: number;
  volume: number;
}

// 交易模拟表单数据
export interface TradeFormData {
  stock_code?: string;
  buy_price?: number;
  buy_date?: string;
  expected_price?: number;
  sell_date?: string;
  quantity?: number;
}

interface StockStore {
  // 选中的股票
  selectedStocks: Stock[];
  setSelectedStocks: (stocks: Stock[]) => void;
  addStock: (stock: Stock) => void;
  removeStock: (code: string) => void;
  clearStocks: () => void;

  // 当前周期
  period: PeriodType;
  setPeriod: (period: PeriodType) => void;

  // 预测结果
  predictions: PredictResult[];
  setPredictions: (predictions: PredictResult[]) => void;
  clearPredictions: () => void;

  // 加载状态
  loading: boolean;
  setLoading: (loading: boolean) => void;

  // 已预测的股票代码（用于委托模拟筛选）
  predictedCodes: string[];

  // 预测K线数据（按股票代码存储）
  predictionKlines: Record<string, PredictionKline[]>;
  setPredictionKlines: (code: string, klines: PredictionKline[]) => void;

  // 交易模拟表单数据
  tradeFormData: TradeFormData;
  setTradeFormData: (data: TradeFormData) => void;

  // 交易模拟结果
  tradeResult: TradeSimulateResponse | null;
  setTradeResult: (result: TradeSimulateResponse | null) => void;

  // 是否包含未来日期（用于盈亏分析显示）
  tradeHasFutureDate: boolean;
  setTradeHasFutureDate: (value: boolean) => void;

  // 清空交易模拟相关数据
  clearTradeData: () => void;

  // 清空所有数据（股票、预测、交易）
  clearAllData: () => void;
}

export const useStockStore = create<StockStore>()(
  persist(
    (set, get) => ({
      selectedStocks: [],
      setSelectedStocks: (stocks) => set({ selectedStocks: stocks }),
      addStock: (stock) => {
        const { selectedStocks } = get();
        if (!selectedStocks.find((s) => s.code === stock.code)) {
          set({ selectedStocks: [...selectedStocks, stock] });
        }
      },
      removeStock: (code) => {
        const { selectedStocks } = get();
        set({ selectedStocks: selectedStocks.filter((s) => s.code !== code) });
      },
      clearStocks: () => set({ selectedStocks: [] }),

      period: "daily",
      setPeriod: (period) => set({ period }),

      predictions: [],
      setPredictions: (predictions) => {
        const codes = predictions.map((p) => p.stock_code);
        set({ predictions, predictedCodes: codes });
      },
      clearPredictions: () => set({ predictions: [], predictedCodes: [] }),

      loading: false,
      setLoading: (loading) => set({ loading }),

      predictedCodes: [],

      predictionKlines: {},
      setPredictionKlines: (code, klines) => {
        const { predictionKlines } = get();
        set({ predictionKlines: { ...predictionKlines, [code]: klines } });
      },

      tradeFormData: { quantity: 100 },
      setTradeFormData: (data) => {
        const { tradeFormData } = get();
        set({ tradeFormData: { ...tradeFormData, ...data } });
      },

      tradeResult: null,
      setTradeResult: (result) => set({ tradeResult: result }),

      tradeHasFutureDate: false,
      setTradeHasFutureDate: (value) => set({ tradeHasFutureDate: value }),

      clearTradeData: () =>
        set({
          tradeFormData: { quantity: 100 },
          tradeResult: null,
          tradeHasFutureDate: false,
        }),

      clearAllData: () =>
        set({
          selectedStocks: [],
          predictions: [],
          predictedCodes: [],
          predictionKlines: {},
          tradeFormData: { quantity: 100 },
          tradeResult: null,
          tradeHasFutureDate: false,
        }),
    }),
    {
      name: "stock-forecast-storage",
      partialize: (state) => ({
        selectedStocks: state.selectedStocks,
        predictions: state.predictions,
        predictedCodes: state.predictedCodes,
        predictionKlines: state.predictionKlines,
        period: state.period,
        tradeFormData: state.tradeFormData,
        tradeResult: state.tradeResult,
        tradeHasFutureDate: state.tradeHasFutureDate,
      }),
    }
  )
);
