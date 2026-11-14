package middleware

import (
	"testing"
	"time"
)

func TestRateLimiterBlocksAfterLimitAndRecovers(t *testing.T) {
	now := time.Unix(1000, 0)
	limiter := NewRateLimiter(2, time.Minute)
	limiter.nowFunc = func() time.Time { return now }

	for attempt := 1; attempt <= 2; attempt++ {
		if allowed, _ := limiter.Allow("1.2.3.4"); !allowed {
			t.Fatalf("attempt %d was blocked", attempt)
		}
	}
	allowed, retryAfter := limiter.Allow("1.2.3.4")
	if allowed || retryAfter <= 0 {
		t.Fatalf("third attempt allowed=%t retryAfter=%s", allowed, retryAfter)
	}
	if allowed, _ := limiter.Allow("5.6.7.8"); !allowed {
		t.Fatal("an unrelated client was blocked")
	}

	now = now.Add(2 * time.Minute)
	if allowed, _ := limiter.Allow("1.2.3.4"); !allowed {
		t.Fatal("limit did not reset after the window")
	}
}
