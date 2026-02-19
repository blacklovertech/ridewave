package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"ridewave/db"
	"ridewave/models"
	"ridewave/stores"
	"ridewave/utils"

	"go.uber.org/zap"
)

// RegisterDriverRoutes defines all driver-facing API endpoints
func RegisterDriverRoutes(r *gin.Engine, authMiddleware gin.HandlerFunc) {
	driverGroup := r.Group("/api/v1/driver")
	{
		// Auth
		driverGroup.POST("/auth/login", DriverLogin)
		driverGroup.POST("/auth/verify", DriverVerify)
		driverGroup.POST("/auth/logout", authMiddleware, DriverLogout)

		// Profile & Status
		driverGroup.GET("/me", authMiddleware, GetLoggedInDriverData)
		driverGroup.PUT("/status", authMiddleware, UpdateDriverStatus)
		driverGroup.PUT("/toggle-online", authMiddleware, ToggleOnline)
		driverGroup.PUT("/notification-token", authMiddleware, UpdateDriverNotificationToken)

		// Vehicle types (shown during registration after OTP verify)
		driverGroup.GET("/vehicle-types", GetVehicleTypes)

		// Live Location
		driverGroup.PUT("/location", authMiddleware, UpdateDriverLocationHandler)
		driverGroup.GET("/ride/:id/user-location", authMiddleware, GetUserLocationForDriver)

		// Ride Management
		driverGroup.GET("/incoming-ride", authMiddleware, GetIncomingRide)
		driverGroup.PUT("/ride/status", authMiddleware, UpdatingRideStatus)
		driverGroup.GET("/rides", authMiddleware, GetDriverRides)
		driverGroup.GET("/ride/:id", authMiddleware, GetSingleDriverRide)
		driverGroup.POST("/rate-user", authMiddleware, RateUser)
		driverGroup.POST("/payment/confirm", authMiddleware, ConfirmPayment)

		// Earnings
		driverGroup.GET("/earnings", authMiddleware, GetEarnings)
		driverGroup.GET("/earnings/daily", authMiddleware, GetDailyEarnings)
		driverGroup.GET("/earnings/weekly", authMiddleware, GetWeeklyEarnings)

		// Public (accessible via query param)
		driverGroup.GET("/list", GetDriversById)
	}
}


// ══════════════════════════════════════════════════
// Driver Authentication
// ══════════════════════════════════════════════════

// POST /api/v1/driver/auth/login
func DriverLogin(c *gin.Context) {
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

	utils.RespondSuccess(c, http.StatusOK, "OTP sent", nil)
}

// Helper to scan a full driver row
const driverSelectCols = `id, name, country, phone_number, email, vehicle_type, registration_number, registration_date, driving_license, vehicle_color, rate, "notificationToken", ratings, "totalEarning", "totalRides", "totalDistance", "pendingRides", "cancelRides", status, "isOnline", "createdAt", "updatedAt", COALESCE("rcBook", ''), COALESCE("profileImage", ''), "upi_id"`

func scanDriver(scanner interface{ Scan(dest ...any) error }, d *models.Driver) error {
	return scanner.Scan(&d.ID, &d.Name, &d.Country, &d.PhoneNumber, &d.Email, &d.VehicleType, &d.RegistrationNumber, &d.RegistrationDate, &d.DrivingLicense, &d.VehicleColor, &d.Rate, &d.NotificationToken, &d.Ratings, &d.TotalEarning, &d.TotalRides, &d.TotalDistance, &d.PendingRides, &d.CancelRides, &d.Status, &d.IsOnline, &d.CreatedAt, &d.UpdatedAt, &d.RCBook, &d.ProfileImage, &d.UpiID)
}

