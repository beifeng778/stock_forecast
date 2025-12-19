package samplegen

import (
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
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
	Rebuild          bool
	Debug            bool
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
		samplegenInfof("disabled")
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
	opts.Rebuild = getEnvBool("LLM_SAMPLE_GEN_REBUILD", false)
	opts.Debug = getEnvBool("LLM_SAMPLE_GEN_DEBUG", false)

	if opts.Daemon {
		mode := "incremental"
		if opts.Rebuild {
			mode = "rebuild"
		}
		samplegenInfof("daemon mode: mode=%s, output=%s, time=%s, on_startup=%v", mode, opts.OutputPath, opts.RunAt, opts.RunOnStartup)
		RunDailyDaemon(opts)
		return nil
	}

	lines, err := GenerateOnce(opts)
	if err != nil {
		return err
	}
	samplegenInfof("done: output=%s, rows=%d", opts.OutputPath, lines)
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
		samplegenInfof("next run: %s (in %v)", nextRun.Format("2006-01-02 15:04:05"), d.Round(time.Minute))
		time.Sleep(d)
		runWithRetry(opts)
	}
}

func runWithRetry(opts Options) {
	for i := 0; i <= opts.RetryCount; i++ {
		if i > 0 {
			samplegenInfof("retry=%d", i)
		} else {
			samplegenInfof("start")
		}

		rows, err := GenerateOnce(opts)
		if err == nil {
			samplegenInfof("done: output=%s, rows=%d", opts.OutputPath, rows)
			return
		}
		samplegenErrorf("failed: %v", err)
		if i < opts.RetryCount {
			samplegenInfof("retry in %d min", opts.RetryIntervalMin)
			time.Sleep(time.Duration(opts.RetryIntervalMin) * time.Minute)
		}
	}
	samplegenErrorf("failed after retries=%d", opts.RetryCount)
}

func GenerateOnce(opts Options) (int, error) {
	if opts.Rebuild {
		return generateRebuild(opts)
	}
	return generateIncremental(opts)
}

