package llmsamples

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const DefaultDBFileName = "llm_samples.db"

type Indicators struct {
	RSI           float64
	Volatility    float64
	Change5D      float64
	MA5Slope      float64
	MomentumScore float64
}

type Sample struct {
	ID            string
	TradeDate     string
	RSI           float64
	Volatility    float64
	Change5D      float64
	MA5Slope      float64
	MomentumScore float64
	Future1D      float64
	Future5D      float64
}

func ResolvePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return p
	}
	if filepath.Ext(p) == "" {
		return filepath.Join(p, DefaultDBFileName)
	}
	if fi, err := os.Stat(p); err == nil && fi.IsDir() {
		return filepath.Join(p, DefaultDBFileName)
	}
	return p
}

func EnsureSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS llm_samples (
			id TEXT PRIMARY KEY,
			trade_date TEXT,
			rsi REAL NOT NULL,
			volatility REAL NOT NULL,
			change_5d REAL NOT NULL,
			ma5_slope REAL NOT NULL,
			momentum_score REAL NOT NULL,
			future_1d REAL NOT NULL,
			future_5d REAL NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_llm_samples_rsi ON llm_samples(rsi);`,
		`CREATE INDEX IF NOT EXISTS idx_llm_samples_volatility ON llm_samples(volatility);`,
		`CREATE INDEX IF NOT EXISTS idx_llm_samples_change5d ON llm_samples(change_5d);`,
		`CREATE INDEX IF NOT EXISTS idx_llm_samples_ma5slope ON llm_samples(ma5_slope);`,
		`CREATE INDEX IF NOT EXISTS idx_llm_samples_momentum ON llm_samples(momentum_score);`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

func QueryTopK(dbPath string, ind Indicators, topK int) ([]Sample, error) {
	if topK <= 0 {
		return nil, nil
	}
	dbPath = ResolvePath(dbPath)
	if _, err := os.Stat(dbPath); err != nil {
		return nil, err
	}

	todayStr := time.Now().Format("2006-01-02")
	timeDecayPerYear := 1.5
	if v := strings.TrimSpace(os.Getenv("LLM_SAMPLES_TIME_DECAY_PER_YEAR")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			if f < 0 {
				f = 0
			}
			if f > 20 {
				f = 20
			}
			timeDecayPerYear = f
		}
	}

	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro", filepath.ToSlash(dbPath)))
	if err != nil {
		return nil, err
	}
	defer db.Close()

	queryBase := `
SELECT
  id,
  trade_date,
  rsi,
  volatility,
  change_5d,
  ma5_slope,
  momentum_score,
  future_1d,
  future_5d,
  (abs(rsi-?)/100.0*2.0 + abs(volatility-?)/0.10*2.0 + abs(change_5d-?)/20.0*1.0 + abs(ma5_slope-?)/5.0*1.0 + abs(momentum_score-?)/100.0*1.0 + max(0, julianday(?) - julianday(trade_date)) / 365.0 * ?) AS score
FROM llm_samples
`

	where := `
WHERE
  rsi BETWEEN ? AND ?
  AND volatility BETWEEN ? AND ?
  AND change_5d BETWEEN ? AND ?
  AND ma5_slope BETWEEN ? AND ?
  AND momentum_score BETWEEN ? AND ?
`

	queryTail := `
ORDER BY score ASC, id ASC
LIMIT ?
`

	args := func(withWhere bool) []any {
		base := []any{ind.RSI, ind.Volatility, ind.Change5D, ind.MA5Slope, ind.MomentumScore, todayStr, timeDecayPerYear}
		if !withWhere {
			return append(base, topK)
		}
		rsiMin, rsiMax := ind.RSI-20, ind.RSI+20
		volMin, volMax := ind.Volatility-0.05, ind.Volatility+0.05
		chgMin, chgMax := ind.Change5D-10, ind.Change5D+10
		slopeMin, slopeMax := ind.MA5Slope-3, ind.MA5Slope+3
		momMin, momMax := ind.MomentumScore-30, ind.MomentumScore+30
		return append(base, rsiMin, rsiMax, volMin, volMax, chgMin, chgMax, slopeMin, slopeMax, momMin, momMax, topK)
	}

	readRows := func(q string, qArgs []any) ([]Sample, error) {
		rows, err := db.Query(q, qArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		out := make([]Sample, 0, topK)
		for rows.Next() {
			var s Sample
			var score float64
			if err := rows.Scan(
				&s.ID,
				&s.TradeDate,
				&s.RSI,
				&s.Volatility,
				&s.Change5D,
				&s.MA5Slope,
				&s.MomentumScore,
				&s.Future1D,
				&s.Future5D,
				&score,
			); err != nil {
				continue
			}
			out = append(out, s)
		}
		if err := rows.Err(); err != nil {
			return out, err
		}
		return out, nil
	}

	q1 := queryBase + where + queryTail
	res, err := readRows(q1, args(true))
	if err == nil && len(res) > 0 {
		return res, nil
	}

	q2 := queryBase + queryTail
	return readRows(q2, args(false))
}