// POST /api/v1/driver/auth/verify
func DriverVerify(c *gin.Context) {
	var body struct {
		PhoneNumber        string `json:"phone_number" binding:"required"`
		OTP                string `json:"otp" binding:"required"`
		Name               string `json:"name"`
		Email              string `json:"email"`
		Country            string `json:"country"`
		VehicleType        string `json:"vehicle_type"`
		RegistrationNumber string `json:"registration_number"`
		DrivingLicense     string `json:"driving_license"`
		VehicleColor       string `json:"vehicle_color"`
		Rate               string `json:"rate"`
		RCBook             string `json:"rc_book"`
		ProfileImage       string `json:"profile_image"`
		UpiID              string `json:"upiId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	if err := utils.VerifyTwilioOTP(body.PhoneNumber, body.OTP); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid OTP", err)
		return
	}

	var driver models.Driver
	row := db.Pool.QueryRow(context.Background(),
		`SELECT `+driverSelectCols+` FROM driver WHERE phone_number=$1`, body.PhoneNumber)
	if err := scanDriver(row, &driver); err == nil {
		// Check driver account status
		if driver.Status == "suspended" {
			utils.RespondError(c, http.StatusForbidden, "Your account has been suspended. Contact support.", nil)
			return
		}
		if driver.Status == "rejected" {
			utils.RespondError(c, http.StatusForbidden, "Your registration was rejected. Contact support.", nil)
			return
		}
		if driver.Status == "pending" {
			// Allow login but inform them about pending verification
			utils.RespondSuccess(c, http.StatusOK, "Your registration is pending admin verification.", gin.H{
				"isPending": true,
				"driver":    driver,
			})
			return
		}
		utils.SendToken(c, &driver, driver.ID)
		return
	}

	// New driver — require registration fields
	if body.Name == "" || body.Email == "" || body.VehicleType == "" || body.RegistrationNumber == "" {
		utils.RespondSuccess(c, http.StatusNotFound, "Driver not registered. Please provide details.", gin.H{"isNewDriver": true})
		return
	}

	row = db.Pool.QueryRow(context.Background(),
		`INSERT INTO driver (id, name, country, phone_number, email, vehicle_type, registration_number, registration_date, driving_license, vehicle_color, rate, ratings, "totalEarning", "totalRides", "totalDistance", "pendingRides", "cancelRides", status, "isOnline", "createdAt", "updatedAt", "rcBook", "profileImage", "upi_id")
		VALUES (gen_random_uuid()::text, $1,$2,$3,$4,$5,$6,NOW(),$7,$8,$9, 0,0,0,0,0,0,'pending',FALSE,NOW(),NOW(), $10, $11, $12)
		RETURNING `+driverSelectCols,
		body.Name, body.Country, body.PhoneNumber, body.Email, body.VehicleType,
		body.RegistrationNumber, body.DrivingLicense, body.VehicleColor, body.Rate, body.RCBook, body.ProfileImage, body.UpiID)
	if err := scanDriver(row, &driver); err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Database error during registration", err)
		return
	}
	// New drivers are pending — don't issue a full token
	utils.RespondSuccess(c, http.StatusCreated, "Registration submitted! Your account is pending admin verification.", gin.H{
		"isPending": true,
		"driver":    driver,
	})
}

// POST /api/v1/driver/auth/logout
func DriverLogout(c *gin.Context) {
	driver := c.MustGet("driver").(*models.Driver)
	db.Pool.Exec(context.Background(),
		`UPDATE driver SET "notificationToken"=NULL, status='inactive', "updatedAt"=NOW() WHERE id=$1`, driver.ID)
	stores.RemoveDriver(driver.ID)
	utils.RespondSuccess(c, http.StatusOK, "Logged out successfully", nil)
}

// ══════════════════════════════════════════════════
// Driver Profile & Status
// ══════════════════════════════════════════════════

// GET /api/v1/driver/me
func GetLoggedInDriverData(c *gin.Context) {
	driver, _ := c.Get("driver")
	utils.RespondSuccess(c, http.StatusOK, "Driver data", gin.H{"driver": driver})
}

// GET /api/v1/driver/list?ids=id1,id2
func GetDriversById(c *gin.Context) {
	ids := c.Query("ids")
	if ids == "" {
		utils.RespondError(c, http.StatusBadRequest, "No driver IDs provided", nil)
		return
	}
	driverIds := strings.Split(ids, ",")

	rows, err := db.Pool.Query(context.Background(),
		`SELECT `+driverSelectCols+` FROM driver WHERE id=ANY($1)`, driverIds)
	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Internal server error", err)
		return
	}
	defer rows.Close()

	var drivers []models.Driver
	for rows.Next() {
		var d models.Driver
		scanDriver(rows, &d)
		drivers = append(drivers, d)
	}
	if drivers == nil {
		drivers = []models.Driver{}
	}
	utils.RespondSuccess(c, http.StatusOK, "Drivers data", gin.H{"drivers": drivers})
}

// PUT /api/v1/driver/status
func UpdateDriverStatus(c *gin.Context) {
	driver := c.MustGet("driver").(*models.Driver)
	var body struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	// Validate allowed status values
	allowed := map[string]bool{"active": true, "inactive": true, "busy": true}
	if !allowed[body.Status] {
		utils.RespondError(c, http.StatusBadRequest, "Invalid status. Use: active, inactive, busy", nil)
		return
	}

	var updated models.Driver
	row := db.Pool.QueryRow(context.Background(),
		`UPDATE driver SET status=$1, "updatedAt"=NOW() WHERE id=$2 RETURNING `+driverSelectCols,
		body.Status, driver.ID)
	if err := scanDriver(row, &updated); err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Database error", err)
		return
	}

	// If going inactive, remove from Redis
	if body.Status == "inactive" {
		stores.RemoveDriver(driver.ID)
	}

	utils.RespondSuccess(c, http.StatusOK, "Status updated", gin.H{"driver": updated})
}

// PUT /api/v1/driver/notification-token
func UpdateDriverNotificationToken(c *gin.Context) {
	driver := c.MustGet("driver").(*models.Driver)
	var body struct {
		NotificationToken string `json:"notificationToken" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	var updated models.Driver
	row := db.Pool.QueryRow(context.Background(),
		`UPDATE driver SET "notificationToken"=$1, "updatedAt"=NOW() WHERE id=$2 RETURNING `+driverSelectCols,
		body.NotificationToken, driver.ID)
	if err := scanDriver(row, &updated); err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Database error", err)
		return
	}
	utils.RespondSuccess(c, http.StatusOK, "Token updated", gin.H{"driver": updated})
}

