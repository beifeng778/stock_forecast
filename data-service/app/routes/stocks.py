from fastapi import APIRouter, Query
from typing import Optional

from app.services.stock_data import stock_data_service
from app.services.indicators import indicators_service

router = APIRouter()


@router.get("/stocks")
async def get_stocks(keyword: Optional[str] = Query(None, description="搜索关键词")):
    """获取股票列表"""
    stocks = stock_data_service.search_stocks(keyword or "")
    return {"data": stocks}


@router.get("/stocks/{code}/info")
async def get_stock_info(code: str):
    """获取股票基本信息"""
    info = stock_data_service.get_stock_info(code)
    if info:
        return info
    return {"code": code, "name": "未知", "market": ""}


@router.get("/stocks/{code}/kline")
async def get_kline(
    code: str,
    period: str = Query("daily", description="周期: daily, weekly, monthly")
):
    """获取K线数据"""
    df = stock_data_service.get_kline(code, period)
    info = stock_data_service.get_stock_info(code)

    if df.empty:
        return {
            "code": code,
            "name": info['name'] if info else "未知",
            "period": period,
            "data": []
        }

    data = df.to_dict(orient='records')
    return {
        "code": code,
        "name": info['name'] if info else "未知",
        "period": period,
        "data": data
    }


@router.get("/stocks/{code}/realtime")
async def get_realtime(code: str):
    """获取实时行情"""
    quote = stock_data_service.get_realtime_quote(code)
    if quote:
        return quote
    return {"error": "获取实时行情失败"}


@router.get("/stocks/{code}/indicators")
async def get_indicators(code: str):
    """获取技术指标"""
    df = stock_data_service.get_kline(code, "daily")
    if df.empty:
        return {"error": "获取数据失败"}

    indicators = indicators_service.calculate_all(df)
    signals = indicators_service.get_signals(indicators)

    return {
        **indicators,
        "signals": signals
    }
