package utils

import (
	"context"
	"ridewave/db"
	"time"

	"go.uber.org/zap"
)

// StartRetentionWorker runs a background process that cleans up old audit logs.
// Default policy: Delete logs older than 30 days every 24 hours.
func StartRetentionWorker(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				runCleanup()
			case <-ctx.Done():
				Logger.Info("Retention Worker shutting down...")
				return
			}
		}
	}()
}

func runCleanup() {
	Logger.Info("Running Audit Log Retention Cleanup...")
	
	// Retention period: 30 days
	cutoff := time.Now().AddDate(0, 0, -30)

	result, err := db.Pool.Exec(context.Background(),
		`DELETE FROM external_api_logs WHERE "createdAt" < $1`, cutoff)
	
	if err != nil {
		Logger.Error("Audit Log Cleanup Failed", zap.Error(err))
		return
	}

	rowsAffected := result.RowsAffected()
	Logger.Info("Audit Log Cleanup Completed", zap.Int64("deletedRows", rowsAffected))
}
