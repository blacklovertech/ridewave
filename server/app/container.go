package app

import (
	"context"
	"log"
	"ridewave/config"
	"ridewave/db"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// App container holds all application state and resources
type App struct {
	Config config.Config
	DB     *pgxpool.Pool
	Redis  *redis.Client
}

// Instance is the global singleton for the app container
var Instance *App

// Initialize bootstraps the app with all dependencies
func Initialize() {
	// 1. Load & Validate Config
	config.LoadAndValidate()

	// 2. Database Connection
	db.Connect() // Still uses the package-level connect for now, but we'll link it
	
	// 3. Redis Connection
	db.InitRedis()

	// 4. Create Container
	Instance = &App{
		Config: config.Envs,
		DB:     db.Pool,
		Redis:  db.RedisClient,
	}

	log.Println("✅ RideWave App Container initialized successfully")
}

// Close gracefully shuts down all resources
func (a *App) Close() {
	if a.DB != nil {
		a.DB.Close()
	}
	if a.Redis != nil {
		a.Redis.Close()
	}
}
