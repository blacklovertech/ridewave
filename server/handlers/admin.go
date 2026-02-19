package handlers

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"ridewave/db"
	"ridewave/models"
	"ridewave/stores"
	"ridewave/utils"
)

// RegisterAdminRoutes defines all administrative API endpoints
func RegisterAdminRoutes(r *gin.Engine, adminMiddleware gin.HandlerFunc) {
	adminGroup := r.Group("/api/v1/admin")
	adminGroup.Use(adminMiddleware)
	{
		// Dashboard
		adminGroup.GET("/dashboard", AdminDashboard)

		// Email OTP (admin-only)
		adminGroup.POST("/email-otp-request", SendingOtpToEmail)
		adminGroup.PUT("/email-otp-verify", VerifyingEmail)

		// User Management
		adminGroup.GET("/users", AdminGetUsers)
		adminGroup.GET("/user/:id", AdminGetUserDetail)
		adminGroup.PUT("/user/:id/status", AdminUpdateUserStatus)

		// Driver Management
		adminGroup.GET("/drivers", AdminGetDrivers)
		adminGroup.GET("/driver/:id", AdminGetDriverDetail)
		adminGroup.PUT("/driver/:id/status", AdminUpdateDriverStatus)
		adminGroup.GET("/drivers/live", AdminGetLiveDrivers)

		// Ride Management
		adminGroup.GET("/rides", AdminGetRides)
		adminGroup.GET("/ride/:id", AdminGetRideDetail)

		// Payment Management
		adminGroup.GET("/payments", AdminGetPayments)

		// Vehicle Type Management
		adminGroup.GET("/vehicle-types", AdminGetAllVehicleTypes)
		adminGroup.PUT("/vehicle-type", AdminUpsertVehicleType)
		adminGroup.DELETE("/vehicle-type/:id", AdminDeleteVehicleType)

		// SOS Alert Management
		adminGroup.GET("/sos-alerts", AdminGetSOSAlerts)
		adminGroup.PUT("/sos/:id/resolve", AdminResolveSOSAlert)

		// Promo Code Management
		adminGroup.GET("/promo-codes", AdminGetPromoCodes)
		adminGroup.POST("/promo-code", AdminCreatePromoCode)
		adminGroup.PUT("/promo-code/:id", AdminUpdatePromoCode)
		adminGroup.DELETE("/promo-code/:id", AdminDeletePromoCode)

		// Analytics
		adminGroup.GET("/analytics/daily", AdminDailyAnalytics)
	}
}


// ══════════════════════════════════════════════════
// Admin Dashboard — rich overview
// ══════════════════════════════════════════════════

