package utils

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"ridewave/models"
)

// SendToken generates a JWT and sends the authenticated response
func SendToken(c *gin.Context, entity interface{}, id string) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":  id,
		"exp": time.Now().Add(30 * 24 * time.Hour).Unix(),
	})
	tokenString, err := token.SignedString([]byte(os.Getenv("ACCESS_TOKEN_SECRET")))
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "Failed to generate token", err)
		return
	}

	switch v := entity.(type) {
	case *models.User:
		RespondSuccess(c, http.StatusOK, "Authentication successful", gin.H{
			"accessToken": tokenString,
			"user":        v,
		})
	case *models.Driver:
		RespondSuccess(c, http.StatusOK, "Authentication successful", gin.H{
			"accessToken": tokenString,
			"driver":      v,
		})
	}
}
