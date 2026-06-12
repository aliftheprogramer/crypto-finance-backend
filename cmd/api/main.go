package main

import (
	"log"
	"time"

	"github.com/alif/crypto-portfolio/config"
	"github.com/alif/crypto-portfolio/domain"
	"github.com/alif/crypto-portfolio/internal/news/delivery/http"
	"github.com/alif/crypto-portfolio/internal/news/repository"
	"github.com/alif/crypto-portfolio/internal/news/usecase"
	portfoliohttp "github.com/alif/crypto-portfolio/internal/portfolio/delivery/http"
	portfoliorepo "github.com/alif/crypto-portfolio/internal/portfolio/repository"
	portfoliousecase "github.com/alif/crypto-portfolio/internal/portfolio/usecase"
	predictorhttp "github.com/alif/crypto-portfolio/internal/predictor/delivery/http"
	predictorusecase "github.com/alif/crypto-portfolio/internal/predictor/usecase"
	"github.com/alif/crypto-portfolio/internal/shared/coingecko"
	"github.com/alif/crypto-portfolio/internal/shared/db"
	"github.com/alif/crypto-portfolio/internal/shared/deepseek"
	"github.com/alif/crypto-portfolio/internal/shared/yahoo"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func main() {
	cfg := config.Load()

	db.GetDB()

	fetchers := portfoliorepo.CreateFetchers(cfg.Sources)
	gecko := coingecko.NewCoinGeckoProvider()
	yhoo := yahoo.NewYahooProvider()

	var ds *deepseek.Client
	if cfg.DeepSeekAPIKey != "" {
		ds = deepseek.NewClient(cfg.DeepSeekAPIKey)
		log.Println("AI: DeepSeek API enabled")
	} else {
		log.Println("AI: DeepSeek API disabled (set DEEPSEEK_API_KEY in .env)")
	}

	portfolioUsecase := portfoliousecase.NewPortfolioUsecase(fetchers, []domain.PriceProvider{
		gecko, yhoo,
	}, gecko, gecko)
	assetUsecase := portfoliousecase.NewAssetUsecase(gecko, gecko, gecko)
	signalUsecase := predictorusecase.NewSignalUsecase(gecko)

	var aiSignalUsecase *predictorusecase.AISignalUsecase
	if ds != nil {
		aiSignalUsecase = predictorusecase.NewAISignalUsecase(ds, gecko)
	}

	newsProvider := repository.NewNewsProvider()
	newsUsecase := usecase.NewNewsUsecase(newsProvider, ds)

	portfolioHandler := portfoliohttp.NewPortfolioHandler(portfolioUsecase, assetUsecase)
	predictorHandler := predictorhttp.NewPredictorHandler(signalUsecase, aiSignalUsecase, portfolioUsecase)
	newsHandler := http.NewNewsHandler(newsUsecase)

	app := fiber.New()
	app.Use(cors.New())

	api := app.Group("/api/v1")
	api.Get("/portfolio", portfolioHandler.GetPortfolio)
	api.Get("/asset/:symbol", portfolioHandler.GetAssetDetail)
	api.Get("/asset/:symbol/signal", predictorHandler.GetSignal)
	api.Get("/asset/:symbol/ai-signal", predictorHandler.GetAISignal)
	api.Get("/news/briefing", newsHandler.GetDailyBriefing)

	go func() {
		if !newsUsecase.HasTodayBriefing() {
			log.Println("[news] generating initial briefing...")
			if err := newsUsecase.Refresh(); err != nil {
				log.Printf("[news] initial briefing: %v", err)
			}
		} else {
			log.Println("[news] today's briefing already exists")
		}

		ticker := time.NewTicker(6 * time.Hour)
		for range ticker.C {
			log.Println("[news] scheduled refresh...")
			if err := newsUsecase.Refresh(); err != nil {
				log.Printf("[news] scheduled refresh: %v", err)
			}
		}
	}()

	log.Printf("Server starting on port %s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
