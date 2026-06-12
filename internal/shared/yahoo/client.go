package yahoo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/alif/crypto-portfolio/pkg/cache"
)

var yahooCache = cache.NewCache(2 * time.Minute)

func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

type YahooProvider struct {
	http       *http.Client
	usdToIDR   float64
	lastRateAt time.Time
	mu         sync.Mutex
}

func NewYahooProvider() *YahooProvider {
	return &YahooProvider{
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

var stockSymbolMap = map[string]string{
	"GOOGLX": "GOOGL",
	"AAPLX":  "AAPL",
	"TSLAX":  "TSLA",
	"AMZNX":  "AMZN",
	"NFLXX":  "NFLX",
	"MSFTX":  "MSFT",
	"NVDAX":  "NVDA",
	"METAX":  "META",
}

func (y *YahooProvider) FetchPrices(symbols []string) (map[string]float64, error) {
	cacheKey := "yahoo:" + joinSorted(symbols)
	if cached := yahooCache.Get(cacheKey); cached != nil {
		return cached.(map[string]float64), nil
	}

	usdRate, err := y.getUSDToIDR()
	if err != nil {
		return nil, fmt.Errorf("yahoo get usd rate: %w", err)
	}

	var stockTickers []string
	stockToSymbol := make(map[string]string)
	for _, sym := range symbols {
		ticker, ok := stockSymbolMap[sym]
		if !ok {
			continue
		}
		stockTickers = append(stockTickers, ticker)
		stockToSymbol[ticker] = sym
	}

	if len(stockTickers) == 0 {
		return map[string]float64{}, nil
	}

	type stockResult struct {
		ticker string
		price  float64
		err    error
	}

	var wg sync.WaitGroup
	wg.Add(len(stockTickers))
	resultsCh := make(chan stockResult, len(stockTickers))

	for _, ticker := range stockTickers {
		t := ticker
		go func() {
			defer wg.Done()
			price, err := y.fetchStockPrice(t)
			resultsCh <- stockResult{ticker: t, price: price, err: err}
		}()
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	prices := make(map[string]float64)
	for r := range resultsCh {
		if r.err != nil {
			continue
		}
		if symbol, ok := stockToSymbol[r.ticker]; ok {
			prices[symbol] = r.price * usdRate
		}
	}

	yahooCache.Set(cacheKey, prices)
	return prices, nil
}

func (y *YahooProvider) fetchStockPrice(ticker string) (float64, error) {
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=1d", ticker)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := y.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("yahoo %s error %d", ticker, resp.StatusCode)
	}

	var result struct {
		Chart struct {
			Result []struct {
				Meta struct {
					RegularMarketPrice interface{} `json:"regularMarketPrice"`
				} `json:"meta"`
			} `json:"result"`
		} `json:"chart"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}
	if len(result.Chart.Result) == 0 {
		return 0, fmt.Errorf("yahoo %s no result", ticker)
	}

	var price float64
	switch v := result.Chart.Result[0].Meta.RegularMarketPrice.(type) {
	case float64:
		price = v
	case string:
		price, _ = parseFloat(v)
	}
	if price <= 0 {
		return 0, fmt.Errorf("yahoo %s invalid price", ticker)
	}

	return price, nil
}

func (y *YahooProvider) getUSDToIDR() (float64, error) {
	y.mu.Lock()
	if y.usdToIDR > 0 && time.Since(y.lastRateAt) < 5*time.Minute {
		defer y.mu.Unlock()
		return y.usdToIDR, nil
	}
	y.mu.Unlock()

	url := "https://api.coingecko.com/api/v3/simple/price?ids=usd-coin&vs_currencies=idr"

	resp, err := y.http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("usd rate error (%d)", resp.StatusCode)
	}

	var result map[string]map[string]float64
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	rate, ok := result["usd-coin"]["idr"]
	if !ok || rate <= 0 {
		return 0, fmt.Errorf("failed to get USD/IDR rate")
	}

	y.mu.Lock()
	y.usdToIDR = rate
	y.lastRateAt = time.Now()
	y.mu.Unlock()

	return rate, nil
}

func joinSorted(s []string) string {
	if len(s) == 0 {
		return ""
	}
	cp := make([]string, len(s))
	copy(cp, s)
	for i := 0; i < len(cp); i++ {
		for j := i + 1; j < len(cp); j++ {
			if cp[i] > cp[j] {
				cp[i], cp[j] = cp[j], cp[i]
			}
		}
	}
	result := ""
	for i, v := range cp {
		if i > 0 {
			result += ","
		}
		result += v
	}
	return result
}
