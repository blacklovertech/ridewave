package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"ridewave/models"
)

type OlaMapsClient struct {
	ApiKey string
}

type OlaDirectionsResponse struct {
	Routes []struct {
		Legs []struct {
			Steps []struct {
				Geometry string `json:"geometry"`
			} `json:"steps"`
			Distance struct {
				Value int `json:"value"`
			} `json:"distance"`
			Duration struct {
				Value int `json:"value"`
			} `json:"duration"`
		} `json:"legs"`
		OverviewPolyline struct {
			Points string `json:"points"`
		} `json:"overview_polyline"`
	} `json:"routes"`
	Status string `json:"status"`
}

func NewOlaMapsClient() *OlaMapsClient {
	return &OlaMapsClient{
		ApiKey: os.Getenv("OLA_MAPS_API_KEY"),
	}
}

func (c *OlaMapsClient) GetDirections(origin, destination string) (string, int, int, string, error) {
	return c.GetDirectionsWithMode(origin, destination, "driving")
}

func (c *OlaMapsClient) GetDirectionsWithMode(origin, destination, mode string) (string, int, int, string, error) {
	if c.ApiKey == "" {
		return "", 0, 0, "", fmt.Errorf("OLA_MAPS_API_KEY is not set")
	}

	start := time.Now()
	url := fmt.Sprintf("https://api.olamaps.io/routing/v1/directions?origin=%s&destination=%s&mode=%s&api_key=%s", origin, destination, mode, c.ApiKey)

	resp, err := http.Get(url)
	if err != nil {
		return "", 0, 0, "", err
	}
	defer resp.Body.Close()

	// Capture Ola Request ID as the RouteID
	routeID := resp.Header.Get("X-Request-Id")

	bodyBytes, _ := io.ReadAll(resp.Body)
	duration := time.Since(start)

	if resp.StatusCode != 200 {
		LogExternalAPI(models.APILog{
			Provider:        "OlaMaps",
			Endpoint:        "/routing/v1/directions",
			RequestID:       &routeID,
			RequestPayload:  map[string]string{"origin": origin, "destination": destination, "mode": mode},
			ResponsePayload: string(bodyBytes),
			StatusCode:      resp.StatusCode,
			DurationMs:      int(duration.Milliseconds()),
		})
		return "", 0, 0, "", fmt.Errorf("ola maps api error: %s - %s", resp.Status, string(bodyBytes))
	}

	var result OlaDirectionsResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", 0, 0, "", err
	}

	// AUDIT LOGGING: Store the full response payload (including massive polyline) in the audit table
	LogExternalAPI(models.APILog{
		Provider:        "OlaMaps",
		Endpoint:        "/routing/v1/directions",
		RequestID:       &routeID,
		RequestPayload:  map[string]string{"origin": origin, "destination": destination, "mode": mode},
		ResponsePayload: result,
		StatusCode:      200,
		DurationMs:      int(duration.Milliseconds()),
	})

	if result.Status != "OK" || len(result.Routes) == 0 {
		return "", 0, 0, "", fmt.Errorf("no routes found or api error: %s", result.Status)
	}

	Route := result.Routes[0]
	if len(Route.Legs) > 0 {
		distance := Route.Legs[0].Distance.Value
		duration := Route.Legs[0].Duration.Value
		polyline := Route.OverviewPolyline.Points
		return polyline, distance, duration, routeID, nil
	}

	return "", 0, 0, "", fmt.Errorf("no legs found in route")
}

type OlaPlacesResponse struct {
	Predictions []struct {
		Description          string `json:"description"`
		PlaceID              string `json:"place_id"`
		Reference            string `json:"reference"`
		StructuredFormatting struct {
			MainText      string `json:"main_text"`
			SecondaryText string `json:"secondary_text"`
		} `json:"structured_formatting"`
	} `json:"predictions"`
	Status string `json:"status"`
}

