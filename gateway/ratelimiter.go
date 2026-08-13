package main

import (
	"sync"
	"time"
)

type RateLimiter interface {
	Allow(deviceID string) bool
}

// SlidingWindowLimiter allows at most maxEvents per deviceID within window.
// Safe for concurrent use — the MQTT handler runs each message in its own
// goroutine, so history must be protected by a mutex.
type SlidingWindowLimiter struct {
	mu        sync.Mutex
	window    time.Duration
	maxEvents int
	history   map[string][]time.Time
	now       func() time.Time // injectable clock, defaults to time.Now
}

func NewSlidingWindowLimiter(window time.Duration, maxEvents int) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		window:    window,
		maxEvents: maxEvents,
		history:   make(map[string][]time.Time),
		now:       time.Now,
	}
}

func (l *SlidingWindowLimiter) Allow(deviceID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	cutoff := now.Add(-l.window)

	var kept []time.Time
	for _, ts := range l.history[deviceID] {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}

	if len(kept) >= l.maxEvents {
		l.history[deviceID] = kept
		return false
	}

	l.history[deviceID] = append(kept, now)
	return true
}

// checkRateLimits evaluates a device's message against both the operational
// rate limiter and the flood detector. The flood detector always sees the
// message (to reflect true throughput), regardless of the ops limiter's
// decision — see main.go for why this order matters.
//
// allowed reports whether normal processing should continue.
// floodDetected reports whether the message also breached the (more
// permissive) flood threshold — a distinct, higher-severity signal.
func checkRateLimits(ops, flood RateLimiter, deviceID string) (allowed, floodDetected bool) {
	floodOK := flood.Allow(deviceID)
	opsOK := ops.Allow(deviceID)

	if !opsOK {
		return false, !floodOK
	}
	return true, false
}
