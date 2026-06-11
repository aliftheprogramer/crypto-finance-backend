package http

import (
	"github.com/alif/crypto-portfolio/domain"
	"github.com/gofiber/fiber/v2"
)

type PortfolioHandler struct {
	portfolioUsecase domain.PortfolioUsecase
	assetUsecase     domain.AssetUsecase
}

func NewPortfolioHandler(portfolioUsecase domain.PortfolioUsecase, assetUsecase domain.AssetUsecase) *PortfolioHandler {
	return &PortfolioHandler{
		portfolioUsecase: portfolioUsecase,
		assetUsecase:     assetUsecase,
	}
}

func (h *PortfolioHandler) GetPortfolio(c *fiber.Ctx) error {
	portfolio, err := h.portfolioUsecase.GetPortfolio()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to fetch portfolio",
		})
	}
	return c.JSON(portfolio)
}

func (h *PortfolioHandler) GetAssetDetail(c *fiber.Ctx) error {
	symbol := c.Params("symbol")
	if symbol == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "symbol required",
		})
	}

	detail, err := h.assetUsecase.GetAssetDetail(symbol)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	if detail == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "asset not found",
		})
	}

	return c.JSON(detail)
}
