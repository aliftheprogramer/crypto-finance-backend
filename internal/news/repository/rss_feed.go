package repository

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/alif/crypto-portfolio/domain"
	"github.com/mmcdole/gofeed"
)

type NewsProvider struct {
	urls []string
	fp   *gofeed.Parser
}

func NewNewsProvider() *NewsProvider {
	return &NewsProvider{
		urls: []string{
			"https://www.coindesk.com/arc/outboundfeeds/rss/",
			"https://cointelegraph.com/rss",
			"https://www.theblock.co/rss",
		},
		fp: gofeed.NewParser(),
	}
}

var keywordTags = map[string][]string{
	"btc":  {"BTC", "Layer-1"},
	"bitcoin": {"BTC", "Layer-1"},
	"ethereum": {"ETH", "Layer-1"},
	"eth":   {"ETH", "Layer-1"},
	"solana": {"SOL", "Layer-1"},
	"sol":   {"SOL", "Layer-1"},
	"bnb":   {"BNB", "Layer-1"},
	"xrp":   {"XRP", "Layer-1"},
	"cardano": {"ADA", "Layer-1"},
	"ada":   {"ADA", "Layer-1"},
	"avalanche": {"AVAX", "Layer-1"},
	"avax":  {"AVAX", "Layer-1"},
	"dot":   {"DOT", "Layer-1"},
	"polkadot": {"DOT", "Layer-1"},
	"arbitrum": {"ARB", "Layer-2"},
	"arb":   {"ARB", "Layer-2"},
	"optimism": {"OP", "Layer-2"},
	"op":    {"OP", "Layer-2"},
	"polygon": {"MATIC", "Layer-2"},
	"matic": {"MATIC", "Layer-2"},
	"base":  {"BASE", "Layer-2"},
	"layer 2": {"Layer-2"},
	"l2":    {"Layer-2"},
	"layer 1": {"Layer-1"},
	"l1":    {"Layer-1"},
	"etf":   {"BTC", "Tokenized-Stocks"},
	"blackrock": {"Tokenized-Stocks"},
	"microstrategy": {"BTC", "Tokenized-Stocks"},
	"tokenized": {"Tokenized-Stocks"},
	"institutional": {"Tokenized-Stocks"},
	"sec":   {"Regulation"},
	"regulation": {"Regulation"},
}

func categoriesFromText(title, desc string) []string {
	combined := strings.ToLower(title + " " + desc)
	seen := make(map[string]bool)
	var tags []string
	for keyword, t := range keywordTags {
		if strings.Contains(combined, keyword) && !seen[t[0]] {
			tags = append(tags, t...)
			for _, tt := range t {
				seen[tt] = true
			}
		}
	}
	if len(tags) == 0 {
		return []string{"General"}
	}
	return uniqStr(tags)
}

func (p *NewsProvider) FetchNews() ([]domain.NewsItem, error) {
	var allNews []domain.NewsItem
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, url := range p.urls {
		wg.Add(1)
		go func(feedURL string) {
			defer wg.Done()
			items, err := p.fetchFeed(feedURL)
			if err != nil {
				log.Printf("[news] rss error %s: %v", feedURL, err)
				return
			}
			mu.Lock()
			allNews = append(allNews, items...)
			mu.Unlock()
		}(url)
	}
	wg.Wait()

	if len(allNews) == 0 {
		log.Println("[news] all RSS feeds failed, using mock data")
		return mockNews()
	}

	// Sort by published_at descending
	for i := 0; i < len(allNews); i++ {
		for j := i + 1; j < len(allNews); j++ {
			if allNews[j].PublishedAt > allNews[i].PublishedAt {
				allNews[i], allNews[j] = allNews[j], allNews[i]
			}
		}
	}

	// Limit to latest 20
	if len(allNews) > 20 {
		allNews = allNews[:20]
	}

	return allNews, nil
}

func (p *NewsProvider) fetchFeed(url string) ([]domain.NewsItem, error) {
	feed, err := p.fp.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse feed: %w", err)
	}

	var items []domain.NewsItem
	for _, entry := range feed.Items {
		if entry.Title == "" {
			continue
		}
		if len(entry.Link) > 20 {
			continue
		}

		source := feed.Title
		if source == "" {
			source = extractDomain(url)
		}

		pubAt := time.Now().Unix()
		if entry.PublishedParsed != nil {
			pubAt = entry.PublishedParsed.Unix()
		}

		desc := ""
		if entry.Description != "" {
			desc = stripHTMLTags(entry.Description)
			if len(desc) > 300 {
				desc = desc[:300]
			}
		}

		tags := categoriesFromText(entry.Title, desc)
		// Skip truly general news with no crypto relevance
		if len(tags) == 1 && tags[0] == "General" && !isCryptoRelated(entry.Title, desc) {
			continue
		}

		link := ""
		if len(entry.Links) > 0 {
			link = entry.Links[0]
		}

		items = append(items, domain.NewsItem{
			Title:          entry.Title,
			URL:            link,
			Source:         source,
			PublishedAt:    pubAt,
			ContentSnippet: desc,
			Tags:           tags,
		})
	}

	return items, nil
}

