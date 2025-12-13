import axios from "axios";
import type {
  Stock,
  KlineResponse,
  PredictRequest,
  PredictResponse,
  TradeSimulateRequest,
  TradeSimulateResponse,
} from "../types";

const API_BASE = import.meta.env.VITE_API_BASE || "/api";

const api = axios.create({
  baseURL: API_BASE,
  timeout: 60000,
});

// 请求拦截器：添加 token
api.interceptors.request.use((config) => {
  const token = localStorage.getItem("auth_token");
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// 响应拦截器：处理 401 错误
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      // token 无效或过期，清除本地存储并刷新页面
      localStorage.removeItem("auth_token");
      localStorage.removeItem("invite_verified");
      window.location.reload();
    }
    return Promise.reject(error);
  }
);

// 获取股票列表
export async function getStocks(
  keyword?: string,
  refresh?: boolean
): Promise<Stock[]> {
  const response = await api.get("/stocks", {
    params: { keyword, refresh: refresh ? "1" : undefined },
  });
  return response.data.data || [];
}

// 获取K线数据
export async function getKline(
  code: string,
  period: string = "daily"
): Promise<KlineResponse> {
  const response = await api.get(`/stocks/${code}/kline`, {
    params: { period },
  });
  return response.data;
}

// 股票预测
export async function predict(
  request: PredictRequest
): Promise<PredictResponse> {
  const response = await api.post("/predict", request);
  return response.data;
}

// 委托模拟
export async function simulateTrade(
  request: TradeSimulateRequest
): Promise<TradeSimulateResponse> {
  const response = await api.post("/trade/simulate", request);
  return response.data;
}

// 验证邀请码
export async function verifyInviteCode(
  code: string
): Promise<{ success: boolean; message: string; token?: string }> {
  const response = await api.post("/auth/verify", { code });
  return response.data;
}

// 检查token是否有效
export async function checkToken(): Promise<{
  valid: boolean;
  message: string;
}> {
  const response = await api.get("/auth/check");
  return response.data;
}

// 操作前验证token（用于没有接口的按钮）
export async function validateBeforeAction(): Promise<boolean> {
  try {
    const result = await checkToken();
    if (!result.valid) {
      // token无效，清除本地存储并刷新页面
      localStorage.removeItem("auth_token");
      localStorage.removeItem("invite_verified");
      window.location.reload();
      return false;
    }
    return true;
  } catch {
    return false;
  }
}

export default api;
