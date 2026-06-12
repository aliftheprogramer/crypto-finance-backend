package repository

import (
	"database/sql"
	"encoding/json"
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

	CREATE TABLE IF NOT EXISTS news_raw (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		url TEXT NOT NULL UNIQUE,
		source TEXT NOT NULL,
		published_at INTEGER NOT NULL,
		content_snippet TEXT DEFAULT '',
		tags TEXT DEFAULT '',
		fetched_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS daily_briefings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		summary_date TEXT NOT NULL UNIQUE,
		points TEXT NOT NULL,
		sentiment TEXT NOT NULL DEFAULT 'NEUTRAL',
		news_count INTEGER DEFAULT 0,
		created_at INTEGER NOT NULL
	);
	`

	_, err := db.Exec(schema)
	return err
}

func SaveNewsRaw(title, url, source string, publishedAt int64, snippet, tags string) error {
	_, err := GetDB().Exec(
		`INSERT OR IGNORE INTO news_raw (title, url, source, published_at, content_snippet, tags, fetched_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		title, url, source, publishedAt, snippet, tags, Now(),
	)
	return err
}

func CountTodayNews() (int, error) {
	startOfDay := startOfDayUnix()
	var count int
	err := GetDB().QueryRow(
		"SELECT COUNT(*) FROM news_raw WHERE fetched_at >= ?", startOfDay,
	).Scan(&count)
	return count, err
}

func SaveBriefing(summaryDate string, points []string, sentiment string, newsCount int) error {
	pointsJSON := marshalJSON(points)
	_, err := GetDB().Exec(
		`INSERT INTO daily_briefings (summary_date, points, sentiment, news_count, created_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(summary_date) DO UPDATE SET
		 	points = excluded.points,
		 	sentiment = excluded.sentiment,
		 	news_count = excluded.news_count,
		 	created_at = excluded.created_at`,
		summaryDate, pointsJSON, sentiment, newsCount, Now(),
	)
	return err
}

func GetLatestBriefing() (*struct {
	SummaryDate string
	Points      string
	Sentiment   string
	NewsCount   int
	CreatedAt   int64
}, error) {
	row := GetDB().QueryRow(
		"SELECT summary_date, points, sentiment, news_count, created_at FROM daily_briefings ORDER BY created_at DESC LIMIT 1",
	)

	var b struct {
		SummaryDate string
		Points      string
		Sentiment   string
		NewsCount   int
		CreatedAt   int64
	}
	err := row.Scan(&b.SummaryDate, &b.Points, &b.Sentiment, &b.NewsCount, &b.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func HasTodayBriefing() bool {
	today := time.Now().Format("2006-01-02")
	var count int
	GetDB().QueryRow("SELECT COUNT(*) FROM daily_briefings WHERE summary_date = ?", today).Scan(&count)
	return count > 0
}

func startOfDayUnix() int64 {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
}

func marshalJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
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
