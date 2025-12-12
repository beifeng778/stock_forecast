from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
import uvicorn

from app.routes import stocks, ml_predict

app = FastAPI(title="股票数据服务", version="1.0.0")

# 配置 CORS
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# 注册路由
app.include_router(stocks.router, prefix="/api", tags=["stocks"])
app.include_router(ml_predict.router, prefix="/api/ml", tags=["ml"])


@app.get("/")
async def root():
    return {"message": "股票数据服务运行中"}


@app.get("/health")
async def health():
    return {"status": "healthy"}


if __name__ == "__main__":
    uvicorn.run("app.main:app", host="0.0.0.0", port=5000, reload=True)
