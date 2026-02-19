package handlers

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"ridewave/db"
	"ridewave/models"
	"ridewave/utils"
)


// RegisterUserRoutes defines all user-facing API endpoints
func RegisterUserRoutes(r *gin.Engine, authMiddleware gin.HandlerFunc) {
	userGroup := r.Group("/api/v1/user")
	{
		// Auth
		userGroup.POST("/auth/login", UserLogin)
		userGroup.POST("/auth/verify", UserVerify)
		userGroup.POST("/auth/logout", authMiddleware, UserLogout)

		// Profile & Settings
		userGroup.GET("/me", authMiddleware, GetLoggedInUserData)
		userGroup.PUT("/profile", authMiddleware, UpdateUserProfile)
		userGroup.PUT("/notification-token", authMiddleware, UpdateUserNotificationToken)

		// Vehicle types (for ride booking — user picks Car, Auto, Bike etc.)
		userGroup.GET("/vehicle-types", authMiddleware, GetVehicleTypes)

		// Ride Operations
		userGroup.GET("/service-availability", authMiddleware, CheckServiceAvailability)
		userGroup.GET("/places/autocomplete", authMiddleware, PlacesAutocomplete)
		userGroup.GET("/places/nearby", authMiddleware, NearbySearch)
		userGroup.POST("/ride/estimate", authMiddleware, GetRideEstimate)
		userGroup.POST("/ride/create", authMiddleware, CreateRide)
		userGroup.POST("/ride/cancel", authMiddleware, CancelRide)
		userGroup.GET("/ride/:id", authMiddleware, GetRideDetails)
		userGroup.GET("/ride/:id/driver-location", authMiddleware, GetDriverLocation)
		userGroup.GET("/rides", authMiddleware, GetUserRides)
		userGroup.GET("/payment/:rideId", authMiddleware, GetPaymentReceipt)
		userGroup.POST("/payment/verify-direct", authMiddleware, VerifyDirectPayment)
		userGroup.POST("/rate-driver", authMiddleware, RateDriver)
		userGroup.POST("/sos", authMiddleware, TriggerSOS)
	}
}

// ══════════════════════════════════════════════════
// User Authentication
// ══════════════════════════════════════════════════

// User select columns — consistent across all queries
const userSelectCols = `id, name, phone_number, email, "notificationToken", ratings, "totalRides", status, "createdAt", "updatedAt"`

func scanUser(scanner interface{ Scan(dest ...any) error }, u *models.User) error {
	return scanner.Scan(&u.ID, &u.Name, &u.PhoneNumber, &u.Email, &u.NotificationToken, &u.Ratings, &u.TotalRides, &u.Status, &u.CreatedAt, &u.UpdatedAt)
}

// POST /api/v1/user/auth/login
func UserLogin(c *gin.Context) {
	var body struct {
		PhoneNumber string `json:"phone_number" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	if err := utils.SendTwilioOTP(body.PhoneNumber); err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Failed to send OTP", err)
		return
	}

	utils.RespondSuccess(c, http.StatusOK, "OTP sent successfully", nil)
}

// POST /api/v1/user/auth/verify
func UserVerify(c *gin.Context) {
	var body struct {
		PhoneNumber string `json:"phone_number" binding:"required"`
		OTP         string `json:"otp" binding:"required"`
		Name        string `json:"name"`
		Email       string `json:"email"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	if err := utils.VerifyTwilioOTP(body.PhoneNumber, body.OTP); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid OTP", err)
		return
	}

	var user models.User
	row := db.Pool.QueryRow(context.Background(),
		`SELECT `+userSelectCols+` FROM "user" WHERE phone_number=$1`, body.PhoneNumber)
	if err := scanUser(row, &user); err == nil {
		// Check if user is blocked
		if user.Status == "suspended" {
			utils.RespondError(c, http.StatusForbidden, "Your account has been suspended. Contact support.", nil)
			return
		}
		if user.Status == "inactive" {
			utils.RespondError(c, http.StatusForbidden, "Your account has been deactivated. Contact support.", nil)
			return
		}
		utils.SendToken(c, &user, user.ID)
		return
	}

	// New user — require name & email
	if body.Name == "" || body.Email == "" {
		utils.RespondSuccess(c, http.StatusNotFound, "User not found. Please register.", gin.H{"isNewUser": true})
		return
	}

	row = db.Pool.QueryRow(context.Background(),
		`INSERT INTO "user" (id, name, email, phone_number, ratings, "totalRides", status, "createdAt", "updatedAt") 
		VALUES (gen_random_uuid()::text, $1, $2, $3, 0, 0, 'active', NOW(), NOW()) 
		RETURNING `+userSelectCols,
		body.Name, body.Email, body.PhoneNumber)
	if err := scanUser(row, &user); err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Failed to create user", err)
		return
	}

	utils.SendToken(c, &user, user.ID)
}

