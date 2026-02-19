package middleware

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"ridewave/utils"
)

// APIKeyAuth validates the x-api-key header against the server's API_KEY env var.
// If API_KEY is not set, all requests pass through (dev mode).
func APIKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		serverKey := os.Getenv("API_KEY")
		if serverKey == "" {
			c.Next()
			return
		}

		clientKey := c.GetHeader("x-api-key")
		if clientKey != serverKey {
			utils.RespondError(c, http.StatusUnauthorized, "Invalid API Key", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequestID adds a unique request ID for tracing/debugging
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := uuid.New().String()
		c.Set("RequestID", id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

// MaxBodySize limits the size of the request body
func MaxBodySize(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}
