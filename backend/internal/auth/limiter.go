package auth

import (
	"sync"
	"time"
)

// Limiter blocks a key (e.g. "ip|email") after too many failed logins within a window.
type Limiter struct {
	Max    int
	Window time.Duration
	Now    func() time.Time

	mu    sync.Mutex
	fails map[string][]time.Time
}

func NewLimiter(max int, window time.Duration) *Limiter {
	return &Limiter{Max: max, Window: window, Now: time.Now, fails: map[string][]time.Time{}}
}

// recent drops expired failures. Caller holds mu.
func (l *Limiter) recent(key string) []time.Time {
	cutoff := l.Now().Add(-l.Window)
	kept := l.fails[key][:0]
	for _, t := range l.fails[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.fails, key)
		return nil
	}
	l.fails[key] = kept
	return kept
}

// Allowed reports whether key may try again, and if not, how long to wait.
func (l *Limiter) Allowed(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	r := l.recent(key)
	if len(r) < l.Max {
		return true, 0
	}
	return false, r[0].Add(l.Window).Sub(l.Now())
}

func (l *Limiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails[key] = append(l.recent(key), l.Now())
}

func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
}
