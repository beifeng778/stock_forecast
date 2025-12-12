import { create } from 'zustand';
import type { Stock, PredictResult, PeriodType } from '../types';

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
}));
