package security

import "sync"

// ReplayDetector flags telemetry whose timestamp is not strictly greater
// than the last one seen for that device. ESP32 timestamps come from
// millis() — a per-device monotonic counter since boot, not a wall clock —
// so comparisons must be against the device's own history, never
// against time.Now() on the gateway.
type ReplayDetector struct {
	mu       sync.Mutex
	lastSeen map[string]int64
}

func NewReplayDetector() *ReplayDetector {
	return &ReplayDetector{
		lastSeen: make(map[string]int64),
	}
}

// Check reports whether this timestamp is fresh (strictly greater than the
// last one recorded for deviceID). It always records the timestamp when
// fresh — a replayed or stale message never advances the device's state.
func (r *ReplayDetector) Check(deviceID string, timestamp int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	last, seen := r.lastSeen[deviceID]
	if seen && timestamp <= last {
		return false
	}

	r.lastSeen[deviceID] = timestamp
	return true
}