// ══════════════════════════════════════════════════
// Driver Live Location
// ══════════════════════════════════════════════════

// PUT /api/v1/driver/location
func UpdateDriverLocationHandler(c *gin.Context) {
	driver := c.MustGet("driver").(*models.Driver)
	var body struct {
		Lat     float64  `json:"lat" binding:"required"`
		Lng     float64  `json:"lng" binding:"required"`
		Heading *float64 `json:"heading"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	// 1. PRODUCTION SMOOTHING: Snap to Road for high-precision mapping
	olaClient := utils.NewOlaMapsClient()
	points := fmt.Sprintf("%f,%f", body.Lat, body.Lng)
	snappedLat, snappedLng, err := olaClient.SnapToRoad(points)
	
	finalLat, finalLng := body.Lat, body.Lng
	if err == nil {
		finalLat, finalLng = snappedLat, snappedLng
	} else {
		utils.Logger.Warn("SnapToRoad failed, using raw coordinates", zap.Error(err))
	}

	// 2. REDIS-EXCLUSIVE UPDATE: Real-time tracking is handled solely by Redis
	// No PostgreSQL IO for moving data to match Ola/Uber efficiency standards.
	stores.UpdateDriverLocation(driver.ID, finalLat, finalLng, "")

	utils.RespondSuccess(c, http.StatusOK, "Location updated", nil)
}

// GET /api/v1/driver/ride/:id/user-location
func GetUserLocationForDriver(c *gin.Context) {
	rideID := c.Param("id")
	driver := c.MustGet("driver").(*models.Driver)

	var originLat, originLng *float64
	var destLat, destLng *float64
	var originName, destName string
	err := db.Pool.QueryRow(context.Background(),
		`SELECT "originLat", "originLng", "destinationLat", "destinationLng", "currentLocationName", "destinationLocationName" 
		FROM rides WHERE id=$1 AND "driverId"=$2`, rideID, driver.ID).
		Scan(&originLat, &originLng, &destLat, &destLng, &originName, &destName)
	if err != nil {
		utils.RespondError(c, http.StatusNotFound, "Ride not found", err)
		return
	}

	utils.RespondSuccess(c, http.StatusOK, "User location", gin.H{
		"originLat":       originLat,
		"originLng":       originLng,
		"destinationLat":  destLat,
		"destinationLng":  destLng,
		"originName":      originName,
		"destinationName": destName,
	})
}

// ══════════════════════════════════════════════════
// Driver Online/Offline Toggle (Start/Stop Rides)
// ══════════════════════════════════════════════════

// PUT /api/v1/driver/toggle-online — driver clicks Start/Stop button
func ToggleOnline(c *gin.Context) {
	driver := c.MustGet("driver").(*models.Driver)

	// Only admin-approved (active) drivers can go online
	if driver.Status != "active" {
		msg := "Your account is not approved yet. Please wait for admin verification."
		if driver.Status == "suspended" {
			msg = "Your account has been suspended. Contact support."
		} else if driver.Status == "rejected" {
			msg = "Your registration was rejected. Contact support."
		}
		utils.RespondError(c, http.StatusForbidden, msg, nil)
		return
	}

	// Toggle the current state
	newOnlineState := !driver.IsOnline

	var updated models.Driver
	row := db.Pool.QueryRow(context.Background(),
		`UPDATE driver SET "isOnline"=$1, "updatedAt"=NOW() WHERE id=$2 RETURNING `+driverSelectCols,
		newOnlineState, driver.ID)
	if err := scanDriver(row, &updated); err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Database error", err)
		return
	}

	if newOnlineState {
		// Going online — driver will appear in nearby searches
		utils.RespondSuccess(c, http.StatusOK, "You are now online and accepting rides!", gin.H{"driver": updated})
	} else {
		// Going offline — remove from Redis geo index
		stores.RemoveDriver(driver.ID)
		utils.RespondSuccess(c, http.StatusOK, "You are now offline. No new rides will be dispatched.", gin.H{"driver": updated})
	}
}

// ══════════════════════════════════════════════════
// Driver Ride Management
// ══════════════════════════════════════════════════

// GET /api/v1/driver/incoming-ride
func GetIncomingRide(c *gin.Context) {
	driver := c.MustGet("driver").(*models.Driver)

	// Only online drivers should see incoming rides
	if !driver.IsOnline {
		utils.RespondSuccess(c, http.StatusOK, "You are offline. Go online to receive rides.", gin.H{"ride": nil, "isOnline": false})
		return
	}
	var ride models.Ride
	var user models.User
	err := db.Pool.QueryRow(context.Background(),
		`SELECT r.id, r."userId", r."driverId", r.charge, r."currentLocationName", r."destinationLocationName", 
		r.distance, COALESCE(r.polyline, ''), COALESCE(r."estimatedDuration", 0), COALESCE(r."estimatedDistance", 0),
		COALESCE(r."vehicleType", ''), r.status, r."originLat", r."originLng", r."destinationLat", r."destinationLng",
		r."createdAt", r."updatedAt",
		u.id, u.name, u.phone_number, u.ratings
		FROM rides r 
		JOIN "user" u ON r."userId"=u.id
		WHERE r."driverId"=$1 AND r.status='Requested'
		ORDER BY r."createdAt" DESC LIMIT 1`, driver.ID).
		Scan(&ride.ID, &ride.UserID, &ride.DriverID, &ride.Charge, &ride.CurrentLocationName, &ride.DestinationLocationName,
			&ride.Distance, &ride.Polyline, &ride.EstimatedDuration, &ride.EstimatedDistance,
			&ride.VehicleType, &ride.Status, &ride.OriginLat, &ride.OriginLng, &ride.DestinationLat, &ride.DestinationLng,
			&ride.CreatedAt, &ride.UpdatedAt,
			&user.ID, &user.Name, &user.PhoneNumber, &user.Ratings)
	if err != nil {
		utils.RespondSuccess(c, http.StatusOK, "No incoming ride", gin.H{"ride": nil})
		return
	}
	ride.User = &user
	utils.RespondSuccess(c, http.StatusOK, "Incoming ride", gin.H{"ride": ride})
}

// PUT /api/v1/driver/ride/status
func UpdatingRideStatus(c *gin.Context) {
	driver := c.MustGet("driver").(*models.Driver)

	// Only online+active drivers can accept/manage rides
	if !driver.IsOnline || driver.Status != "active" {
		utils.RespondError(c, http.StatusForbidden, "You must be online and approved to manage rides.", nil)
		return
	}
	var body struct {
		RideID     string `json:"rideId" binding:"required"`
		RideStatus string `json:"rideStatus" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid input data", err)
		return
	}

	// Validate allowed status transitions
	allowed := map[string]bool{"Accepted": true, "InProgress": true, "Completed": true, "Cancelled": true}
	if !allowed[body.RideStatus] {
		utils.RespondError(c, http.StatusBadRequest, "Invalid ride status", nil)
		return
	}

	var charge float64
	err := db.Pool.QueryRow(context.Background(), `SELECT charge FROM rides WHERE id=$1`, body.RideID).Scan(&charge)
	if err != nil {
		utils.RespondError(c, http.StatusNotFound, "Ride not found", err)
		return
	}

	// Set lifecycle timestamp based on status transition
	timestampCol := ""
	switch body.RideStatus {
	case "Accepted":
		timestampCol = `,"acceptedAt"=NOW()`
	case "InProgress":
		timestampCol = `,"startedAt"=NOW()`
	case "Completed":
		timestampCol = `,"completedAt"=NOW()`
	case "Cancelled":
		timestampCol = `,"cancelledAt"=NOW()`
	}

	var updated models.Ride
	var user models.User
	err = db.Pool.QueryRow(context.Background(),
		`UPDATE rides SET status=$1, "updatedAt"=NOW()`+timestampCol+` 
		WHERE id=$2 AND "driverId"=$3 
		RETURNING id, "userId", "driverId", charge, "currentLocationName", "destinationLocationName", distance, status, rating, "createdAt", "updatedAt"`,
		body.RideStatus, body.RideID, driver.ID).
		Scan(&updated.ID, &updated.UserID, &updated.DriverID, &updated.Charge, &updated.CurrentLocationName, &updated.DestinationLocationName, &updated.Distance, &updated.Status, &updated.Rating, &updated.CreatedAt, &updated.UpdatedAt)

	if err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Failed to update ride", err)
		return
	}

	db.Pool.QueryRow(context.Background(),
		`SELECT id, name, phone_number, ratings FROM "user" WHERE id=$1`, updated.UserID).
		Scan(&user.ID, &user.Name, &user.PhoneNumber, &user.Ratings)
	updated.User = &user

	if body.RideStatus == "Completed" {
		var distVal float64
		fmt.Sscanf(updated.Distance, "%f", &distVal)
		db.Pool.Exec(context.Background(),
			`UPDATE driver SET "totalEarning"="totalEarning"+$1, "totalRides"="totalRides"+1, "totalDistance"="totalDistance"+$2, "updatedAt"=NOW() WHERE id=$3`,
			charge, distVal, driver.ID)
		db.Pool.Exec(context.Background(),
			`UPDATE "user" SET "totalRides"="totalRides"+1, "updatedAt"=NOW() WHERE id=$1`, updated.UserID)
	}

	// Send FCM notification to the User
	var userToken *string
	db.Pool.QueryRow(context.Background(), `SELECT "notificationToken" FROM "user" WHERE id=$1`, updated.UserID).Scan(&userToken)
	
	if userToken != nil && *userToken != "" {
		title := "Ride Update"
		msg := "Your ride status has changed."
		switch body.RideStatus {
		case "Accepted":
			title = "Ride Accepted! 🚗"
			msg = fmt.Sprintf("%s has accepted your request and is on the way.", driver.Name)
		case "InProgress":
			title = "Ride Started 🚀"
			msg = "You are on your way to the destination."
		case "Completed":
			title = "Ride Completed ✅"
			msg = fmt.Sprintf("You have reached your destination. Total fare: ₹%.2f", charge)
		case "Cancelled":
			title = "Ride Cancelled ❌"
			msg = "The driver has cancelled the ride."
		}
		
		go utils.SendPushNotification(*userToken, title, msg, utils.FCMData{
			"type":       "ride_status",
			"rideId":     updated.ID,
			"status":     body.RideStatus,
			"driverName": driver.Name,
			"driverId":   driver.ID,
		})
	}
	utils.RespondSuccess(c, http.StatusOK, "Ride status updated", gin.H{"updatedRide": updated})
}

