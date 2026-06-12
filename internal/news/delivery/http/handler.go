package http

import (
	"log"

	"github.com/alif/crypto-portfolio/internal/news/usecase"
	"github.com/gofiber/fiber/v2"
)

type NewsHandler struct {
	newsUsecase *usecase.NewsUsecase
}

func NewNewsHandler(newsUsecase *usecase.NewsUsecase) *NewsHandler {
	return &NewsHandler{newsUsecase: newsUsecase}
}

// GetDailyBriefing returns the latest crypto news briefing with sentiment analysis.
// @Summary Get daily news briefing
// @Description Get the latest crypto news briefing aggregated from RSS feeds with AI-powered sentiment analysis
// @Tags news
// @Accept json
// @Produce json
// @Success 200 {object} domain.DailyBriefing
// @Failure 404 {object} map[string]interface{}
// @Router /news/briefing [get]
func (h *NewsHandler) GetDailyBriefing(c *fiber.Ctx) error {
	log.Print("[http] GET /api/v1/news/briefing")

	briefing, err := h.newsUsecase.GetLatestBriefing()
	if err != nil {
		log.Printf("[http] GET /api/v1/news/briefing → 404: %v", err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "belum ada briefing untuk hari ini",
		})
	}

	log.Printf("[http] GET /api/v1/news/briefing → 200: %s %s", briefing.SummaryDate, briefing.Sentiment)
	return c.JSON(briefing)
}
