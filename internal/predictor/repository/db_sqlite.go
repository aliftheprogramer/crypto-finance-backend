package repository

import (
	"github.com/alif/crypto-portfolio/internal/shared/db"
)

func timeNow() int64 {
	return db.Now()
}

func SaveSignal(symbol, action string, confidence float64, reasons []string, price, rsi, sma50, sma200, macdLine, macdSignal, macdHist float64) error {
	reasonStr := ""
	for i, r := range reasons {
		if i > 0 {
			reasonStr += "|"
		}
		reasonStr += r
	}

	_, err := db.GetDB().Exec(`
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
}, error) {
	row := db.GetDB().QueryRow(`
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
	err := db.GetDB().QueryRow("SELECT generated_at FROM signals WHERE symbol = ?", symbol).Scan(&genAt)
	if err != nil {
		return false
	}
	return timeNow()-genAt < maxAgeSec
}
