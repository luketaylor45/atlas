package sftp

import (
	"net"
	"sync"
	"time"
)

// failureLimiter blocks an address after repeated failed logins. SFTP is the
// one daemon port that must be public, so it is constantly probed.
type failureLimiter struct {
	mu          sync.Mutex
	entries     map[string]*failureEntry
	maxFailures int
	window      time.Duration
	blockFor    time.Duration
	now         func() time.Time
}

type failureEntry struct {
	failures     int
	windowStart  time.Time
	blockedUntil time.Time
}

func newFailureLimiter(maxFailures int, window, blockFor time.Duration) *failureLimiter {
	return &failureLimiter{
		entries:     make(map[string]*failureEntry),
		maxFailures: maxFailures,
		window:      window,
		blockFor:    blockFor,
		now:         time.Now,
	}
}

func hostOf(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}

func (l *failureLimiter) blocked(host string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.entries[host]
	return ok && l.now().Before(entry.blockedUntil)
}

// fail records a failed attempt and reports whether the host is now blocked.
func (l *failureLimiter) fail(host string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.prune(now)

	entry, ok := l.entries[host]
	if !ok || now.Sub(entry.windowStart) > l.window {
		entry = &failureEntry{windowStart: now}
		l.entries[host] = entry
	}
	entry.failures++
	if entry.failures >= l.maxFailures {
		entry.blockedUntil = now.Add(l.blockFor)
		return true
	}
	return false
}

func (l *failureLimiter) succeed(host string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, host)
}

// prune drops expired entries so the map cannot grow without bound. The caller holds the lock.
func (l *failureLimiter) prune(now time.Time) {
	if len(l.entries) < 4096 {
		return
	}
	for host, entry := range l.entries {
		if now.After(entry.blockedUntil) && now.Sub(entry.windowStart) > l.window {
			delete(l.entries, host)
		}
	}
}
