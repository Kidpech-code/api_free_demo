package handler

import (
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"api_free_demo/internal/infrastructure/tmd"
)

// TMDHandler exposes cached TMD forecast data via HTTP.
type TMDHandler struct {
	cache  *tmd.CacheRepository
	logger *zap.Logger
}

// NewTMDHandler creates a handler backed by the TMD cache repository.
func NewTMDHandler(cache *tmd.CacheRepository, logger *zap.Logger) *TMDHandler {
	return &TMDHandler{cache: cache, logger: logger}
}

// GetForecast returns cached forecast JSON.
//
//	GET /api/v1/weather/:type/:location
//	:type     = hourly | daily | area
//	:location = location code (e.g. "bangkok")
func (h *TMDHandler) GetForecast(c *fiber.Ctx) error {
	ftParam := c.Params("type")
	location := c.Params("location")

	var ft tmd.ForecastType
	switch ftParam {
	case "hourly":
		ft = tmd.ForecastHourly
	case "daily":
		ft = tmd.ForecastDaily
	case "area":
		ft = tmd.ForecastArea
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "invalid forecast type; must be hourly, daily, or area",
		})
	}

	data, err := h.cache.Get(c.UserContext(), ft, location)
	if err != nil {
		h.logger.Error("cache read error", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "internal error",
		})
	}
	if data == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "no cached data for this location/type — try again later",
		})
	}

	c.Set(fiber.HeaderContentType, "application/json")
	return c.Status(fiber.StatusOK).Send(data)
}

// ListLocations returns the list of pre-cached location codes.
func (h *TMDHandler) ListLocations(c *fiber.Ctx) error {
	locs := tmd.DefaultLocations()
	result := make([]fiber.Map, 0, len(locs))
	for _, l := range locs {
		entry := fiber.Map{
			"code": l.Code,
			"lat":  l.Lat,
			"lon":  l.Lon,
		}
		if l.Province != "" {
			entry["province"] = l.Province
		}
		if l.Amphoe != "" {
			entry["amphoe"] = l.Amphoe
		}
		if l.Tambon != "" {
			entry["tambon"] = l.Tambon
		}
		result = append(result, entry)
	}
	return c.JSON(fiber.Map{
		"success":   true,
		"locations": result,
	})
}
