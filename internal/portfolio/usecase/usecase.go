package usecase

import (
	"log"
	"sync"

	"github.com/alif/crypto-portfolio/domain"
)

type portfolioUsecase struct {
	fetchers       []domain.BalanceFetcher
	priceProviders []domain.PriceProvider
	changeProvider domain.ChangeProvider
	rateProvider   domain.ExchangeRateProvider
}

func NewPortfolioUsecase(fetchers []domain.BalanceFetcher, priceProviders []domain.PriceProvider, changeProvider domain.ChangeProvider, rateProvider domain.ExchangeRateProvider) domain.PortfolioUsecase {
	return &portfolioUsecase{
		fetchers:       fetchers,
		priceProviders: priceProviders,
		changeProvider: changeProvider,
		rateProvider:   rateProvider,
	}
}

func (uc *portfolioUsecase) GetPortfolio() (*domain.Portfolio, error) {
	type result struct {
		fetcherName string
		balances    []domain.RawBalance
		err         error
	}

	var allResults []result

	var wg sync.WaitGroup
	wg.Add(len(uc.fetchers))

	resultsCh := make(chan result, len(uc.fetchers))
	for _, f := range uc.fetchers {
		fetcher := f
		go func() {
			defer wg.Done()
			balances, err := fetcher.FetchBalances()
			resultsCh <- result{
				fetcherName: fetcher.Name(),
				balances:    balances,
				err:         err,
			}
		}()
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	for r := range resultsCh {
		if r.err != nil {
			log.Printf("[portfolio] fetcher %s skipped: %v", r.fetcherName, r.err)
			continue
		}
		allResults = append(allResults, r)
	}
	log.Printf("[portfolio] fetchers: %d success, %d skipped", len(allResults), len(uc.fetchers)-len(allResults))

	symbolSet := make(map[string]bool)
	for _, res := range allResults {
		for _, b := range res.balances {
			symbolSet[b.Symbol] = true
		}
	}

	symbols := make([]string, 0, len(symbolSet))
	for s := range symbolSet {
		symbols = append(symbols, s)
	}

	prices := make(map[string]float64)
	for _, provider := range uc.priceProviders {
		providerPrices, err := provider.FetchPrices(symbols)
		if err != nil {
			log.Printf("[portfolio] price provider error: %v", err)
			continue
		}
		for k, v := range providerPrices {
			if v > 0 {
				prices[k] = v
			}
		}
	}
	log.Printf("[portfolio] prices: %d/%d symbols priced", len(prices), len(symbols))

	changes := make(map[string]float64)
	if uc.changeProvider != nil {
		var err error
		changes, err = uc.changeProvider.FetchChanges24h(symbols)
		if err != nil {
			log.Printf("Warning: change provider error: %v", err)
		}
	}

	var rates map[string]float64
	if uc.rateProvider != nil {
		var err error
		rates, err = uc.rateProvider.FetchExchangeRates()
		if err != nil {
			log.Printf("Warning: exchange rate error: %v", err)
		}
	}

	var assets []domain.Asset
	for _, res := range allResults {
		for _, b := range res.balances {
			assets = append(assets, domain.Asset{
				Name:           b.AssetName,
				Symbol:         b.Symbol,
				Amount:         b.Amount,
				PriceChange24h: changes[b.Symbol],
				Location:       res.fetcherName,
			})
		}
	}

	portfolio := domain.NewPortfolio(assets, prices, rates)
	log.Printf("[portfolio] total: Rp %.0f (%d assets)", portfolio.TotalNetWorthIDR, len(portfolio.Assets))
	return portfolio, nil
}

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