func (c *OlaMapsClient) Autocomplete(input string) ([]map[string]string, error) {
	if c.ApiKey == "" {
		return nil, fmt.Errorf("OLA_MAPS_API_KEY is not set")
	}

	// URL encode input
	encodedInput := url.QueryEscape(input)
	url := fmt.Sprintf("https://api.olamaps.io/places/v1/autocomplete?input=%s&api_key=%s", encodedInput, c.ApiKey)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result OlaPlacesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Status != "OK" && result.Status != "ok" {
		return nil, fmt.Errorf("places api error: %s", result.Status)
	}

	var places []map[string]string
	for _, p := range result.Predictions {
		places = append(places, map[string]string{
			"description": p.Description,
			"place_id":    p.PlaceID,
			"main_text":   p.StructuredFormatting.MainText,
		})
	}
	return places, nil
}

type OlaGeocodeResponse struct {
	Results []struct {
		Geometry struct {
			Location struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			} `json:"location"`
		} `json:"geometry"`
		FormattedAddress string `json:"formatted_address"`
	} `json:"results"`
	Status string `json:"status"`
}

func (c *OlaMapsClient) Geocode(address string) (float64, float64, error) {
	if c.ApiKey == "" {
		return 0, 0, fmt.Errorf("OLA_MAPS_API_KEY is not set")
	}

	url := fmt.Sprintf("https://api.olamaps.io/places/v1/geocode?address=%s&api_key=%s", address, c.ApiKey)

	resp, err := http.Get(url)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	var result OlaGeocodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, 0, err
	}

	if result.Status != "OK" || len(result.Results) == 0 {
		return 0, 0, fmt.Errorf("geocode api error: %s", result.Status)
	}

	location := result.Results[0].Geometry.Location
	return location.Lat, location.Lng, nil
}

type OlaSnapResponse struct {
	Status        string `json:"status"`
	SnappedPoints []struct {
		Location struct {
			Lat float64 `json:"lat"`
			Lng float64 `json:"lng"`
		} `json:"location"`
	} `json:"snapped_points"`
}

func (c *OlaMapsClient) SnapToRoad(points string) (float64, float64, error) {
	if c.ApiKey == "" {
		return 0, 0, fmt.Errorf("OLA_MAPS_API_KEY is not set")
	}

	url := fmt.Sprintf("https://api.olamaps.io/routing/v1/snapToRoad?points=%s&api_key=%s", points, c.ApiKey)

	resp, err := http.Get(url)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	var result OlaSnapResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, 0, err
	}

	if result.Status != "SUCCESS" || len(result.SnappedPoints) == 0 {
		return 0, 0, fmt.Errorf("snap api error or no segments: %s", result.Status)
	}

	snap := result.SnappedPoints[0].Location
	return snap.Lat, snap.Lng, nil
}

type OlaNearbyResponse struct {
	Predictions []struct {
		Description string   `json:"description"`
		PlaceID     string   `json:"place_id"`
		Distance    int      `json:"distance_meters"`
		Types       []string `json:"types"`
	} `json:"predictions"`
	Status string `json:"status"`
}

func (c *OlaMapsClient) NearbySearch(lat, lng float64, types string, radius int) ([]map[string]interface{}, error) {
	if c.ApiKey == "" {
		return nil, fmt.Errorf("OLA_MAPS_API_KEY is not set")
	}

	url := fmt.Sprintf("https://api.olamaps.io/places/v1/nearbysearch?location=%f,%f&types=%s&radius=%d&api_key=%s", lat, lng, types, radius, c.ApiKey)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result OlaNearbyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Status != "ok" {
		return nil, fmt.Errorf("nearby api error: %s", result.Status)
	}

	var results []map[string]interface{}
	for _, p := range result.Predictions {
		results = append(results, map[string]interface{}{
			"description": p.Description,
			"place_id":    p.PlaceID,
			"distance":    p.Distance,
			"types":       p.Types,
		})
	}
	return results, nil
}

