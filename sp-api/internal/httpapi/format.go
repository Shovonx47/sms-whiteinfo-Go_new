package httpapi

import (
	"log"
	"net/http"

	"github.com/gofiber/fiber/v3"
)

// databaseError returns a 500 JSON error response for unexpected DB errors.
func databaseError(ctx fiber.Ctx, err error) error {
	// Log the real cause server-side; do not leak SQL error text to the client.
	log.Printf("database error on %s %s: %v", ctx.Method(), ctx.Path(), err)

	return ctx.Status(http.StatusInternalServerError).JSON(fiber.Map{
		"message": "A database error occurred.",
	})
}
