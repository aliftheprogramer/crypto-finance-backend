package usecase

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"

	"github.com/alif/crypto-portfolio/domain"
	"github.com/alif/crypto-portfolio/internal/repository"
)

type AISignalUsecase struct {
	deepseek        *repository.DeepSeekProvider
	historyProvider  domain.HistoryProvider
}

func NewAISignalUsecase(deepseek *repository.DeepSeekProvider, historyProvider domain.HistoryProvider) *AISignalUsecase {
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
