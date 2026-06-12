package coingecko

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/alif/crypto-portfolio/domain"
	"github.com/alif/crypto-portfolio/pkg/cache"
)

type combinedPriceData struct {
	prices  map[string]float64
	changes map[string]float64
}

var combinedCache = cache.NewCache(2 * time.Minute)
var historyCache = cache.NewCache(5 * time.Minute)

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

func joinSorted(s []string) string {
	copied := make([]string, len(s))
	copy(copied, s)
	sort.Strings(copied)
	return strings.Join(copied, ",")
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
// symbols that can be fetched from Binance USDT pairs
var binanceSymbols = map[string]string{
	"BTC": "BTCUSDT",
	"ETH": "ETHUSDT",
	"BNB": "BNBUSDT",
	"SOL": "SOLUSDT",
	"XRP": "XRPUSDT",
	"ADA": "ADAUSDT",
	"DOGE": "DOGEUSDT",
	"DOT": "DOTUSDT",
	"AVAX": "AVAXUSDT",
	"MATIC": "MATICUSDT",
	"LINK": "LINKUSDT",
	"UNI": "UNIUSDT",
	"ATOM": "ATOMUSDT",
	"NEAR": "NEARUSDT",
	"APT": "APTUSDT",
	"ARB": "ARBUSDT",
	"OP": "OPUSDT",
	"SUI": "SUIUSDT",
	"SEI": "SEIUSDT",
	"TIA": "TIAUSDT",
	"PEPE": "PEPEUSDT",
	"INJ": "INJUSDT",
	"FTM": "FTMUSDT",
	"AAVE": "AAVEUSDT",
	"CRV": "CRVUSDT",
	"ORCA": "ORCAUSDT",
}

func (c *CoinGeckoProvider) fetchHistoryFromBinance(symbol string, limit int) ([]domain.PricePoint, error) {
	pair, ok := binanceSymbols[symbol]
	if !ok {
		log.Printf("[binance] %s: symbol not found", symbol)
		return nil, fmt.Errorf("binance: unknown symbol %s", symbol)
	}
	log.Printf("[binance] %s: fetching %s (%d candles)", symbol, pair, limit)

	url := fmt.Sprintf("https://api.binance.com/api/v3/klines?symbol=%s&interval=1d&limit=%d", pair, limit)

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
		return nil, fmt.Errorf("binance history error (%d)", resp.StatusCode)
	}

	var raw [][]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	// Get USD/IDR rate for conversion (fallback if CoinGecko fails)
	usdToIDR := 17938.0 // default fallback rate
	if usdRate, err := c.FetchExchangeRates(); err == nil {
		if rate, ok := usdRate["usd"]; ok && rate > 0 {
			usdToIDR = 1.0 / rate
		}
	}

	prices := make([]domain.PricePoint, 0, len(raw))
	for _, item := range raw {
		if len(item) < 5 {
			continue
		}

		timestamp, ok := item[0].(float64)
		if !ok {
			continue
		}

		closeStr, ok := item[4].(string)
		if !ok {
			continue
		}

		var closePrice float64
		if _, err := fmt.Sscanf(closeStr, "%f", &closePrice); err != nil {
			continue
		}

		if closePrice <= 0 {
			continue
		}

		prices = append(prices, domain.PricePoint{
			Timestamp: int64(timestamp) / 1000,
			Price:     closePrice * usdToIDR,
		})
	}

	log.Printf("[binance] %s: %d data points fetched", pair, len(prices))
	return prices, nil
}

// Update rpcCache TTL for history
var binanceHistCache = cache.NewCache(10 * time.Minute)

func (c *CoinGeckoProvider) FetchHistory(symbol string, days int) ([]domain.PricePoint, error) {
	cacheKey := fmt.Sprintf("hist:%s:%d", symbol, days)
	if cached := historyCache.Get(cacheKey); cached != nil {
		return cached.([]domain.PricePoint), nil
	}
	if cached := binanceHistCache.Get(cacheKey); cached != nil {
		return cached.([]domain.PricePoint), nil
	}

	// Try CoinGecko first
	geckoID, ok := symbolToGeckoID[symbol]
	if ok {
		for attempt := 0; attempt < 3; attempt++ {
			if attempt > 0 {
				time.Sleep(time.Duration(attempt*3) * time.Second)
			}
			history, err := c.doFetchHistory(geckoID, days)
			if err == nil {
				historyCache.Set(cacheKey, history)
				return history, nil
			}
		}
	}

	// Fallback to Binance
	history, err := c.fetchHistoryFromBinance(symbol, days)
	if err == nil && len(history) > 0 {
		binanceHistCache.Set(cacheKey, history)
		return history, nil
	}

	return []domain.PricePoint{}, nil
}

func (c *CoinGeckoProvider) doFetchHistory(geckoID string, days int) ([]domain.PricePoint, error) {
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/coins/%s/market_chart?vs_currency=idr&days=%d", geckoID, days)

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
		return nil, fmt.Errorf("coingecko history error (%d)", resp.StatusCode)
	}

	var result struct {
		Prices [][]float64 `json:"prices"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	points := result.Prices
	if len(points) > 200 {
		step := float64(len(points)) / 200
		var sampled [][]float64
		for i := 0; i < 200; i++ {
			idx := int(float64(i) * step)
			if idx >= len(points) {
				idx = len(points) - 1
			}
			sampled = append(sampled, points[idx])
		}
		points = sampled
	}

	history := make([]domain.PricePoint, len(points))
	for i, p := range points {
		history[i] = domain.PricePoint{
			Timestamp: int64(p[0]) / 1000,
			Price:     p[1],
		}
	}

	return history, nil
}
