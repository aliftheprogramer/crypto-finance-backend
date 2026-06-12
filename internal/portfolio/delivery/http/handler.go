package http

import (
	"log"

	"github.com/alif/crypto-portfolio/domain"
	"github.com/gofiber/fiber/v2"
)

type PortfolioHandler struct {
	portfolioUsecase domain.PortfolioUsecase
	assetUsecase     domain.AssetUsecase
}

func NewPortfolioHandler(portfolioUsecase domain.PortfolioUsecase, assetUsecase domain.AssetUsecase) *PortfolioHandler {
	return &PortfolioHandler{portfolioUsecase: portfolioUsecase, assetUsecase: assetUsecase}
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
