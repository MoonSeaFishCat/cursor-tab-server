package ratelimit

import (
	"sync"
	"time"
)

type window struct {
	started time.Time
	count   int
}

type Limiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	subject map[string]window
}

func New(limit int, duration time.Duration) *Limiter {
	return &Limiter{limit: limit, window: duration, subject: make(map[string]window)}
}

// Limit returns the current per-window allowance.
func (l *Limiter) Limit() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.limit
}

// SetLimit updates the per-window allowance at runtime.
func (l *Limiter) SetLimit(limit int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.limit = limit
}

func (l *Limiter) Allow(subject string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	started := now.UTC().Truncate(l.window)
	for key, value := range l.subject {
		if value.started.Before(started) {
			delete(l.subject, key)
		}
	}
	current := l.subject[subject]
	if !current.started.Equal(started) {
		current = window{started: started}
	}
	if current.count >= l.limit {
		return false
	}
	current.count++
	l.subject[subject] = current
	return true
}
