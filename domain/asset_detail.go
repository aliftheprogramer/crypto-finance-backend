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

type SignalResponse struct {
	Symbol      string             `json:"symbol"`
	Action      string             `json:"action"`
	Confidence  float64            `json:"confidence"`
	Reasons     []string           `json:"reasons"`
	Price       float64            `json:"price"`
	Indicators  SignalIndicators   `json:"indicators"`
	GeneratedAt int64              `json:"generated_at"`
}

type SignalIndicators struct {
	RSI         float64 `json:"rsi"`
	SMA50       float64 `json:"sma_50"`
	SMA200      float64 `json:"sma_200"`
	MACDLine    float64 `json:"macd_line"`
	MACDSignal  float64 `json:"macd_signal"`
	MACDHist    float64 `json:"macd_histogram"`
}

type AISignalRequest struct {
	Symbol string `json:"symbol"`
}

type AIUsage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	CostUSD          float64 `json:"cost_usd"`
}

type AIRecommendation struct {
	Action string `json:"action"`
	Target string `json:"target"`
	Reason string `json:"reason"`
}

type AISignalResponse struct {
	Symbol         string             `json:"symbol"`
	Action         string             `json:"action"`
	Confidence     float64            `json:"confidence"`
	Reasoning      string             `json:"reasoning"`
	Currency       string             `json:"currency"`
	Indicators     SignalIndicators   `json:"indicators"`
	Recommendation *AIRecommendation  `json:"recommendation,omitempty"`
	Usage          *AIUsage           `json:"usage,omitempty"`
}