// POST /api/v1/user/auth/logout
func UserLogout(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	db.Pool.Exec(context.Background(),
		`UPDATE "user" SET "notificationToken"=NULL, "updatedAt"=NOW() WHERE id=$1`, user.ID)
	utils.RespondSuccess(c, http.StatusOK, "Logged out successfully", nil)
}

// ══════════════════════════════════════════════════
// Email OTP (Admin-only)
// ══════════════════════════════════════════════════

// POST /api/v1/admin/email-otp-request
func SendingOtpToEmail(c *gin.Context) {
	var body struct {
		Email  string `json:"email"`
		Name   string `json:"name"`
		UserID string `json:"userId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid body", err)
		return
	}

	if body.Email == "" {
		var user models.User
		row := db.Pool.QueryRow(context.Background(),
			`UPDATE "user" SET name=$1, "updatedAt"=NOW() WHERE id=$2 RETURNING `+userSelectCols,
			body.Name, body.UserID)
		if err := scanUser(row, &user); err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Database error", err)
			return
		}
		utils.SendToken(c, &user, user.ID)
		return
	}

	otp := strconv.Itoa(1000 + rand.Intn(9000))
	payload := map[string]interface{}{
		"userId": body.UserID,
		"name":   body.Name,
		"email":  body.Email,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user": payload,
		"otp":  otp,
		"exp":  time.Now().Add(5 * time.Minute).Unix(),
	})
	tokenStr, _ := token.SignedString([]byte(os.Getenv("EMAIL_ACTIVATION_SECRET")))

	emailBody := fmt.Sprintf(`<p>Hi %s,</p><p>Your Ridewave verification code is <strong>%s</strong>. This code expires in 5 minutes.</p><p>If you didn't request this, please ignore this email.</p><p>Thanks,<br>Ridewave Team</p>`, body.Name, otp)
	if err := utils.SendEmail([]string{body.Email}, "Verify your email address!", emailBody); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Failed to send email", err)
		return
	}
	utils.RespondSuccess(c, http.StatusCreated, "OTP sent to email", gin.H{"token": tokenStr})
}

// PUT /api/v1/admin/email-otp-verify
func VerifyingEmail(c *gin.Context) {
	var body struct {
		OTP   string `json:"otp"`
		Token string `json:"token"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	token, err := jwt.Parse(body.Token, func(t *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("EMAIL_ACTIVATION_SECRET")), nil
	})
	if err != nil || !token.Valid {
		utils.RespondError(c, http.StatusBadRequest, "Your OTP is expired!", err)
		return
	}

	claims := token.Claims.(jwt.MapClaims)
	if claims["otp"].(string) != body.OTP {
		utils.RespondError(c, http.StatusBadRequest, "OTP is not correct or expired!", nil)
		return
	}

	userMap := claims["user"].(map[string]interface{})
	name := userMap["name"].(string)
	email := userMap["email"].(string)
	userID := userMap["userId"].(string)

	var user models.User
	row := db.Pool.QueryRow(context.Background(),
		`UPDATE "user" SET name=$1, email=$2, "updatedAt"=NOW() WHERE id=$3 RETURNING `+userSelectCols,
		name, email, userID)
	if err = scanUser(row, &user); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Failed to update profile", err)
		return
	}
	utils.SendToken(c, &user, user.ID)
}

// ══════════════════════════════════════════════════
// User Profile & Settings
// ══════════════════════════════════════════════════

// GET /api/v1/user/me
func GetLoggedInUserData(c *gin.Context) {
	user, _ := c.Get("user")
	utils.RespondSuccess(c, http.StatusOK, "User data retrieved", gin.H{"user": user})
}

// PUT /api/v1/user/profile
func UpdateUserProfile(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	var body struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	var updated models.User
	row := db.Pool.QueryRow(context.Background(),
		`UPDATE "user" SET name=COALESCE(NULLIF($1,''), name), email=COALESCE(NULLIF($2,''), email), "updatedAt"=NOW() WHERE id=$3 
		RETURNING `+userSelectCols,
		body.Name, body.Email, user.ID)
	if err := scanUser(row, &updated); err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Failed to update profile", err)
		return
	}
	utils.RespondSuccess(c, http.StatusOK, "Profile updated", gin.H{"user": updated})
}

// PUT /api/v1/user/notification-token
func UpdateUserNotificationToken(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	var body struct {
		NotificationToken string `json:"notificationToken"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	var updated models.User
	row := db.Pool.QueryRow(context.Background(),
		`UPDATE "user" SET "notificationToken"=$1, "updatedAt"=NOW() WHERE id=$2 RETURNING `+userSelectCols,
		body.NotificationToken, user.ID)
	if err := scanUser(row, &updated); err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Database error", err)
		return
	}
	utils.RespondSuccess(c, http.StatusOK, "Token updated", gin.H{"user": updated})
}