// ---------------------------------------------------------------------
// Reverse Geocoding
// ---------------------------------------------------------------------

type OlaReverseGeocodeResponse struct {
	Results []struct {
		FormattedAddress string `json:"formatted_address"`
		Geometry         struct {
			Location struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			} `json:"location"`
		} `json:"geometry"`
		PlaceID string   `json:"place_id"`
		Types   []string `json:"types"`
	} `json:"results"`
	Status string `json:"status"`
}

func (c *OlaMapsClient) ReverseGeocode(lat, lng float64) (string, error) {
	if c.ApiKey == "" {
		return "", fmt.Errorf("OLA_MAPS_API_KEY is not set")
	}

	url := fmt.Sprintf("https://api.olamaps.io/places/v1/reverse-geocode?latlng=%f,%f&api_key=%s", lat, lng, c.ApiKey)

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("reverse geocode api error: %s", resp.Status)
	}

	var result OlaReverseGeocodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.Status != "OK" && result.Status != "ok" || len(result.Results) == 0 {
		return "", fmt.Errorf("no address found: %s", result.Status)
	}

	return result.Results[0].FormattedAddress, nil
}

// ---------------------------------------------------------------------
// Place Details
// ---------------------------------------------------------------------

type OlaPlaceDetailsResponse struct {
	Result struct {
		FormattedAddress string `json:"formatted_address"`
		Geometry         struct {
			Location struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			} `json:"location"`
		} `json:"geometry"`
		Name    string   `json:"name"`
		PlaceID string   `json:"place_id"`
		Types   []string `json:"types"`
	} `json:"result"`
	Status string `json:"status"`
}

func (c *OlaMapsClient) GetPlaceDetails(placeID string) (map[string]interface{}, error) {
	if c.ApiKey == "" {
		return nil, fmt.Errorf("OLA_MAPS_API_KEY is not set")
	}

	url := fmt.Sprintf("https://api.olamaps.io/places/v1/details?place_id=%s&api_key=%s", placeID, c.ApiKey)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("place details api error: %s", resp.Status)
	}

	var result OlaPlaceDetailsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Status != "OK" && result.Status != "ok" {
		return nil, fmt.Errorf("place details api error: %s", result.Status)
	}

	details := map[string]interface{}{
		"name":              result.Result.Name,
		"formatted_address": result.Result.FormattedAddress,
		"lat":               result.Result.Geometry.Location.Lat,
		"lng":               result.Result.Geometry.Location.Lng,
		"place_id":          result.Result.PlaceID,
		"types":             result.Result.Types,
	}

	return details, nil
}

// ---------------------------------------------------------------------
// Distance Matrix
// ---------------------------------------------------------------------

type OlaDistanceMatrixResponse struct {
	DestinationAddresses []string `json:"destination_addresses"`
	OriginAddresses      []string `json:"origin_addresses"`
	Rows                 []struct {
		Elements []struct {
			Distance struct {
				Text  string `json:"text"`
				Value int    `json:"value"`
			} `json:"distance"`
			Duration struct {
				Text  string `json:"text"`
				Value int    `json:"value"`
			} `json:"duration"`
			Status string `json:"status"`
		} `json:"elements"`
	} `json:"rows"`
	Status    string `json:"status"`
	RequestID string `json:"request_id,omitempty"` // Captured from X-Request-Id header
}

// GetDistanceMatrix calculates distance and duration between multiple origins and destinations.
func (c *OlaMapsClient) GetDistanceMatrix(origins []string, destinations []string) (*OlaDistanceMatrixResponse, error) {
	if c.ApiKey == "" {
		return nil, fmt.Errorf("OLA_MAPS_API_KEY is not set")
	}

	// Helper to join with pipe
	join := func(s []string) string {
		res := ""
		for i, v := range s {
			if i > 0 {
				res += "|"
			}
			res += v
		}
		return res
	}

	originsStr := join(origins)
	destinationsStr := join(destinations)

	url := fmt.Sprintf("https://api.olamaps.io/routing/v1/distanceMatrix?origins=%s&destinations=%s&api_key=%s", originsStr, destinationsStr, c.ApiKey)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("distance matrix api error: %s - %s", resp.Status, string(bodyBytes))
	}

	var result OlaDistanceMatrixResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Status != "OK" && result.Status != "ok" && result.Status != "SUCCESS" {
		return nil, fmt.Errorf("distance matrix api error: %s", result.Status)
	}

	// Capture the Request ID
	result.RequestID = resp.Header.Get("X-Request-Id")

	return &result, nil
}

