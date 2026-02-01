package middleware

import (
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
	"github.com/workany-ai/clawork/util"
)

// BearerAuth returns a middleware that validates Bearer token
func BearerAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get configured token
			configuredToken := viper.GetString("api.token")
			if configuredToken == "" {
				// No token configured, skip auth (for development)
				return next(c)
			}

			// Get Authorization header
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return util.Unauthorized(c, "missing authorization header")
			}

			// Check Bearer prefix
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				return util.Unauthorized(c, "invalid authorization format, expected: Bearer <token>")
			}

			// Validate token
			token := parts[1]
			if token != configuredToken {
				return util.Unauthorized(c, "invalid token")
			}

			return next(c)
		}
	}
}
