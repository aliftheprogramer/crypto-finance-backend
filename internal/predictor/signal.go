package predictor

func GenerateSignal(prices []float64) *SignalResult {
	if len(prices) < 200 {
		return &SignalResult{
			Action:     ActionHold,
			Confidence: 0,
			Reasons:    []string{"Data historis tidak mencukupi (min 200 data point)"},
		}
	}

	rsi := CalculateRSI(prices, 14)
	sma50 := CalculateSMA(prices, 50)
	sma200 := CalculateSMA(prices, 200)
	macd := CalculateMACD(prices)
	latestPrice := prices[len(prices)-1]

	var score int
	var reasons []string

	// --- Trend Indicators (max ±60) ---

	// SMA Death/Golden Cross (±30)
	if sma50 > 0 && sma200 > 0 {
		if latestPrice > sma50 && sma50 > sma200 {
			score += 30
			reasons = append(reasons, "Golden cross (SMA50 > SMA200) — bullish trend")
		} else if latestPrice < sma50 && sma50 < sma200 {
			score -= 30
			reasons = append(reasons, "Death cross (SMA50 < SMA200) — bearish trend")
		} else if latestPrice > sma50 {
			score += 15
			reasons = append(reasons, "Harga di atas SMA(50) — bullish")
		} else {
			score -= 15
			reasons = append(reasons, "Harga di bawah SMA(50) — bearish")
		}
	}

	// MACD Histogram (±30)
	if macd.Histogram > 0 && macd.MACDLine > macd.SignalLine {
		score += 30
		reasons = append(reasons, "MACD bullish crossover — momentum positif")
	} else if macd.Histogram < 0 && macd.MACDLine < macd.SignalLine {
		score -= 30
		reasons = append(reasons, "MACD bearish crossover — momentum negatif")
	} else if macd.Histogram > 0 {
		score += 15
		reasons = append(reasons, "MACD histogram positif")
	} else {
		score -= 15
		reasons = append(reasons, "MACD histogram negatif")
	}

	// --- Momentum Indicators (max ±40) ---

	// RSI
	if rsi < 30 {
		score += 40
		reasons = append(reasons, "RSI oversold — potensi reversal naik")
	} else if rsi > 70 {
		score -= 40
		reasons = append(reasons, "RSI overbought — potensi reversal turun")
	}

	// --- Map score to action & confidence ---

	var action string
	var confidence float64

	switch {
	case score >= 50:
		action = ActionBuy
		confidence = float64(score)
		if confidence > 100 {
			confidence = 100
		}
	case score >= 15:
		action = ActionBuy
		confidence = float64(score)
	case score <= -50:
		action = ActionSell
		confidence = float64(-score)
		if confidence > 100 {
			confidence = 100
		}
	case score <= -15:
		action = ActionSell
		confidence = float64(-score)
	default:
		action = ActionHold
		confidence = 50
	}

	return &SignalResult{
		Action:     action,
		Confidence: confidence,
		Reasons:    reasons,
		Price:      latestPrice,
		Indicators: IndicatorResult{
			RSI:        rsi,
			SMA50:      sma50,
			SMA200:     sma200,
			MACDLine:   macd.MACDLine,
			MACDSignal: macd.SignalLine,
			MACDHist:   macd.Histogram,
		},
	}
}
