import pandas as pd
import numpy as np
from typing import Dict, Optional
import ta


class TechnicalIndicators:
    """技术指标计算服务"""

    @staticmethod
    def calculate_all(df: pd.DataFrame) -> Dict:
        """计算所有技术指标"""
        if df.empty or len(df) < 60:
            return {}

        result = {}

        # 确保数据类型正确
        df['close'] = pd.to_numeric(df['close'], errors='coerce')
        df['high'] = pd.to_numeric(df['high'], errors='coerce')
        df['low'] = pd.to_numeric(df['low'], errors='coerce')
        df['volume'] = pd.to_numeric(df['volume'], errors='coerce')

        # 当前价格
        result['current_price'] = float(df['close'].iloc[-1])

        # 均线
        result['ma5'] = float(df['close'].rolling(5).mean().iloc[-1])
        result['ma10'] = float(df['close'].rolling(10).mean().iloc[-1])
        result['ma20'] = float(df['close'].rolling(20).mean().iloc[-1])
        result['ma60'] = float(df['close'].rolling(60).mean().iloc[-1])

        # MACD
        macd = ta.trend.MACD(df['close'])
        result['macd'] = float(macd.macd().iloc[-1]) if not pd.isna(macd.macd().iloc[-1]) else 0
        result['signal'] = float(macd.macd_signal().iloc[-1]) if not pd.isna(macd.macd_signal().iloc[-1]) else 0
        result['hist'] = float(macd.macd_diff().iloc[-1]) if not pd.isna(macd.macd_diff().iloc[-1]) else 0

        # RSI
        rsi = ta.momentum.RSIIndicator(df['close'])
        result['rsi'] = float(rsi.rsi().iloc[-1]) if not pd.isna(rsi.rsi().iloc[-1]) else 50

        # KDJ
        stoch = ta.momentum.StochasticOscillator(df['high'], df['low'], df['close'])
        result['kdj_k'] = float(stoch.stoch().iloc[-1]) if not pd.isna(stoch.stoch().iloc[-1]) else 50
        result['kdj_d'] = float(stoch.stoch_signal().iloc[-1]) if not pd.isna(stoch.stoch_signal().iloc[-1]) else 50
        result['kdj_j'] = 3 * result['kdj_k'] - 2 * result['kdj_d']

        # 布林带
        boll = ta.volatility.BollingerBands(df['close'])
        result['boll_upper'] = float(boll.bollinger_hband().iloc[-1]) if not pd.isna(boll.bollinger_hband().iloc[-1]) else 0
        result['boll_middle'] = float(boll.bollinger_mavg().iloc[-1]) if not pd.isna(boll.bollinger_mavg().iloc[-1]) else 0
        result['boll_lower'] = float(boll.bollinger_lband().iloc[-1]) if not pd.isna(boll.bollinger_lband().iloc[-1]) else 0

        # 支撑位和压力位（简单计算）
        recent_low = df['low'].tail(20).min()
        recent_high = df['high'].tail(20).max()
        result['support_level'] = float(recent_low)
        result['resistance_level'] = float(recent_high)

        return result

    @staticmethod
    def get_signals(indicators: Dict) -> list:
        """根据指标生成信号"""
        signals = []

        # MACD信号
        if indicators.get('macd', 0) > indicators.get('signal', 0):
            signals.append({"name": "MACD", "type": "bullish", "desc": "金叉"})
        else:
            signals.append({"name": "MACD", "type": "bearish", "desc": "死叉"})

        # RSI信号
        rsi = indicators.get('rsi', 50)
        if rsi > 70:
            signals.append({"name": "RSI", "type": "bearish", "desc": "超买"})
        elif rsi < 30:
            signals.append({"name": "RSI", "type": "bullish", "desc": "超卖"})
        else:
            signals.append({"name": "RSI", "type": "neutral", "desc": "中性"})

        # KDJ信号
        kdj_j = indicators.get('kdj_j', 50)
        if kdj_j > 80:
            signals.append({"name": "KDJ", "type": "bearish", "desc": "超买"})
        elif kdj_j < 20:
            signals.append({"name": "KDJ", "type": "bullish", "desc": "超卖"})
        else:
            signals.append({"name": "KDJ", "type": "neutral", "desc": "中性"})

        # 均线信号
        price = indicators.get('current_price', 0)
        ma5 = indicators.get('ma5', 0)
        ma20 = indicators.get('ma20', 0)
        if price > ma5 > ma20:
            signals.append({"name": "均线", "type": "bullish", "desc": "多头排列"})
        elif price < ma5 < ma20:
            signals.append({"name": "均线", "type": "bearish", "desc": "空头排列"})
        else:
            signals.append({"name": "均线", "type": "neutral", "desc": "交织"})

        return signals


indicators_service = TechnicalIndicators()
