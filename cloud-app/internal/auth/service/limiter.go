package service

import (
	"sync"
	"time"
)

type attempt struct {
	n     int
	until time.Time
}
type Limiter struct {
	mu      sync.Mutex
	entries map[string]attempt
}

func NewLimiter() *Limiter { return &Limiter{entries: map[string]attempt{}} }
func (l *Limiter) Allow(key string, budget int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if len(l.entries) > 10000 {
		for k, v := range l.entries {
			if now.After(v.until) {
				delete(l.entries, k)
			}
		}
		if len(l.entries) > 10000 {
			return false
		}
	}
	v := l.entries[key]
	if now.After(v.until) {
		v = attempt{until: now.Add(window)}
	}
	if v.n >= budget {
		return false
	}
	v.n++
	l.entries[key] = v
	return true
}
