package http

import (
	"fmt"
	"log"

	"github.com/alif/crypto-portfolio/domain"
	"github.com/alif/crypto-portfolio/internal/usecase"
	"github.com/gofiber/fiber/v2"
)

type PortfolioHandler struct {
	portfolioUsecase domain.PortfolioUsecase
	assetUsecase     domain.AssetUsecase
	signalUsecase    *usecase.SignalUsecase
	aiSignalUsecase  *usecase.AISignalUsecase
}

func NewPortfolioHandler(portfolioUsecase domain.PortfolioUsecase, assetUsecase domain.AssetUsecase, signalUsecase *usecase.SignalUsecase, aiSignalUsecase *usecase.AISignalUsecase) *PortfolioHandler {
	return &PortfolioHandler{
		portfolioUsecase: portfolioUsecase,
		assetUsecase:     assetUsecase,
		signalUsecase:    signalUsecase,
		aiSignalUsecase:  aiSignalUsecase,
	}
}

func (h *PortfolioHandler) GetPortfolio(c *fiber.Ctx) error {
	log.Print("[http] GET /api/v1/portfolio")
	portfolio, err := h.portfolioUsecase.GetPortfolio()
	if err != nil {
		log.Printf("[http] GET /api/v1/portfolio → 500: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to fetch portfolio",
		})
	}
	log.Printf("[http] GET /api/v1/portfolio → 200: %d assets, Rp %.0f", len(portfolio.Assets), portfolio.TotalNetWorthIDR)
	return c.JSON(portfolio)
}

func (h *PortfolioHandler) GetAssetDetail(c *fiber.Ctx) error {
	symbol := c.Params("symbol")
	log.Printf("[http] GET /api/v1/asset/%s", symbol)
	if symbol == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "symbol required",
		})
	}

	detail, err := h.assetUsecase.GetAssetDetail(symbol)
	if err != nil {
		log.Printf("[http] GET /api/v1/asset/%s → 500: %v", symbol, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	if detail == nil {
		log.Printf("[http] GET /api/v1/asset/%s → 404", symbol)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "asset not found",
		})
	}

	log.Printf("[http] GET /api/v1/asset/%s → 200: price Rp %.0f", symbol, detail.PriceIDR)
	return c.JSON(detail)
}

func (h *PortfolioHandler) GetSignal(c *fiber.Ctx) error {
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

func (h *PortfolioHandler) GetAISignal(c *fiber.Ctx) error {
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

	// Fetch portfolio to get user assets for recommendation context
	// Ignore error — if portfolio fails, proceed with empty assets
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