// ══════════════════════════════════════════════════
// User Ride Operations
// ══════════════════════════════════════════════════

// GET /api/v1/user/rides
func GetUserRides(c *gin.Context) {
	user := c.MustGet("user").(*models.User)

	rows, err := db.Pool.Query(context.Background(),
		`SELECT r.id, r."userId", r."driverId", r.charge, r."currentLocationName", r."destinationLocationName", 
		 r.distance, r.status, r.rating, COALESCE(r."vehicleType",''), COALESCE(r."paymentMode",''), 
		 COALESCE(r."paymentStatus",'Pending'), COALESCE(r.tips, 0), r."createdAt", r."updatedAt",
		 COALESCE(d.id,''), COALESCE(d.name,''), COALESCE(d.phone_number,''), COALESCE(d.vehicle_type,''),
		 COALESCE(d.vehicle_color,''), COALESCE(d.registration_number,''), COALESCE(d.ratings,0),
		 COALESCE(d."profileImage",'')
		FROM rides r 
		LEFT JOIN driver d ON r."driverId"=d.id 
		WHERE r."userId"=$1 ORDER BY r."createdAt" DESC`, user.ID)
	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Internal server error", err)
		return
	}
	defer rows.Close()

	type RideWithDriver struct {
		ID                      string   `json:"id"`
		UserID                  string   `json:"userId"`
		DriverID                *string  `json:"driverId"`
		Charge                  float64  `json:"charge"`
		CurrentLocationName     string   `json:"currentLocationName"`
		DestinationLocationName string   `json:"destinationLocationName"`
		Distance                string   `json:"distance"`
		Status                  string   `json:"status"`
		Rating                  *float64 `json:"rating"`
		VehicleType             string   `json:"vehicleType"`
		PaymentMode             string   `json:"paymentMode"`
		PaymentStatus           string   `json:"paymentStatus"`
		Tips                    float64  `json:"tips"`
		CreatedAt               string   `json:"createdAt"`
		UpdatedAt               string   `json:"updatedAt"`
		Driver                  *gin.H   `json:"driver,omitempty"`
	}

	var rides []RideWithDriver
	for rows.Next() {
		var r RideWithDriver
		var dID, dName, dPhone, dVehicle, dColor, dRegNo, dImage string
		var dRating float64
		rows.Scan(&r.ID, &r.UserID, &r.DriverID, &r.Charge, &r.CurrentLocationName, &r.DestinationLocationName,
			&r.Distance, &r.Status, &r.Rating, &r.VehicleType, &r.PaymentMode, &r.PaymentStatus, &r.Tips,
			&r.CreatedAt, &r.UpdatedAt,
			&dID, &dName, &dPhone, &dVehicle, &dColor, &dRegNo, &dRating, &dImage)
		if dID != "" {
			r.Driver = &gin.H{
				"id":                 dID,
				"name":               dName,
				"phoneNumber":        dPhone,
				"vehicleType":        dVehicle,
				"vehicleColor":       dColor,
				"registrationNumber": dRegNo,
				"ratings":            dRating,
				"profileImage":       dImage,
			}
		}
		rides = append(rides, r)
	}
	if rides == nil {
		rides = []RideWithDriver{}
	}
	utils.RespondSuccess(c, http.StatusOK, "Rides retrieved", gin.H{"rides": rides})
}

