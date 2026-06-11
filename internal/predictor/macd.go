package predictor

type MACDResult struct {
	MACDLine   float64
	SignalLine float64
	Histogram  float64
}

func CalculateMACD(prices []float64) MACDResult {
	ema12 := CalculateEMA(prices, 12)
	ema26 := CalculateEMA(prices, 26)
	macdLine := ema12 - ema26

	macdHistory := make([]float64, len(prices))
	for i := 25; i < len(prices); i++ {
		e12 := calculateEMASlice(prices[:i+1], 12)
		e26 := calculateEMASlice(prices[:i+1], 26)
		macdHistory[i] = e12 - e26
	}

	signalLine := calculateEMASlice(macdHistory, 9)

	return MACDResult{
		MACDLine:   macdLine,
		SignalLine: signalLine,
		Histogram:  macdLine - signalLine,
	}
}

func calculateEMASlice(prices []float64, period int) float64 {
	if len(prices) < period {
		return 0
	}

	multiplier := 2.0 / float64(period+1)

	var sum float64
	for i := 0; i < period; i++ {
		sum += prices[i]
	}
	ema := sum / float64(period)

	for i := period; i < len(prices); i++ {
		ema = (prices[i]-ema)*multiplier + ema
	}

	return ema
}
