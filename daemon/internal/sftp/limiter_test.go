package sftp

import (
	"testing"
	"time"
)

func TestFailureLimiterBlocksAfterRepeatedFailures(t *testing.T) {
	now := time.Unix(1000, 0)
	limiter := newFailureLimiter(3, time.Minute, 10*time.Minute)
	limiter.now = func() time.Time { return now }

	if limiter.fail("1.2.3.4") || limiter.fail("1.2.3.4") {
		t.Fatal("blocked before reaching the failure limit")
	}
	if !limiter.fail("1.2.3.4") {
		t.Fatal("third failure did not block the host")
	}
	if !limiter.blocked("1.2.3.4") {
		t.Fatal("host is not reported as blocked")
	}
	if limiter.blocked("5.6.7.8") {
		t.Fatal("an unrelated host was blocked")
	}

	now = now.Add(11 * time.Minute)
	if limiter.blocked("1.2.3.4") {
		t.Fatal("block did not expire")
	}
}

func TestFailureLimiterResetsOnSuccessAndWindowExpiry(t *testing.T) {
	now := time.Unix(1000, 0)
	limiter := newFailureLimiter(2, time.Minute, 10*time.Minute)
	limiter.now = func() time.Time { return now }

	limiter.fail("1.2.3.4")
	limiter.succeed("1.2.3.4")
	if limiter.fail("1.2.3.4") {
		t.Fatal("a success did not reset the failure count")
	}

	now = now.Add(2 * time.Minute)
	if limiter.fail("1.2.3.4") {
		t.Fatal("failures outside the window were still counted")
	}
}
