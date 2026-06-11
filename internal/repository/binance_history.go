package repository

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/alif/crypto-portfolio/domain"
)

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
var binanceHistCache = NewCache(10 * time.Minute)

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
