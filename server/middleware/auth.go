package middleware

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"ridewave/db"
	"ridewave/models"
	"ridewave/utils"
)

func IsAuthenticated() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			utils.RespondError(c, http.StatusUnauthorized, "Please log in to access this content", nil)
			c.Abort()
			return
		}
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			utils.RespondError(c, http.StatusUnauthorized, "Invalid authorization format. Use: Bearer <token>", nil)
			c.Abort()
			return
		}
		tokenStr := parts[1]

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("ACCESS_TOKEN_SECRET")), nil
		})
		if err != nil || !token.Valid {
			utils.RespondError(c, http.StatusUnauthorized, "Invalid or expired token", err)
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			utils.RespondError(c, http.StatusUnauthorized, "Invalid token claims", nil)
			c.Abort()
			return
		}
		id, ok := claims["id"].(string)
		if !ok || id == "" {
			utils.RespondError(c, http.StatusUnauthorized, "Invalid token payload", nil)
			c.Abort()
			return
		}

		var user models.User
		err = db.Pool.QueryRow(context.Background(),
			`SELECT id, name, phone_number, email, "notificationToken", ratings, "totalRides", status, "createdAt", "updatedAt" FROM "user" WHERE id=$1`, id).
			Scan(&user.ID, &user.Name, &user.PhoneNumber, &user.Email, &user.NotificationToken, &user.Ratings, &user.TotalRides, &user.Status, &user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			utils.RespondError(c, http.StatusUnauthorized, "User not found", err)
			c.Abort()
			return
		}

		// Block suspended/inactive users
		if user.Status == "suspended" {
			utils.RespondError(c, http.StatusForbidden, "Your account has been suspended. Contact support.", nil)
			c.Abort()
			return
		}
		if user.Status == "inactive" {
			utils.RespondError(c, http.StatusForbidden, "Your account has been deactivated. Contact support.", nil)
			c.Abort()
			return
		}

		c.Set("user", &user)
		c.Next()
	}
}

func IsAuthenticatedDriver() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			utils.RespondError(c, http.StatusUnauthorized, "Please log in to access this content", nil)
			c.Abort()
			return
		}
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			utils.RespondError(c, http.StatusUnauthorized, "Invalid authorization format. Use: Bearer <token>", nil)
			c.Abort()
			return
		}
		tokenStr := parts[1]

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("ACCESS_TOKEN_SECRET")), nil
		})
		if err != nil || !token.Valid {
			utils.RespondError(c, http.StatusUnauthorized, "Invalid or expired token", err)
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			utils.RespondError(c, http.StatusUnauthorized, "Invalid token claims", nil)
			c.Abort()
			return
		}
		id, ok := claims["id"].(string)
		if !ok || id == "" {
			utils.RespondError(c, http.StatusUnauthorized, "Invalid token payload", nil)
			c.Abort()
			return
		}

		var driver models.Driver
		err = db.Pool.QueryRow(context.Background(),
			`SELECT id, name, country, phone_number, email, vehicle_type, registration_number, registration_date, driving_license, vehicle_color, rate, "notificationToken", ratings, "totalEarning", "totalRides", "totalDistance", "pendingRides", "cancelRides", status, "createdAt", "updatedAt", COALESCE("rcBook", ''), COALESCE("profileImage", '') FROM driver WHERE id=$1`, id).
			Scan(&driver.ID, &driver.Name, &driver.Country, &driver.PhoneNumber, &driver.Email, &driver.VehicleType, &driver.RegistrationNumber, &driver.RegistrationDate, &driver.DrivingLicense, &driver.VehicleColor, &driver.Rate, &driver.NotificationToken, &driver.Ratings, &driver.TotalEarning, &driver.TotalRides, &driver.TotalDistance, &driver.PendingRides, &driver.CancelRides, &driver.Status, &driver.CreatedAt, &driver.UpdatedAt, &driver.RCBook, &driver.ProfileImage)
		if err != nil {
			utils.RespondError(c, http.StatusUnauthorized, "Driver not found", err)
			c.Abort()
			return
		}

		// Block suspended/rejected drivers
		if driver.Status == "suspended" {
			utils.RespondError(c, http.StatusForbidden, "Your account has been suspended. Contact support.", nil)
			c.Abort()
			return
		}
		if driver.Status == "rejected" {
			utils.RespondError(c, http.StatusForbidden, "Your registration was rejected. Contact support.", nil)
			c.Abort()
			return
		}

		c.Set("driver", &driver)
		c.Next()
	}
}

// IsAdmin validates admin access via x-admin-secret header
func IsAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		adminSecret := os.Getenv("ADMIN_SECRET")
		if adminSecret == "" {
			utils.RespondError(c, http.StatusInternalServerError, "Admin access not configured", nil)
			c.Abort()
			return
		}

		headerSecret := c.GetHeader("x-admin-secret")
		if headerSecret == "" || headerSecret != adminSecret {
			utils.RespondError(c, http.StatusForbidden, "Forbidden: Invalid admin credentials", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}
