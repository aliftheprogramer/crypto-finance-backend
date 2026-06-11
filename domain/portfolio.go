package domain

import "time"

type Asset struct {
	Name           string  `json:"name"`
	Symbol         string  `json:"symbol"`
	Amount         float64 `json:"amount"`
	ValueIDR       float64 `json:"value_idr"`
	PriceChange24h float64 `json:"price_change_24h"`
	Location       string  `json:"location"`
}

type Portfolio struct {
	TotalNetWorthIDR float64            `json:"total_net_worth_idr"`
	ExchangeRates    map[string]float64 `json:"exchange_rates"`
	LastUpdated      string             `json:"last_updated"`
	Assets           []Asset            `json:"assets"`
}

type RawBalance struct {
	AssetName string
	Symbol    string
	Amount    float64
}

type BalanceFetcher interface {
	Name() string
	FetchBalances() ([]RawBalance, error)
}

type PriceProvider interface {
	FetchPrices(symbols []string) (map[string]float64, error)
}

type ChangeProvider interface {
	FetchChanges24h(symbols []string) (map[string]float64, error)
}

type ExchangeRateProvider interface {
	FetchExchangeRates() (map[string]float64, error)
}

type PortfolioUsecase interface {
	GetPortfolio() (*Portfolio, error)
}

func NewPortfolio(balances []Asset, prices map[string]float64, rates map[string]float64) *Portfolio {
	var totalNetWorth float64
	for i := range balances {
		value := balances[i].Amount * prices[balances[i].Symbol]
		balances[i].ValueIDR = value
		totalNetWorth += value
	}

	if balances == nil {
		balances = []Asset{}
	}
	if rates == nil {
		rates = map[string]float64{"idr": 1}
	} else {
		rates["idr"] = 1
	}

	return &Portfolio{
		TotalNetWorthIDR: totalNetWorth,
		ExchangeRates:    rates,
		LastUpdated:      time.Now().UTC().Format(time.RFC3339),
		Assets:           balances,
	}
}
