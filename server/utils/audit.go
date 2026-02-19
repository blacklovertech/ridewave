package utils

import (
	"context"
	"encoding/json"
	"ridewave/db"
	"ridewave/models"

	"go.uber.org/zap"
)

// LogExternalAPI records a request/response pair from an external provider (like Ola Maps)
// for auditing and to keep the main rides table lightweight.
func LogExternalAPI(log models.APILog) {
	// Background execution using SafeGo to track for graceful shutdown
	SafeGo(func() {
		reqJSON, _ := json.Marshal(log.RequestPayload)
		respJSON, _ := json.Marshal(log.ResponsePayload)

		_, err := db.Pool.Exec(context.Background(),
			`INSERT INTO external_api_logs (
				id, provider, endpoint, "requestId", "requestPayload", "responsePayload", "statusCode", "durationMs", "createdAt"
			) VALUES (
				gen_random_uuid()::text, $1, $2, $3, $4, $5, $6, $7, NOW()
			)`,
			log.Provider, log.Endpoint, log.RequestID, reqJSON, respJSON, log.StatusCode, log.DurationMs,
		)

		if err != nil {
			Logger.Error("Failed to log external API call", zap.Error(err))
		}
	})
}
