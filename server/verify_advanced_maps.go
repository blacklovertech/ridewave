package main

import (
	"fmt"
	"os"
	"ridewave/utils"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		fmt.Println("Warning: Error loading .env file")
	}

	// Ensure API Key is set
	if os.Getenv("OLA_MAPS_API_KEY") == "" {
		fmt.Println("Error: OLA_MAPS_API_KEY is not set. Please set it in your environment or .env file.")
		return
	}

	fmt.Println("Starting Advanced Ola Maps Verification...")
	client := utils.NewOlaMapsClient()

	// 1. Geofencing
	fmt.Println("\n--- Testing Geofence ---")
	projectId := "verify-project-1"

	// Create
	req := utils.GeofenceCreateRequest{
		Name:        "Test Geofence " + time.Now().Format(time.RFC3339),
		Type:        "circle",
		Radius:      150,
		Coordinates: [][]float64{{12.931, 77.615}},
		Status:      "active",
		ProjectId:   projectId,
	}

	var geofenceID string

	createResp, err := client.CreateGeofence(req)
	if err != nil {
		fmt.Printf("Create Geofence Failed: %v\n", err)
	} else {
		fmt.Printf("Geofence Created: ID=%s, Status=%s\n", createResp.GeofenceId, createResp.Status)
		geofenceID = createResp.GeofenceId

		// Get
		getResp, err := client.GetGeofence(geofenceID)
		if err != nil {
			fmt.Printf("Get Geofence Failed: %v\n", err)
		} else {
			fmt.Printf("Geofence Details: Name=%s, Type=%s\n", getResp.Name, getResp.Type)
		}

		// Status Check
		statusResp, err := client.GetGeofenceStatus(geofenceID, 12.931, 77.615)
		if err != nil {
			fmt.Printf("Geofence Status Check Failed: %v\n", err)
		} else {
			fmt.Printf("Geofence Status: Inside=%v\n", statusResp.IsInside)
		}
	}

	// 2. Route Optimizer
	fmt.Println("\n--- Testing Route Optimizer ---")
	locations := "12.938399,77.632873|12.938041,77.628285|12.931,77.615"
	optResp, err := client.RouteOptimizer(locations, "first", "last", false, "driving")
	if err != nil {
		fmt.Printf("Route Optimizer Failed: %v\n", err)
	} else {
		if len(optResp.Routes) > 0 {
			fmt.Printf("Optimized Route Found: %s\n", optResp.Routes[0].OverviewPolyline[:10]+"...")
			fmt.Printf("Waypoint Order: %v\n", optResp.Routes[0].WaypointOrder)
		} else {
			fmt.Println("Route Optimizer returned 0 routes.")
		}
	}

	// 3. Fleet Planner
	fmt.Println("\n--- Testing Fleet Planner ---")
	// Using inferred schema from API docs description:
	// "Required package fields: id, weightInGrams, and at least one location. Vehicle must contain id and capacityInKG."
	inputJSON := `{
		"vehicles": [
			{
				"id": "v1",
				"capacityInKG": 50
			}
		],
		"packages": [
			{
				"id": "p1",
				"weightInGrams": 5000,
				"location": { "lat": 12.935, "lng": 77.615 }
			}
		]
	}`

	fleetResp, err := client.FleetPlanner("optimal", []byte(inputJSON))
	if err != nil {
		fmt.Printf("Fleet Planner Failed: %v\n", err)
	} else {
		fmt.Printf("Fleet Planner Success: Vehicles Assigned=%d\n", len(fleetResp.Vehicles))
		fmt.Printf("Unassigned Packages: %v\n", fleetResp.Unassigned)
	}

	// Clean up Geofence at the end if created
	if geofenceID != "" {
		fmt.Println("\n--- Cleaning up Geofence ---")
		err = client.DeleteGeofence(geofenceID)
		if err != nil {
			fmt.Printf("Delete Geofence Failed: %v\n", err)
		} else {
			fmt.Printf("Geofence Deleted Successfully\n")
		}
	}
}
