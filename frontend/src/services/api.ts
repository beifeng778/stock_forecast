import axios from 'axios';
import type {
  Stock,
  KlineResponse,
  PredictRequest,
  PredictResponse,
  TradeSimulateRequest,
  TradeSimulateResponse,
} from '../types';

const API_BASE = import.meta.env.VITE_API_BASE || '/api';

const api = axios.create({
  baseURL: API_BASE,
  timeout: 60000,
});

// 获取股票列表
export async function getStocks(keyword?: string): Promise<Stock[]> {
  const response = await api.get('/stocks', {
    params: { keyword },
  });
  return response.data.data || [];
}

// 获取K线数据
export async function getKline(code: string, period: string = 'daily'): Promise<KlineResponse> {
  const response = await api.get(`/stocks/${code}/kline`, {
    params: { period },
  });
  return response.data;
}

// 股票预测
export async function predict(request: PredictRequest): Promise<PredictResponse> {
  const response = await api.post('/predict', request);
  return response.data;
}

// 委托模拟
export async function simulateTrade(request: TradeSimulateRequest): Promise<TradeSimulateResponse> {
  const response = await api.post('/trade/simulate', request);
  return response.data;
}

export default api;
