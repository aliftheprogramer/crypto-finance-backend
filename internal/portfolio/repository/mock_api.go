package repository

import (
	"github.com/alif/crypto-portfolio/domain"
)

type MockFetcher struct {
	name string
}

func NewMockFetcher(name string) *MockFetcher {
	return &MockFetcher{name: name}
}

func (m *MockFetcher) Name() string {
	return m.name
}

func (m *MockFetcher) FetchBalances() ([]domain.RawBalance, error) {
	return []domain.RawBalance{
		{AssetName: "Bitcoin", Symbol: "BTC", Amount: 0.15},
		{AssetName: "Ethereum", Symbol: "ETH", Amount: 0.4},
	}, nil
}
