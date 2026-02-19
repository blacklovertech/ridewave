package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
	"ridewave/utils"
)

// IP-based Rate Limiter with automatic cleanup
type IPRateLimiter struct {
	ips map[string]*rate.Limiter
	mu  *sync.RWMutex
	r   rate.Limit
	b   int
}

func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	i := &IPRateLimiter{
		ips: make(map[string]*rate.Limiter),
		mu:  &sync.RWMutex{},
		r:   r,
		b:   b,
	}

	// Periodic cleanup to prevent memory leak
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			i.mu.Lock()
			i.ips = make(map[string]*rate.Limiter)
			i.mu.Unlock()
		}
	}()

	return i
}

func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter, exists := i.ips[ip]
	if !exists {
		limiter = rate.NewLimiter(i.r, i.b)
		i.ips[ip] = limiter
	}

	return limiter
}

var limiter = NewIPRateLimiter(5, 10) // 5 req/sec, burst of 10

// RateLimit enforces per-IP rate limiting
func RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		l := limiter.GetLimiter(ip)
		if !l.Allow() {
			utils.RespondError(c, http.StatusTooManyRequests, "Too many requests. Please slow down.", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

// TimeoutMiddleware prevents long-hanging requests (10s max)
func TimeoutMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)

		done := make(chan struct{})
		go func() {
			c.Next()
			close(done)
		}()

		select {
		case <-done:
			// Completed
		case <-ctx.Done():
			utils.RespondError(c, http.StatusGatewayTimeout, "Request timed out", nil)
			c.Abort()
		}
	}
}
