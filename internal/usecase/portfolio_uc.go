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
			log.Printf("Warning: skipping %s — %v", r.fetcherName, r.err)
			continue
		}
		allResults = append(allResults, r)
	}

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
			log.Printf("Warning: price provider error: %v", err)
			continue
		}
		for k, v := range providerPrices {
			if v > 0 {
				prices[k] = v
			}
		}
	}

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

	return domain.NewPortfolio(assets, prices, rates), nil
}
