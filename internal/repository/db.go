package repository

import (
	"database/sql"
	"log"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

func timeNow() int64 {
	return time.Now().Unix()
}

func Now() int64 {
	return time.Now().Unix()
}

var (
	db     *sql.DB
	dbOnce sync.Once
)

func GetDB() *sql.DB {
	dbOnce.Do(func() {
		var err error
		db, err = sql.Open("sqlite", "./portfolio.db")
		if err != nil {
			log.Fatalf("Failed to open database: %v", err)
		}
		db.SetMaxOpenConns(1)

		if err := migrate(); err != nil {
			log.Fatalf("Failed to migrate database: %v", err)
		}
		log.Print("[db] SQLite initialized, tables ready")
	})
	return db
}

func migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS price_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		symbol TEXT NOT NULL,
		timestamp INTEGER NOT NULL,
		price REAL NOT NULL,
		UNIQUE(symbol, timestamp)
	);

	CREATE INDEX IF NOT EXISTS idx_price_symbol ON price_history(symbol);
	CREATE INDEX IF NOT EXISTS idx_price_timestamp ON price_history(timestamp);

	CREATE TABLE IF NOT EXISTS signals (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		symbol TEXT NOT NULL UNIQUE,
		action TEXT NOT NULL,
		confidence REAL NOT NULL,
		reasons TEXT NOT NULL,
		price REAL DEFAULT 0,
		rsi REAL DEFAULT 0,
		sma_50 REAL DEFAULT 0,
		sma_200 REAL DEFAULT 0,
		macd_line REAL DEFAULT 0,
		macd_signal REAL DEFAULT 0,
		macd_histogram REAL DEFAULT 0,
		generated_at INTEGER NOT NULL
	);
	`

	_, err := db.Exec(schema)
	return err
}

func SavePriceHistory(symbol string, timestamp int64, price float64) error {
	_, err := GetDB().Exec(
		"INSERT OR IGNORE INTO price_history (symbol, timestamp, price) VALUES (?, ?, ?)",
		symbol, timestamp, price,
	)
	return err
}

func GetPriceHistory(symbol string, limit int) ([]struct {
	Timestamp int64
	Price     float64
}, error) {
	rows, err := GetDB().Query(
		"SELECT timestamp, price FROM price_history WHERE symbol = ? ORDER BY timestamp ASC LIMIT ?",
		symbol, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []struct {
		Timestamp int64
		Price     float64
	}
	for rows.Next() {
		var ts int64
		var price float64
		if err := rows.Scan(&ts, &price); err != nil {
			return nil, err
		}
		result = append(result, struct {
			Timestamp int64
			Price     float64
		}{ts, price})
	}
	return result, nil
}

func CountPriceHistory(symbol string) (int, error) {
	var count int
	err := GetDB().QueryRow("SELECT COUNT(*) FROM price_history WHERE symbol = ?", symbol).Scan(&count)
	return count, err
}

func SaveSignal(symbol, action string, confidence float64, reasons []string, price, rsi, sma50, sma200, macdLine, macdSignal, macdHist float64) error {
	reasonStr := ""
	for i, r := range reasons {
		if i > 0 {
			reasonStr += "|"
		}
		reasonStr += r
	}

	_, err := GetDB().Exec(`
		INSERT INTO signals (symbol, action, confidence, reasons, price, rsi, sma_50, sma_200, macd_line, macd_signal, macd_histogram, generated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(symbol) DO UPDATE SET
			action = excluded.action,
			confidence = excluded.confidence,
			reasons = excluded.reasons,
			price = excluded.price,
			rsi = excluded.rsi,
			sma_50 = excluded.sma_50,
			sma_200 = excluded.sma_200,
			macd_line = excluded.macd_line,
			macd_signal = excluded.macd_signal,
			macd_histogram = excluded.macd_histogram,
			generated_at = excluded.generated_at
	`, symbol, action, confidence, reasonStr, price, rsi, sma50, sma200, macdLine, macdSignal, macdHist, timeNow())
	return err
}

func GetSignal(symbol string) (*struct {
	Action       string
	Confidence   float64
	Reasons      string
	Price        float64
	RSI          float64
	SMA50        float64
	SMA200       float64
	MACDLine     float64
	MACDSignal   float64
	MACDHist     float64
	GeneratedAt  int64
}, error) {
	row := GetDB().QueryRow(`
		SELECT action, confidence, reasons, price, rsi, sma_50, sma_200, macd_line, macd_signal, macd_histogram, generated_at
		FROM signals WHERE symbol = ?
	`, symbol)

	var s struct {
		Action      string
		Confidence  float64
		Reasons     string
		Price       float64
		RSI         float64
		SMA50       float64
		SMA200      float64
		MACDLine    float64
		MACDSignal  float64
		MACDHist    float64
		GeneratedAt int64
	}
	err := row.Scan(&s.Action, &s.Confidence, &s.Reasons, &s.Price, &s.RSI, &s.SMA50, &s.SMA200, &s.MACDLine, &s.MACDSignal, &s.MACDHist, &s.GeneratedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func IsSignalFresh(symbol string, maxAgeSec int64) bool {
	var genAt int64
	err := GetDB().QueryRow("SELECT generated_at FROM signals WHERE symbol = ?", symbol).Scan(&genAt)
	if err != nil {
		return false
	}
	return timeNow()-genAt < maxAgeSec
}
