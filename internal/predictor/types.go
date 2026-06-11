package predictor

type IndicatorResult struct {
	RSI         float64
	SMA50       float64
	SMA200      float64
	MACDLine    float64
	MACDSignal  float64
	MACDHist    float64
}

type SignalResult struct {
	Action     string          `json:"action"`
	Confidence float64         `json:"confidence"`
	Reasons    []string        `json:"reasons"`
	Price      float64         `json:"price"`
	Indicators IndicatorResult `json:"indicators"`
}

const (
	ActionBuy  = "buy"
	ActionSell = "sell"
	ActionHold = "hold"
)
