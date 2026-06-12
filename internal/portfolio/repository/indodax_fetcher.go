package repository

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/alif/crypto-portfolio/domain"
)

type IndodaxFetcher struct {
	name      string
	apiKey    string
	apiSecret string
	http      *http.Client
	nonce     int64
}

func NewIndodaxFetcher(name, apiKey, apiSecret string) *IndodaxFetcher {
	return &IndodaxFetcher{
		name:      name,
		apiKey:    apiKey,
		apiSecret: apiSecret,
		http:      &http.Client{Timeout: 10 * time.Second},
		nonce:     time.Now().UnixMilli(),
	}
}

func (i *IndodaxFetcher) Name() string {
	return i.name
}

type indodaxReturn struct {
	Balance     map[string]interface{} `json:"balance"`
	BalanceHold map[string]interface{} `json:"balance_hold"`
}

type indodaxInfoResp struct {
	Success int             `json:"success"`
	Return  indodaxReturn   `json:"return"`
}

func (i *IndodaxFetcher) FetchBalances() ([]domain.RawBalance, error) {
	i.nonce++

	payload := url.Values{}
	payload.Set("method", "getInfo")
	payload.Set("nonce", strconv.FormatInt(i.nonce, 10))

	mac := hmac.New(sha512.New, []byte(i.apiSecret))
	mac.Write([]byte(payload.Encode()))
	signature := fmt.Sprintf("%x", mac.Sum(nil))

	req, err := http.NewRequest("POST", "https://indodax.com/tapi", strings.NewReader(payload.Encode()))
	if err != nil {
		return nil, fmt.Errorf("%s create request: %w", i.name, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Key", i.apiKey)
	req.Header.Set("Sign", signature)

	resp, err := i.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s request: %w", i.name, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s read body: %w", i.name, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s API error (%d): %s", i.name, resp.StatusCode, string(body))
	}

	var info indodaxInfoResp
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("%s parse: %w", i.name, err)
	}
	if info.Success != 1 {
		return nil, fmt.Errorf("%s API returned error", i.name)
	}

	assetNameMap := map[string]string{
		"btc": "Bitcoin", "eth": "Ethereum", "usdt": "Tether",
		"bnb": "BNB", "sol": "Solana", "xrp": "Ripple",
		"ada": "Cardano", "matic": "Polygon", "doge": "Dogecoin",
		"avax": "Avalanche", "dot": "Polkadot", "link": "Chainlink",
	}

	allSymbols := make(map[string]float64)
	collectBalances(info.Return.Balance, allSymbols)
	collectBalances(info.Return.BalanceHold, allSymbols)

	var balances []domain.RawBalance
	for sym, total := range allSymbols {
		if total <= 0 {
			continue
		}
		name := assetNameMap[strings.ToLower(sym)]
		if name == "" {
			name = sym
		}
		balances = append(balances, domain.RawBalance{
			AssetName: name,
			Symbol:    sym,
			Amount:    total,
		})
	}

	return balances, nil
}

func collectBalances(src map[string]interface{}, dest map[string]float64) {
	for sym, val := range src {
		var amount float64
		switch v := val.(type) {
		case string:
			amount, _ = strconv.ParseFloat(v, 64)
		case float64:
			amount = v
		case int:
			amount = float64(v)
		}
		if amount > 0 {
			dest[strings.ToUpper(sym)] += amount
		}
	}
}
