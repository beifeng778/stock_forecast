import { ConfigProvider, theme } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import StockSelector from './components/StockSelector';
import TrendChart from './components/TrendChart';
import PredictionPanel from './components/PredictionPanel';
import TradeSimulator from './components/TradeSimulator';
import './App.css';

function App() {
  return (
    <ConfigProvider
      locale={zhCN}
      theme={{
        algorithm: theme.darkAlgorithm,
        token: {
          colorPrimary: '#6366f1',
          colorBgContainer: 'rgba(30, 41, 59, 0.8)',
          colorBgElevated: 'rgba(30, 41, 59, 0.95)',
          colorBorder: 'rgba(99, 102, 241, 0.3)',
          colorText: '#e2e8f0',
          colorTextSecondary: '#94a3b8',
          borderRadius: 12,
        },
      }}
    >
      <div className="app">
        <header className="app-header">
          <div className="header-left">
            <div className="header-logo">
              <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M3 3v18h18" />
                <path d="M18 9l-5 5-4-4-3 3" />
              </svg>
            </div>
            <div className="header-title">
              <h1>AI 股票预测系统</h1>
              <span className="subtitle">基于机器学习的 A 股趋势智能分析</span>
            </div>
          </div>
          <div className="header-badge">
            AI 模型运行中
          </div>
        </header>

        <main className="app-main">
          <div className="main-top">
            <div className="selector-section">
              <StockSelector />
            </div>
            <div className="chart-section">
              <TrendChart />
            </div>
          </div>

          <div className="main-middle">
            <PredictionPanel />
          </div>

          <div className="main-bottom">
            <TradeSimulator />
          </div>
        </main>

        <footer className="app-footer">
          <p>免责声明：本系统仅供学习研究使用，不构成任何投资建议。股市有风险，投资需谨慎。</p>
        </footer>
      </div>
    </ConfigProvider>
  );
}

export default App;
