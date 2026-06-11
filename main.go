package main

import (
	"log"

	"github.com/alif/crypto-portfolio/config"
	"github.com/alif/crypto-portfolio/domain"
	"github.com/alif/crypto-portfolio/internal/delivery/http"
	"github.com/alif/crypto-portfolio/internal/repository"
	"github.com/alif/crypto-portfolio/internal/usecase"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func main() {
	cfg := config.Load()

	fetchers := repository.CreateFetchers(cfg.Sources)
	coingecko := repository.NewCoinGeckoProvider()

	portfolioUsecase := usecase.NewPortfolioUsecase(fetchers, []domain.PriceProvider{
		coingecko,
		repository.NewYahooProvider(),
	}, coingecko, coingecko)

	assetUsecase := usecase.NewAssetUsecase(coingecko, coingecko, coingecko)

	handler := http.NewPortfolioHandler(portfolioUsecase, assetUsecase)

	app := fiber.New()
	app.Use(cors.New())

	api := app.Group("/api/v1")
	api.Get("/portfolio", handler.GetPortfolio)
	api.Get("/asset/:symbol", handler.GetAssetDetail)

	log.Printf("Server starting on port %s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
