package utils

import (
	"sync"
	"time"
)

// GlobalWaitGroup tracks all active background tasks (logging, notifications, etc.)
var GlobalWaitGroup sync.WaitGroup

// SafeGo runs a function in a background goroutine while tracking it for graceful shutdown.
func SafeGo(fn func()) {
	GlobalWaitGroup.Add(1)
	go func() {
		defer GlobalWaitGroup.Done()
		fn()
	}()
}

// WaitForBackgroundTasks blocks until all tracked background tasks are completed, or timeout.
func WaitForBackgroundTasks(timeout time.Duration) {
	c := make(chan struct{})
	go func() {
		defer close(c)
		GlobalWaitGroup.Wait()
	}()

	select {
	case <-c:
		Logger.Info("All background tasks completed successfully.")
	case <-time.After(timeout):
		Logger.Warn("Graceful shutdown timed out. Some background tasks may have been terminated.")
	}
}
