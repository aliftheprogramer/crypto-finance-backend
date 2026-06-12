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

// GetPortfolio returns the aggregated portfolio with all assets and total net worth in IDR.
// @Summary Get portfolio
// @Description Get aggregated portfolio with all assets, total net worth, and exchange rates
// @Tags portfolio
// @Accept json
// @Produce json
// @Success 200 {object} domain.Portfolio
// @Failure 500 {object} map[string]interface{}
// @Router /portfolio [get]
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

// GetAssetDetail returns detailed information about a specific asset including price history.
// @Summary Get asset detail
// @Description Get detailed information about a specific asset including current price, changes, and historical data
// @Tags portfolio
// @Accept json
// @Produce json
// @Param symbol path string true "Asset symbol (e.g., BTC, ETH)"
// @Success 200 {object} domain.AssetDetail
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /asset/{symbol} [get]
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
