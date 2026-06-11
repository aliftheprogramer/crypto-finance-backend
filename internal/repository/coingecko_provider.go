package repository

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type combinedPriceData struct {
	prices  map[string]float64
	changes map[string]float64
}

var combinedCache = NewCache(2 * time.Minute)
var historyCache = NewCache(5 * time.Minute)

type CoinGeckoProvider struct {
	http        *http.Client
	rates       map[string]float64
	ratesAt     time.Time
	mu          sync.Mutex
}

func NewCoinGeckoProvider() *CoinGeckoProvider {
	return &CoinGeckoProvider{
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

var symbolToGeckoID = map[string]string{
	"BTC":  "bitcoin",
	"ETH":  "ethereum",
	"USDT": "tether",
	"USDC": "usd-coin",
	"BNB":  "binancecoin",
	"SOL":  "solana",
	"XRP":  "ripple",
	"ADA":  "cardano",
	"DOGE": "dogecoin",
	"MATIC": "matic-network",
	"AVAX": "avalanche-2",
	"DOT":  "polkadot",
	"LINK": "chainlink",
	"UNI":  "uniswap",
	"ARB":  "arbitrum",
	"OP":   "optimism",
	"SHIB": "shiba-inu",
	"TRX":  "tron",
	"ATOM": "cosmos",
	"NEAR": "near",
	"SUI":  "sui",
	"APT":  "aptos",
	"FTM":  "fantom",
	"CRV":  "curve-dao-token",
	"AAVE": "aave",
	"PEPE": "pepe",
	"INJ":  "injective-protocol",
	"SEI":  "sei-network",
	"TIA":  "celestia",
	"ORCA": "orca",
}

func (c *CoinGeckoProvider) FetchPrices(symbols []string) (map[string]float64, error) {
	data, err := c.getCombined(symbols)
	if err != nil {
		return nil, err
	}
	return data.prices, nil
}

func (c *CoinGeckoProvider) FetchChanges24h(symbols []string) (map[string]float64, error) {
	data, err := c.getCombined(symbols)
	if err != nil {
		return nil, err
	}
	return data.changes, nil
}

func (c *CoinGeckoProvider) getCombined(symbols []string) (*combinedPriceData, error) {
	sorted := joinSorted(symbols)
	cacheKey := "cg:" + sorted
	if cached := combinedCache.Get(cacheKey); cached != nil {
		log.Printf("[coingecko] fetch combined: %s → cache HIT", sorted)
		return cached.(*combinedPriceData), nil
	}

	geckoIDs := make([]string, 0, len(symbols))
	seen := make(map[string]bool)
	for _, s := range symbols {
		id, ok := symbolToGeckoID[s]
		if !ok || seen[id] {
			continue
		}
		geckoIDs = append(geckoIDs, id)
		seen[id] = true
	}

	if len(geckoIDs) == 0 {
		return &combinedPriceData{
			prices:  map[string]float64{},
			changes: map[string]float64{},
		}, nil
	}

	tryOnce := func() (*combinedPriceData, error) {
		return c.doFetchCombined(geckoIDs)
	}

	result, err := tryOnce()
	if err == nil {
		log.Printf("[coingecko] fetch combined: %s → 200 OK", sorted)
		combinedCache.Set(cacheKey, result)
		return result, nil
	}

	// Retry once after 1s
	log.Printf("[coingecko] fetch combined: %s → retry after error", sorted)
	time.Sleep(1 * time.Second)
	result, err = tryOnce()
	if err == nil {
		log.Printf("[coingecko] fetch combined: %s → 200 OK (retry)", sorted)
		combinedCache.Set(cacheKey, result)
		return result, nil
	}

	log.Printf("[coingecko] fetch combined: %s → failed: %v", sorted, err)
	return nil, err
}

func (c *CoinGeckoProvider) doFetchCombined(geckoIDs []string) (*combinedPriceData, error) {
	idsParam := strings.Join(geckoIDs, ",")
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/simple/price?ids=%s&vs_currencies=idr&include_24hr_change=true", idsParam)

	resp, err := c.http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("coingecko request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("coingecko read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("coingecko API error (%d)", resp.StatusCode)
	}

	var raw map[string]map[string]float64
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("coingecko parse: %w", err)
	}

	geckoToSymbol := make(map[string]string)
	for s, id := range symbolToGeckoID {
		geckoToSymbol[id] = s
	}

	result := &combinedPriceData{
		prices:  make(map[string]float64),
		changes: make(map[string]float64),
	}

	for geckoID, data := range raw {
		symbol, ok := geckoToSymbol[geckoID]
		if !ok {
			continue
		}
		if price, ok := data["idr"]; ok {
			result.prices[symbol] = price
		}
		if change, ok := data["idr_24h_change"]; ok {
			result.changes[symbol] = change
		}
	}

	return result, nil
}

func (c *CoinGeckoProvider) FetchExchangeRates() (map[string]float64, error) {
	c.mu.Lock()
	if c.rates != nil && time.Since(c.ratesAt) < 5*time.Minute {
		defer c.mu.Unlock()
		cp := make(map[string]float64, len(c.rates))
		for k, v := range c.rates {
			cp[k] = v
		}
		return cp, nil
	}
	c.mu.Unlock()

	var rates map[string]float64
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}

		rates, lastErr = c.doFetchRates()
		if lastErr == nil {
			c.mu.Lock()
			c.rates = rates
			c.ratesAt = time.Now()
			c.mu.Unlock()
			return rates, nil
		}
	}

	return nil, lastErr
}

func (c *CoinGeckoProvider) doFetchRates() (map[string]float64, error) {
	url := "https://api.coingecko.com/api/v3/simple/price?ids=usd-coin&vs_currencies=idr"

	resp, err := c.http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("coingecko rate error (%d)", resp.StatusCode)
	}

	var raw map[string]map[string]float64
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	usdData, ok := raw["usd-coin"]
	if !ok {
		return nil, fmt.Errorf("no usd-coin data")
	}

	idrPerUSD, ok := usdData["idr"]
	if !ok || idrPerUSD <= 0 {
		return nil, fmt.Errorf("no idr rate")
	}

	return map[string]float64{
		"usd": 1 / idrPerUSD,
		"sgd": 1.35 / idrPerUSD,
		"myr": 4.40 / idrPerUSD,
		"jpy": 156.5 / idrPerUSD,
	}, nil
}


