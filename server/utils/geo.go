package utils

import (
	"strconv"
	"strings"
	"math"
)

func ParseLatLng(latLngStr string) (float64, float64) {
	parts := strings.Split(latLngStr, ",")
	if len(parts) != 2 {
		return 0, 0
	}
	lat, _ := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	lon, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	return lat, lon
}

// CalculateDistance returns the distance between two points in KM (Haversine formula)
func CalculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371 // KM

	dLat := (lat2 - lat1) * (math.Pi / 180.0)
	dLon := (lon2 - lon1) * (math.Pi / 180.0)

	lat1Rad := lat1 * (math.Pi / 180.0)
	lat2Rad := lat2 * (math.Pi / 180.0)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Sin(dLon/2)*math.Sin(dLon/2)*math.Cos(lat1Rad)*math.Cos(lat2Rad)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}
