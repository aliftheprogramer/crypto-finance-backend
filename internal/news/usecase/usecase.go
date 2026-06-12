package usecase

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/alif/crypto-portfolio/domain"
	"github.com/alif/crypto-portfolio/internal/news/repository"
	"github.com/alif/crypto-portfolio/internal/shared/deepseek"
)

type NewsUsecase struct {
	newsProvider *repository.NewsProvider
	deepseek     *deepseek.Client
}

func NewNewsUsecase(newsProvider *repository.NewsProvider, deepseek *deepseek.Client) *NewsUsecase {
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
	var l1l2Lines, stocksLines, generalLines []string
	for i, item := range news {
		tags := strings.Join(item.Tags, ", ")
		line := fmt.Sprintf("%d. [%s] %s\n   Sumber: %s\n   %s",
			i+1, tags, item.Title, item.Source, item.ContentSnippet)

		var hasL1L2, hasStocks bool
		for _, t := range item.Tags {
			switch t {
			case "Layer-1", "Layer-2":
				hasL1L2 = true
			case "Tokenized-Stocks":
				hasStocks = true
			}
		}
		if hasL1L2 {
			l1l2Lines = append(l1l2Lines, line)
		}
		if hasStocks {
			stocksLines = append(stocksLines, line)
		}
		if !hasL1L2 && !hasStocks {
			generalLines = append(generalLines, line)
		}
	}

	var sections []string
	if len(l1l2Lines) > 0 {
		sections = append(sections, "=== LAYER 1 & LAYER 2 ===\n"+strings.Join(l1l2Lines, "\n"))
	}
	if len(stocksLines) > 0 {
		sections = append(sections, "=== TOKENIZED STOCKS & INSTITUTIONAL ===\n"+strings.Join(stocksLines, "\n"))
	}
	if len(generalLines) > 0 {
		sections = append(sections, "=== BERITA LAINNYA ===\n"+strings.Join(generalLines, "\n"))
	}

	systemPrompt := `Kamu adalah analis finansial crypto senior. Tugasmu merangkum berita kripto harian menjadi briefing eksekutif yang padat dan aksionable.

Aturan:
- Gunakan bahasa Indonesia baku, natural, dan profesional
- Output HANYA JSON, tidak ada teks lain di luar JSON
- Setiap poin maksimal 2 kalimat, langsung ke inti berita dan dampaknya ke pasar
- Jangan gunakan markdown, bold, atau karakter formatting di dalam string JSON`

	userPrompt := fmt.Sprintf(`Berikut adalah berita kripto hari ini yang sudah dikelompokkan berdasarkan kategori:

%s

Buat ringkasan eksekutif maksimal 5 poin penting yang wajib diketahui trader hari ini.
Setiap poin maksimal 2 kalimat — langsung ke inti berita dan dampaknya ke pasar.

Pisahkan poin secara alami ke dalam kategori berikut:
1. Layer 1 & Layer 2 — perkembangan protokol utama dan solusi scaling
2. Tokenized Stocks & Institutional — ETF, inflow institusional, saham on-chain, adopsi korporasi
3. Dampak ke Harga Jangka Pendek — estimasi pergerakan harga 1-2 minggu ke depan berdasarkan berita di atas

Akhiri dengan sentimen pasar secara keseluruhan (BULLISH / BEARISH / NEUTRAL) berdasarkan bobot berita.

Output JSON dengan format:
{
  "sentiment": "BULLISH" atau "BEARISH" atau "NEUTRAL",
  "points": [
    "Poin pertama...",
    "Poin kedua...",
    "Poin ketiga..."
  ]
}`,
		strings.Join(sections, "\n\n"),
	)

	content, pt, ct, err := uc.deepseek.Chat(systemPrompt, userPrompt, 0.3, 1500)
	if err != nil {
		return fmt.Errorf("deepseek briefing: %w", err)
	}

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
		resp.Points = []string{"Belum ada ringkasan tersedia."}
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