// GET /api/v1/admin/dashboard
func AdminDashboard(c *gin.Context) {
	var totalUsers, totalDrivers, totalRides, activeDrivers, completedRides, cancelledRides, requestedRides, ongoingRides int
	var totalRevenue, avgRating float64

	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM "user"`).Scan(&totalUsers)
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM driver`).Scan(&totalDrivers)
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM driver WHERE status='active'`).Scan(&activeDrivers)
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rides`).Scan(&totalRides)
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rides WHERE status='Completed'`).Scan(&completedRides)
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rides WHERE status='Cancelled'`).Scan(&cancelledRides)
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rides WHERE status='Requested'`).Scan(&requestedRides)
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rides WHERE status IN ('Accepted','Arriving','InProgress')`).Scan(&ongoingRides)
	db.Pool.QueryRow(context.Background(), `SELECT COALESCE(SUM(charge), 0) FROM rides WHERE status='Completed'`).Scan(&totalRevenue)
	db.Pool.QueryRow(context.Background(), `SELECT COALESCE(AVG(rating), 0) FROM rides WHERE rating IS NOT NULL`).Scan(&avgRating)

	// Today's stats
	var todayRides, todayCompleted, todayNewUsers, todayNewDrivers int
	var todayRevenue float64
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rides WHERE DATE("createdAt")=CURRENT_DATE`).Scan(&todayRides)
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rides WHERE status='Completed' AND DATE("createdAt")=CURRENT_DATE`).Scan(&todayCompleted)
	db.Pool.QueryRow(context.Background(), `SELECT COALESCE(SUM(charge), 0) FROM rides WHERE status='Completed' AND DATE("createdAt")=CURRENT_DATE`).Scan(&todayRevenue)
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM "user" WHERE DATE("createdAt")=CURRENT_DATE`).Scan(&todayNewUsers)
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM driver WHERE DATE("createdAt")=CURRENT_DATE`).Scan(&todayNewDrivers)

	// This week
	var weekRides int
	var weekRevenue float64
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rides WHERE "createdAt" >= NOW() - INTERVAL '7 days'`).Scan(&weekRides)
	db.Pool.QueryRow(context.Background(), `SELECT COALESCE(SUM(charge), 0) FROM rides WHERE status='Completed' AND "createdAt" >= NOW() - INTERVAL '7 days'`).Scan(&weekRevenue)

	// Vehicle type popularity
	type VehicleStat struct {
		VehicleType string  `json:"vehicleType"`
		Count       int     `json:"count"`
		Revenue     float64 `json:"revenue"`
	}
	vtRows, _ := db.Pool.Query(context.Background(),
		`SELECT COALESCE("vehicleType",'Unknown'), COUNT(*), COALESCE(SUM(charge),0) 
		 FROM rides WHERE status='Completed' GROUP BY "vehicleType" ORDER BY COUNT(*) DESC LIMIT 10`)
	var vehicleStats []VehicleStat
	if vtRows != nil {
		defer vtRows.Close()
		for vtRows.Next() {
			var vs VehicleStat
			vtRows.Scan(&vs.VehicleType, &vs.Count, &vs.Revenue)
			vehicleStats = append(vehicleStats, vs)
		}
	}
	if vehicleStats == nil {
		vehicleStats = []VehicleStat{}
	}

	// Recent rides (last 5)
	type RecentRide struct {
		ID         string  `json:"id"`
		UserName   string  `json:"userName"`
		DriverName string  `json:"driverName"`
		Origin     string  `json:"origin"`
		Dest       string  `json:"destination"`
		Charge     float64 `json:"charge"`
		Status     string  `json:"status"`
		CreatedAt  string  `json:"createdAt"`
	}
	rrRows, _ := db.Pool.Query(context.Background(),
		`SELECT r.id, COALESCE(u.name,''), COALESCE(d.name,''), r."currentLocationName", r."destinationLocationName", r.charge, r.status, r."createdAt"
		 FROM rides r LEFT JOIN "user" u ON r."userId"=u.id LEFT JOIN driver d ON r."driverId"=d.id 
		 ORDER BY r."createdAt" DESC LIMIT 5`)
	var recentRides []RecentRide
	if rrRows != nil {
		defer rrRows.Close()
		for rrRows.Next() {
			var rr RecentRide
			rrRows.Scan(&rr.ID, &rr.UserName, &rr.DriverName, &rr.Origin, &rr.Dest, &rr.Charge, &rr.Status, &rr.CreatedAt)
			recentRides = append(recentRides, rr)
		}
	}
	if recentRides == nil {
		recentRides = []RecentRide{}
	}

	utils.RespondSuccess(c, http.StatusOK, "Dashboard stats", gin.H{
		"users": gin.H{
			"total":    totalUsers,
			"newToday": todayNewUsers,
		},
		"drivers": gin.H{
			"total":    totalDrivers,
			"active":   activeDrivers,
			"newToday": todayNewDrivers,
		},
		"rides": gin.H{
			"total":     totalRides,
			"completed": completedRides,
			"cancelled": cancelledRides,
			"requested": requestedRides,
			"ongoing":   ongoingRides,
			"avgRating": math.Round(avgRating*100) / 100,
		},
		"revenue": gin.H{
			"total": totalRevenue,
			"today": todayRevenue,
			"week":  weekRevenue,
		},
		"today": gin.H{
			"rides":     todayRides,
			"completed": todayCompleted,
			"revenue":   todayRevenue,
		},
		"vehicleStats": vehicleStats,
		"recentRides":  recentRides,
	})
}

// ══════════════════════════════════════════════════
// Admin: User Management
// ══════════════════════════════════════════════════

// GET /api/v1/admin/users?page=1&limit=20&search=query
func AdminGetUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	search := c.Query("search")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int

	baseQuery := `SELECT id, name, phone_number, email, "notificationToken", ratings, "totalRides", "createdAt", "updatedAt" FROM "user"`
	countQuery := `SELECT COUNT(*) FROM "user"`

	var args []interface{}
	whereClause := ""

	if search != "" {
		searchPattern := "%" + search + "%"
		whereClause = ` WHERE name ILIKE $1 OR phone_number ILIKE $1 OR email ILIKE $1`
		args = append(args, searchPattern)
	}

	db.Pool.QueryRow(context.Background(), countQuery+whereClause, args...).Scan(&total)

	var queryArgs []interface{}
	if search != "" {
		queryArgs = []interface{}{"%" + search + "%", limit, offset}
		baseQuery += ` WHERE name ILIKE $1 OR phone_number ILIKE $1 OR email ILIKE $1 ORDER BY "createdAt" DESC LIMIT $2 OFFSET $3`
	} else {
		queryArgs = []interface{}{limit, offset}
		baseQuery += ` ORDER BY "createdAt" DESC LIMIT $1 OFFSET $2`
	}

	rows, err := db.Pool.Query(context.Background(), baseQuery, queryArgs...)
	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Failed to fetch users", err)
		return
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		rows.Scan(&u.ID, &u.Name, &u.PhoneNumber, &u.Email, &u.NotificationToken, &u.Ratings, &u.TotalRides, &u.CreatedAt, &u.UpdatedAt)
		users = append(users, u)
	}
	if users == nil {
		users = []models.User{}
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	utils.RespondSuccess(c, http.StatusOK, "Users", gin.H{
		"users": users, "total": total, "page": page, "limit": limit, "totalPages": totalPages,
	})
}

// GET /api/v1/admin/user/:id — full user detail with ride history & stats
func AdminGetUserDetail(c *gin.Context) {
	userID := c.Param("id")

	var user models.User
	err := db.Pool.QueryRow(context.Background(),
		`SELECT id, name, phone_number, email, "notificationToken", ratings, "totalRides", "createdAt", "updatedAt" FROM "user" WHERE id=$1`, userID).
		Scan(&user.ID, &user.Name, &user.PhoneNumber, &user.Email, &user.NotificationToken, &user.Ratings, &user.TotalRides, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		utils.RespondError(c, http.StatusNotFound, "User not found", err)
		return
	}

	// User's ride stats
	var completedRides, cancelledRides, totalRidesCount int
	var totalSpent, avgRide float64
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rides WHERE "userId"=$1`, userID).Scan(&totalRidesCount)
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rides WHERE "userId"=$1 AND status='Completed'`, userID).Scan(&completedRides)
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rides WHERE "userId"=$1 AND status='Cancelled'`, userID).Scan(&cancelledRides)
	db.Pool.QueryRow(context.Background(), `SELECT COALESCE(SUM(charge), 0) FROM rides WHERE "userId"=$1 AND status='Completed'`, userID).Scan(&totalSpent)
	if completedRides > 0 {
		avgRide = totalSpent / float64(completedRides)
	}

	// Payment Breakdown
	var cashRides, onlineRides int
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rides WHERE "userId"=$1 AND "paymentMode"='cash' AND status='Completed'`, userID).Scan(&cashRides)
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rides WHERE "userId"=$1 AND "paymentMode"!='cash' AND status='Completed'`, userID).Scan(&onlineRides)

	// Recent rides (last 20)
	rideRows, _ := db.Pool.Query(context.Background(),
		`SELECT r.id, r.charge, r."currentLocationName", r."destinationLocationName", r.distance, r.status, 
		 COALESCE(r."vehicleType",''), r."createdAt", COALESCE(d.name,'') as driverName,
		 COALESCE(r."paymentMode",''), COALESCE(r.tips, 0), COALESCE(r."estimatedDuration", 0)
		 FROM rides r LEFT JOIN driver d ON r."driverId"=d.id 
		 WHERE r."userId"=$1 ORDER BY r."createdAt" DESC LIMIT 20`, userID)

	type UserRide struct {
		ID          string  `json:"id"`
		Charge      float64 `json:"charge"`
		Origin      string  `json:"origin"`
		Destination string  `json:"destination"`
		Distance    string  `json:"distance"`
		Status      string  `json:"status"`
		VehicleType string  `json:"vehicleType"`
		CreatedAt   string  `json:"createdAt"`
		DriverName  string  `json:"driverName"`
		PaymentMode string  `json:"paymentMode"`
		Tips        float64 `json:"tips"`
		Duration    int     `json:"duration"`
	}
	var rides []UserRide
	if rideRows != nil {
		defer rideRows.Close()
		for rideRows.Next() {
			var r UserRide
			rideRows.Scan(&r.ID, &r.Charge, &r.Origin, &r.Destination, &r.Distance, &r.Status, &r.VehicleType, &r.CreatedAt, &r.DriverName, &r.PaymentMode, &r.Tips, &r.Duration)
			rides = append(rides, r)
		}
	}
	if rides == nil {
		rides = []UserRide{}
	}

	utils.RespondSuccess(c, http.StatusOK, "User detail", gin.H{
		"user": user,
		"stats": gin.H{
			"totalRides":     totalRidesCount,
			"completedRides": completedRides,
			"cancelledRides": cancelledRides,
			"totalSpent":     totalSpent,
			"avgRideValue":   math.Round(avgRide*100) / 100,
			"cashRides":      cashRides,
			"onlineRides":    onlineRides,
		},
		"recentRides": rides,
	})
}