// ---------------------------------------------------------------------
// Geofencing
// ---------------------------------------------------------------------

type GeofenceCreateRequest struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"` // "circle" or "polygon"
	Radius      float64     `json:"radius,omitempty"`
	Coordinates [][]float64 `json:"coordinates"`
	Status      string      `json:"status"` // "active"
	ProjectId   string      `json:"projectId"`
}

type GeofenceResponse struct {
	GeofenceId string `json:"geofenceId"`
	Status     string `json:"status"`
	Message    string `json:"message"`
}

func (c *OlaMapsClient) CreateGeofence(req GeofenceCreateRequest) (*GeofenceResponse, error) {
	if c.ApiKey == "" {
		return nil, fmt.Errorf("OLA_MAPS_API_KEY is not set")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.olamaps.io/places/v1/geofence?api_key=%s", c.ApiKey)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create geofence api error: %s - %s", resp.Status, string(bodyBytes))
	}

	var result GeofenceResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *OlaMapsClient) UpdateGeofence(id string, req GeofenceCreateRequest) (*GeofenceResponse, error) {
	if c.ApiKey == "" {
		return nil, fmt.Errorf("OLA_MAPS_API_KEY is not set")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.olamaps.io/places/v1/geofence/%s?api_key=%s", id, c.ApiKey)
	reqHttp, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	reqHttp.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(reqHttp)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("update geofence api error: %s - %s", resp.Status, string(bodyBytes))
	}

	var result GeofenceResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

type GeofenceDetails struct {
	GeofenceId  string      `json:"geofenceId"`
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Coordinates [][]float64 `json:"coordinates"`
	Radius      float64     `json:"radius,omitempty"`
	Status      string      `json:"status"`
	ProjectId   string      `json:"projectId"`
}

func (c *OlaMapsClient) GetGeofence(id string) (*GeofenceDetails, error) {
	if c.ApiKey == "" {
		return nil, fmt.Errorf("OLA_MAPS_API_KEY is not set")
	}

	url := fmt.Sprintf("https://api.olamaps.io/places/v1/geofence/%s?api_key=%s", id, c.ApiKey)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("get geofence api error: %s", resp.Status)
	}

	var result GeofenceDetails
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *OlaMapsClient) DeleteGeofence(id string) error {
	if c.ApiKey == "" {
		return fmt.Errorf("OLA_MAPS_API_KEY is not set")
	}

	url := fmt.Sprintf("https://api.olamaps.io/places/v1/geofence/%s?api_key=%s", id, c.ApiKey)
	reqHttp, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	client := &http.Client{}
	resp, err := client.Do(reqHttp)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete geofence api error: %s - %s", resp.Status, string(bodyBytes))
	}
	return nil
}

type GeofenceListResponse struct {
	Geofences []GeofenceDetails `json:"geofences"`
	Total     int               `json:"total"`
}

func (c *OlaMapsClient) ListGeofences(projectId string, page, size int) (*GeofenceListResponse, error) {
	if c.ApiKey == "" {
		return nil, fmt.Errorf("OLA_MAPS_API_KEY is not set")
	}

	url := fmt.Sprintf("https://api.olamaps.io/places/v1/geofences?projectId=%s&page=%d&size=%d&api_key=%s", projectId, page, size, c.ApiKey)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("list geofences api error: %s", resp.Status)
	}

	var result GeofenceListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

