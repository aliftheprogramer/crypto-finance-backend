package usecase

import (
	"log"

	"github.com/alif/crypto-portfolio/domain"
)

type assetUsecase struct {
	priceProvider   domain.PriceProvider
	changeProvider  domain.ChangeProvider
	historyProvider domain.HistoryProvider
}

func NewAssetUsecase(priceProvider domain.PriceProvider, changeProvider domain.ChangeProvider, historyProvider domain.HistoryProvider) domain.AssetUsecase {
	return &assetUsecase{
		priceProvider:   priceProvider,
		changeProvider:  changeProvider,
		historyProvider: historyProvider,
	}
}

func (uc *assetUsecase) GetAssetDetail(symbol string) (*domain.AssetDetail, error) {
	prices, _ := uc.priceProvider.FetchPrices([]string{symbol})
	price := prices[symbol]

	changes, err := uc.changeProvider.FetchChanges24h([]string{symbol})
	if err != nil {
		changes = map[string]float64{}
	}

	history, err := uc.historyProvider.FetchHistory(symbol, 365)
	if err != nil {
		history = []domain.PricePoint{}
	}

	if price <= 0 && len(history) > 0 {
		price = history[len(history)-1].Price
		log.Printf("[asset] %s: price fallback from history = Rp %.0f", symbol, price)
	}

	changePct := computeChanges(history)

	if changePct == nil {
		changePct = map[string]float64{}
	}
	if c24h, ok := changes[symbol]; ok {
		changePct["1d"] = c24h
	}

	log.Printf("[asset] %s: price Rp %.0f, history %d points", symbol, price, len(history))

	return &domain.AssetDetail{
		Name:     symbol,
		Symbol:   symbol,
		PriceIDR: price,
		Changes:  changePct,
		History:  history,
	}, nil
}

func computeChanges(history []domain.PricePoint) map[string]float64 {
	if len(history) < 2 {
		return nil
	}

	latest := history[len(history)-1].Price
	if latest <= 0 {
		return nil
	}

	changes := map[string]float64{}

	targets := []struct {
		key  string
		days int
	}{
		{"7d", 7},
		{"30d", 30},
		{"365d", 365},
	}

	now := history[len(history)-1].Timestamp
	for _, t := range targets {
		targetTime := now - int64(t.days*86400)
		closest := findClosest(history, targetTime)
		if closest > 0 && closest != latest {
			changes[t.key] = (latest - closest) / closest * 100
		}
	}

	return changes
}

func findClosest(history []domain.PricePoint, targetTime int64) float64 {
	var closest float64
	var minDiff int64 = -1

	for _, p := range history {
		diff := p.Timestamp - targetTime
		if diff < 0 {
			diff = -diff
		}
		if minDiff == -1 || diff < minDiff {
			minDiff = diff
			closest = p.Price
		}
	}

	return closest
}