// PUT /api/v1/admin/user/:id/status — activate, deactivate, suspend user
func AdminUpdateUserStatus(c *gin.Context) {
	userID := c.Param("id")
	var body struct {
		Action string `json:"action" binding:"required"` // "activate", "deactivate", "suspend"
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	switch body.Action {
	case "activate":
		db.Pool.Exec(context.Background(),
			`UPDATE "user" SET status='active', "updatedAt"=NOW() WHERE id=$1`, userID)
	case "deactivate":
		db.Pool.Exec(context.Background(),
			`UPDATE "user" SET status='inactive', "notificationToken"=NULL, "updatedAt"=NOW() WHERE id=$1`, userID)
	case "suspend":
		db.Pool.Exec(context.Background(),
			`UPDATE "user" SET status='suspended', "notificationToken"=NULL, "updatedAt"=NOW() WHERE id=$1`, userID)
	default:
		utils.RespondError(c, http.StatusBadRequest, "Invalid action. Use: activate, deactivate, suspend", nil)
		return
	}

	utils.RespondSuccess(c, http.StatusOK, "User status updated", gin.H{"userId": userID, "action": body.Action})
}

// ══════════════════════════════════════════════════
// Admin: Driver Management
// ══════════════════════════════════════════════════

// GET /api/v1/admin/drivers?page=1&limit=20&status=active
func AdminGetDrivers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	statusFilter := c.Query("status")
	search := c.Query("search")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int
	var queryArgs []interface{}
	var query string
	argIdx := 1

	whereClause := ""
	var filterArgs []interface{}

	if statusFilter != "" {
		whereClause += " WHERE status=$" + strconv.Itoa(argIdx)
		filterArgs = append(filterArgs, statusFilter)
		argIdx++
	}
	if search != "" {
		if whereClause == "" {
			whereClause += " WHERE"
		} else {
			whereClause += " AND"
		}
		whereClause += " (name ILIKE $" + strconv.Itoa(argIdx) + " OR phone_number ILIKE $" + strconv.Itoa(argIdx) + ")"
		filterArgs = append(filterArgs, "%"+search+"%")
		argIdx++
	}

	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM driver`+whereClause, filterArgs...).Scan(&total)

	query = `SELECT ` + driverSelectCols + ` FROM driver` + whereClause + ` ORDER BY "createdAt" DESC LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	queryArgs = append(filterArgs, limit, offset)

	rows, err := db.Pool.Query(context.Background(), query, queryArgs...)
	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Failed to fetch drivers", err)
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

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	utils.RespondSuccess(c, http.StatusOK, "Drivers", gin.H{
		"drivers": drivers, "total": total, "page": page, "limit": limit, "totalPages": totalPages,
	})
}

