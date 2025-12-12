import akshare as ak
import pandas as pd
from typing import List, Dict, Optional
from functools import lru_cache
import numpy as np
import os
import time
import requests


class StockDataService:
    """股票数据服务"""

    @staticmethod
    @lru_cache(maxsize=1)
    def get_stock_list() -> List[Dict]:
        """获取A股股票列表（仅600和000开头）"""
        try:
            # 获取沪市股票
            sh_stocks = ak.stock_info_sh_name_code()
            sh_stocks = sh_stocks[sh_stocks['证券代码'].str.startswith('6')]
            sh_list = [
                {"code": row['证券代码'], "name": row['证券简称'], "market": "SH"}
                for _, row in sh_stocks.iterrows()
            ]

            # 获取深市股票
            sz_stocks = ak.stock_info_sz_name_code()
            sz_stocks = sz_stocks[sz_stocks['A股代码'].str.startswith('0')]
            sz_list = [
                {"code": row['A股代码'], "name": row['A股简称'], "market": "SZ"}
                for _, row in sz_stocks.iterrows()
            ]

            return sh_list + sz_list
        except Exception as e:
            print(f"获取股票列表失败: {e}")
            return []

    @staticmethod
    def search_stocks(keyword: str) -> List[Dict]:
        """搜索股票"""
        all_stocks = StockDataService.get_stock_list()
        if not keyword:
            # 返回所有股票（沪市+深市）
            return all_stocks

        keyword = keyword.upper()
        return [
            s for s in all_stocks
            if keyword in s['code'] or keyword in s['name'].upper()
        ][:100]

    @staticmethod
    def get_stock_info(code: str) -> Optional[Dict]:
        """获取股票基本信息"""
        all_stocks = StockDataService.get_stock_list()
        for stock in all_stocks:
            if stock['code'] == code:
                return stock
        return None

    @staticmethod
    def get_kline(code: str, period: str = "daily") -> pd.DataFrame:
        """获取K线数据，优先使用新浪接口"""
        # 先尝试新浪接口（更稳定）
        print(f"尝试获取 {code} 的K线数据...")

        df = StockDataService._get_kline_sina(code, period)
        if df is not None and not df.empty:
            print(f"新浪接口成功获取 {code} 数据，共 {len(df)} 条")
            return df

        # 新浪失败，尝试 AKShare
        df = StockDataService._get_kline_akshare(code, period)
        if df is not None and not df.empty:
            print(f"AKShare 成功获取 {code} 数据，共 {len(df)} 条")
            return df

        print(f"所有方法都无法获取 {code} 的K线数据")
        return pd.DataFrame()

    @staticmethod
    def _get_kline_akshare(code: str, period: str) -> pd.DataFrame:
        """使用 AKShare 获取K线"""
        for retry in range(3):
            try:
                time.sleep(0.3 * retry)

                if period == "weekly":
                    df = ak.stock_zh_a_hist(symbol=code, period="weekly", adjust="qfq")
                elif period == "monthly":
                    df = ak.stock_zh_a_hist(symbol=code, period="monthly", adjust="qfq")
                else:
                    df = ak.stock_zh_a_hist(symbol=code, period="daily", adjust="qfq")

                if df is not None and not df.empty:
                    df = df.rename(columns={
                        '日期': 'date',
                        '开盘': 'open',
                        '收盘': 'close',
                        '最高': 'high',
                        '最低': 'low',
                        '成交量': 'volume',
                        '成交额': 'amount'
                    })
                    columns = ['date', 'open', 'close', 'high', 'low', 'volume', 'amount']
                    df = df[[c for c in columns if c in df.columns]]
                    df['date'] = pd.to_datetime(df['date']).dt.strftime('%Y-%m-%d')
                    return df.tail(250)

            except Exception as e:
                print(f"AKShare 重试 {retry+1}/3 失败: {e}")

        return pd.DataFrame()

    @staticmethod
    def _get_kline_sina(code: str, period: str) -> pd.DataFrame:
        """使用新浪接口获取K线"""
        try:
            # 确定市场前缀
            if code.startswith('6'):
                symbol = f"sh{code}"
            else:
                symbol = f"sz{code}"

            # 新浪K线接口
            scale_map = {"daily": "240", "weekly": "1680", "monthly": "7200"}
            scale = scale_map.get(period, "240")

            url = f"https://quotes.sina.cn/cn/api/jsonp_v2.php/var%20_{symbol}_{scale}/CN_MarketDataService.getKLineData"
            params = {
                "symbol": symbol,
                "scale": scale,
                "ma": "no",
                "datalen": "250"
            }

            headers = {
                "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
                "Referer": "https://finance.sina.com.cn",
                "Accept": "*/*",
            }

            print(f"请求新浪接口: {symbol}")
            session = requests.Session()
            session.trust_env = False  # 禁用环境变量中的代理
            response = session.get(url, params=params, headers=headers, timeout=15)
            print(f"新浪接口响应状态: {response.status_code}")

            if response.status_code == 200:
                text = response.text
                start = text.find('(') + 1
                end = text.rfind(')')
                if start > 0 and end > start:
                    import json
                    json_str = text[start:end]
                    data = json.loads(json_str)

                    if data and len(data) > 0:
                        df = pd.DataFrame(data)
                        df = df.rename(columns={
                            'day': 'date',
                            'open': 'open',
                            'close': 'close',
                            'high': 'high',
                            'low': 'low',
                            'volume': 'volume'
                        })
                        df['amount'] = 0
                        df['open'] = pd.to_numeric(df['open'], errors='coerce')
                        df['close'] = pd.to_numeric(df['close'], errors='coerce')
                        df['high'] = pd.to_numeric(df['high'], errors='coerce')
                        df['low'] = pd.to_numeric(df['low'], errors='coerce')
                        df['volume'] = pd.to_numeric(df['volume'], errors='coerce')
                        print(f"新浪接口解析成功，共 {len(df)} 条数据")
                        return df
                    else:
                        print("新浪接口返回空数据")
                else:
                    print(f"新浪接口响应格式异常: {text[:200]}")
            else:
                print(f"新浪接口请求失败: {response.status_code}")

        except Exception as e:
            print(f"新浪接口获取失败: {e}")

        return pd.DataFrame()

    @staticmethod
    def get_realtime_quote(code: str) -> Optional[Dict]:
        """获取实时行情"""
        try:
            df = ak.stock_zh_a_spot_em()
            stock = df[df['代码'] == code]
            if stock.empty:
                return None

            row = stock.iloc[0]
            return {
                "code": code,
                "name": row['名称'],
                "price": float(row['最新价']),
                "change": float(row['涨跌额']),
                "change_pct": float(row['涨跌幅']),
                "open": float(row['今开']),
                "high": float(row['最高']),
                "low": float(row['最低']),
                "volume": float(row['成交量']),
                "amount": float(row['成交额']),
            }
        except Exception as e:
            print(f"获取实时行情失败: {e}")
            return None


    @staticmethod
    def get_stock_news(code: str, limit: int = 5) -> List[Dict]:
        """获取股票公告/新闻（使用东方财富公告接口）"""
        import json
        try:
            url = f"https://np-anotice-stock.eastmoney.com/api/security/ann?sr=-1&page_size={limit}&page_index=1&ann_type=A&stock_list={code}&f_node=0&s_node=0"
            headers = {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"}

            session = requests.Session()
            session.trust_env = False
            resp = session.get(url, headers=headers, timeout=10)
            data = resp.json()

            news_list = []
            items = data.get('data', {}).get('list', [])
            for item in items[:limit]:
                news_list.append({
                    "title": item.get('title', ''),
                    "time": item.get('notice_date', '')[:10] if item.get('notice_date') else '',
                    "source": "东方财富",
                })
            return news_list
        except Exception as e:
            print(f"获取股票新闻失败: {e}")
        return []


stock_data_service = StockDataService()