func isCryptoRelated(title, desc string) bool {
	combined := strings.ToLower(title + " " + desc)
	keywords := []string{"bitcoin", "ethereum", "crypto", "blockchain", "btc", "eth",
		"token", "defi", "nft", "mining", "halving", "altcoin", "coin",
		"exchange", "wallet", "web3", "solana", "layer", "etf", "sec"}
	for _, k := range keywords {
		if strings.Contains(combined, k) {
			return true
		}
	}
	return false
}

func extractDomain(url string) string {
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return url
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

func stripHTMLTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}

func mockNews() ([]domain.NewsItem, error) {
	now := time.Now()
	return []domain.NewsItem{
		{
			Title:          "Ethereum Layer 2 Transaction Volume Surpasses Ethereum Mainnet for the First Time",
			URL:            "https://cointelegraph.com/news/ethereum-l2-volume-surpasses-mainnet",
			Source:         "CoinTelegraph",
			PublishedAt:    now.Add(-2 * time.Hour).Unix(),
			ContentSnippet: "For the first time in Ethereum's history, Layer 2 networks have processed more transactions than the Ethereum mainnet in a single day.",
			Tags:           []string{"ETH", "ARB", "OP", "Layer-2"},
		},
		{
			Title:          "Solana Overtakes BNB to Become 4th Largest Cryptocurrency by Market Cap",
			URL:            "https://coindesk.com/solana-overtakes-bnb-marketcap",
			Source:         "CoinDesk",
			PublishedAt:    now.Add(-4 * time.Hour).Unix(),
			ContentSnippet: "Solana's native token SOL has surged past BNB to claim the fourth position in cryptocurrency rankings.",
			Tags:           []string{"SOL", "Layer-1"},
		},
		{
			Title:          "BlackRock Bitcoin ETF Sees Record $1.3B Daily Inflow as Institutional Demand Surges",
			URL:            "https://theblock.co/blackrock-bitcoin-etf-record-inflow",
			Source:         "The Block",
			PublishedAt:    now.Add(-6 * time.Hour).Unix(),
			ContentSnippet: "BlackRock's iShares Bitcoin Trust (IBIT) recorded its largest single-day inflow of $1.3 billion.",
			Tags:           []string{"BTC", "Tokenized-Stocks"},
		},
		{
			Title:          "Arbitrum DAO Approves Historic Token Unlock Schedule Worth Over $1 Billion",
			URL:            "https://cointelegraph.com/arbitrum-token-unlock-approved",
			Source:         "CoinTelegraph",
			PublishedAt:    now.Add(-8 * time.Hour).Unix(),
			ContentSnippet: "Arbitrum DAO has approved a proposal to unlock over 1.1 billion ARB tokens over the next four years.",
			Tags:           []string{"ARB", "Layer-2"},
		},
		{
			Title:          "Bitcoin Hashrate Hits New All-Time High Above 700 EH/s",
			URL:            "https://coindesk.com/bitcoin-hashrate-new-ath",
			Source:         "CoinDesk",
			PublishedAt:    now.Add(-10 * time.Hour).Unix(),
			ContentSnippet: "Bitcoin's network hashrate has reached a new all-time high of 700 exahashes per second.",
			Tags:           []string{"BTC", "Layer-1"},
		},
		{
			Title:          "Coinbase Base Network TVL Surges Past $8 Billion Milestone",
			URL:            "https://theblock.co/base-network-tvl-8b",
			Source:         "The Block",
			PublishedAt:    now.Add(-12 * time.Hour).Unix(),
			ContentSnippet: "Coinbase's Ethereum Layer 2 network Base has surpassed $8 billion in total value locked.",
			Tags:           []string{"ETH", "Layer-2"},
		},
		{
			Title:          "MicroStrategy Adds 12,000 BTC to Corporate Treasury, Now Holds Over 450,000 BTC",
			URL:            "https://cointelegraph.com/microstrategy-adds-12000-btc",
			Source:         "CoinTelegraph",
			PublishedAt:    now.Add(-14 * time.Hour).Unix(),
			ContentSnippet: "MicroStrategy has purchased an additional 12,000 Bitcoin, bringing its total holdings to over 450,000 BTC.",
			Tags:           []string{"BTC", "Tokenized-Stocks"},
		},
		{
			Title:          "Ethereum Pectra Upgrade: Key Changes Validators Need to Know",
			URL:            "https://coindesk.com/ethereum-pectra-upgrade-validators",
			Source:         "CoinDesk",
			PublishedAt:    now.Add(-16 * time.Hour).Unix(),
			ContentSnippet: "Ethereum's next major upgrade Pectra introduces several EIPs aimed at improving validator experience.",
			Tags:           []string{"ETH", "Layer-1"},
		},
	}, nil
}