// GET /api/v1/driver/rides
func GetDriverRides(c *gin.Context) {
	driver := c.MustGet("driver").(*models.Driver)

	rows, err := db.Pool.Query(context.Background(),
		`SELECT r.id, r."userId", r."driverId", r.charge, r."currentLocationName", r."destinationLocationName", 
		 r.distance, r.status, r.rating, COALESCE(r."vehicleType",''), COALESCE(r."paymentMode",''),
		 COALESCE(r."paymentStatus",'Pending'), COALESCE(r.tips, 0), r."createdAt", r."updatedAt",
		 u.id, u.name, u.phone_number, u.ratings
		FROM rides r 
		JOIN "user" u ON r."userId"=u.id 
		WHERE r."driverId"=$1 ORDER BY r."createdAt" DESC`, driver.ID)
	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Internal server error", err)
		return
	}
	defer rows.Close()

	type RideWithUser struct {
		ID                      string       `json:"id"`
		UserID                  string       `json:"userId"`
		DriverID                *string      `json:"driverId"`
		Charge                  float64      `json:"charge"`
		CurrentLocationName     string       `json:"currentLocationName"`
		DestinationLocationName string       `json:"destinationLocationName"`
		Distance                string       `json:"distance"`
		Status                  string       `json:"status"`
		Rating                  *float64     `json:"rating"`
		VehicleType             string       `json:"vehicleType"`
		PaymentMode             string       `json:"paymentMode"`
		PaymentStatus           string       `json:"paymentStatus"`
		Tips                    float64      `json:"tips"`
		CreatedAt               time.Time    `json:"createdAt"`
		UpdatedAt               time.Time    `json:"updatedAt"`
		User                    *models.User `json:"user,omitempty"`
	}

	var rides []RideWithUser
	for rows.Next() {
		var r RideWithUser
		var uID, uName, uPhone string
		var uRating float64
		rows.Scan(&r.ID, &r.UserID, &r.DriverID, &r.Charge, &r.CurrentLocationName, &r.DestinationLocationName,
			&r.Distance, &r.Status, &r.Rating, &r.VehicleType, &r.PaymentMode, &r.PaymentStatus, &r.Tips,
			&r.CreatedAt, &r.UpdatedAt,
			&uID, &uName, &uPhone, &uRating)
		
		r.User = &models.User{
			ID:          uID,
			Name:        &uName,
			PhoneNumber: uPhone,
			Ratings:     uRating,
		}
		rides = append(rides, r)
	}
	if rides == nil {
		rides = []RideWithUser{}
	}
	utils.RespondSuccess(c, http.StatusOK, "Rides retrieved", gin.H{"rides": rides})
}

