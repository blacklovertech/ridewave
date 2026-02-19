package config

import (
	"log"
	"os"
)

// Config holds all validated environment variables
type Config struct {
	Port         string
	DBURL        string
	RedisURL     string
	OlaMapsKey   string
	TwilioSID    string
	TwilioAuth   string
	TwilioVerify string
	AdminSecret  string
	JWTSecret    string
}

// Global instance
var Envs Config

// LoadAndValidate ensures all required ENV keys are present
func LoadAndValidate() {
	Envs = Config{
		Port:         getReq("PORT"),
		DBURL:        getReq("DATABASE_URL"),
		RedisURL:     getReq("REDIS_URL"),
		OlaMapsKey:   getReq("OLA_MAPS_API_KEY"),
		TwilioSID:    getReq("TWILIO_ACCOUNT_SID"),
		TwilioAuth:   getReq("TWILIO_AUTH_TOKEN"),
		TwilioVerify: getReq("TWILIO_VERIFY_SERVICE_SID"),
		AdminSecret:  getReq("ADMIN_SECRET"),
		JWTSecret:    getReq("JWT_SECRET"),
	}
}

func getReq(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("❌ FATAL: Environment variable %s is required but missing", key)
	}
	return val
}
