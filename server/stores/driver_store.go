package stores

import (
	"context"
	"encoding/json"
	"ridewave/db"
	"time"

	"github.com/redis/go-redis/v9"
)

type DriverLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	SocketID  string  `json:"socketId"`
	DriverID  string  `json:"driverId"`
}

const (
	DriverGeoKey        = "drivers:geo"
	DriverDataKeyPrefix = "drivers:data:"
	RouteCacheKeyPrefix = "routes:cache:"
)

type CachedRoute struct {
	Polyline          string  `json:"polyline"`
	Distance          int     `json:"distance"`
	Duration          int     `json:"duration"`
	Fare              float64 `json:"fare"`
	VehicleType       string  `json:"vehicleType"`
	OriginName        string  `json:"originName"`
	DestinationName   string  `json:"destinationName"`
	OriginLat         float64 `json:"originLat"`
	OriginLng         float64 `json:"originLng"`
	DestinationLat    float64 `json:"destinationLat"`
	DestinationLng    float64 `json:"destinationLng"`
}

func StorePlannedRoute(routeID string, route CachedRoute) error {
	ctx := context.Background()
	val, err := json.Marshal(route)
	if err != nil {
		return err
	}
	// Cache for 15 minutes (standard booking window)
	return db.RedisClient.Set(ctx, RouteCacheKeyPrefix+routeID, val, 15*time.Minute).Err()
}

func GetPlannedRoute(routeID string) (*CachedRoute, error) {
	ctx := context.Background()
	val, err := db.RedisClient.Get(ctx, RouteCacheKeyPrefix+routeID).Result()
	if err != nil {
		return nil, err
	}

	var route CachedRoute
	if err := json.Unmarshal([]byte(val), &route); err != nil {
		return nil, err
	}
	return &route, nil
}

func UpdateDriverLocation(driverID string, lat, lon float64, socketID string) error {
	ctx := context.Background()

	// Add to Geo index
	err := db.RedisClient.GeoAdd(ctx, DriverGeoKey, &redis.GeoLocation{
		Name:      driverID,
		Longitude: lon,
		Latitude:  lat,
	}).Err()
	if err != nil {
		return err
	}

	// Store additional metadata
	data := DriverLocation{
		Latitude:  lat,
		Longitude: lon,
		SocketID:  socketID,
		DriverID:  driverID,
	}
	val, _ := json.Marshal(data)

	// Set with TTL (e.g., 1 hour to auto-expire stale sessions)
	return db.RedisClient.Set(ctx, DriverDataKeyPrefix+driverID, val, time.Hour).Err()
}

func RemoveDriver(driverID string) error {
	ctx := context.Background()
	db.RedisClient.ZRem(ctx, DriverGeoKey, driverID)
	return db.RedisClient.Del(ctx, DriverDataKeyPrefix+driverID).Err()
}

func GetNearbyDrivers(lat, lon, radiusKm float64) ([]DriverLocation, error) {
	ctx := context.Background()

	// Find drivers within radius
	locs, err := db.RedisClient.GeoRadius(ctx, DriverGeoKey, lon, lat, &redis.GeoRadiusQuery{
		Radius:      radiusKm,
		Unit:        "km",
		WithCoord:   true,
		WithDist:    true,
		Sort:        "ASC",
	}).Result()

	if err != nil {
		return nil, err
	}

	var drivers []DriverLocation
	for _, loc := range locs {
		// Fetch metadata (socket ID)
		val, err := db.RedisClient.Get(ctx, DriverDataKeyPrefix+loc.Name).Result()
		if err == nil {
			var d DriverLocation
			if json.Unmarshal([]byte(val), &d) == nil {
				d.Latitude = loc.Latitude
				d.Longitude = loc.Longitude
				drivers = append(drivers, d)
			}
		}
	}
	return drivers, nil
}

const RideRequestChannel = "ride_requests"

type RideRequestEvent struct {
	RideID      string  `json:"rideId"`
	UserID      string  `json:"userId"`
	PickupLat   float64 `json:"pickupLat"`
	PickupLon   float64 `json:"pickupLon"`
	Destination string  `json:"destination"`
	Fare        float64 `json:"fare"`
	Distance    int     `json:"distance"`
	Duration    int     `json:"duration"`
}

func PublishRideRequest(ctx context.Context, event RideRequestEvent) error {
	val, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return db.RedisClient.Publish(ctx, RideRequestChannel, val).Err()
}

func SubscribeToRideRequests(ctx context.Context) *redis.PubSub {
	return db.RedisClient.Subscribe(ctx, RideRequestChannel)
}
