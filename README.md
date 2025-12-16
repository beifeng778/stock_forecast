# 股票预测系统

基于 React + TypeScript 前端和 Golang 后端的 A 股个股趋势预测系统。

## 功能特性

- **股票选择**：支持搜索和多选沪深 A 股（600/000 开头）
- **趋势图表**：日 K 线展示
- **智能预测**：结合技术指标分析和 LLM（OpenAI 兼容接口）综合分析
- **预测输出**：趋势方向、价格区间、目标价、置信度、支撑/压力位、技术信号、AI 分析
- **委托模拟**：模拟买卖计算盈亏，包含手续费（佣金、印花税、过户费）

## 系统架构

```
前端 (React+TS:5173) → Golang 主服务 (:8080)
```

## 技术栈

| 层级 | 技术 |
|------|------|
| 前端 | React 18 + TypeScript + Ant Design + ECharts + Zustand |
| 后端 | Golang + Gin |
| 数据源 | 东方财富 / 新浪财经 API |
| LLM | OpenAI Chat Completions 兼容接口（例如 DeepSeek） |

## 快速开始

### 环境要求

- Node.js >= 18
- Go >= 1.21

### 首次安装

```bash
# 1. 配置 LLM 接口（OpenAI 兼容）
cd backend
cp .env.example .env
# 编辑 .env 文件，填入你的 LLM_BASE_URL / LLM_AUTH_TOKEN
cd ..

# 2. 安装前端依赖
cd frontend
npm install
cd ..
```

### 启动服务

**方式一：分别启动（推荐开发时使用）**

打开两个终端窗口，分别运行：

```bash
# 终端 1: Golang 后端
./scripts/start-backend.sh

# 终端 2: 前端
./scripts/start-frontend.sh
```

**方式二：一键启动**

```bash
./scripts/start-all.sh
```

**停止服务**

```bash
./scripts/stop-all.sh
```

### 访问地址

| 服务 | 地址 |
|------|------|
| 前端界面 | http://localhost:5173 |
| Golang 后端 | http://localhost:8080 |

## 配置说明

### LLM 接口（OpenAI 兼容）

- 环境变量：
  - `LLM_BASE_URL`（例如 `https://api.deepseek.com`）
  - `LLM_AUTH_TOKEN`（例如 `sk-...`）
  - `LLM_MODEL`（例如 `deepseek-chat`）

## API 接口

### Golang 主服务 (localhost:8080)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/stocks | 获取股票列表 |
| GET | /api/stocks/{code}/kline | 获取 K 线数据 |
| GET | /api/stocks/{code}/indicators | 获取技术指标 |
| GET | /api/stocks/{code}/news | 获取股票新闻 |
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