func generateRebuild(opts Options) (int, error) {
	samplegenInfof("start: mode=rebuild, output=%s, max_stocks=%d, days=%d, min_history=%d, debug=%v", opts.OutputPath, opts.MaxStocks, opts.DaysPerStock, opts.MinHistoryLen, opts.Debug)
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
	sort.Slice(stocks, func(i, j int) bool { return stocks[i].Code < stocks[j].Code })
	if opts.MaxStocks > 0 && opts.MaxStocks < len(stocks) {
		stocks = stocks[:opts.MaxStocks]
	}

	written := 0
	stocksWritten := 0
	stocksSkipped := 0
	startAt := time.Now()
	for idx, s := range stocks {
		kline, err := stockdata.GetKline(s.Code, "daily")
		if err != nil || kline == nil || len(kline.Data) < opts.MinHistoryLen+6 {
			stocksSkipped++
			samplegenDebugf(opts.Debug, "skip stock=%s reason=insufficient_kline", s.Code)
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
			stocksSkipped++
			if opts.Debug {
				log.Printf("[sample-gen] skip stock=%s reason=no_valid_window", s.Code)
			}
			continue
		}

		stockStartWritten := written

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

		stockWritten := written - stockStartWritten
		if stockWritten > 0 {
			stocksWritten++
		}
		samplegenDebugf(opts.Debug, "stock=%s wrote=%d", s.Code, stockWritten)

		if (idx+1)%50 == 0 {
			elapsed := time.Since(startAt)
			samplegenInfof("progress: %d/%d stocks, rows=%d, elapsed=%s", idx+1, len(stocks), written, elapsed.Truncate(time.Second))
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
	samplegenInfof("done: mode=rebuild, stocks=%d, stocks_written=%d, stocks_skipped=%d, rows=%d, elapsed=%s", len(stocks), stocksWritten, stocksSkipped, written, time.Since(startAt).Truncate(time.Second))
	return written, nil
}

func generateIncremental(opts Options) (int, error) {
	samplegenInfof("start: mode=incremental, output=%s, max_stocks=%d, days=%d, min_history=%d, debug=%v", opts.OutputPath, opts.MaxStocks, opts.DaysPerStock, opts.MinHistoryLen, opts.Debug)
	if err := os.MkdirAll(filepath.Dir(opts.OutputPath), 0o755); err != nil {
		return 0, fmt.Errorf("创建输出目录失败: %w", err)
	}

	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s", filepath.ToSlash(opts.OutputPath)))
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

	maxDateStmt, err := tx.Prepare("SELECT MAX(trade_date) FROM llm_samples WHERE id LIKE ?")
	if err != nil {
		_ = tx.Rollback()
		_ = db.Close()
		return 0, err
	}
	defer maxDateStmt.Close()

	stocks, err := stockdata.GetStockList()
	if err != nil {
		_ = tx.Rollback()
		_ = db.Close()
		return 0, err
	}
	sort.Slice(stocks, func(i, j int) bool { return stocks[i].Code < stocks[j].Code })
	if opts.MaxStocks > 0 && opts.MaxStocks < len(stocks) {
		stocks = stocks[:opts.MaxStocks]
	}

	firstIndexAfter := func(data []stockdata.KlineData, d string) int {
		if strings.TrimSpace(d) == "" {
			return 0
		}
		for i := 0; i < len(data); i++ {
			if data[i].Date > d {
				return i
			}
		}
		return len(data)
	}

	written := 0
	stocksWritten := 0
	stocksSkipped := 0
	startAt := time.Now()
	for idx, s := range stocks {
		kline, err := stockdata.GetKline(s.Code, "daily")
		if err != nil || kline == nil || len(kline.Data) < opts.MinHistoryLen+6 {
			stocksSkipped++
			samplegenDebugf(opts.Debug, "skip stock=%s reason=insufficient_kline", s.Code)
			continue
		}

		var lastDate sql.NullString
		if err := maxDateStmt.QueryRow(s.Code + "_%").Scan(&lastDate); err != nil {
			_ = tx.Rollback()
			_ = db.Close()
			return 0, err
		}

		end := len(kline.Data) - 5
		start := opts.MinHistoryLen
		if opts.DaysPerStock > 0 {
			cand := end - opts.DaysPerStock + 1
			if cand > start {
				start = cand
			}
		}
		startDate := ""
		lastDateStr := ""
		if lastDate.Valid {
			lastDateStr = lastDate.String
		}
		if start >= 0 && start < len(kline.Data) {
			startDate = kline.Data[start].Date
		}
		if lastDate.Valid {
			cand := firstIndexAfter(kline.Data, lastDate.String)
			if cand > start {
				start = cand
			}
		}
		if start < opts.MinHistoryLen {
			start = opts.MinHistoryLen
		}
		if start > end {
			stocksSkipped++
			samplegenDebugf(opts.Debug, "skip stock=%s last_date=%s reason=no_new_trade_date", s.Code, lastDateStr)
			continue
		}
		if start >= 0 && start < len(kline.Data) {
			startDate = kline.Data[start].Date
		}

		stockStartWritten := written

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
				return 0, err
			}
			written++
		}

		stockWritten := written - stockStartWritten
		if stockWritten > 0 {
			stocksWritten++
		}
		samplegenDebugf(opts.Debug, "stock=%s last_date=%s start_date=%s wrote=%d", s.Code, lastDateStr, startDate, stockWritten)

		if (idx+1)%50 == 0 {
			elapsed := time.Since(startAt)
			samplegenInfof("progress: %d/%d stocks, rows=%d, elapsed=%s", idx+1, len(stocks), written, elapsed.Truncate(time.Second))
		}
	}

	if err := tx.Commit(); err != nil {
		_ = db.Close()
		return 0, err
	}
	if err := db.Close(); err != nil {
		return 0, err
	}
	samplegenInfof("done: mode=incremental, stocks=%d, stocks_written=%d, stocks_skipped=%d, rows=%d, elapsed=%s", len(stocks), stocksWritten, stocksSkipped, written, time.Since(startAt).Truncate(time.Second))
	return written, nil
}

func samplegenInfof(format string, args ...any) {
	log.Printf("[INFO][sample-gen] "+format, args...)
}

func samplegenErrorf(format string, args ...any) {
	log.Printf("[ERROR][sample-gen] "+format, args...)
}

func samplegenDebugf(enabled bool, format string, args ...any) {
	if !enabled {
		return
	}
	log.Printf("[DEBUG][sample-gen] "+format, args...)
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