type GeofenceStatusResponse struct {
	GeofenceId string `json:"geofenceId"`
	IsInside   bool   `json:"isInside"`
	Message    string `json:"message"`
}

func (c *OlaMapsClient) GetGeofenceStatus(id string, lat, lng float64) (*GeofenceStatusResponse, error) {
	if c.ApiKey == "" {
		return nil, fmt.Errorf("OLA_MAPS_API_KEY is not set")
	}

	url := fmt.Sprintf("https://api.olamaps.io/places/v1/geofence/status?geofenceId=%s&coordinates=%f,%f&api_key=%s", id, lat, lng, c.ApiKey)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("geofence status api error: %s", resp.Status)
	}

	var result GeofenceStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ---------------------------------------------------------------------
// Route Optimizer
// ---------------------------------------------------------------------

type RouteOptimizerResponse struct {
	Routes []struct {
		Legs []struct {
			Distance struct {
				Text  string  `json:"readable_distance"`
				Value float64 `json:"distance"` // Meters
			} `json:"distance_data"` // Custom unmarshal might be needed matching response
			Duration struct {
				Text  string  `json:"readable_duration"`
				Value float64 `json:"duration"` // Seconds
			} `json:"duration_data"`
			StartAddress string `json:"start_address"`
			EndAddress   string `json:"end_address"`
			Steps        []struct {
				Instructions string  `json:"instructions"`
				Distance     float64 `json:"distance"`
				Duration     float64 `json:"duration"`
			} `json:"steps"`
		} `json:"legs"`
		OverviewPolyline string `json:"overview_polyline"`
		WaypointOrder    []int  `json:"waypoint_order"`
		Bounds           string `json:"bounds"`
	} `json:"routes"`
	Status string `json:"status"`
}

func (c *OlaMapsClient) RouteOptimizer(locations string, source, destination string, roundTrip bool, mode string) (*RouteOptimizerResponse, error) {
	if c.ApiKey == "" {
		return nil, fmt.Errorf("OLA_MAPS_API_KEY is not set")
	}

	params := url.Values{}
	params.Add("locations", locations)
	params.Add("source", source)
	params.Add("destination", destination)
	params.Add("round_trip", strconv.FormatBool(roundTrip))
	params.Add("mode", mode)
	params.Add("api_key", c.ApiKey)

	reqUrl := "https://api.olamaps.io/routing/v1/routeOptimizer?" + params.Encode()

	resp, err := http.Post(reqUrl, "application/json", bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("route optimizer api error: %s - %s", resp.Status, string(bodyBytes))
	}

	var result RouteOptimizerResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ---------------------------------------------------------------------
// Fleet Planner
// ---------------------------------------------------------------------

type FleetPlannerResponse struct {
	Vehicles []struct {
		Vehicle struct {
			ID string `json:"id"`
		} `json:"vehicle"`
		Route struct {
			Legs []struct {
				StartLocation struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"start_location"`
				EndLocation struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"end_location"`
			} `json:"legs"`
			OverviewPolyline string `json:"overview_polyline"`
		} `json:"route"`
	} `json:"vehicles"`
	Unassigned []string `json:"spill_package_ids"`
}

func (c *OlaMapsClient) FleetPlanner(strategy string, inputJSON []byte) (*FleetPlannerResponse, error) {
	if c.ApiKey == "" {
		return nil, fmt.Errorf("OLA_MAPS_API_KEY is not set")
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add the file (input.json)
	part, err := writer.CreateFormFile("input", "input.json")
	if err != nil {
		return nil, err
	}
	part.Write(inputJSON)
	writer.Close()

	reqUrl := fmt.Sprintf("https://api.olamaps.io/routing/v1/fleetPlanner?strategy=%s&api_key=%s", strategy, c.ApiKey)
	req, err := http.NewRequest("POST", reqUrl, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fleet planner api error: %s - %s", resp.Status, string(bodyBytes))
	}

	var result FleetPlannerResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
