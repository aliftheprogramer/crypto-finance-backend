package repository

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/alif/crypto-portfolio/domain"
)

type NewsProvider struct {
	apiKey string
	http   *http.Client
}

func NewNewsProvider(apiKey string) *NewsProvider {
	return &NewsProvider{
		apiKey: apiKey,
		http:   &http.Client{Timeout: 15 * time.Second},
	}
}

type cryptopanicPost struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	PublishedAt string `json:"published_at"`
	URL         string `json:"url"`
	Source      struct {
		Title string `json:"title"`
	} `json:"source"`
	Currencies []struct {
		Code string `json:"code"`
	} `json:"currencies"`
}

type cryptopanicResponse struct {
	Results []cryptopanicPost `json:"results"`
}

func (p *NewsProvider) FetchNews() ([]domain.NewsItem, error) {
	if p.apiKey == "" {
		log.Println("[news] no CryptoPanic API key, using mock data")
		return mockNews()
	}
	return p.fetchCryptopanic()
}

func (p *NewsProvider) fetchCryptopanic() ([]domain.NewsItem, error) {
	url := fmt.Sprintf("https://cryptopanic.com/api/v1/posts/?auth_token=%s&public=true&kind=news&regions=en", p.apiKey)

	resp, err := p.http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("cryptopanic request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cryptopanic read: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cryptopanic API error (%d): %s", resp.StatusCode, string(body))
	}

	var result cryptopanicResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("cryptopanic parse: %w", err)
	}

	var news []domain.NewsItem
	for _, post := range result.Results {
		tags := []string{}
		for _, c := range post.Currencies {
			tags = append(tags, c.Code)
		}

		t, _ := time.Parse(time.RFC3339, post.PublishedAt)

		news = append(news, domain.NewsItem{
			Title:          post.Title,
			URL:            post.URL,
			Source:         post.Source.Title,
			PublishedAt:    t.Unix(),
			ContentSnippet: "",
			Tags:           tags,
		})
	}

	return news, nil
}

func mockNews() ([]domain.NewsItem, error) {
	now := time.Now()
	return []domain.NewsItem{
		{
			Title:          "Ethereum Layer 2 Transaction Volume Surpasses Ethereum Mainnet for the First Time",
			URL:            "https://cointelegraph.com/news/ethereum-l2-volume-surpasses-mainnet",
			Source:         "CoinTelegraph",
			PublishedAt:    now.Add(-2 * time.Hour).Unix(),
			ContentSnippet: "For the first time in Ethereum's history, Layer 2 networks have processed more transactions than the Ethereum mainnet in a single day, signaling a major shift towards scaling solutions.",
			Tags:           []string{"ETH", "ARB", "OP"},
		},
		{
			Title:          "Solana Overtakes BNB to Become 4th Largest Cryptocurrency by Market Cap",
			URL:            "https://coindesk.com/solana-overtakes-bnb-marketcap",
			Source:         "CoinDesk",
			PublishedAt:    now.Add(-4 * time.Hour).Unix(),
			ContentSnippet: "Solana's native token SOL has surged past BNB to claim the fourth position in cryptocurrency rankings, driven by growing DeFi activity and memecoin trading on the network.",
			Tags:           []string{"SOL"},
		},
		{
			Title:          "BlackRock Bitcoin ETF Sees Record $1.3B Daily Inflow as Institutional Demand Surges",
			URL:            "https://theblock.co/blackrock-bitcoin-etf-record-inflow",
			Source:         "The Block",
			PublishedAt:    now.Add(-6 * time.Hour).Unix(),
			ContentSnippet: "BlackRock's iShares Bitcoin Trust (IBIT) recorded its largest single-day inflow of $1.3 billion, reflecting unprecedented institutional demand for Bitcoin exposure through regulated ETFs.",
			Tags:           []string{"BTC"},
		},
		{
			Title:          "Arbitrum DAO Approves Historic Token Unlock Schedule Worth Over $1 Billion",
			URL:            "https://cointelegraph.com/arbitrum-token-unlock-approved",
			Source:         "CoinTelegraph",
			PublishedAt:    now.Add(-8 * time.Hour).Unix(),
			ContentSnippet: "Arbitrum DAO has approved a proposal to unlock over 1.1 billion ARB tokens over the next four years, sparking debate about tokenomics and selling pressure.",
			Tags:           []string{"ARB"},
		},
		{
			Title:          "Bitcoin Hashrate Hits New All-Time High Above 700 EH/s",
			URL:            "https://coindesk.com/bitcoin-hashrate-new-ath",
			Source:         "CoinDesk",
			PublishedAt:    now.Add(-10 * time.Hour).Unix(),
			ContentSnippet: "Bitcoin's network hashrate has reached a new all-time high of 700 exahashes per second, underscoring the growing security and miner confidence in the network.",
			Tags:           []string{"BTC"},
		},
		{
			Title:          "Coinbase Base Network TVL Surges Past $8 Billion Milestone",
			URL:            "https://theblock.co/base-network-tvl-8b",
			Source:         "The Block",
			PublishedAt:    now.Add(-12 * time.Hour).Unix(),
			ContentSnippet: "Coinbase's Ethereum Layer 2 network Base has surpassed $8 billion in total value locked, cementing its position as the second-largest L2 network behind Arbitrum.",
			Tags:           []string{"ETH"},
		},
		{
			Title:          "MicroStrategy Adds 12,000 BTC to Corporate Treasury, Now Holds Over 450,000 BTC",
			URL:            "https://cointelegraph.com/microstrategy-adds-12000-btc",
			Source:         "CoinTelegraph",
			PublishedAt:    now.Add(-14 * time.Hour).Unix(),
			ContentSnippet: "MicroStrategy has purchased an additional 12,000 Bitcoin for approximately $1.1 billion, bringing its total holdings to over 450,000 BTC worth approximately $42 billion.",
			Tags:           []string{"BTC"},
		},
		{
			Title:          "Ethereum Pectra Upgrade: Key Changes Validators Need to Know",
			URL:            "https://coindesk.com/ethereum-pectra-upgrade-validators",
			Source:         "CoinDesk",
			PublishedAt:    now.Add(-16 * time.Hour).Unix(),
			ContentSnippet: "Ethereum's next major upgrade Pectra introduces several EIPs aimed at improving validator experience, account abstraction, and network efficiency.",
			Tags:           []string{"ETH"},
		},
	}, nil
}

func CategoriesFromTags(tags []string) string {
	var cats []string
	for _, t := range tags {
		t = strings.ToUpper(t)
		switch t {
		case "BTC", "ETH", "SOL", "BNB", "ADA", "AVAX", "DOT", "XRP":
			cats = append(cats, "Layer-1")
		case "ARB", "OP", "MATIC", "BASE":
			cats = append(cats, "Layer-2")
		default:
			cats = append(cats, "Tokenized-Stocks")
		}
	}
	if len(cats) == 0 {
		return "General"
	}
	return strings.Join(uniqStr(cats), ",")
}

func uniqStr(items []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range items {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
