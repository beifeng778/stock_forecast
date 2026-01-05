# 股票预测系统

基于 React + TypeScript 前端和 Golang 后端的 A 股个股趋势预测系统。

## 功能特性

- **股票选择**：支持搜索和多选沪深 A 股（600/000 开头）
- **趋势图表**：日 K 线展示，支持盘中实时刷新
- **智能预测**：结合技术指标分析和 LLM（OpenAI 兼容接口）综合分析
- **预测输出**：趋势方向、价格区间、目标价、置信度、支撑/压力位、技术信号、AI 分析
- **委托模拟**：模拟买卖计算盈亏，包含手续费（佣金、印花税、过户费）
- **盘中支持**：交易时段内支持实时刷新第三方日K数据

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
| 缓存 | Redis（可选，不可用时自动降级为内存缓存） |
| 样本库 | SQLite（LLM 蒸馏样本，支持 TopK 检索） |

## 快速开始

### 环境要求

- Node.js >= 18
- Go >= 1.21
- Redis（可选）

### 首次安装

```bash
# 1. 配置 LLM 接口（OpenAI 兼容）
cd backend
cp .env.example .env.local
# 编辑 .env.local 文件，填入你的 LLM_BASE_URL / LLM_AUTH_TOKEN / LLM_MODEL
cd ..

# 2. 安装前端依赖
cd frontend
npm install
cd ..
```

### 生成样本库（可选，但推荐）

**方式一：使用脚本生成**

```bash
# 1. 配置样本生成参数
cp sample-gen/.env.local.example sample-gen/.env.local
# 编辑 sample-gen/.env.local，配置 LLM 相关参数

# 2. 运行生成脚本
./scripts/gen-llm-samples.sh
```

**方式二：手动生成**

```bash
# 1. 配置参数（同上）
cp sample-gen/.env.local.example sample-gen/.env.local

# 2. 直接运行样本生成程序
cd sample-gen
go run main.go
cd ..
```

### 启动服务

**方式一：分别启动（推荐）**

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

必填参数：

- `LLM_BASE_URL`（例如 `https://api.deepseek.com`）
- `LLM_AUTH_TOKEN`（例如 `sk-...`）
- `LLM_MODEL`（例如 `deepseek-chat`）

可选参数：

- `LLM_SAMPLES_PATH`（样本库路径，默认 `../rag`）
- `LLM_DEBUG_SAMPLES`（调试模式，默认 `false`）

### 样本生成配置

主要参数：

- `LLM_SAMPLE_GEN_MAX_STOCKS`（最大股票数，默认 `50`）
- `LLM_SAMPLE_GEN_REBUILD`（是否全量重建，默认 `false`）

## 部署

### 生产环境部署

```bash
# 1. 配置部署参数
cp Makefile.example Makefile
# 编辑 Makefile，修改 PROD_HOST 等参数

# 2. 完整后端部署（包含前端）
make deploy-backend

# 3. 仅部署样本生成服务
make deploy-sample-gen

# 4. 部署数据库（Redis）
make deploy-db
```

### 可用的 Make 命令

| 命令 | 说明 |
|------|------|
| `make help` | 显示所有可用命令 |
| `make build-frontend` | 构建前端 |
| `make deploy-frontend` | 部署前端到生产环境 |
| `make build-images` | 构建 Docker 镜像 |
| `make push-images` | 推送镜像到服务器 |
| `make deploy-backend` | 完整后端部署 |
| `make deploy-sample-gen` | 仅部署样本生成服务 |
| `make deploy-db` | 部署数据库（Redis） |

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

### 盘中特性

- **实时刷新**：交易时段内支持强制刷新第三方日K数据（`refresh=1`）
- **B1口径**：盘中预测使用"未收盘日K"计算技术指标
- **冷却机制**：刷新按钮有冷却时间（成功5分钟/失败2分钟）

## 手续费计算规则

- **佣金**：成交金额 × 0.025%（最低 5 元，买卖双向收取）
- **印花税**：卖出金额 × 0.05%（仅卖出收取）
- **过户费**：成交金额 × 0.001%（仅沪市收取）

## 趋势判断说明

系统通过多维度分析给出三种趋势判断：

- **看涨 (↗️)**：预期股价上涨，目标价高于当前价
- **看跌 (↘️)**：预期股价下跌，目标价低于当前价
- **震荡 (➡️)**：多空信号相互抵消，价格在当前区间波动

判断依据包括：技术指标、ML模型预测、动量分析、突破信号、量价关系等。

## 开发说明

### 构建

```bash
# 前端构建（跳过 tsc 检查）
cd frontend && npx vite build

# 后端构建
cd backend && go build ./...
```

### 调试

```bash
# 启用样本库调试日志
echo "LLM_DEBUG_SAMPLES=true" >> backend/.env.local

# 前端开发服务器
cd frontend && npm run dev

# 后端直接运行
cd backend && go run cmd/server/main.go
```

## 免责声明

本系统仅供学习研究使用，不构成任何投资建议。股市有风险，投资需谨慎。

## License

MIT License - 详见 [LICENSE](LICENSE) 文件