// GET /api/v1/driver/ride/:id
func GetSingleDriverRide(c *gin.Context) {
	rideID := c.Param("id")
	driver := c.MustGet("driver").(*models.Driver)

	var ride models.Ride
	var user models.User
	err := db.Pool.QueryRow(context.Background(),
		`SELECT r.id, r."userId", r."driverId", r.charge, r."currentLocationName", r."destinationLocationName", 
		r.distance, COALESCE(r.polyline, ''), COALESCE(r."estimatedDuration", 0), COALESCE(r."estimatedDistance", 0),
		COALESCE(r."vehicleType", ''), r.status, r.rating, COALESCE(r."paymentMode", ''), COALESCE(r."paymentStatus", 'Pending'),
		r."originLat", r."originLng", r."destinationLat", r."destinationLng",
		r."createdAt", r."updatedAt",
		u.id, u.name, u.phone_number, u.ratings
		FROM rides r 
		JOIN "user" u ON r."userId"=u.id
		WHERE r.id=$1 AND r."driverId"=$2`, rideID, driver.ID).
		Scan(&ride.ID, &ride.UserID, &ride.DriverID, &ride.Charge, &ride.CurrentLocationName, &ride.DestinationLocationName,
			&ride.Distance, &ride.Polyline, &ride.EstimatedDuration, &ride.EstimatedDistance,
			&ride.VehicleType, &ride.Status, &ride.Rating, &ride.PaymentMode, &ride.PaymentStatus,
			&ride.OriginLat, &ride.OriginLng, &ride.DestinationLat, &ride.DestinationLng,
			&ride.CreatedAt, &ride.UpdatedAt,
			&user.ID, &user.Name, &user.PhoneNumber, &user.Ratings)
	if err != nil {
		utils.RespondError(c, http.StatusNotFound, "Ride not found", err)
		return
	}
	ride.User = &user
	ride.Driver = driver
	utils.RespondSuccess(c, http.StatusOK, "Ride details", gin.H{"ride": ride})
}

