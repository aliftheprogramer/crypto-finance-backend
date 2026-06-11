package domain

type AssetDetail struct {
	Name     string             `json:"name"`
	Symbol   string             `json:"symbol"`
	PriceIDR float64            `json:"price_idr"`
	Changes  map[string]float64 `json:"changes"`
	History  []PricePoint       `json:"history"`
}

type PricePoint struct {
	Timestamp int64   `json:"timestamp"`
	Price     float64 `json:"price"`
}

type AssetUsecase interface {
	GetAssetDetail(symbol string) (*AssetDetail, error)
}

type HistoryProvider interface {
	FetchHistory(symbol string, days int) ([]PricePoint, error)
}
