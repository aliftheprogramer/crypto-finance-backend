package repository

import (
	"encoding/json"
	"time"

	"github.com/alif/crypto-portfolio/internal/shared/db"
)

func SaveNewsRaw(title, url, source string, publishedAt int64, snippet, tags string) error {
	_, err := db.GetDB().Exec(
		`INSERT OR IGNORE INTO news_raw (title, url, source, published_at, content_snippet, tags, fetched_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		title, url, source, publishedAt, snippet, tags, db.Now(),
	)
	return err
}

func CountTodayNews() (int, error) {
	startOfDay := startOfDayUnix()
	var count int
	err := db.GetDB().QueryRow(
		"SELECT COUNT(*) FROM news_raw WHERE fetched_at >= ?", startOfDay,
	).Scan(&count)
	return count, err
}

func SaveBriefing(summaryDate string, points []string, sentiment string, newsCount int) error {
	pointsJSON := mustMarshalJSON(points)
	_, err := db.GetDB().Exec(
		`INSERT INTO daily_briefings (summary_date, points, sentiment, news_count, created_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(summary_date) DO UPDATE SET
		 	points = excluded.points,
		 	sentiment = excluded.sentiment,
		 	news_count = excluded.news_count,
		 	created_at = excluded.created_at`,
		summaryDate, pointsJSON, sentiment, newsCount, db.Now(),
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
	row := db.GetDB().QueryRow(
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
	db.GetDB().QueryRow("SELECT COUNT(*) FROM daily_briefings WHERE summary_date = ?", today).Scan(&count)
	return count > 0
}

func startOfDayUnix() int64 {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
}

func mustMarshalJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
