package usecase

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"

	"github.com/alif/crypto-portfolio/domain"
	"github.com/alif/crypto-portfolio/internal/predictor/engine"
	"github.com/alif/crypto-portfolio/internal/predictor/repository"
	"github.com/alif/crypto-portfolio/internal/shared/db"
	"github.com/alif/crypto-portfolio/internal/shared/deepseek"
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
		db.SavePriceHistory(symbol, p.Timestamp, p.Price)
		prices[i] = p.Price
	}

	// Also fill from local DB if needed
	if len(prices) < 200 {
		local, _ := db.GetPriceHistory(symbol, 400)
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

	signal := engine.GenerateSignal(prices)
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
		GeneratedAt: db.Now(),
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

type AISignalUsecase struct {
	deepseek        *deepseek.Client
	historyProvider  domain.HistoryProvider
}

func NewAISignalUsecase(deepseek *deepseek.Client, historyProvider domain.HistoryProvider) *AISignalUsecase {
	return &AISignalUsecase{deepseek: deepseek, historyProvider: historyProvider}
}

var currencyConfig = map[string]struct {
	symbol string
	code   string
}{
	"idr": {"Rp", "IDR"},
	"usd": {"$", "USD"},
	"sgd": {"S$", "SGD"},
	"myr": {"RM", "MYR"},
	"jpy": {"¥", "JPY"},
}

var exchangeRates = map[string]float64{
	"idr": 1,
	"usd": 0.0000557,
	"sgd": 0.0000752,
	"myr": 0.000245,
	"jpy": 0.00872,
}

func (uc *AISignalUsecase) GetAISignal(symbol, currency string, userAssets []domain.Asset) (*domain.AISignalResponse, error) {
	signalUc := &SignalUsecase{historyProvider: uc.historyProvider}
	signal, err := signalUc.GetSignal(symbol)
	if err != nil {
		return nil, err
	}

	cfg, ok := currencyConfig[currency]
	if !ok {
		cfg = currencyConfig["idr"]
		currency = "idr"
	}

	rate := exchangeRates[currency]
	if rate <= 0 {
		rate = 1
	}

	convertedPrice := signal.Price * rate
	ind := signal.Indicators
	convertedSMA50 := ind.SMA50 * rate
	convertedSMA200 := ind.SMA200 * rate

	// Build portfolio context for AI
	var portfolioLines []string
	for _, a := range userAssets {
		if a.ValueIDR > 0 {
			convertedValue := a.ValueIDR * rate
			portfolioLines = append(portfolioLines, fmt.Sprintf("  - %s: %.4f %s (%s %.0f)",
				a.Name, a.Amount, a.Symbol, cfg.symbol, convertedValue))
		}
	}
	portfolioStr := strings.Join(portfolioLines, "\n")
	if portfolioStr == "" {
		portfolioStr = "  (tidak ada aset)"
	}

	prompt := fmt.Sprintf(`Analisa aset kripto berikut dan berikan rekomendasi trading.

Aset: %s
Harga saat ini: %s %.0f %s
RSI(14): %.1f %s
SMA50: %s %.0f
SMA200: %s %.0f
%s
%s

Sinyal teknikal saat ini: %s (confidence: %.0f%%)

Portfolio kamu saat ini:
%s

Jika rekomendasinya adalah jual, kemana dana sebaiknya dipindahkan?
Pilihan: stablecoin (USDT/USDC), hold di cash/Rupiah, atau akumulasi aset lain yang sudah kamu miliki.

Output JSON dengan format:
{
  "action": "buy" atau "sell" atau "hold",
  "confidence": angka 0-100,
  "reasoning": "analisa 2-3 paragraf dalam bahasa Indonesia",
  "recommendation": {
    "action": "move_to_stablecoin" atau "move_to_asset" atau "hold_cash" atau "accumulate_asset",
    "target": "USDT" atau "USDC" atau "IDR" atau simbol aset lain,
    "reason": "penjelasan singkat mengapa pindah ke target tersebut"
  }
}

Catatan: field recommendation hanya diisi jika action adalah "sell". Jika action "buy" atau "hold", recommendation bisa null.`,
		symbol,
		cfg.symbol, convertedPrice, cfg.code,
		ind.RSI, rsiLabel(ind.RSI),
		cfg.symbol, convertedSMA50,
		cfg.symbol, convertedSMA200,
		smaLabel(ind.SMA50, ind.SMA200),
		macdLabel(ind.MACDHist, ind.MACDLine, ind.MACDSignal),
		signal.Action, math.Round(signal.Confidence),
		portfolioStr,
	)

	content, pt, ct, err := uc.deepseek.Analyze(prompt)
	if err != nil {
		return nil, err
	}
	costUSD := (float64(pt)*0.27 + float64(ct)*1.10) / 1_000_000
	log.Printf("[ai] %s: prompt %d tok → response %d tok → $%.6f", symbol, pt, ct, costUSD)

	var aiResp struct {
		Action         string                `json:"action"`
		Confidence     float64               `json:"confidence"`
		Reasoning      string                `json:"reasoning"`
		Recommendation *domain.AIRecommendation `json:"recommendation"`
	}

	if err := json.Unmarshal([]byte(content), &aiResp); err != nil {
		return nil, fmt.Errorf("deepseek parse: %w — content: %s", err, content)
	}

	switch aiResp.Action {
	case "buy", "sell", "hold":
	default:
		aiResp.Action = "hold"
	}

	return &domain.AISignalResponse{
		Symbol:         symbol,
		Action:         aiResp.Action,
		Confidence:     math.Min(100, math.Max(0, aiResp.Confidence)),
		Reasoning:      aiResp.Reasoning,
		Currency:       currency,
		Recommendation: aiResp.Recommendation,
		Indicators: domain.SignalIndicators{
			RSI:        ind.RSI,
			SMA50:      ind.SMA50,
			SMA200:     ind.SMA200,
			MACDLine:   ind.MACDLine,
			MACDSignal: ind.MACDSignal,
			MACDHist:   ind.MACDHist,
		},
		Usage: &domain.AIUsage{
			PromptTokens:     pt,
			CompletionTokens: ct,
			TotalTokens:      pt + ct,
			CostUSD:          math.Round(costUSD*1_000_000) / 1_000_000,
		},
	}, nil
}

func rsiLabel(rsi float64) string {
	if rsi < 30 {
		return "(oversold)"
	} else if rsi > 70 {
		return "(overbought)"
	}
	return "(netral)"
}

func smaLabel(sma50, sma200 float64) string {
	if sma50 > sma200 {
		return "Golden cross (SMA50 > SMA200) — tren naik"
	}
	return "Death cross (SMA50 < SMA200) — tren turun"
}

func macdLabel(hist, line, signal float64) string {
	if hist > 0 && line > signal {
		return "MACD bullish crossover — momentum positif"
	} else if hist < 0 && line < signal {
		return "MACD bearish crossover — momentum negatif"
	} else if hist > 0 {
		return "MACD histogram positif"
	}
	return "MACD histogram negatif"
}
