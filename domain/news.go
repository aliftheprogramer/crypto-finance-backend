package domain

type NewsItem struct {
	Title          string   `json:"title"`
	URL            string   `json:"url"`
	Source         string   `json:"source"`
	PublishedAt    int64    `json:"published_at"`
	ContentSnippet string   `json:"content_snippet"`
	Tags           []string `json:"tags"`
}

type DailyBriefing struct {
	SummaryDate string   `json:"summary_date"`
	Points      []string `json:"points"`
	Sentiment   string   `json:"sentiment"`
	NewsCount   int      `json:"news_count"`
	GeneratedAt string   `json:"generated_at"`
}