// GET /api/v1/admin/driver/:id — full driver detail with ride history, earnings, live location
func AdminGetDriverDetail(c *gin.Context) {
	driverID := c.Param("id")

	var driver models.Driver
	row := db.Pool.QueryRow(context.Background(),
		`SELECT `+driverSelectCols+` FROM driver WHERE id=$1`, driverID)
	if err := scanDriver(row, &driver); err != nil {
		utils.RespondError(c, http.StatusNotFound, "Driver not found", err)
		return
	}

	// Driver stats
	var completedRides, cancelledRides, totalRidesCount int
	var totalEarned, avgEarning, totalDistanceTraveled float64
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rides WHERE "driverId"=$1`, driverID).Scan(&totalRidesCount)
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rides WHERE "driverId"=$1 AND status='Completed'`, driverID).Scan(&completedRides)
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rides WHERE "driverId"=$1 AND status='Cancelled'`, driverID).Scan(&cancelledRides)
	db.Pool.QueryRow(context.Background(), `SELECT COALESCE(SUM(charge), 0) FROM rides WHERE "driverId"=$1 AND status='Completed'`, driverID).Scan(&totalEarned)
	db.Pool.QueryRow(context.Background(), `SELECT COALESCE(SUM(CAST(distance AS DOUBLE PRECISION)), 0) FROM rides WHERE "driverId"=$1 AND status='Completed'`, driverID).Scan(&totalDistanceTraveled)
	if completedRides > 0 {
		avgEarning = totalEarned / float64(completedRides)
	}

	// Live location from DB
	var liveLocation *gin.H
	var lat, lng float64
	var heading *float64
	var locUpdatedAt time.Time
	err := db.Pool.QueryRow(context.Background(),
		`SELECT lat, lng, heading, "updatedAt" FROM driver_location WHERE "driverId"=$1`, driverID).
		Scan(&lat, &lng, &heading, &locUpdatedAt)
	if err == nil {
		liveLocation = &gin.H{
			"lat":       lat,
			"lng":       lng,
			"heading":   heading,
			"updatedAt": locUpdatedAt,
		}
	}

	// Recent rides (last 10)
	rideRows, _ := db.Pool.Query(context.Background(),
		`SELECT r.id, r.charge, r."currentLocationName", r."destinationLocationName", r.distance, r.status, 
		 COALESCE(r."vehicleType",''), r."createdAt", COALESCE(u.name,'') as userName
		 FROM rides r LEFT JOIN "user" u ON r."userId"=u.id 
		 WHERE r."driverId"=$1 ORDER BY r."createdAt" DESC LIMIT 10`, driverID)

	type DriverRide struct {
		ID          string  `json:"id"`
		Charge      float64 `json:"charge"`
		Origin      string  `json:"origin"`
		Destination string  `json:"destination"`
		Distance    string  `json:"distance"`
		Status      string  `json:"status"`
		VehicleType string  `json:"vehicleType"`
		CreatedAt   string  `json:"createdAt"`
		UserName    string  `json:"userName"`
	}
	var rides []DriverRide
	if rideRows != nil {
		defer rideRows.Close()
		for rideRows.Next() {
			var r DriverRide
			rideRows.Scan(&r.ID, &r.Charge, &r.Origin, &r.Destination, &r.Distance, &r.Status, &r.VehicleType, &r.CreatedAt, &r.UserName)
			rides = append(rides, r)
		}
	}
	if rides == nil {
		rides = []DriverRide{}
	}

	// Daily earnings (last 7 days)
	type DayEarning struct {
		Day      time.Time `json:"day"`
		Rides    int       `json:"rides"`
		Earnings float64   `json:"earnings"`
	}
	earningRows, _ := db.Pool.Query(context.Background(),
		`SELECT DATE(r."createdAt") as day, COUNT(*), COALESCE(SUM(r.charge), 0)
		 FROM rides r WHERE r."driverId"=$1 AND r.status='Completed' AND r."createdAt" >= NOW() - INTERVAL '7 days'
		 GROUP BY DATE(r."createdAt") ORDER BY day DESC`, driverID)
	var dailyEarnings []DayEarning
	if earningRows != nil {
		defer earningRows.Close()
		for earningRows.Next() {
			var de DayEarning
			earningRows.Scan(&de.Day, &de.Rides, &de.Earnings)
			dailyEarnings = append(dailyEarnings, de)
		}
	}
	if dailyEarnings == nil {
		dailyEarnings = []DayEarning{}
	}

	utils.RespondSuccess(c, http.StatusOK, "Driver detail", gin.H{
		"driver": driver,
		"stats": gin.H{
			"totalRides":     totalRidesCount,
			"completedRides": completedRides,
			"cancelledRides": cancelledRides,
			"totalEarned":    totalEarned,
			"avgEarning":     math.Round(avgEarning*100) / 100,
			"totalDistance":  totalDistanceTraveled,
		},
		"liveLocation":  liveLocation,
		"recentRides":   rides,
		"dailyEarnings": dailyEarnings,
	})
}

// PUT /api/v1/admin/driver/:id/status
func AdminUpdateDriverStatus(c *gin.Context) {
	driverID := c.Param("id")
	var body struct {
		Status string `json:"status" binding:"required"` // "active", "inactive", "suspended", "approved", "rejected"
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	validStatuses := map[string]bool{"active": true, "inactive": true, "suspended": true, "pending": true, "rejected": true}
	if !validStatuses[body.Status] {
		utils.RespondError(c, http.StatusBadRequest, "Invalid status. Use: active, inactive, suspended, pending, rejected", nil)
		return
	}

	// When deactivating/suspending/rejecting, also force offline
	if body.Status == "inactive" || body.Status == "suspended" || body.Status == "rejected" || body.Status == "pending" {
		_, err := db.Pool.Exec(context.Background(),
			`UPDATE driver SET status=$1, "isOnline"=FALSE, "updatedAt"=NOW() WHERE id=$2`, body.Status, driverID)
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Failed to update driver", err)
			return
		}
		stores.RemoveDriver(driverID)
	} else {
		_, err := db.Pool.Exec(context.Background(),
			`UPDATE driver SET status=$1, "updatedAt"=NOW() WHERE id=$2`, body.Status, driverID)
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Failed to update driver", err)
			return
		}
	}

	utils.RespondSuccess(c, http.StatusOK, "Driver status updated", gin.H{"driverId": driverID, "status": body.Status})
}

// ══════════════════════════════════════════════════
// Admin: Live Driver Locations — all online drivers
// ══════════════════════════════════════════════════

// GET /api/v1/admin/drivers/live
func AdminGetLiveDrivers(c *gin.Context) {
	// Fetch all driver locations from the DB (more reliable for admin)
	rows, err := db.Pool.Query(context.Background(),
		`SELECT dl."driverId", dl.lat, dl.lng, dl.heading, dl."updatedAt",
		 d.name, d.phone_number, d.vehicle_type, COALESCE(d.vehicle_color, ''), d.registration_number, d.status
		 FROM driver_location dl
		 JOIN driver d ON dl."driverId"=d.id
		 WHERE d."isOnline"=TRUE AND d.status='active'
		 ORDER BY dl."updatedAt" DESC`)
	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Failed to fetch live drivers", err)
		return
	}
	defer rows.Close()

	type LiveDriver struct {
		DriverID           string    `json:"driverId"`
		Lat                float64   `json:"lat"`
		Lng                float64   `json:"lng"`
		Heading            *float64  `json:"heading"`
		UpdatedAt          time.Time `json:"updatedAt"`
		Name               string    `json:"name"`
		PhoneNumber        string    `json:"phoneNumber"`
		VehicleType        string    `json:"vehicleType"`
		VehicleColor       string    `json:"vehicleColor"`
		RegistrationNumber string    `json:"registrationNumber"`
		Status             string    `json:"status"`
	}

	var drivers []LiveDriver
	for rows.Next() {
		var d LiveDriver
		rows.Scan(&d.DriverID, &d.Lat, &d.Lng, &d.Heading, &d.UpdatedAt,
			&d.Name, &d.PhoneNumber, &d.VehicleType, &d.VehicleColor, &d.RegistrationNumber, &d.Status)
		drivers = append(drivers, d)
	}
	if drivers == nil {
		drivers = []LiveDriver{}
	}

	utils.RespondSuccess(c, http.StatusOK, "Live drivers", gin.H{
		"drivers": drivers,
		"count":   len(drivers),
	})
}

// ══════════════════════════════════════════════════
// Admin: Ride Management
// ══════════════════════════════════════════════════

// GET /api/v1/admin/rides?page=1&limit=20&status=Completed&vehicleType=Car
func AdminGetRides(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	statusFilter := c.Query("status")
	vehicleFilter := c.Query("vehicleType")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int
	whereClause := ""
	var filterArgs []interface{}
	argIdx := 1

	if statusFilter != "" {
		whereClause += " WHERE r.status=$" + strconv.Itoa(argIdx)
		filterArgs = append(filterArgs, statusFilter)
		argIdx++
	}
	if vehicleFilter != "" {
		if whereClause == "" {
			whereClause += " WHERE"
		} else {
			whereClause += " AND"
		}
		whereClause += " r.\"vehicleType\"=$" + strconv.Itoa(argIdx)
		filterArgs = append(filterArgs, vehicleFilter)
		argIdx++
	}

	countWhere := whereClause
	if countWhere != "" {
		countWhere = " WHERE" + countWhere[6:] // replace " WHERE r." with " WHERE "
	}
	db.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM rides r`+whereClause, filterArgs...).Scan(&total)

	query := `SELECT r.id, r."userId", r."driverId", r.charge, r."currentLocationName", r."destinationLocationName", 
		r.distance, r.status, r.rating, COALESCE(r."vehicleType", ''), COALESCE(r."paymentStatus", 'Pending'), 
		COALESCE(r."paymentMode",''), COALESCE(r.tips, 0), COALESCE(r."estimatedDuration", 0), r."createdAt",
		COALESCE(u.name, '') as userName, u.phone_number as userPhone,
		COALESCE(d.name, '') as driverName, COALESCE(d.phone_number, '') as driverPhone
		FROM rides r 
		LEFT JOIN "user" u ON r."userId"=u.id 
		LEFT JOIN driver d ON r."driverId"=d.id` +
		whereClause + ` ORDER BY r."createdAt" DESC LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)

	queryArgs := append(filterArgs, limit, offset)

	rows, err := db.Pool.Query(context.Background(), query, queryArgs...)
	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Failed to fetch rides", err)
		return
	}
	defer rows.Close()

	type AdminRide struct {
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
		PaymentStatus           string   `json:"paymentStatus"`
		PaymentMode             string   `json:"paymentMode"`
		Tips                    float64  `json:"tips"`
		Duration                int      `json:"duration"`
		CreatedAt               string   `json:"createdAt"`
		UserName                string   `json:"userName"`
		UserPhone               string   `json:"userPhone"`
		DriverName              string   `json:"driverName"`
		DriverPhone             string   `json:"driverPhone"`
	}
	var rides []AdminRide
	for rows.Next() {
		var r AdminRide
		rows.Scan(&r.ID, &r.UserID, &r.DriverID, &r.Charge, &r.CurrentLocationName, &r.DestinationLocationName,
			&r.Distance, &r.Status, &r.Rating, &r.VehicleType, &r.PaymentStatus, &r.PaymentMode, &r.Tips, &r.Duration, &r.CreatedAt,
			&r.UserName, &r.UserPhone, &r.DriverName, &r.DriverPhone)
		rides = append(rides, r)
	}
	if rides == nil {
		rides = []AdminRide{}
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	utils.RespondSuccess(c, http.StatusOK, "Rides", gin.H{
		"rides": rides, "total": total, "page": page, "limit": limit, "totalPages": totalPages,
	})
}

// GET /api/v1/admin/ride/:id — full ride detail with polyline, coordinates, travel route
func AdminGetRideDetail(c *gin.Context) {
	rideID := c.Param("id")

	var ride models.Ride
	var driver models.Driver
	var user models.User

	err := db.Pool.QueryRow(context.Background(),
		`SELECT 
			r.id, r."userId", r."driverId", r.charge, r."currentLocationName", r."destinationLocationName", 
			r.distance, r.status, COALESCE(r."paymentMode", ''), COALESCE(r."paymentStatus", 'Pending'), 
			COALESCE(r.otp, ''), COALESCE(r.polyline, ''), COALESCE(r."estimatedDuration", 0), COALESCE(r."estimatedDistance", 0),
			COALESCE(r."vehicleType", ''), r.rating, COALESCE(r."cancelReason", ''),
			r."originLat", r."originLng", r."destinationLat", r."destinationLng",
			r."createdAt", r."updatedAt",
			COALESCE(d.id, ''), COALESCE(d.name, ''), COALESCE(d.phone_number, ''), COALESCE(d.vehicle_type, ''), 
			COALESCE(d.vehicle_color, ''), COALESCE(d.registration_number, ''), COALESCE(d.ratings, 0), 
			COALESCE(d."totalRides", 0), COALESCE(d."totalDistance", 0), COALESCE(d."profileImage", ''),
			u.id, u.name, u.phone_number, u.email, u.ratings
		FROM rides r
		LEFT JOIN driver d ON r."driverId" = d.id
		JOIN "user" u ON r."userId" = u.id
		WHERE r.id=$1`, rideID).
		Scan(
			&ride.ID, &ride.UserID, &ride.DriverID, &ride.Charge, &ride.CurrentLocationName, &ride.DestinationLocationName,
			&ride.Distance, &ride.Status, &ride.PaymentMode, &ride.PaymentStatus,
			&ride.OTP, &ride.Polyline, &ride.EstimatedDuration, &ride.EstimatedDistance,
			&ride.VehicleType, &ride.Rating, &ride.CancelReason,
			&ride.OriginLat, &ride.OriginLng, &ride.DestinationLat, &ride.DestinationLng,
			&ride.CreatedAt, &ride.UpdatedAt,
			&driver.ID, &driver.Name, &driver.PhoneNumber, &driver.VehicleType,
			&driver.VehicleColor, &driver.RegistrationNumber, &driver.Ratings,
			&driver.TotalRides, &driver.TotalDistance, &driver.ProfileImage,
			&user.ID, &user.Name, &user.PhoneNumber, &user.Email, &user.Ratings,
		)

	if err != nil {
		utils.RespondError(c, http.StatusNotFound, "Ride not found", err)
		return
	}

	// Payment info
	var payment *models.Payment
	var p models.Payment
	pErr := db.Pool.QueryRow(context.Background(),
		`SELECT id, "rideId", amount, mode, status, "createdAt" FROM payments WHERE "rideId"=$1`, rideID).
		Scan(&p.ID, &p.RideID, &p.Amount, &p.Mode, &p.Status, &p.CreatedAt)
	if pErr == nil {
		payment = &p
	}

	// Build response
	rideDetail := gin.H{
		"id":                      ride.ID,
		"userId":                  ride.UserID,
		"driverId":                ride.DriverID,
		"charge":                  ride.Charge,
		"currentLocationName":     ride.CurrentLocationName,
		"destinationLocationName": ride.DestinationLocationName,
		"distance":                ride.Distance,
		"status":                  ride.Status,
		"paymentMode":             ride.PaymentMode,
		"paymentStatus":           ride.PaymentStatus,
		"otp":                     ride.OTP,
		"vehicleType":             ride.VehicleType,
		"rating":                  ride.Rating,
		"cancelReason":            ride.CancelReason,
		"estimatedDuration":       ride.EstimatedDuration,
		"estimatedDistance":        ride.EstimatedDistance,
		"createdAt":               ride.CreatedAt,
		"updatedAt":               ride.UpdatedAt,
		"route": gin.H{
			"originLat":      ride.OriginLat,
			"originLng":      ride.OriginLng,
			"destinationLat": ride.DestinationLat,
			"destinationLng": ride.DestinationLng,
			"polyline":       ride.Polyline,
		},
	}

	driverDetail := gin.H(nil)
	if driver.ID != "" {
		driverDetail = gin.H{
			"id":                 driver.ID,
			"name":               driver.Name,
			"phoneNumber":        driver.PhoneNumber,
			"vehicleType":        driver.VehicleType,
			"vehicleColor":       driver.VehicleColor,
			"registrationNumber": driver.RegistrationNumber,
			"ratings":            driver.Ratings,
			"totalRides":         driver.TotalRides,
			"totalDistance":      driver.TotalDistance,
			"profileImage":       driver.ProfileImage,
		}
	}

	userDetail := gin.H{
		"id":          user.ID,
		"name":        user.Name,
		"phoneNumber": user.PhoneNumber,
		"email":       user.Email,
		"ratings":     user.Ratings,
	}

	utils.RespondSuccess(c, http.StatusOK, "Ride detail", gin.H{
		"ride":    rideDetail,
		"driver":  driverDetail,
		"user":    userDetail,
		"payment": payment,
	})
}

// ══════════════════════════════════════════════════
// Admin: Payment Management
// ══════════════════════════════════════════════════

// GET /api/v1/admin/payments?page=1&limit=20&mode=Cash
func AdminGetPayments(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	modeFilter := c.Query("mode")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int
	var totalAmount, paidAmount, pendingAmount float64

	db.Pool.QueryRow(context.Background(), `SELECT COALESCE(SUM(amount), 0) FROM payments WHERE status='paid'`).Scan(&paidAmount)
	db.Pool.QueryRow(context.Background(), `SELECT COALESCE(SUM(amount), 0) FROM payments WHERE status='pending'`).Scan(&pendingAmount)

	var queryArgs []interface{}
	var query string

	if modeFilter != "" {
		db.Pool.QueryRow(context.Background(), `SELECT COUNT(*), COALESCE(SUM(amount), 0) FROM payments WHERE mode=$1`, modeFilter).Scan(&total, &totalAmount)
		query = `SELECT p.id, p."rideId", p.amount, p.mode, p.status, p."createdAt"
				 FROM payments p WHERE p.mode=$1 ORDER BY p."createdAt" DESC LIMIT $2 OFFSET $3`
		queryArgs = []interface{}{modeFilter, limit, offset}
	} else {
		db.Pool.QueryRow(context.Background(), `SELECT COUNT(*), COALESCE(SUM(amount), 0) FROM payments`).Scan(&total, &totalAmount)
		query = `SELECT p.id, p."rideId", p.amount, p.mode, p.status, p."createdAt"
				 FROM payments p ORDER BY p."createdAt" DESC LIMIT $1 OFFSET $2`
		queryArgs = []interface{}{limit, offset}
	}

	rows, err := db.Pool.Query(context.Background(), query, queryArgs...)
	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Failed to fetch payments", err)
		return
	}
	defer rows.Close()

	var payments []models.Payment
	for rows.Next() {
		var p models.Payment
		rows.Scan(&p.ID, &p.RideID, &p.Amount, &p.Mode, &p.Status, &p.CreatedAt)
		payments = append(payments, p)
	}
	if payments == nil {
		payments = []models.Payment{}
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	utils.RespondSuccess(c, http.StatusOK, "Payments", gin.H{
		"payments":      payments,
		"total":         total,
		"totalAmount":   totalAmount,
		"paidAmount":    paidAmount,
		"pendingAmount": pendingAmount,
		"page":          page,
		"limit":         limit,
		"totalPages":    totalPages,
	})
}

// ══════════════════════════════════════════════════
// Admin: Vehicle Type Management
// ══════════════════════════════════════════════════

// GET /api/v1/admin/vehicle-types — all vehicle types (including inactive)
func AdminGetAllVehicleTypes(c *gin.Context) {
	rows, err := db.Pool.Query(context.Background(),
		`SELECT id, name, "baseFare", "perKmRate", "perMinRate", COALESCE(icon, ''), "isActive", "createdAt", "updatedAt" 
		 FROM vehicle_types ORDER BY "baseFare" ASC`)
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
	utils.RespondSuccess(c, http.StatusOK, "All vehicle types", gin.H{"vehicleTypes": types})
}

// PUT /api/v1/admin/vehicle-type — create or update
func AdminUpsertVehicleType(c *gin.Context) {
	var body struct {
		ID         string  `json:"id"`
		Name       string  `json:"name" binding:"required"`
		BaseFare   float64 `json:"baseFare" binding:"required"`
		PerKmRate  float64 `json:"perKmRate" binding:"required"`
		PerMinRate float64 `json:"perMinRate" binding:"required"`
		Icon       string  `json:"icon"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	if body.ID != "" {
		_, err := db.Pool.Exec(context.Background(),
			`UPDATE vehicle_types SET name=$1, "baseFare"=$2, "perKmRate"=$3, "perMinRate"=$4, icon=$5, "updatedAt"=NOW() WHERE id=$6`,
			body.Name, body.BaseFare, body.PerKmRate, body.PerMinRate, body.Icon, body.ID)
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Failed to update vehicle type", err)
			return
		}
		utils.RespondSuccess(c, http.StatusOK, "Vehicle type updated", nil)
	} else {
		var id string
		err := db.Pool.QueryRow(context.Background(),
			`INSERT INTO vehicle_types (id, name, "baseFare", "perKmRate", "perMinRate", icon) 
			 VALUES (gen_random_uuid()::text, $1, $2, $3, $4, $5) RETURNING id`,
			body.Name, body.BaseFare, body.PerKmRate, body.PerMinRate, body.Icon).Scan(&id)
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Failed to create vehicle type", err)
			return
		}
		utils.RespondSuccess(c, http.StatusCreated, "Vehicle type created", gin.H{"id": id})
	}
}

