package usecase

import (
	"log"
	"math"

	"github.com/alif/crypto-portfolio/domain"
	"github.com/alif/crypto-portfolio/internal/predictor"
	"github.com/alif/crypto-portfolio/internal/repository"
)

type SignalUsecase struct {
	historyProvider domain.HistoryProvider
}

func NewSignalUsecase(historyProvider domain.HistoryProvider) *SignalUsecase {
	return &SignalUsecase{historyProvider: historyProvider}
}

func (uc *SignalUsecase) GetSignal(symbol string) (*domain.SignalResponse, error) {
	if repository.IsSignalFresh(symbol, 3600) {
		s, err := repository.GetSignal(symbol)
		if err == nil {
			log.Printf("[signal] %s: cache HIT", symbol)
			reasons := splitReasons(s.Reasons)
			return &domain.SignalResponse{
				Symbol:     symbol,
				Action:     s.Action,
				Confidence: s.Confidence,
				Reasons:    reasons,
				Price:      s.Price,
				Indicators: domain.SignalIndicators{
					RSI:        s.RSI,
					SMA50:      s.SMA50,
					SMA200:     s.SMA200,
					MACDLine:   s.MACDLine,
					MACDSignal: s.MACDSignal,
					MACDHist:   s.MACDHist,
				},
				GeneratedAt: s.GeneratedAt,
			}, nil
		}
	}

	log.Printf("[signal] %s: cache MISS → fetching history", symbol)
	history, err := uc.historyProvider.FetchHistory(symbol, 400)
	if err != nil {
		return nil, err
	}
	log.Printf("[signal] %s: history %d points", symbol, len(history))

	prices := make([]float64, len(history))
	for i, p := range history {
		repository.SavePriceHistory(symbol, p.Timestamp, p.Price)
		prices[i] = p.Price
	}

	// Also fill from local DB if needed
	if len(prices) < 200 {
		local, _ := repository.GetPriceHistory(symbol, 400)
		if len(local) > len(prices) {
			prices = make([]float64, len(local))
			for i, p := range local {
				prices[i] = p.Price
			}
		}
	}

	// Floor prices to avoid NaN from tiny values
	for i := range prices {
		if prices[i] <= 0 {
			prices[i] = 0.001
		}
	}

	signal := predictor.GenerateSignal(prices)
	if signal == nil {
		return nil, nil
	}
	log.Printf("[signal] %s: %s %.0f%% (RSI %.1f)", symbol, signal.Action, signal.Confidence, signal.Indicators.RSI)

	// Round values for display
	rsi := math.Round(signal.Indicators.RSI*100) / 100
	sma50 := math.Round(signal.Indicators.SMA50*100) / 100
	sma200 := math.Round(signal.Indicators.SMA200*100) / 100
	macdLine := math.Round(signal.Indicators.MACDLine*100) / 100
	macdSig := math.Round(signal.Indicators.MACDSignal*100) / 100
	macdHist := math.Round(signal.Indicators.MACDHist*100) / 100
	price := roundPrice(signal.Price)

	repository.SaveSignal(symbol, signal.Action, signal.Confidence, signal.Reasons,
		price, rsi, sma50, sma200, macdLine, macdSig, macdHist)

	return &domain.SignalResponse{
		Symbol:     symbol,
		Action:     signal.Action,
		Confidence: math.Round(signal.Confidence*10) / 10,
		Reasons:    signal.Reasons,
		Price:      price,
		Indicators: domain.SignalIndicators{
			RSI:        rsi,
			SMA50:      sma50,
			SMA200:     sma200,
			MACDLine:   macdLine,
			MACDSignal: macdSig,
			MACDHist:   macdHist,
		},
		GeneratedAt: repository.Now(),
	}, nil
}

func roundPrice(v float64) float64 {
	return math.Round(v*100) / 100
}

func splitReasons(s string) []string {
	if s == "" {
		return nil
	}
	parts := []string{}
	current := ""
	for _, c := range s {
		if c == '|' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
