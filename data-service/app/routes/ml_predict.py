from fastapi import APIRouter, Query

from app.services.stock_data import stock_data_service
from app.services.ml_predict import ml_predict_service

router = APIRouter()


@router.get("/predict")
async def predict(
    code: str = Query(..., description="股票代码"),
    period: str = Query("daily", description="周期")
):
    """ML模型预测"""
    # 获取K线数据
    df = stock_data_service.get_kline(code, period)

    if df.empty:
        return {
            "error": "获取数据失败",
            "lstm_trend": "sideways",
            "lstm_price": 0,
            "lstm_confidence": 0.5,
            "prophet_trend": "sideways",
            "prophet_price": 0,
            "prophet_confidence": 0.5,
            "xgboost_trend": "sideways",
            "xgboost_price": 0,
            "xgboost_confidence": 0.5
        }

    # 运行预测
    result = ml_predict_service.predict_all(df)
    return result
