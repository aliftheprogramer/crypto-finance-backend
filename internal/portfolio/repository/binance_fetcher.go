package repository

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/alif/crypto-portfolio/domain"
)

type BinanceFetcher struct {
	name      string
	apiKey    string
	apiSecret string
	http      *http.Client
}

func NewBinanceFetcher(name, apiKey, apiSecret string) *BinanceFetcher {
	return &BinanceFetcher{
		name:      name,
		apiKey:    apiKey,
		apiSecret: apiSecret,
		http:      &http.Client{Timeout: 10 * time.Second},
	}
}

func (b *BinanceFetcher) Name() string {
	return b.name
}

type binanceBalanceItem struct {
	Asset  string `json:"asset"`
	Free   string `json:"free"`
	Locked string `json:"locked"`
}

type binanceAccountResp struct {
	Balances []binanceBalanceItem `json:"balances"`
}

func (b *BinanceFetcher) FetchBalances() ([]domain.RawBalance, error) {
	timestamp := time.Now().UnixMilli()
	query := fmt.Sprintf("timestamp=%d", timestamp)

	mac := hmac.New(sha256.New, []byte(b.apiSecret))
	mac.Write([]byte(query))
	signature := hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequest("GET", "https://api.binance.com/api/v3/account?"+query+"&signature="+signature, nil)
	if err != nil {
		return nil, fmt.Errorf("%s create request: %w", b.name, err)
	}
	req.Header.Set("X-MBX-APIKEY", b.apiKey)

	resp, err := b.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s request: %w", b.name, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s read body: %w", b.name, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s API error (%d): %s", b.name, resp.StatusCode, string(body))
	}

	var account binanceAccountResp
	if err := json.Unmarshal(body, &account); err != nil {
		return nil, fmt.Errorf("%s parse: %w", b.name, err)
	}

	var balances []domain.RawBalance
	for _, item := range account.Balances {
		free, _ := strconv.ParseFloat(item.Free, 64)
		locked, _ := strconv.ParseFloat(item.Locked, 64)
		total := free + locked
		if total > 0 {
			balances = append(balances, domain.RawBalance{
				AssetName: item.Asset,
				Symbol:    item.Asset,
				Amount:    total,
			})
		}
	}

	return balances, nil
}
