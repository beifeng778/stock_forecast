import { create } from 'zustand';
import type { Stock, PredictResult, PeriodType } from '../types';

// 预测K线数据类型
export interface PredictionKline {
  date: string;
  open: number;
  close: number;
  high: number;
  low: number;
  volume: number;
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
}

export const useStockStore = create<StockStore>((set, get) => ({
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

  period: 'daily',
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
}));
