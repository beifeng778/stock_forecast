import pandas as pd
import numpy as np
from typing import Dict, Optional
from sklearn.preprocessing import MinMaxScaler
from sklearn.ensemble import GradientBoostingClassifier
import warnings

warnings.filterwarnings('ignore')


class MLPredictService:
    """机器学习预测服务"""

    def __init__(self):
        self.scaler = MinMaxScaler()

    def predict_all(self, df: pd.DataFrame) -> Dict:
        """运行所有预测模型"""
        if df.empty or len(df) < 60:
            return self._empty_result()

        result = {}

        # LSTM预测
        lstm_result = self._lstm_predict(df)
        result.update({
            'lstm_trend': lstm_result['trend'],
            'lstm_price': lstm_result['price'],
            'lstm_confidence': lstm_result['confidence']
        })

        # Prophet预测
        prophet_result = self._prophet_predict(df)
        result.update({
            'prophet_trend': prophet_result['trend'],
            'prophet_price': prophet_result['price'],
            'prophet_confidence': prophet_result['confidence']
        })

        # XGBoost预测
        xgb_result = self._xgboost_predict(df)
        result.update({
            'xgboost_trend': xgb_result['trend'],
            'xgboost_price': xgb_result['price'],
            'xgboost_confidence': xgb_result['confidence']
        })

        return result

    def _lstm_predict(self, df: pd.DataFrame) -> Dict:
        """LSTM预测（简化版本，使用移动平均模拟）"""
        try:
            close = df['close'].values

            # 使用指数移动平均预测
            ema_short = pd.Series(close).ewm(span=5).mean().iloc[-1]
            ema_long = pd.Series(close).ewm(span=20).mean().iloc[-1]

            current_price = close[-1]

            # 预测价格（基于趋势外推）
            trend_factor = (ema_short - ema_long) / ema_long
            predicted_price = current_price * (1 + trend_factor * 0.5)

            # 判断趋势
            if predicted_price > current_price * 1.01:
                trend = "up"
            elif predicted_price < current_price * 0.99:
                trend = "down"
            else:
                trend = "sideways"

            # 计算置信度（基于趋势一致性）
            recent_returns = np.diff(close[-10:]) / close[-11:-1]
            consistency = np.sum(recent_returns > 0) / len(recent_returns) if trend == "up" else np.sum(recent_returns < 0) / len(recent_returns)
            confidence = min(0.9, max(0.3, consistency))

            return {
                'trend': trend,
                'price': float(predicted_price),
                'confidence': float(confidence)
            }
        except Exception as e:
            print(f"LSTM预测失败: {e}")
            return {'trend': 'sideways', 'price': float(df['close'].iloc[-1]), 'confidence': 0.5}

    def _prophet_predict(self, df: pd.DataFrame) -> Dict:
        """Prophet预测（简化版本，使用线性回归模拟）"""
        try:
            close = df['close'].values[-30:]  # 使用最近30天数据

            # 简单线性回归
            x = np.arange(len(close))
            slope, intercept = np.polyfit(x, close, 1)

            # 预测未来5天
            future_x = len(close) + 5
            predicted_price = slope * future_x + intercept

            current_price = close[-1]

            # 判断趋势
            if slope > 0 and predicted_price > current_price * 1.01:
                trend = "up"
            elif slope < 0 and predicted_price < current_price * 0.99:
                trend = "down"
            else:
                trend = "sideways"

            # 计算置信度（基于R²）
            y_pred = slope * x + intercept
            ss_res = np.sum((close - y_pred) ** 2)
            ss_tot = np.sum((close - np.mean(close)) ** 2)
            r2 = 1 - (ss_res / ss_tot) if ss_tot > 0 else 0
            confidence = min(0.9, max(0.3, abs(r2)))

            return {
                'trend': trend,
                'price': float(predicted_price),
                'confidence': float(confidence)
            }
        except Exception as e:
            print(f"Prophet预测失败: {e}")
            return {'trend': 'sideways', 'price': float(df['close'].iloc[-1]), 'confidence': 0.5}

    def _xgboost_predict(self, df: pd.DataFrame) -> Dict:
        """XGBoost分类预测"""
        try:
            # 准备特征
            df_features = df.copy()
            df_features['return_1'] = df_features['close'].pct_change(1)
            df_features['return_5'] = df_features['close'].pct_change(5)
            df_features['return_10'] = df_features['close'].pct_change(10)
            df_features['ma5'] = df_features['close'].rolling(5).mean()
            df_features['ma20'] = df_features['close'].rolling(20).mean()
            df_features['ma_ratio'] = df_features['ma5'] / df_features['ma20']
            df_features['volatility'] = df_features['close'].rolling(10).std()

            # 目标变量：未来5天涨跌
            df_features['target'] = (df_features['close'].shift(-5) > df_features['close']).astype(int)

            # 删除NaN
            df_features = df_features.dropna()

            if len(df_features) < 50:
                return {'trend': 'sideways', 'price': float(df['close'].iloc[-1]), 'confidence': 0.5}

            # 特征列
            feature_cols = ['return_1', 'return_5', 'return_10', 'ma_ratio', 'volatility']
            X = df_features[feature_cols].values[:-5]  # 排除最后5行（没有目标值）
            y = df_features['target'].values[:-5]

            # 训练模型
            model = GradientBoostingClassifier(n_estimators=50, max_depth=3, random_state=42)
            model.fit(X, y)

            # 预测
            X_latest = df_features[feature_cols].values[-1:].reshape(1, -1)
            prob = model.predict_proba(X_latest)[0]

            # 判断趋势
            if prob[1] > 0.6:
                trend = "up"
                confidence = prob[1]
            elif prob[0] > 0.6:
                trend = "down"
                confidence = prob[0]
            else:
                trend = "sideways"
                confidence = max(prob)

            # 预测价格
            current_price = float(df['close'].iloc[-1])
            if trend == "up":
                predicted_price = current_price * (1 + 0.03 * confidence)
            elif trend == "down":
                predicted_price = current_price * (1 - 0.03 * confidence)
            else:
                predicted_price = current_price

            return {
                'trend': trend,
                'price': float(predicted_price),
                'confidence': float(confidence)
            }
        except Exception as e:
            print(f"XGBoost预测失败: {e}")
            return {'trend': 'sideways', 'price': float(df['close'].iloc[-1]), 'confidence': 0.5}

    def _empty_result(self) -> Dict:
        """返回空结果"""
        return {
            'lstm_trend': 'sideways',
            'lstm_price': 0,
            'lstm_confidence': 0.5,
            'prophet_trend': 'sideways',
            'prophet_price': 0,
            'prophet_confidence': 0.5,
            'xgboost_trend': 'sideways',
            'xgboost_price': 0,
            'xgboost_confidence': 0.5
        }


ml_predict_service = MLPredictService()
