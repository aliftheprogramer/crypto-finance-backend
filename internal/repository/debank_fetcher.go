package repository

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/alif/crypto-portfolio/domain"
)

var debankCache = NewCache(2 * time.Minute)

type DebankFetcher struct {
	name    string
	address string
	chains  []string
	http    *http.Client
	rpc     *rpcClient
}

func NewDebankFetcher(name, address string, chains []string) *DebankFetcher {
	return &DebankFetcher{
		name:    name,
		address: address,
		chains:  chains,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
		rpc: newRPCClient(),
	}
}

func (d *DebankFetcher) Name() string {
	return d.name
}

type debankToken struct {
	ID       string  `json:"id"`
	Chain    string  `json:"chain"`
	Name     string  `json:"name"`
	Symbol   string  `json:"symbol"`
	Decimals int     `json:"decimals"`
	Amount   float64 `json:"amount"`
}

type debankTokenResp struct {
	Data      []debankToken `json:"data"`
	ErrorCode int           `json:"error_code"`
}

func (d *DebankFetcher) FetchBalances() ([]domain.RawBalance, error) {
	cacheKey := "debank:" + d.address + ":" + strings.Join(d.chains, ",")
	if cached := debankCache.Get(cacheKey); cached != nil {
		return cached.([]domain.RawBalance), nil
	}

	var all []domain.RawBalance

	for i, chain := range d.chains {
		if i > 0 {
			time.Sleep(500 * time.Millisecond)
		}

		balances, err := d.fetchChainFast(chain)
		if err != nil {
			balances, err = d.rpc.fetchNativeBalance(chain, d.address)
			if err != nil {
				continue
			}
		}
		all = append(all, balances...)
	}

	debankCache.Set(cacheKey, all)
	return all, nil
}

func (d *DebankFetcher) fetchChainFast(chainID string) ([]domain.RawBalance, error) {
	url := fmt.Sprintf("https://api.debank.com/token/list?id=%s&chain_id=%s", d.address, chainID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := d.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("debank status %d", resp.StatusCode)
	}

	var result debankTokenResp
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.ErrorCode != 0 {
		return nil, fmt.Errorf("debank error code %d", result.ErrorCode)
	}

	var balances []domain.RawBalance
	for _, t := range result.Data {
		if t.Amount <= 0 {
			continue
		}
		divisor := math.Pow10(t.Decimals)
		humanAmount := t.Amount / divisor
		if humanAmount <= 0 {
			continue
		}
		balances = append(balances, domain.RawBalance{
			AssetName: t.Name,
			Symbol:    t.Symbol,
			Amount:    humanAmount,
		})
	}

	return balances, nil
}

type rpcClient struct {
	http *http.Client
}

func newRPCClient() *rpcClient {
	return &rpcClient{
		http: &http.Client{Timeout: 8 * time.Second},
	}
}

var chainRPC = map[string]struct {
	rpcURL string
	symbol string
	name   string
}{
	"eth":   {"https://eth.llamarpc.com", "ETH", "Ethereum"},
	"bsc":   {"https://bsc-dataseed1.binance.org", "BNB", "BNB"},
	"matic": {"https://polygon-rpc.com", "MATIC", "Polygon"},
	"arb":   {"https://arb1.arbitrum.io/rpc", "ETH", "Ethereum (Arbitrum)"},
	"op":    {"https://mainnet.optimism.io", "ETH", "Ethereum (Optimism)"},
}

func (r *rpcClient) fetchNativeBalance(chainID, address string) ([]domain.RawBalance, error) {
	chain, ok := chainRPC[chainID]
	if !ok {
		return nil, fmt.Errorf("unsupported chain: %s", chainID)
	}

	payload := fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"eth_getBalance","params":["%s","latest"],"id":1}`,
		strings.ToLower(address),
	)

	resp, err := r.http.Post(chain.rpcURL, "application/json", strings.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Result == "" || result.Result == "0x" || result.Result == "0x0" {
		return nil, fmt.Errorf("no native balance")
	}

	balance := weiToEther(result.Result)
	if balance <= 0 {
		return nil, fmt.Errorf("no native balance")
	}

	return []domain.RawBalance{
		{
			AssetName: chain.name,
			Symbol:    chain.symbol,
			Amount:    balance,
		},
	}, nil
}

func weiToEther(hex string) float64 {
	if len(hex) < 3 {
		return 0
	}
	hexStr := hex[2:]
	if hexStr == "" {
		return 0
	}

	weiInt, ok := new(big.Int).SetString(hexStr, 16)
	if !ok || weiInt.Sign() == 0 {
		return 0
	}

	weiFloat := new(big.Float).SetInt(weiInt)
	divisor := new(big.Float).SetFloat64(1e18)
	ether := new(big.Float).Quo(weiFloat, divisor)
	result, _ := ether.Float64()
	return result
}
