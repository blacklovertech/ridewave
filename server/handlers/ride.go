package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"ridewave/db"
	"ridewave/models"
	"ridewave/stores"
	"ridewave/utils"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"
	"go.uber.org/zap"
)

var serviceZones []models.ServiceZone

func init() {
	// Parse authorized zones from ENV (Format: Name:Lat:Lng:RadiusKM;...)
	zonesEnv := os.Getenv("SERVICE_ZONES")
	if zonesEnv == "" {
		zonesEnv = "New Delhi:28.6139:77.2090:50;Chennai:13.0827:80.2707:50;Bengaluru:12.9716:77.5946:50;Madurai:9.9252:78.1198:50"
	}

	for _, zone := range strings.Split(zonesEnv, ";") {
		parts := strings.Split(zone, ":")
		if len(parts) < 4 {
			continue
		}
		name := parts[0]
		zLat, _ := strconv.ParseFloat(parts[1], 64)
		zLng, _ := strconv.ParseFloat(parts[2], 64)
		radius, _ := strconv.ParseFloat(parts[3], 64)

		serviceZones = append(serviceZones, models.ServiceZone{
			Name:   name,
			Lat:    zLat,
			Lng:    zLng,
			Radius: radius,
		})
	}
}

// CalculateFare fetches fare rates from the vehicle_types table and calculates the estimated fare.
// Falls back to default rates if the vehicle type is not found in the database.
func CalculateFare(vehicleType string, distanceMeters int, durationSeconds int) float64 {
	var baseFare, perKmRate, perMinRate float64

	err := db.Pool.QueryRow(context.Background(),
		`SELECT "baseFare", "perKmRate", "perMinRate" FROM vehicle_types WHERE name=$1 AND "isActive"=TRUE`,
		vehicleType).Scan(&baseFare, &perKmRate, &perMinRate)

	if err != nil {
		// Fallback defaults if DB lookup fails
		baseFare = 50.0
		perKmRate = 12.0
		perMinRate = 2.0
	}

	distanceKm := float64(distanceMeters) / 1000.0
	durationMin := float64(durationSeconds) / 60.0

	// Core Ride Cost
	rideCost := baseFare + (distanceKm * perKmRate) + (durationMin * perMinRate)

	// Platform Fee (Commission) - Configurable via ENV
	feePercentStr := os.Getenv("PLATFORM_FEE_PERCENTAGE")
	feePercent := 15.0 // Default 15%
	if val, err := strconv.ParseFloat(feePercentStr, 64); err == nil {
		feePercent = val
	}
	
	platformFee := rideCost * (feePercent / 100.0)
	
	// Total Fare
	totalFare := rideCost + platformFee

	return math.Ceil(totalFare)
}

// GET /api/v1/user/vehicle-types & /api/v1/driver/vehicle-types
func GetVehicleTypes(c *gin.Context) {
	rows, err := db.Pool.Query(context.Background(),
		`SELECT id, name, "baseFare", "perKmRate", "perMinRate", COALESCE(icon, ''), "isActive", "createdAt", "updatedAt" 
		 FROM vehicle_types WHERE "isActive"=TRUE ORDER BY "baseFare" ASC`)
	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Failed to fetch vehicle types", err)
		return
	}
	defer rows.Close()

	var types []models.VehicleTypeConfig
	for rows.Next() {
		var vt models.VehicleTypeConfig
		rows.Scan(&vt.ID, &vt.Name, &vt.BaseFare, &vt.PerKmRate, &vt.PerMinRate, &vt.Icon, &vt.IsActive, &vt.CreatedAt, &vt.UpdatedAt)
		types = append(types, vt)
	}
	if types == nil {
		types = []models.VehicleTypeConfig{}
	}
	utils.RespondSuccess(c, http.StatusOK, "Vehicle types", gin.H{"vehicleTypes": types})
}