// ══════════════════════════════════════════════════
// Driver Location & Payment
// ══════════════════════════════════════════════════

// GET /api/v1/user/ride/:id/driver-location
func GetDriverLocation(c *gin.Context) {
	rideID := c.Param("id")
	user := c.MustGet("user").(*models.User)

	// Verify this ride belongs to the user
	var driverID *string
	err := db.Pool.QueryRow(context.Background(),
		`SELECT "driverId" FROM rides WHERE id=$1 AND "userId"=$2`, rideID, user.ID).Scan(&driverID)
	if err != nil || driverID == nil {
		utils.RespondError(c, http.StatusNotFound, "Ride or driver not found", err)
		return
	}

	// Get driver's live location
	var lat, lng float64
	var heading *float64
	var updatedAt time.Time
	err = db.Pool.QueryRow(context.Background(),
		`SELECT lat, lng, heading, "updatedAt" FROM driver_location WHERE "driverId"=$1`, *driverID).
		Scan(&lat, &lng, &heading, &updatedAt)
	if err != nil {
		utils.RespondError(c, http.StatusNotFound, "Driver location not available", err)
		return
	}

	utils.RespondSuccess(c, http.StatusOK, "Driver location", gin.H{
		"lat":       lat,
		"lng":       lng,
		"heading":   heading,
		"updatedAt": updatedAt,
	})
}

// GET /api/v1/user/payment/:rideId
func GetPaymentReceipt(c *gin.Context) {
	rideID := c.Param("rideId")
	user := c.MustGet("user").(*models.User)

	// Verify ride belongs to user
	var rideExists bool
	db.Pool.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM rides WHERE id=$1 AND "userId"=$2)`, rideID, user.ID).Scan(&rideExists)
	if !rideExists {
		utils.RespondError(c, http.StatusNotFound, "Ride not found", nil)
		return
	}

	var payment models.Payment
	err := db.Pool.QueryRow(context.Background(),
		`SELECT id, "rideId", amount, mode, status, "createdAt" FROM payments WHERE "rideId"=$1`, rideID).
		Scan(&payment.ID, &payment.RideID, &payment.Amount, &payment.Mode, &payment.Status, &payment.CreatedAt)
	if err != nil {
		utils.RespondError(c, http.StatusNotFound, "Payment not found", err)
		return
	}

	utils.RespondSuccess(c, http.StatusOK, "Payment receipt", gin.H{"payment": payment})
}

// POST /api/v1/user/payment/verify-direct
// Verifies a direct payment (Cash/UPI) to the driver without a gateway.
func VerifyDirectPayment(c *gin.Context) {
	var body struct {
		RideID string  `json:"rideId" binding:"required"`
		Amount float64 `json:"amount" binding:"required"`
		Mode   string  `json:"mode" binding:"required"` // "cash", "upi", "qr"
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	// 1. Validate Ride
	var rideStatus string
	var driverID *string
	err := db.Pool.QueryRow(context.Background(), 
		`SELECT status, "driverId" FROM rides WHERE id=$1`, body.RideID).Scan(&rideStatus, &driverID)
	if err != nil {
		utils.RespondError(c, http.StatusNotFound, "Ride not found", err)
		return
	}

	// 2. Record Payment
	_, err = db.Pool.Exec(context.Background(),
		`INSERT INTO payments ("rideId", amount, mode, status, "createdAt") VALUES ($1, $2, $3, 'success', NOW())`,
		body.RideID, body.Amount, body.Mode)
	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Failed to record payment", err)
		return
	}

	// 3. Update Ride Status & Payment Info
	_, err = db.Pool.Exec(context.Background(),
		`UPDATE rides SET "paymentStatus"='Paid', "paymentMode"=$1, "updatedAt"=NOW() WHERE id=$2`,
		body.Mode, body.RideID)
	
	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Failed to update ride payment status", err)
		return
	}

	utils.RespondSuccess(c, http.StatusOK, "Payment recorded successfully", nil)
}


