package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"ridewave/db"
	"ridewave/handlers"
	"ridewave/middleware"
	"ridewave/socket"
	"ridewave/utils"
)

var serverStartTime time.Time

func main() {
	serverStartTime = time.Now()

	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	utils.InitLogger()
	utils.Logger.Info("Starting RideWave Server...")

	// Connect to DB
	db.Connect()
	defer db.Close()

	// Auto-migrate tables
	db.Migrate()
	db.InitRedis()

	// Context for background services (cancellation)
	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()

	// Start Phase 2 background services
	utils.StartRetentionWorker(bgCtx)

	// Use release mode in production
	if os.Getenv("GIN_MODE") == "release" || os.Getenv("NODE_ENV") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize Socket.IO server (compatible with socket.io-client v4)
	io := socket.InitSocketIO()

	r := gin.Default()
	r.SetTrustedProxies(nil)

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, x-api-key")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Security Middleware
	r.Use(middleware.RequestID())
	r.Use(middleware.SecureHeaders())
	r.Use(middleware.RateLimit())
	r.Use(middleware.TimeoutMiddleware())
	r.Use(middleware.MaxBodySize(10 * 1024 * 1024)) // 10MB limit

	// API Key Authentication (Global)
	r.Use(middleware.APIKeyAuth())

	// Health Check
	r.GET("/health", func(c *gin.Context) {
		dbStatus := "connected"
		dbLatency := "N/A"
		start := time.Now()
		err := db.Pool.Ping(context.Background())
		if err != nil {
			dbStatus = fmt.Sprintf("error: %v", err)
		} else {
			dbLatency = fmt.Sprintf("%dms", time.Since(start).Milliseconds())
		}

		redisStatus := "connected"
		redisLatency := "N/A"
		if db.RedisClient != nil {
			start = time.Now()
			_, err = db.RedisClient.Ping(context.Background()).Result()
			if err != nil {
				redisStatus = fmt.Sprintf("error: %v", err)
			} else {
				redisLatency = fmt.Sprintf("%dms", time.Since(start).Milliseconds())
			}
		}

		uptime := time.Since(serverStartTime)
		uptimeStr := fmt.Sprintf("%dd %dh %dm %ds",
			int(uptime.Hours())/24, int(uptime.Hours())%24, int(uptime.Minutes())%60, int(uptime.Seconds())%60)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"status":  "healthy",
			"server": gin.H{
				"version":   "2.5.0",
				"goVersion": runtime.Version(),
				"uptime":    uptimeStr,
				"startedAt": serverStartTime.Format(time.RFC3339),
			},
			"database": gin.H{"status": dbStatus, "latency": dbLatency},
			"redis":    gin.H{"status": redisStatus, "latency": redisLatency},
		})
	})

	// Load Routes (Modular registration with middleware injection)
	handlers.RegisterUserRoutes(r, middleware.IsAuthenticated())
	handlers.RegisterDriverRoutes(r, middleware.IsAuthenticatedDriver())
	handlers.RegisterAdminRoutes(r, middleware.IsAdmin())

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	// Mount Socket.IO on /socket.io/ and Gin HTTP routes on everything else
	mux := http.NewServeMux()
	mux.Handle("/socket.io/", socket.GetHandler(io))
	mux.Handle("/", r)

	// Graceful Shutdown
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	log.Printf("🚀 RideWave Server v2.5 running on port %s", port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// 1. Cancel background workers
	bgCancel()

	// 2. Shutdown HTTP server (stop accepting new requests)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	// 3. Wait for tracked background tasks (SafeGo) to complete
	log.Println("Waiting for background tasks to drain...")
	utils.WaitForBackgroundTasks(5 * time.Second)

	log.Println("Server exiting")
}
