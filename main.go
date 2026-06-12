package main

import (
	"log"
	"time"

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

	// Init DB (SQLite)
	repository.GetDB()

	fetchers := repository.CreateFetchers(cfg.Sources)
	coingecko := repository.NewCoinGeckoProvider()

	portfolioUsecase := usecase.NewPortfolioUsecase(fetchers, []domain.PriceProvider{
		coingecko,
		repository.NewYahooProvider(),
	}, coingecko, coingecko)

	assetUsecase := usecase.NewAssetUsecase(coingecko, coingecko, coingecko)
	signalUsecase := usecase.NewSignalUsecase(coingecko)

	var deepseek *repository.DeepSeekProvider
	if cfg.DeepSeekAPIKey != "" {
		deepseek = repository.NewDeepSeekProvider(cfg.DeepSeekAPIKey)
		log.Println("AI: DeepSeek API enabled")
	} else {
		log.Println("AI: DeepSeek API disabled (set DEEPSEEK_API_KEY in .env)")
	}

	var aiSignalUsecase *usecase.AISignalUsecase
	if deepseek != nil {
		aiSignalUsecase = usecase.NewAISignalUsecase(deepseek, coingecko)
		log.Println("AI Signal: enabled")
	} else {
		log.Println("AI Signal: disabled")
	}

	newsProvider := repository.NewNewsProvider(cfg.CryptoPanicAPIKey)
	newsUsecase := usecase.NewNewsUsecase(newsProvider, deepseek)
	log.Println("Daily Briefing: enabled")

	handler := http.NewPortfolioHandler(portfolioUsecase, assetUsecase, signalUsecase, aiSignalUsecase, newsUsecase)

	app := fiber.New()
	app.Use(cors.New())

	api := app.Group("/api/v1")
	api.Get("/portfolio", handler.GetPortfolio)
	api.Get("/asset/:symbol", handler.GetAssetDetail)
	api.Get("/asset/:symbol/signal", handler.GetSignal)
	api.Get("/asset/:symbol/ai-signal", handler.GetAISignal)
	api.Get("/news/briefing", handler.GetDailyBriefing)

	// Auto-generate daily briefing on startup + every 6 hours
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
