package models

import "time"

type User struct {
	ID                string    `json:"id"`
	Name              *string   `json:"name"`
	PhoneNumber       string    `json:"phone_number"`
	Email             *string   `json:"email"`
	NotificationToken *string   `json:"notificationToken"`
	Ratings           float64   `json:"ratings"`
	TotalRides        float64   `json:"totalRides"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type Driver struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Country            string    `json:"country"`
	PhoneNumber        string    `json:"phone_number"`
	Email              string    `json:"email"`
	VehicleType        string    `json:"vehicle_type"`
	RegistrationNumber string    `json:"registration_number"`
	RegistrationDate   string    `json:"registration_date"`
	DrivingLicense     string    `json:"driving_license"`
	RCBook             string    `json:"rc_book"`
	ProfileImage       string    `json:"profile_image"`
	VehicleColor       *string   `json:"vehicle_color"`
	Rate               string    `json:"rate"`
	NotificationToken  *string   `json:"notificationToken"`
	Ratings            float64   `json:"ratings"`
	TotalEarning       float64   `json:"totalEarning"`
	TotalRides         float64   `json:"totalRides"`
	TotalDistance      float64   `json:"totalDistance"`
	PendingRides       float64   `json:"pendingRides"`
	CancelRides        float64   `json:"cancelRides"`
	Status             string    `json:"status"`
	IsOnline           bool      `json:"isOnline"`
	UpiID              *string   `json:"upiId"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type Ride struct {
	ID                      string      `json:"id"`
	UserID                  string      `json:"userId"`
	DriverID                *string     `json:"driverId"`
	Charge                  float64     `json:"charge"`
	CurrentLocationName     string      `json:"currentLocationName"`
	DestinationLocationName string      `json:"destinationLocationName"`
	Distance                string      `json:"distance"`
	Polyline                string      `json:"polyline"`
	RouteID                 string      `json:"routeId"`
	EstimatedDuration       int         `json:"estimatedDuration"`
	EstimatedDistance        int         `json:"estimatedDistance"`
	VehicleType             string      `json:"vehicleType"`
	OriginLat               *float64    `json:"originLat"`
	OriginLng               *float64    `json:"originLng"`
	DestinationLat          *float64    `json:"destinationLat"`
	DestinationLng          *float64    `json:"destinationLng"`
	Status                  string      `json:"status"`
	Rating                  *float64    `json:"rating"`
	PaymentMode             string      `json:"paymentMode"`
	PaymentStatus           string      `json:"paymentStatus"`
	Tips                    float64     `json:"tips"`
	CancelReason            string      `json:"cancelReason"`
	OTP                     string      `json:"otp"`
	AcceptedAt              *time.Time  `json:"acceptedAt,omitempty"`
	StartedAt               *time.Time  `json:"startedAt,omitempty"`
	CompletedAt             *time.Time  `json:"completedAt,omitempty"`
	CancelledAt             *time.Time  `json:"cancelledAt,omitempty"`
	CreatedAt               time.Time   `json:"createdAt"`
	UpdatedAt               time.Time   `json:"updatedAt"`
	Driver                  *Driver     `json:"driver,omitempty"`
	User                    interface{} `json:"user,omitempty"`
}

type Payment struct {
	ID        string    `json:"id"`
	RideID    string    `json:"rideId"`
	Amount    float64   `json:"amount"`
	Mode      string    `json:"mode"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

type VehicleTypeConfig struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	BaseFare   float64   `json:"baseFare"`
	PerKmRate  float64   `json:"perKmRate"`
	PerMinRate float64   `json:"perMinRate"`
	Icon       string    `json:"icon"`
	IsActive   bool      `json:"isActive"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type SOSAlert struct {
	ID         string     `json:"id"`
	RideID     *string    `json:"rideId"`
	UserID     string     `json:"userId"`
	Lat        *float64   `json:"lat"`
	Lng        *float64   `json:"lng"`
	Status     string     `json:"status"`
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}

type PromoCode struct {
	ID            string     `json:"id"`
	Code          string     `json:"code"`
	DiscountType  string     `json:"discountType"`
	DiscountValue float64    `json:"discountValue"`
	MaxDiscount   *float64   `json:"maxDiscount"`
	MinRideAmount float64    `json:"minRideAmount"`
	UsageLimit    int        `json:"usageLimit"`
	UsedCount     int        `json:"usedCount"`
	ExpiresAt     *time.Time `json:"expiresAt,omitempty"`
	IsActive      bool       `json:"isActive"`
	CreatedAt     time.Time  `json:"createdAt"`
}

type ServiceZone struct {
	Name   string  `json:"name"`
	Lat    float64 `json:"lat"`
	Lng    float64 `json:"lng"`
	Radius float64 `json:"radius"`
}
type APILog struct {
	ID              string      `json:"id"`
	Provider        string      `json:"provider"`
	Endpoint        string      `json:"endpoint"`
	RequestID       *string     `json:"requestId"`
	RequestPayload  interface{} `json:"requestPayload"`
	ResponsePayload interface{} `json:"responsePayload"`
	StatusCode      int         `json:"statusCode"`
	DurationMs      int         `json:"durationMs"`
	CreatedAt       time.Time   `json:"createdAt"`
}
