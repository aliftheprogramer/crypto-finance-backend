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
