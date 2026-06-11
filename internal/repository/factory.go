package repository

import (
	"github.com/alif/crypto-portfolio/config"
	"github.com/alif/crypto-portfolio/domain"
)

func CreateFetchers(cfgs []config.SourceConfig) []domain.BalanceFetcher {
	var fetchers []domain.BalanceFetcher

	for _, cfg := range cfgs {
		switch cfg.Type {
		case "mock":
			fetchers = append(fetchers, NewMockFetcher(cfg.Name))
		case "binance":
			if cfg.APIKey == "" || cfg.APISecret == "" {
				continue
			}
			fetchers = append(fetchers, NewBinanceFetcher(cfg.Name, cfg.APIKey, cfg.APISecret))
		case "indodax":
			if cfg.APIKey == "" || cfg.APISecret == "" {
				continue
			}
			fetchers = append(fetchers, NewIndodaxFetcher(cfg.Name, cfg.APIKey, cfg.APISecret))
		case "wallet":
			if cfg.Address == "" {
				continue
			}
			chains := cfg.Chains
			if len(chains) == 0 {
				chains = []string{"eth"}
			}
			fetchers = append(fetchers, NewDebankFetcher(cfg.Name, cfg.Address, chains))
		}
	}

	return fetchers
}
