package socket

import (
	"context"
	"encoding/json"
	"net/http"
	"ridewave/utils"

	socketio "github.com/zishang520/socket.io/v2/socket"
	"github.com/zishang520/engine.io/v2/types"
	"go.uber.org/zap"

	"ridewave/stores"
)



// InitSocketIO creates and returns a Socket.IO server
func InitSocketIO() *socketio.Server {
	opts := &socketio.ServerOptions{}
	opts.SetCors(&types.Cors{
		Origin: "*",
	})

	io := socketio.NewServer(nil, opts)

	io.On("connection", func(clients ...any) {
		socket := clients[0].(*socketio.Socket)
		utils.Logger.Info("A user connected", zap.String("socketID", string(socket.Id())))

		// locationUpdate — driver sends their GPS position
		socket.On("locationUpdate", func(args ...any) {
			if len(args) == 0 {
				return
			}
			data, ok := args[0].(map[string]any)
			if !ok {
				return
			}

			role, _ := data["role"].(string)
			driverId, _ := data["driverId"].(string)

			if role == "driver" && driverId != "" {
				lat, _ := data["latitude"].(float64)
				lon, _ := data["longitude"].(float64)

				// Update via Redis Store
				err := stores.UpdateDriverLocation(driverId, lat, lon, string(socket.Id()))
				if err != nil {
					utils.Logger.Error("Error updating driver location", zap.Error(err))
				}

				// Join driver to their own room for targeted dispatch
				socket.Join(socketio.Room("driver:" + driverId))

				// If driver is in a ride, broadcast to the user
				userId, _ := data["userId"].(string)
				if userId != "" {
					io.To(socketio.Room(userId)).Emit("rideUpdate", data)
				}
				
				// utils.Logger.Debug("Updated driver location", zap.String("driverId", driverId))
			}
		})

		// userJoin - User joins a room with their ID to receive personal updates
		socket.On("joinUserRoom", func(args ...any) {
			if len(args) == 0 { return }
			data, ok := args[0].(map[string]any)
			if !ok { return }
			userId, _ := data["userId"].(string)
			if userId != "" {
				socket.Join(socketio.Room(userId))
				utils.Logger.Info("User joined room", zap.String("userId", userId))
			}
		})
		
		// startRide - Driver/User signals ride start
		socket.On("startRide", func(args ...any) {
             // Logic to notify parties that ride started
             // In future: cache ride state in Redis
		})

		// requestRide — user requests nearby drivers
		socket.On("requestRide", func(args ...any) {
			if len(args) == 0 {
				return
			}
			data, ok := args[0].(map[string]any)
			if !ok {
				return
			}

			role, _ := data["role"].(string)
			userId, _ := data["userId"].(string)

			if role == "user" {
				lat, _ := data["latitude"].(float64)
				lon, _ := data["longitude"].(float64)

				utils.Logger.Info("Ride requested by user", zap.String("userId", userId))

				// Find nearby drivers using Redis
				drivers, err := stores.GetNearbyDrivers(lat, lon, 5.0)
				if err != nil {
					utils.Logger.Error("Error finding nearby drivers", zap.Error(err))
				}

				// Map to response format
				var nearby []map[string]any
				for _, d := range drivers {
					nearby = append(nearby, map[string]any{
						"id":        d.DriverID,
						"latitude":  d.Latitude,
						"longitude": d.Longitude,
						"socketId":  d.SocketID,
					})
				}

				socket.Emit("nearbyDrivers", map[string]any{
					"drivers": nearby,
				})
			}
		})

		// disconnect — remove driver from active list
		socket.On("disconnect", func(args ...any) {
			utils.Logger.Info("User disconnected", zap.String("socketID", string(socket.Id())))
			// In Redis architecture, determining WHICH driver disconnected by socketID is harder 
			// without a reverse index (SocketID -> DriverID).
			// For now, we rely on TTL or we could add a map/store method for this.
			// Ideally: When connecting, map SocketID -> DriverID in Redis.
		})
	})

	// Subscribe to Redis Ride Requests for dispatching
	go func() {
		ctx := context.Background()
		pubsub := stores.SubscribeToRideRequests(ctx)
		defer pubsub.Close()

		ch := pubsub.Channel()
		for msg := range ch {
			var event stores.RideRequestEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				utils.Logger.Error("Error unmarshalling ride request", zap.Error(err))
				continue
			}

			// Find nearby drivers
			drivers, err := stores.GetNearbyDrivers(event.PickupLat, event.PickupLon, 5.0) // 5km radius
			if err != nil {
				utils.Logger.Error("Error finding drivers for dispatch", zap.Error(err))
				continue
			}

			utils.Logger.Info("Dispatching ride", zap.String("rideId", event.RideID), zap.Int("driverCount", len(drivers)))

			for _, d := range drivers {
				// Emit to specific driver socket
				// Note: io.To(socketId) works if using default adapter or redis adapter
				// Since we are using redis adapter implicitly via go-socket.io redis store (if configured) or just local
				// For now, assuming single instance or sticky sessions where socketID is valid.
				// If multiple instances, we need the Redis Adapter for Socket.IO to broadcast accessing other nodes.
				// But we are using our custom Redis Pub/Sub, so we only know SocketID. 
				// Problem: SocketID might be on another server.
				// Solution for "Better Go Server": We need to broadcast to "driver:<driverId>" room.
				// We should have joined drivers to their own rooms on connection.
				
				// Let's assume we joined drivers to "driver:<driverId>" on connection (we will add this).
				io.To(socketio.Room("driver:" + d.DriverID)).Emit("newRide", event)
			}
		}
	}()

	return io
}

// GetHandler returns the HTTP handler for Socket.IO
func GetHandler(io *socketio.Server) http.Handler {
	return io.ServeHandler(nil)
}