// DELETE /api/v1/admin/vehicle-type/:id — soft delete (deactivate)
func AdminDeleteVehicleType(c *gin.Context) {
	id := c.Param("id")
	_, err := db.Pool.Exec(context.Background(),
		`UPDATE vehicle_types SET "isActive"=FALSE, "updatedAt"=NOW() WHERE id=$1`, id)
	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Failed to deactivate vehicle type", err)
		return
	}
	utils.RespondSuccess(c, http.StatusOK, "Vehicle type deactivated", nil)
}

// ══════════════════════════════════════════════════
// Admin: SOS Alert Management
// ══════════════════════════════════════════════════

// GET /api/v1/admin/sos-alerts?status=active
func AdminGetSOSAlerts(c *gin.Context) {
	statusFilter := c.DefaultQuery("status", "active")

	rows, err := db.Pool.Query(context.Background(),
		`SELECT s.id, COALESCE(s."rideId",''), s."userId", COALESCE(s.lat, 0), COALESCE(s.lng, 0), s.status, s."createdAt",
		 COALESCE(u.name,'') as userName, u.phone_number as userPhone,
		 COALESCE(r."currentLocationName",'') as origin, COALESCE(r."destinationLocationName",'') as destination,
		 COALESCE(d.name,'') as driverName, COALESCE(d.phone_number,'') as driverPhone
		 FROM sos_alerts s
		 LEFT JOIN "user" u ON s."userId"=u.id
		 LEFT JOIN rides r ON s."rideId"=r.id
		 LEFT JOIN driver d ON r."driverId"=d.id
		 WHERE s.status=$1
		 ORDER BY s."createdAt" DESC`, statusFilter)
	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Failed to fetch SOS alerts", err)
		return
	}
	defer rows.Close()

	type SOSDetail struct {
		ID          string  `json:"id"`
		RideID      string  `json:"rideId"`
		UserID      string  `json:"userId"`
		Lat         float64 `json:"lat"`
		Lng         float64 `json:"lng"`
		Status      string  `json:"status"`
		CreatedAt   string  `json:"createdAt"`
		UserName    string  `json:"userName"`
		UserPhone   string  `json:"userPhone"`
		Origin      string  `json:"origin"`
		Destination string  `json:"destination"`
		DriverName  string  `json:"driverName"`
		DriverPhone string  `json:"driverPhone"`
	}

	var alerts []SOSDetail
	for rows.Next() {
		var a SOSDetail
		rows.Scan(&a.ID, &a.RideID, &a.UserID, &a.Lat, &a.Lng, &a.Status, &a.CreatedAt,
			&a.UserName, &a.UserPhone, &a.Origin, &a.Destination, &a.DriverName, &a.DriverPhone)
		alerts = append(alerts, a)
	}
	if alerts == nil {
		alerts = []SOSDetail{}
	}

	utils.RespondSuccess(c, http.StatusOK, "SOS alerts", gin.H{
		"alerts": alerts,
		"count":  len(alerts),
	})
}

