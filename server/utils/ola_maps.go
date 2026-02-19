package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
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
	
	bodyBytes, _ := ioutil.ReadAll(resp.Body)
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
		Description string `json:"description"`
		PlaceID     string `json:"place_id"`
		Reference   string `json:"reference"`
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

	url := fmt.Sprintf("https://api.olamaps.io/places/v1/autocomplete?input=%s&api_key=%s", input, c.ApiKey)
	
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result OlaPlacesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Status != "OK" {
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
		Description string `json:"description"`
		PlaceID     string `json:"place_id"`
		Distance    int    `json:"distance_meters"`
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
			"distance":     p.Distance,
			"types":        p.Types,
		})
	}
	return results, nil
}
