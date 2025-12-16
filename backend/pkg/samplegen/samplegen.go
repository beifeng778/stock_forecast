package samplegen

import (
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"stock-forecast-backend/internal/stockdata"
	"stock-forecast-backend/pkg/llmsamples"

	_ "modernc.org/sqlite"
)

type Options struct {
	OutputPath       string
	MaxStocks        int
	DaysPerStock     int
	MinHistoryLen    int
	Daemon           bool
	RunAt            string
	RunOnStartup     bool
	RetryCount       int
	RetryIntervalMin int
	Enabled          bool
}

func Execute(args []string) error {
	fs := flag.NewFlagSet("llm_sample_gen", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var opts Options
	fs.StringVar(&opts.OutputPath, "output", "", "")
	fs.IntVar(&opts.MaxStocks, "max-stocks", 0, "")
	fs.IntVar(&opts.DaysPerStock, "days", 180, "")
	fs.IntVar(&opts.MinHistoryLen, "min-history", 30, "")
	fs.BoolVar(&opts.Daemon, "daemon", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}

	opts.Enabled = getEnvBool("LLM_SAMPLE_GEN_ENABLED", true)
	if !opts.Enabled {
		log.Println("sample-gen disabled")
		return nil
	}

	if strings.TrimSpace(opts.OutputPath) == "" {
		opts.OutputPath = os.Getenv("LLM_SAMPLES_PATH")
	}
	if strings.TrimSpace(opts.OutputPath) == "" {
		opts.OutputPath = "/app/rag"
	}
	opts.OutputPath = llmsamples.ResolvePath(opts.OutputPath)

	if opts.MaxStocks == 0 {
		opts.MaxStocks = getEnvInt("LLM_SAMPLE_GEN_MAX_STOCKS", 0)
	}
	if opts.DaysPerStock == 180 {
		opts.DaysPerStock = getEnvInt("LLM_SAMPLE_GEN_DAYS", 180)
	}
	if opts.MinHistoryLen == 30 {
		opts.MinHistoryLen = getEnvInt("LLM_SAMPLE_GEN_MIN_HISTORY", 30)
	}
	if !opts.Daemon {
		opts.Daemon = getEnvBool("LLM_SAMPLE_GEN_DAEMON", false)
	}

	opts.RunAt = os.Getenv("LLM_SAMPLE_GEN_TIME")
	if strings.TrimSpace(opts.RunAt) == "" {
		opts.RunAt = "04:20"
	}
	opts.RunOnStartup = getEnvBool("LLM_SAMPLE_GEN_ON_STARTUP", false)
	opts.RetryCount = getEnvInt("LLM_SAMPLE_GEN_RETRY_COUNT", 3)
	opts.RetryIntervalMin = getEnvInt("LLM_SAMPLE_GEN_RETRY_INTERVAL", 10)

	if opts.Daemon {
		log.Printf("sample-gen daemon mode: output=%s, time=%s, on_startup=%v", opts.OutputPath, opts.RunAt, opts.RunOnStartup)
		RunDailyDaemon(opts)
		return nil
	}

	lines, err := GenerateOnce(opts)
	if err != nil {
		return err
	}
	log.Printf("完成: output=%s, rows=%d", opts.OutputPath, lines)
	return nil
}

func RunDailyDaemon(opts Options) {
	hour, minute, err := parseHHMM(opts.RunAt)
	if err != nil {
		log.Fatalf("invalid LLM_SAMPLE_GEN_TIME: %v", err)
	}

	if opts.RunOnStartup {
		runWithRetry(opts)
	}

	for {
		now := time.Now()
		nextRun := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		if now.After(nextRun) {
			nextRun = nextRun.Add(24 * time.Hour)
		}
		d := nextRun.Sub(now)
		log.Printf("下次样本生成时间: %s（%v后）", nextRun.Format("2006-01-02 15:04:05"), d.Round(time.Minute))
		time.Sleep(d)
		runWithRetry(opts)
	}
}

func runWithRetry(opts Options) {
	for i := 0; i <= opts.RetryCount; i++ {
		if i > 0 {
			log.Printf("第 %d 次重试生成样本...", i)
		} else {
			log.Println("开始生成样本...")
		}

		rows, err := GenerateOnce(opts)
		if err == nil {
			log.Printf("样本生成完成: output=%s, rows=%d", opts.OutputPath, rows)
			return
		}
		log.Printf("生成样本失败: %v", err)
		if i < opts.RetryCount {
			log.Printf("将在 %d 分钟后重试", opts.RetryIntervalMin)
			time.Sleep(time.Duration(opts.RetryIntervalMin) * time.Minute)
		}
	}
	log.Printf("生成样本失败，已重试 %d 次", opts.RetryCount)
}

func GenerateOnce(opts Options) (int, error) {
	if err := os.MkdirAll(filepath.Dir(opts.OutputPath), 0o755); err != nil {
		return 0, fmt.Errorf("创建输出目录失败: %w", err)
	}

	tmpPath := opts.OutputPath + ".tmp"
	_ = os.Remove(tmpPath)

	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s", filepath.ToSlash(tmpPath)))
	if err != nil {
		return 0, err
	}

	if _, err := db.Exec("PRAGMA journal_mode=OFF;"); err != nil {
		_ = db.Close()
		return 0, err
	}
	if _, err := db.Exec("PRAGMA synchronous=OFF;"); err != nil {
		_ = db.Close()
		return 0, err
	}
	if err := llmsamples.EnsureSchema(db); err != nil {
		_ = db.Close()
		return 0, err
	}

	tx, err := db.Begin()
	if err != nil {
		_ = db.Close()
		return 0, err
	}

	stmt, err := tx.Prepare(`
INSERT OR REPLACE INTO llm_samples(
  id, trade_date, rsi, volatility, change_5d, ma5_slope, momentum_score, future_1d, future_5d
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
`)
	if err != nil {
		_ = tx.Rollback()
		_ = db.Close()
		return 0, err
	}
	defer stmt.Close()

	stocks, err := stockdata.GetStockList()
	if err != nil {
		_ = tx.Rollback()
		_ = db.Close()
		return 0, err
	}
	if opts.MaxStocks > 0 && opts.MaxStocks < len(stocks) {
		stocks = stocks[:opts.MaxStocks]
	}

	written := 0
	startAt := time.Now()
	for idx, s := range stocks {
		kline, err := stockdata.GetKline(s.Code, "daily")
		if err != nil || kline == nil || len(kline.Data) < opts.MinHistoryLen+6 {
			continue
		}

		end := len(kline.Data) - 5
		start := opts.MinHistoryLen
		if opts.DaysPerStock > 0 {
			cand := end - opts.DaysPerStock + 1
			if cand > start {
				start = cand
			}
		}
		if start < opts.MinHistoryLen {
			start = opts.MinHistoryLen
		}
		if start > end {
			continue
		}

		for i := start; i <= end; i++ {
			baseClose := kline.Data[i-1].Close
			if baseClose <= 0 {
				continue
			}
			ind, err := stockdata.CalculateIndicators(kline.Data[:i])
			if err != nil || ind == nil {
				continue
			}

			f1 := (kline.Data[i].Close - baseClose) / baseClose * 100
			f5 := (kline.Data[i+4].Close - baseClose) / baseClose * 100

			id := fmt.Sprintf("%s_%s", s.Code, kline.Data[i].Date)
			if _, err := stmt.Exec(
				id,
				kline.Data[i].Date,
				ind.RSI,
				ind.Volatility,
				ind.Change5D,
				ind.MA5Slope,
				ind.MomentumScore,
				f1,
				f5,
			); err != nil {
				_ = tx.Rollback()
				_ = db.Close()
				_ = os.Remove(tmpPath)
				return 0, err
			}
			written++
		}

		if (idx+1)%50 == 0 {
			elapsed := time.Since(startAt)
			fmt.Printf("进度: %d/%d stocks, rows=%d, elapsed=%s\n", idx+1, len(stocks), written, elapsed.Truncate(time.Second))
		}
	}

	if err := tx.Commit(); err != nil {
		_ = db.Close()
		_ = os.Remove(tmpPath)
		return 0, err
	}
	if err := db.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return 0, err
	}
	if err := os.Rename(tmpPath, opts.OutputPath); err != nil {
		_ = os.Remove(tmpPath)
		return 0, err
	}
	return written, nil
}

func parseHHMM(s string) (int, int, error) {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time format")
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("invalid hour/minute")
	}
	return h, m, nil
}

func getEnvInt(key string, defaultValue int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func getEnvBool(key string, defaultValue bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}