// POST /api/v1/user/ride/estimate
func GetRideEstimate(c *gin.Context) {
	var body struct {
		Origin      string `json:"origin"`      // "lat,lng"
		Destination string `json:"destination"` // "lat,lng"
		VehicleType string `json:"vehicleType"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	olaClient := utils.NewOlaMapsClient()
	
	// Map vehicle types to Ola Modes
	mode := "driving"
	if body.VehicleType == "Bike" {
		mode = "bike"
	} else if body.VehicleType == "Auto" {
		mode = "auto"
	}

	// Refactor directions call to support mode (using literal URL for now or updating client)
	// For production, we use the client's refactored method.
	polyline, distance, duration, routeID, err := olaClient.GetDirectionsWithMode(body.Origin, body.Destination, mode)
	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Failed to calculate route", err)
		return
	}

	pickupLat, pickupLng := utils.ParseLatLng(body.Origin)
	destLat, destLng := utils.ParseLatLng(body.Destination)
	fare := CalculateFare(body.VehicleType, distance, duration)

	// OLA/UBER OPTIMIZATION: Cache the planned route in Redis
	// This prevents fare tampering and reduces frontend payload size.
	stores.StorePlannedRoute(routeID, stores.CachedRoute{
		Polyline:        polyline,
		Distance:        distance,
		Duration:        duration,
		Fare:            fare,
		VehicleType:     body.VehicleType,
		OriginName:      body.Origin, // Or body.OriginName if you have it
		DestinationName: body.Destination,
		OriginLat:       pickupLat,
		OriginLng:       pickupLng,
		DestinationLat:  destLat,
		DestinationLng:  destLng,
	})

	utils.RespondSuccess(c, http.StatusOK, "Ride estimate", gin.H{
		"polyline":  polyline,
		"distance":  fmt.Sprintf("%.2f km", float64(distance)/1000.0),
		"duration":  fmt.Sprintf("%d mins", int(float64(duration)/60.0)),
		"fare":      fare,
		"routeId":   routeID,
	})
}



// GET /api/v1/user/service-availability?lat=...&lng=...
func CheckServiceAvailability(c *gin.Context) {
	lat, _ := strconv.ParseFloat(c.Query("lat"), 64)
	lng, _ := strconv.ParseFloat(c.Query("lng"), 64)

	isAvailable := false
	nearestCity := ""
	
	for _, zone := range serviceZones {
		dist := utils.CalculateDistance(lat, lng, zone.Lat, zone.Lng)
		if dist <= zone.Radius {
			isAvailable = true
			nearestCity = zone.Name
			break
		}
	}

	msg := "Service is available in your area (" + nearestCity + ")"
	if !isAvailable {
		// Construct dynamic list of available cities
		var cities []string
		for _, z := range serviceZones {
			cities = append(cities, z.Name)
		}
		msg = fmt.Sprintf("Service not available. We operate in: %s", strings.Join(cities, ", "))
	}

	utils.RespondSuccess(c, http.StatusOK, "Service check", gin.H{
		"isAvailable": isAvailable,
		"message":     msg,
	})
}

// GET /api/v1/user/places/autocomplete?input=...
func PlacesAutocomplete(c *gin.Context) {
	input := c.Query("input")
	if input == "" {
		utils.RespondError(c, http.StatusBadRequest, "Input is required", nil)
		return
	}

	olaClient := utils.NewOlaMapsClient()
	places, err := olaClient.Autocomplete(input)
	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Search failed", err)
		return
	}

	utils.RespondSuccess(c, http.StatusOK, "Search results", gin.H{"places": places})
}

// GET /api/v1/user/places/nearby?lat=...&lng=...&types=...
func NearbySearch(c *gin.Context) {
	lat, _ := strconv.ParseFloat(c.Query("lat"), 64)
	lng, _ := strconv.ParseFloat(c.Query("lng"), 64)
	types := c.Query("types")
	radius, _ := strconv.Atoi(c.DefaultQuery("radius", "5000"))

	olaClient := utils.NewOlaMapsClient()
	results, err := olaClient.NearbySearch(lat, lng, types, radius)
	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Nearby search failed", err)
		return
	}

	utils.RespondSuccess(c, http.StatusOK, "Nearby places", gin.H{"results": results})
}

// POST /api/v1/user/ride/create
func CreateRide(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	var body struct {
		RouteID     string `json:"routeId"`
		VehicleType string `json:"vehicleType"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// 1. Retrieve the audited route from Redis cache
	cached, err := stores.GetPlannedRoute(body.RouteID)
	if err != nil {
		utils.RespondError(c, http.StatusGone, "This route has expired. Please get a fresh estimate.", err)
		return
	}

	var rideId string
	err = db.Pool.QueryRow(context.Background(),
		`INSERT INTO rides (
			id, "userId", "driverId", charge, "currentLocationName", "destinationLocationName", 
			distance, polyline, "routeId", "estimatedDuration", "estimatedDistance", "vehicleType",
			"originLat", "originLng", "destinationLat", "destinationLng",
			status, "createdAt", "updatedAt"
		) VALUES (
			gen_random_uuid()::text, $1, NULL, $2, $3, $4, 
			$5, NULL, $6, $7, $8, $9,
			$10, $11, $12, $13,
			'Requested', NOW(), NOW()
		) RETURNING id`,
		user.ID, cached.Fare, cached.OriginName, cached.DestinationName,
		fmt.Sprintf("%d", cached.Distance), body.RouteID, cached.Duration, cached.Distance, cached.VehicleType,
		cached.OriginLat, cached.OriginLng, cached.DestinationLat, cached.DestinationLng,
	).Scan(&rideId)

	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Failed to create ride", err)
		return
	}

	// Find nearby drivers from Redis (5km radius)
	nearbyDrivers, _ := stores.GetNearbyDrivers(cached.OriginLat, cached.OriginLng, 5.0)

	// Background: filter by online+active status and send push notifications
	utils.SafeGo(func() {
		if len(nearbyDrivers) == 0 {
			return
		}

		// Collect nearby driver IDs
		driverIDs := make([]string, 0, len(nearbyDrivers))
		for _, d := range nearbyDrivers {
			driverIDs = append(driverIDs, d.DriverID)
		}


		// Cross-check with DB: only online + active drivers of requested vehicle type get notifications
		rows, err := db.Pool.Query(context.Background(),
			`SELECT id, "notificationToken" FROM driver 
			 WHERE id=ANY($1) AND "isOnline"=TRUE AND status='active' AND "vehicle_type"=$2 AND "notificationToken" IS NOT NULL AND "notificationToken" != ''`,
			driverIDs, body.VehicleType)
		if err != nil {
			utils.Logger.Error("Failed to query online drivers", zap.Error(err))
			return
		}
		defer rows.Close()

		var tokens []string
		for rows.Next() {
			var id string
			var token *string
			rows.Scan(&id, &token)
			if token != nil && *token != "" {
				tokens = append(tokens, *token)
			}
		}

		// Send FCM push notifications to all online nearby drivers
		if len(tokens) > 0 {
			utils.SendPushToMultiple(tokens,
				"🚗 New Ride Request!",
				fmt.Sprintf("Pickup: %s → %s (₹%.0f)", cached.OriginName, cached.DestinationName, cached.Fare),
				utils.FCMData{
					"type":            "ride_request",
					"rideId":          rideId,
					"pickupLat":       fmt.Sprintf("%.6f", cached.OriginLat),
					"pickupLng":       fmt.Sprintf("%.6f", cached.OriginLng),
					"originName":      cached.OriginName,
					"destinationName": cached.DestinationName,
					"fare":            fmt.Sprintf("%.2f", cached.Fare),
					"vehicleType":     cached.VehicleType,
				},
			)
		}

		// Also publish to Redis pub/sub for WebSocket listeners
		pubCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		stores.PublishRideRequest(pubCtx, stores.RideRequestEvent{
			RideID:      rideId,
			UserID:      user.ID,
			PickupLat:   cached.OriginLat,
			PickupLon:   cached.OriginLng,
			Destination: cached.DestinationName,
			Fare:        cached.Fare,
			Distance:    cached.Distance,
			Duration:    cached.Duration,
		})
	})

	utils.RespondSuccess(c, http.StatusCreated, "Ride requested", gin.H{
		"rideId":        rideId,
		"nearbyDrivers": len(nearbyDrivers),
	})
}

