package routes

import (
	"teo/internal/middleware"
	bot_router "teo/internal/services/bot"
	queue_router "teo/internal/services/queue"

	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app *fiber.App) {
	prefix := ""
	router := app.Group(prefix, middleware.Protected)

	bot_router.BotRouter(router)
	queue_router.QueueRouter(router)
}