// ══════════════════════════════════════════════════
// Driver Earnings
// ══════════════════════════════════════════════════

// GET /api/v1/driver/earnings
func GetEarnings(c *gin.Context) {
	driver := c.MustGet("driver").(*models.Driver)
	utils.RespondSuccess(c, http.StatusOK, "Earnings summary", gin.H{
		"totalEarning":   driver.TotalEarning,
		"totalRides":     driver.TotalRides,
		"totalDistance":   driver.TotalDistance,
		"pendingRides":   driver.PendingRides,
		"cancelledRides": driver.CancelRides,
	})
}

// GET /api/v1/driver/earnings/daily
func GetDailyEarnings(c *gin.Context) {
	driver := c.MustGet("driver").(*models.Driver)

	rows, err := db.Pool.Query(context.Background(),
		`SELECT DATE(r."createdAt") as day, COUNT(*) as rides, COALESCE(SUM(r.charge), 0) as earnings
		FROM rides r 
		WHERE r."driverId"=$1 AND r.status='Completed' AND r."createdAt" >= NOW() - INTERVAL '7 days'
		GROUP BY DATE(r."createdAt") ORDER BY day DESC`, driver.ID)
	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Failed to fetch earnings", err)
		return
	}
	defer rows.Close()

	type DayEarning struct {
		Day      time.Time `json:"day"`
		Rides    int       `json:"rides"`
		Earnings float64   `json:"earnings"`
	}
	var daily []DayEarning
	for rows.Next() {
		var d DayEarning
		rows.Scan(&d.Day, &d.Rides, &d.Earnings)
		daily = append(daily, d)
	}
	if daily == nil {
		daily = []DayEarning{}
	}
	utils.RespondSuccess(c, http.StatusOK, "Daily earnings", gin.H{"daily": daily})
}

