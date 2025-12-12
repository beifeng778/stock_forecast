# 股票预测系统

基于 React + TypeScript 前端和 Golang + LangChain 后端的 A 股个股趋势预测系统。

## 功能特性

- **股票选择**：支持搜索和多选沪深 A 股（600/000 开头）
- **趋势图表**：支持日/周/月 K 线展示
- **智能预测**：结合传统 ML 模型（LSTM/Prophet/XGBoost）和 LLM（通义千问）分析
- **预测输出**：趋势方向、价格区间、目标价、置信度、支撑/压力位、技术信号、AI 分析
- **委托模拟**：模拟买卖计算盈亏，包含手续费（佣金、印花税、过户费）

## 系统架构

```
前端 (React+TS:5173) → Golang 主服务 (:8080) → Python 数据服务 (:5000)
```

## 技术栈

| 层级 | 技术 |
|------|------|
| 前端 | React 18 + TypeScript + Ant Design + ECharts + Zustand |
| 后端 | Golang + Gin + LangChain |
| 数据服务 | Python + FastAPI + AKShare + scikit-learn/XGBoost |
| LLM | 通义千问 (Qwen) |

## 快速开始

### 环境要求

- Node.js >= 18
- Go >= 1.21
- Python >= 3.10

### 首次安装

```bash
# 1. 安装 Python 依赖
cd data-service
python -m venv venv
source venv/Scripts/activate  # Linux/Mac: source venv/bin/activate
pip install -r requirements.txt
cd ..

# 2. 配置通义千问 API Key
cd backend
cp .env.example .env
# 编辑 .env 文件，填入你的 DASHSCOPE_API_KEY
cd ..

# 3. 安装前端依赖（已完成）
cd frontend
npm install
cd ..
```

### 启动服务

**方式一：分别启动（推荐开发时使用）**

打开三个终端窗口，分别运行：

```bash
# 终端 1: Python 数据服务
./start-python.sh

# 终端 2: Golang 后端
./start-backend.sh

# 终端 3: 前端
./start-frontend.sh
```

**方式二：一键启动**

```bash
./start-all.sh
```

**停止服务**

```bash
./stop-all.sh
```

### 访问地址

| 服务 | 地址 |
|------|------|
| 前端界面 | http://localhost:5173 |
| Golang 后端 | http://localhost:8080 |
| Python 数据服务 | http://localhost:5000 |

## 配置说明

### 通义千问 API

1. 访问 [阿里云灵积平台](https://dashscope.console.aliyun.com/) 注册账号
2. 创建 API Key
3. 将 API Key 填入 `backend/.env` 文件的 `DASHSCOPE_API_KEY`

### 可选模型

- `qwen-turbo`：速度快，成本低
- `qwen-plus`：平衡性能和成本（推荐）
- `qwen-max`：最强能力，成本较高

## API 接口

### Python 数据服务 (localhost:5000)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/stocks | 获取股票列表 |
| GET | /api/stocks/{code}/kline | 获取 K 线数据 |
| GET | /api/stocks/{code}/indicators | 获取技术指标 |
| GET | /api/ml/predict | ML 模型预测 |

### Golang 主服务 (localhost:8080)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/stocks | 获取股票列表 |
| GET | /api/stocks/{code}/kline | 获取 K 线数据 |
| POST | /api/predict | 综合预测 |
| POST | /api/trade/simulate | 委托盈亏模拟 |

## 手续费计算规则

- **佣金**：成交金额 × 0.025%（最低 5 元，买卖双向收取）
- **印花税**：卖出金额 × 0.05%（仅卖出收取）
- **过户费**：成交金额 × 0.001%（仅沪市收取）

## 免责声明

本系统仅供学习研究使用，不构成任何投资建议。股市有风险，投资需谨慎。

## License

MIT
