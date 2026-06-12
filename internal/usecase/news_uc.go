package usecase

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/alif/crypto-portfolio/domain"
	"github.com/alif/crypto-portfolio/internal/repository"
)

type NewsUsecase struct {
	newsProvider *repository.NewsProvider
	deepseek     *repository.DeepSeekProvider
}

func NewNewsUsecase(newsProvider *repository.NewsProvider, deepseek *repository.DeepSeekProvider) *NewsUsecase {
	return &NewsUsecase{newsProvider: newsProvider, deepseek: deepseek}
}

func (uc *NewsUsecase) Refresh() error {
	log.Println("[news] refreshing...")

	news, err := uc.newsProvider.FetchNews()
	if err != nil {
		return fmt.Errorf("fetch news: %w", err)
	}

	if len(news) == 0 {
		log.Println("[news] no news fetched")
		return nil
	}

	for _, item := range news {
		tags := strings.Join(item.Tags, ",")
		cats := repository.CategoriesFromTags(item.Tags)
		if cats != "General" {
			if tags != "" {
				tags += ","
			}
			tags += cats
		}
		if err := repository.SaveNewsRaw(item.Title, item.URL, item.Source, item.PublishedAt, item.ContentSnippet, tags); err != nil {
			log.Printf("[news] skip duplicate: %s", item.Title)
		}
	}

	newsCount, _ := repository.CountTodayNews()

	if err := uc.generateBriefing(news, newsCount); err != nil {
		return fmt.Errorf("generate briefing: %w", err)
	}

	log.Printf("[news] done — %d news, %d today total", len(news), newsCount)
	return nil
}

func (uc *NewsUsecase) generateBriefing(news []domain.NewsItem, newsCount int) error {
	today := time.Now().Format("2006-01-02")

	if uc.deepseek != nil {
		return uc.aiBriefing(today, news, newsCount)
	}

	return uc.mockBriefing(today, newsCount)
}

func (uc *NewsUsecase) aiBriefing(date string, news []domain.NewsItem, newsCount int) error {
	var lines []string
	for i, item := range news {
		tags := strings.Join(item.Tags, ", ")
		lines = append(lines, fmt.Sprintf("%d. [%s] %s — %s\n   %s",
			i+1, tags, item.Title, item.Source, item.ContentSnippet))
	}

	userPrompt := fmt.Sprintf(`Berikut adalah berita kripto hari ini:

%s

Buat ringkasan eksekutif maksimal 5 poin penting yang wajib diketahui trader hari ini.
Kategorikan secara natural ke dalam:
- Layer 1 & Layer 2
- Tokenized Stocks / Sentimen Pasar
- Dampak ke Harga Jangka Pendek

Akhiri dengan sentimen pasar secara keseluruhan.

Output JSON dengan format:
{
  "points": ["poin 1", "poin 2", ...],
  "sentiment": "BULLISH" atau "BEARISH" atau "NEUTRAL"
}`, strings.Join(lines, "\n\n"))

	content, pt, ct, err := uc.deepseek.Chat(
		"Kamu adalah analis finansial crypto senior. Gunakan bahasa Indonesia yang natural, profesional, dan mudah dipahami. Output dalam format JSON.",
		userPrompt,
		0.3,
		1500,
	)
	if err != nil {
		return fmt.Errorf("deepseek briefing: %w", err)
	}

	_ = pt
	costUSD := (float64(pt)*0.27 + float64(ct)*1.10) / 1_000_000
	log.Printf("[news] ai briefing: prompt %d tok → response %d tok → $%.6f", pt, ct, costUSD)

	var resp struct {
		Points    []string `json:"points"`
		Sentiment string   `json:"sentiment"`
	}
	if err := json.Unmarshal([]byte(content), &resp); err != nil {
		return fmt.Errorf("deepseek parse: %w — content: %s", err, content)
	}

	switch resp.Sentiment {
	case "BULLISH", "BEARISH", "NEUTRAL":
	default:
		resp.Sentiment = "NEUTRAL"
	}
	if len(resp.Points) == 0 {
		resp.Points = []string{"Tidak ada ringkasan tersedia."}
	}

	if err := repository.SaveBriefing(date, resp.Points, resp.Sentiment, newsCount); err != nil {
		return fmt.Errorf("save briefing: %w", err)
	}

	return nil
}

func (uc *NewsUsecase) mockBriefing(date string, newsCount int) error {
	points := []string{
		"Layer 1 & Layer 2: Transaction volume Ethereum L2 melampaui mainnet — adopsi scaling solution makin masif. Solana overtake BNB ke posisi 4 market cap. Base Network TVL tembus $8 miliar, jadi L2 terbesar kedua.",
		"Tokenized Stocks & Institutional: BlackRock Bitcoin ETF cetak rekor inflow $1.3 miliar dalam sehari — sinyal institusional sangat positif. MicroStrategy tambah 12.000 BTC, total holding 450K+ BTC.",
		"Dampak Harga Jangka Pendek: Kombinasi inflow institusional besar dan akumulasi whale menekan supply yang tersedia. Potensi bullish dalam 1-2 minggu ke depan untuk BTC dan ETH. Layer 2 tokens seperti ARB dan OP berpotensi ikut terdorong.",
	}

	if err := repository.SaveBriefing(date, points, "BULLISH", newsCount); err != nil {
		return fmt.Errorf("save briefing: %w", err)
	}

	log.Println("[news] mock briefing saved")
	return nil
}

func (uc *NewsUsecase) GetLatestBriefing() (*domain.DailyBriefing, error) {
	b, err := repository.GetLatestBriefing()
	if err != nil {
		return nil, err
	}

	var points []string
	if err := json.Unmarshal([]byte(b.Points), &points); err != nil {
		return nil, fmt.Errorf("unmarshal points: %w", err)
	}

	return &domain.DailyBriefing{
		SummaryDate: b.SummaryDate,
		Points:      points,
		Sentiment:   b.Sentiment,
		NewsCount:   b.NewsCount,
		GeneratedAt: time.Unix(b.CreatedAt, 0).UTC().Format(time.RFC3339),
	}, nil
}

func (uc *NewsUsecase) HasTodayBriefing() bool {
	return repository.HasTodayBriefing()
}