// PUT /api/v1/admin/sos/:id/resolve
func AdminResolveSOSAlert(c *gin.Context) {
	alertID := c.Param("id")
	_, err := db.Pool.Exec(context.Background(),
		`UPDATE sos_alerts SET status='resolved', "resolvedAt"=NOW() WHERE id=$1`, alertID)
	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Failed to resolve SOS alert", err)
		return
	}
	utils.RespondSuccess(c, http.StatusOK, "SOS alert resolved", nil)
}

// ══════════════════════════════════════════════════
// Admin: Promo Code Management
// ══════════════════════════════════════════════════

// GET /api/v1/admin/promo-codes
func AdminGetPromoCodes(c *gin.Context) {
	rows, err := db.Pool.Query(context.Background(),
		`SELECT id, code, "discountType", "discountValue", "maxDiscount", "minRideAmount", 
		 "usageLimit", "usedCount", "expiresAt", "isActive", "createdAt"
		 FROM promo_codes ORDER BY "createdAt" DESC`)
	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Failed to fetch promo codes", err)
		return
	}
	defer rows.Close()

	var codes []models.PromoCode
	for rows.Next() {
		var pc models.PromoCode
		rows.Scan(&pc.ID, &pc.Code, &pc.DiscountType, &pc.DiscountValue, &pc.MaxDiscount,
			&pc.MinRideAmount, &pc.UsageLimit, &pc.UsedCount, &pc.ExpiresAt, &pc.IsActive, &pc.CreatedAt)
		codes = append(codes, pc)
	}
	if codes == nil {
		codes = []models.PromoCode{}
	}
	utils.RespondSuccess(c, http.StatusOK, "Promo codes", gin.H{"promoCodes": codes})
}

