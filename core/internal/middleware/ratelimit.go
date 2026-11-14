package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter is a fixed-window limiter keyed by client address.
type RateLimiter struct {
	mu      sync.Mutex
	hits    map[string]*rateWindow
	limit   int
	window  time.Duration
	nowFunc func() time.Time
}

type rateWindow struct {
	count int
	start time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		hits:    make(map[string]*rateWindow),
		limit:   limit,
		window:  window,
		nowFunc: time.Now,
	}
}

// Allow records one attempt and reports whether it is within the limit, along
// with how long the caller should wait when it is not.
func (l *RateLimiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.nowFunc()
	if len(l.hits) > 10000 {
		for existing, window := range l.hits {
			if now.Sub(window.start) > l.window {
				delete(l.hits, existing)
			}
		}
	}

	window, ok := l.hits[key]
	if !ok || now.Sub(window.start) > l.window {
		l.hits[key] = &rateWindow{count: 1, start: now}
		return true, 0
	}

	window.count++
	if window.count > l.limit {
		return false, l.window - now.Sub(window.start)
	}
	return true, 0
}

// Middleware limits requests per client IP.
func (l *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		allowed, retryAfter := l.Allow(c.ClientIP())
		if !allowed {
			c.Header("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many attempts. Please wait a few minutes and try again.",
			})
			return
		}
		c.Next()
	}
}
