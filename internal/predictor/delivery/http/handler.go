package http

import (
	"fmt"
	"log"

	"github.com/alif/crypto-portfolio/domain"
	"github.com/alif/crypto-portfolio/internal/predictor/usecase"
	"github.com/gofiber/fiber/v2"
)

type PredictorHandler struct {
	signalUsecase   *usecase.SignalUsecase
	aiSignalUsecase *usecase.AISignalUsecase
	portfolioUsecase domain.PortfolioUsecase
}

func NewPredictorHandler(signalUsecase *usecase.SignalUsecase, aiSignalUsecase *usecase.AISignalUsecase, portfolioUsecase domain.PortfolioUsecase) *PredictorHandler {
	return &PredictorHandler{
		signalUsecase:   signalUsecase,
		aiSignalUsecase: aiSignalUsecase,
		portfolioUsecase: portfolioUsecase,
	}
}

func (h *PredictorHandler) GetSignal(c *fiber.Ctx) error {
	symbol := c.Params("symbol")
	log.Printf("[http] GET /api/v1/asset/%s/signal", symbol)
	if symbol == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "symbol required",
		})
	}

	signal, err := h.signalUsecase.GetSignal(symbol)
	if err != nil {
		log.Printf("[http] GET /api/v1/asset/%s/signal → 500: %v", symbol, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	log.Printf("[http] GET /api/v1/asset/%s/signal → 200: %s %.0f%%", symbol, signal.Action, signal.Confidence)
	return c.JSON(signal)
}

func (h *PredictorHandler) GetAISignal(c *fiber.Ctx) error {
	symbol := c.Params("symbol")
	currency := c.Query("currency", "idr")
	log.Printf("[http] GET /api/v1/asset/%s/ai-signal?currency=%s", symbol, currency)
	if symbol == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "symbol required",
		})
	}

	if h.aiSignalUsecase == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "AI signal tidak tersedia (DeepSeek API key tidak dikonfigurasi)",
		})
	}

	var userAssets []domain.Asset
	if portfolio, err := h.portfolioUsecase.GetPortfolio(); err == nil {
		userAssets = portfolio.Assets
	}

	signal, err := h.aiSignalUsecase.GetAISignal(symbol, currency, userAssets)
	if err != nil {
		log.Printf("[http] GET /api/v1/asset/%s/ai-signal → 500: %v", symbol, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	costStr := "$0"
	if signal.Usage != nil {
		costStr = fmt.Sprintf("$%.6f", signal.Usage.CostUSD)
	}
	log.Printf("[http] GET /api/v1/asset/%s/ai-signal → 200: %s %.0f%% %s", symbol, signal.Action, signal.Confidence, costStr)
	return c.JSON(signal)
}