// POST /api/v1/admin/promo-code
func AdminCreatePromoCode(c *gin.Context) {
	var body struct {
		Code          string   `json:"code" binding:"required"`
		DiscountType  string   `json:"discountType" binding:"required"` // "percentage" or "flat"
		DiscountValue float64  `json:"discountValue" binding:"required"`
		MaxDiscount   *float64 `json:"maxDiscount"`
		MinRideAmount float64  `json:"minRideAmount"`
		UsageLimit    int      `json:"usageLimit"`
		ExpiresAt     *string  `json:"expiresAt"` // ISO date string
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	if body.UsageLimit == 0 {
		body.UsageLimit = 100
	}

	var id string
	var expiresAt interface{}
	if body.ExpiresAt != nil {
		expiresAt = *body.ExpiresAt
	}

	err := db.Pool.QueryRow(context.Background(),
		`INSERT INTO promo_codes (id, code, "discountType", "discountValue", "maxDiscount", "minRideAmount", "usageLimit", "expiresAt")
		 VALUES (gen_random_uuid()::text, $1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		body.Code, body.DiscountType, body.DiscountValue, body.MaxDiscount, body.MinRideAmount, body.UsageLimit, expiresAt).Scan(&id)
	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "Failed to create promo code", err)
		return
	}
	utils.RespondSuccess(c, http.StatusCreated, "Promo code created", gin.H{"id": id})
}

// PUT /api/v1/admin/promo-code/:id
func AdminUpdatePromoCode(c *gin.Context) {
	promoID := c.Param("id")
	var body struct {
		IsActive  *bool `json:"isActive"`
		UsageLimit *int `json:"usageLimit"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	if body.IsActive != nil {
		db.Pool.Exec(context.Background(),
			`UPDATE promo_codes SET "isActive"=$1 WHERE id=$2`, *body.IsActive, promoID)
	}
	if body.UsageLimit != nil {
		db.Pool.Exec(context.Background(),
			`UPDATE promo_codes SET "usageLimit"=$1 WHERE id=$2`, *body.UsageLimit, promoID)
	}

	utils.RespondSuccess(c, http.StatusOK, "Promo code updated", nil)
}

// DELETE /api/v1/admin/promo-code/:id
func AdminDeletePromoCode(c *gin.Context) {
	id := c.Param("id")
	db.Pool.Exec(context.Background(),
		`UPDATE promo_codes SET "isActive"=FALSE WHERE id=$1`, id)
	utils.RespondSuccess(c, http.StatusOK, "Promo code deactivated", nil)
}

// ══════════════════════════════════════════════════
// Admin: Analytics — daily chart data
// ══════════════════════════════════════════════════

// GET /api/v1/admin/analytics/daily?days=30
func AdminDailyAnalytics(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days < 1 || days > 365 {
		days = 30
	}

	// ── 1. Overall summary stats ──
	type OverallStats struct {
		TotalRides      int     `json:"totalRides"`
		CompletedRides  int     `json:"completedRides"`
		CancelledRides  int     `json:"cancelledRides"`
		InProgressRides int     `json:"inProgressRides"`
		RequestedRides  int     `json:"requestedRides"`
		TotalRevenue    float64 `json:"totalRevenue"`
		AverageFare     float64 `json:"averageFare"`
		TotalTips       float64 `json:"totalTips"`
		TotalDistance    float64 `json:"totalDistanceKm"`
		TotalUsers      int     `json:"totalUsers"`
		TotalDrivers    int     `json:"totalDrivers"`
		PendingDrivers  int     `json:"pendingDrivers"`
		OnlineDrivers   int     `json:"onlineDrivers"`
		ActiveUsers     int     `json:"activeUsers"`
	}

	var summary OverallStats
	db.Pool.QueryRow(context.Background(),
		`SELECT 
		 COUNT(*),
		 SUM(CASE WHEN status='Completed' THEN 1 ELSE 0 END),
		 SUM(CASE WHEN status='Cancelled' THEN 1 ELSE 0 END),
		 SUM(CASE WHEN status='InProgress' THEN 1 ELSE 0 END),
		 SUM(CASE WHEN status='Requested' THEN 1 ELSE 0 END),
		 COALESCE(SUM(CASE WHEN status='Completed' THEN charge ELSE 0 END), 0),
		 COALESCE(AVG(CASE WHEN status='Completed' THEN charge END), 0),
		 COALESCE(SUM(tips), 0),
		 COALESCE(SUM(CASE WHEN status='Completed' THEN CAST(distance AS DOUBLE PRECISION) ELSE 0 END) / 1000.0, 0)
		 FROM rides`).
		Scan(&summary.TotalRides, &summary.CompletedRides, &summary.CancelledRides,
			&summary.InProgressRides, &summary.RequestedRides,
			&summary.TotalRevenue, &summary.AverageFare, &summary.TotalTips, &summary.TotalDistance)

	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM "user"`).Scan(&summary.TotalUsers)
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM "user" WHERE status='active'`).Scan(&summary.ActiveUsers)
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM driver`).Scan(&summary.TotalDrivers)
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM driver WHERE status='pending'`).Scan(&summary.PendingDrivers)
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM driver WHERE "isOnline"=TRUE AND status='active'`).Scan(&summary.OnlineDrivers)

	// ── 2. Peak hours analysis (which hours get most rides) ──
	type PeakHour struct {
		Hour       int     `json:"hour"`
		RideCount  int     `json:"rideCount"`
		Revenue    float64 `json:"revenue"`
	}

	peakRows, _ := db.Pool.Query(context.Background(),
		`SELECT EXTRACT(HOUR FROM "createdAt")::int as hour, COUNT(*) as rides, 
		 COALESCE(SUM(CASE WHEN status='Completed' THEN charge ELSE 0 END), 0) as revenue
		 FROM rides WHERE "createdAt" >= NOW() - ($1 || ' days')::interval
		 GROUP BY EXTRACT(HOUR FROM "createdAt") ORDER BY rides DESC`, days)

	var peakHours []PeakHour
	if peakRows != nil {
		defer peakRows.Close()
		for peakRows.Next() {
			var p PeakHour
			peakRows.Scan(&p.Hour, &p.RideCount, &p.Revenue)
			peakHours = append(peakHours, p)
		}
	}
	if peakHours == nil {
		peakHours = []PeakHour{}
	}

	// ── 3. Daily chart data ──
	type DayStat struct {
		Day        time.Time `json:"day"`
		Rides      int       `json:"rides"`
		Completed  int       `json:"completed"`
		Cancelled  int       `json:"cancelled"`
		Revenue    float64   `json:"revenue"`
		NewUsers   int       `json:"newUsers"`
		NewDrivers int       `json:"newDrivers"`
	}

	dailyRows, _ := db.Pool.Query(context.Background(),
		`SELECT d.day,
		 COALESCE(r.total, 0), COALESCE(r.completed, 0), COALESCE(r.cancelled, 0), COALESCE(r.revenue, 0),
		 COALESCE(u.new_users, 0), COALESCE(dr.new_drivers, 0)
		 FROM generate_series(CURRENT_DATE - ($1 || ' days')::interval, CURRENT_DATE, '1 day') AS d(day)
		 LEFT JOIN (
			SELECT DATE("createdAt") as day, COUNT(*) as total,
			SUM(CASE WHEN status='Completed' THEN 1 ELSE 0 END) as completed,
			SUM(CASE WHEN status='Cancelled' THEN 1 ELSE 0 END) as cancelled,
			COALESCE(SUM(CASE WHEN status='Completed' THEN charge ELSE 0 END), 0) as revenue
			FROM rides GROUP BY DATE("createdAt")
		 ) r ON r.day = d.day
		 LEFT JOIN (
			SELECT DATE("createdAt") as day, COUNT(*) as new_users FROM "user" GROUP BY DATE("createdAt")
		 ) u ON u.day = d.day
		 LEFT JOIN (
			SELECT DATE("createdAt") as day, COUNT(*) as new_drivers FROM driver GROUP BY DATE("createdAt")
		 ) dr ON dr.day = d.day
		 ORDER BY d.day ASC`, days)

	var dailyStats []DayStat
	if dailyRows != nil {
		defer dailyRows.Close()
		for dailyRows.Next() {
			var s DayStat
			dailyRows.Scan(&s.Day, &s.Rides, &s.Completed, &s.Cancelled, &s.Revenue, &s.NewUsers, &s.NewDrivers)
			dailyStats = append(dailyStats, s)
		}
	}
	if dailyStats == nil {
		dailyStats = []DayStat{}
	}

	// ── 4. Top drivers (by earnings & rides) ──
	type TopDriver struct {
		ID           string  `json:"id"`
		Name         string  `json:"name"`
		Phone        string  `json:"phone"`
		TotalEarning float64 `json:"totalEarning"`
		TotalRides   float64 `json:"totalRides"`
		TotalKm      float64 `json:"totalDistanceKm"`
		Ratings      float64 `json:"ratings"`
		IsOnline     bool    `json:"isOnline"`
	}

	topRows, _ := db.Pool.Query(context.Background(),
		`SELECT id, name, phone_number, "totalEarning", "totalRides", "totalDistance", ratings, "isOnline"
		 FROM driver WHERE status='active' ORDER BY "totalEarning" DESC LIMIT 10`)

	var topDrivers []TopDriver
	if topRows != nil {
		defer topRows.Close()
		for topRows.Next() {
			var t TopDriver
			topRows.Scan(&t.ID, &t.Name, &t.Phone, &t.TotalEarning, &t.TotalRides, &t.TotalKm, &t.Ratings, &t.IsOnline)
			topDrivers = append(topDrivers, t)
		}
	}
	if topDrivers == nil {
		topDrivers = []TopDriver{}
	}

	// ── 5. Vehicle type breakdown ──
	type VehicleBreakdown struct {
		VehicleType string  `json:"vehicleType"`
		RideCount   int     `json:"rideCount"`
		Revenue     float64 `json:"revenue"`
	}

	vtRows, _ := db.Pool.Query(context.Background(),
		`SELECT COALESCE("vehicleType", 'Unknown'), COUNT(*), 
		 COALESCE(SUM(CASE WHEN status='Completed' THEN charge ELSE 0 END), 0)
		 FROM rides WHERE "createdAt" >= NOW() - ($1 || ' days')::interval
		 GROUP BY "vehicleType" ORDER BY COUNT(*) DESC`, days)

	var vehicleBreakdown []VehicleBreakdown
	if vtRows != nil {
		defer vtRows.Close()
		for vtRows.Next() {
			var v VehicleBreakdown
			vtRows.Scan(&v.VehicleType, &v.RideCount, &v.Revenue)
			vehicleBreakdown = append(vehicleBreakdown, v)
		}
	}
	if vehicleBreakdown == nil {
		vehicleBreakdown = []VehicleBreakdown{}
	}

	utils.RespondSuccess(c, http.StatusOK, "Comprehensive analytics", gin.H{
		"summary":          summary,
		"peakHours":        peakHours,
		"daily":            dailyStats,
		"topDrivers":       topDrivers,
		"vehicleBreakdown": vehicleBreakdown,
		"days":             days,
	})
}


