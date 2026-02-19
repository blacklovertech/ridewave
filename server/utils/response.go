package utils

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Standard Response Structure
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// SuccessResponse sends a standard success response
func RespondSuccess(c *gin.Context, code int, message string, data interface{}) {
	c.JSON(code, APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// ErrorResponse sends a standard error response
func RespondError(c *gin.Context, code int, message string, err error) {
	if err != nil {
		// Log the internal error for debugging (if needed) but don't expose it raw unless strictly necessary
		// For now, we just log it if we have a logger, or rely on caller to log.
		// Let's assume the message passed is safe for the user.
		Logger.Error(message, zap.Error(err))
	}
	c.JSON(code, APIResponse{
		Success: false,
		Message: message,
	})
}