// POST /api/v1/user/ride/cancel
func CancelRide(c *gin.Context) {
	var body struct {
		RideID       string `json:"rideId"`
		CancelReason string `json:"cancelReason"`
		Role         string `json:"role"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	var driverID *string
	err := db.Pool.QueryRow(context.Background(),
		`UPDATE rides SET status='Cancelled', "cancelReason"=$1, "cancelledAt"=NOW(), "updatedAt"=NOW() 
		 WHERE id=$2 RETURNING "driverId"`,
		body.CancelReason, body.RideID).Scan(&driverID)

	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Failed to cancel ride", err)
		return
	}

	// If a driver was assigned, notify them
	if driverID != nil && *driverID != "" {
		var driverToken *string
		db.Pool.QueryRow(context.Background(), `SELECT "notificationToken" FROM driver WHERE id=$1`, *driverID).Scan(&driverToken)
		
		if driverToken != nil && *driverToken != "" {
			go utils.SendPushNotification(*driverToken, "Ride Cancelled ❌", "The user has cancelled the ride request.", utils.FCMData{
				"type":   "ride_cancelled",
				"rideId": body.RideID,
			})
		}
		// Also remove driver from busy status if needed, but usually they just go back to online
	}

	utils.RespondSuccess(c, http.StatusOK, "Ride cancelled", nil)
}

// GET /api/v1/user/ride/:id
func GetRideDetails(c *gin.Context) {
	rideID := c.Param("id")
	currentUser, _ := c.Get("user")

	var ride models.Ride
	var driver models.Driver
	var user models.User

	err := db.Pool.QueryRow(context.Background(),
		`SELECT 
			r.id, r."userId", r."driverId", r.charge, r."currentLocationName", r."destinationLocationName", 
			r.distance, r.status, COALESCE(r."paymentMode", ''), COALESCE(r."paymentStatus", 'Pending'), 
			COALESCE(r.otp, ''), COALESCE(r.polyline, ''), COALESCE(r."routeId", ''),
			r."originLat", r."originLng", r."destinationLat", r."destinationLng",
			r."createdAt",
			COALESCE(d.id, ''), COALESCE(d.name, ''), COALESCE(d.phone_number, ''), COALESCE(d.vehicle_type, ''), 
			COALESCE(d.vehicle_color, ''), COALESCE(d.registration_number, ''), COALESCE(d.ratings, 0), COALESCE(d."totalRides", 0), 
			COALESCE(d."totalDistance", 0), COALESCE(d."profileImage", ''), d.upi_id,
			u.id, u.name, u.phone_number, u.ratings
		FROM rides r
		LEFT JOIN driver d ON r."driverId" = d.id
		JOIN "user" u ON r."userId" = u.id
		WHERE r.id=$1`, rideID).
		Scan(
			&ride.ID, &ride.UserID, &ride.DriverID, &ride.Charge, &ride.CurrentLocationName, &ride.DestinationLocationName,
			&ride.Distance, &ride.Status, &ride.PaymentMode, &ride.PaymentStatus,
			&ride.OTP, &ride.Polyline, &ride.RouteID,
			&ride.OriginLat, &ride.OriginLng, &ride.DestinationLat, &ride.DestinationLng,
			&ride.CreatedAt,
			&driver.ID, &driver.Name, &driver.PhoneNumber, &driver.VehicleType,
			&driver.VehicleColor, &driver.RegistrationNumber, &driver.Ratings, &driver.TotalRides,
			&driver.TotalDistance, &driver.ProfileImage, &driver.UpiID,
			&user.ID, &user.Name, &user.PhoneNumber, &user.Ratings,
		)

	if err != nil {
		utils.RespondError(c, http.StatusNotFound, "Ride not found", err)
		return
	}

	// Security: IDOR prevention
	if currentUser != nil {
		u := currentUser.(*models.User)
		if ride.UserID != u.ID {
			utils.RespondError(c, http.StatusForbidden, "Unauthorized access to this ride", nil)
			return
		}
	}

	if driver.ID != "" {
		ride.Driver = &driver
	}
	ride.User = &user

	// FALLBACK: If polyline is missing from the optimized rides table, fetch it from the Audit Log
	if ride.Polyline == "" && ride.RouteID != "" {
		var respPayload []byte
		err := db.Pool.QueryRow(context.Background(),
			`SELECT "responsePayload" FROM external_api_logs WHERE "requestId" = $1`,
			ride.RouteID).Scan(&respPayload)
		
		if err == nil {
			// Extract polyline from the logged Ola Maps response
			var result struct {
				Routes []struct {
					OverviewPolyline struct {
						Points string `json:"points"`
					} `json:"overview_polyline"`
				} `json:"routes"`
			}
			if json.Unmarshal(respPayload, &result) == nil && len(result.Routes) > 0 {
				ride.Polyline = result.Routes[0].OverviewPolyline.Points
			}
		}
	}

	// Generate UPI QR Code if driver has UPI ID
	var qrCodeBase64 string
	if driver.UpiID != nil && *driver.UpiID != "" {
		// Construct UPI URL: upi://pay?pa=<upi_id>&pn=<name>&am=<amount>&cu=INR
		// Encoded properly for QR generation.
		param := fmt.Sprintf("upi://pay?pa=%s&pn=%s&am=%.2f&cu=INR", *driver.UpiID, driver.Name, ride.Charge)
		
		// Create QR code (Medium redundancy)
		png, err := qrcode.Encode(param, qrcode.Medium, 256)
		if err == nil {
			qrCodeBase64 = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
		} else {
			utils.Logger.Error("Failed to generate QR code", zap.Error(err))
		}
	}

	utils.RespondSuccess(c, http.StatusOK, "Ride details", gin.H{
		"ride":      ride,
		"paymentQr": qrCodeBase64, // Send QR image string to frontend
	})
}

// POST /api/v1/user/rate-driver
func RateDriver(c *gin.Context) {
	var body struct {
		RideID   string  `json:"rideId"`
		DriverID string  `json:"driverId"`
		Rating   float64 `json:"rating"`
		Comment  string  `json:"comment"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	db.Pool.Exec(context.Background(), `UPDATE rides SET rating=$1 WHERE id=$2`, body.Rating, body.RideID)

	_, err := db.Pool.Exec(context.Background(),
		`UPDATE driver SET ratings = (ratings * "totalRides" + $1) / ("totalRides" + 1), "updatedAt"=NOW() WHERE id=$2`,
		body.Rating, body.DriverID)

	if err != nil {
		utils.Logger.Error("Failed to update driver rating", zap.Error(err))
	}

	utils.RespondSuccess(c, http.StatusOK, "Rating submitted", nil)
}

// POST /api/v1/driver/rate-user
func RateUser(c *gin.Context) {
	var body struct {
		UserID string  `json:"userId"`
		Rating float64 `json:"rating"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	_, err := db.Pool.Exec(context.Background(),
		`UPDATE "user" SET ratings = (ratings * "totalRides" + $1) / ("totalRides" + 1), "updatedAt"=NOW() WHERE id=$2`,
		body.Rating, body.UserID)

	if err != nil {
		utils.Logger.Error("Failed to update user rating", zap.Error(err))
	}

	utils.RespondSuccess(c, http.StatusOK, "Rating submitted", nil)
}

// POST /api/v1/user/sos
func TriggerSOS(c *gin.Context) {
	var body struct {
		RideID string  `json:"rideId"`
		UserID string  `json:"userId"`
		Lat    float64 `json:"latitude"`
		Lng    float64 `json:"longitude"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	utils.Logger.Error("SOS TRIGGERED", zap.String("rideId", body.RideID), zap.Float64("lat", body.Lat), zap.Float64("lng", body.Lng))

	// Persist SOS alert for admin audit trail
	db.Pool.Exec(context.Background(),
		`INSERT INTO sos_alerts (id, "rideId", "userId", lat, lng, status, "createdAt")
		VALUES (gen_random_uuid()::text, $1, $2, $3, $4, 'active', NOW())`,
		body.RideID, body.UserID, body.Lat, body.Lng)

	utils.RespondSuccess(c, http.StatusOK, "SOS Alert Sent!", nil)
}

// POST /api/v1/driver/payment/confirm
func ConfirmPayment(c *gin.Context) {
	var body struct {
		RideID string  `json:"rideId"`
		Amount float64 `json:"amount"`
		Mode   string  `json:"mode"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	_, err := db.Pool.Exec(context.Background(),
		`UPDATE rides SET "paymentStatus"='Paid', "paymentMode"=$1, "updatedAt"=NOW() WHERE id=$2`,
		body.Mode, body.RideID)
	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Failed to confirm payment", err)
		return
	}

	if body.Amount == 0 {
		db.Pool.QueryRow(context.Background(), `SELECT charge FROM rides WHERE id=$1`, body.RideID).Scan(&body.Amount)
	}

	db.Pool.Exec(context.Background(),
		`INSERT INTO payments (id, "rideId", amount, mode, status, "createdAt")
		VALUES (gen_random_uuid()::text, $1, $2, $3, 'paid', NOW())`,
		body.RideID, body.Amount, body.Mode)

	utils.RespondSuccess(c, http.StatusOK, "Payment confirmed", nil)
}
