package utils

import (
	"go.uber.org/zap"
)

var Logger *zap.Logger

func InitLogger() {
	var err error
	// Use production config for JSON logs, or development for colored console logs
	// For "Better Go Server", structured logging (JSON) is preferred for aggregation tools (ELK, Datadog)
	config := zap.NewProductionConfig()
	config.OutputPaths = []string{"stdout"}
	Logger, err = config.Build()
	if err != nil {
		panic(err)
	}
	defer Logger.Sync()
}