// GET /api/v1/driver/earnings/weekly
func GetWeeklyEarnings(c *gin.Context) {
	driver := c.MustGet("driver").(*models.Driver)

	rows, err := db.Pool.Query(context.Background(),
		`SELECT DATE_TRUNC('week', r."createdAt") as week, COUNT(*) as rides, COALESCE(SUM(r.charge), 0) as earnings
		FROM rides r 
		WHERE r."driverId"=$1 AND r.status='Completed' AND r."createdAt" >= NOW() - INTERVAL '4 weeks'
		GROUP BY DATE_TRUNC('week', r."createdAt") ORDER BY week DESC`, driver.ID)
	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Failed to fetch earnings", err)
		return
	}
	defer rows.Close()

	type WeekEarning struct {
		Week     time.Time `json:"week"`
		Rides    int       `json:"rides"`
		Earnings float64   `json:"earnings"`
	}
	var weekly []WeekEarning
	for rows.Next() {
		var w WeekEarning
		rows.Scan(&w.Week, &w.Rides, &w.Earnings)
		weekly = append(weekly, w)
	}
	if weekly == nil {
		weekly = []WeekEarning{}
	}
	utils.RespondSuccess(c, http.StatusOK, "Weekly earnings", gin.H{"weekly": weekly})
}
