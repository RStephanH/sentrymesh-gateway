package main

import (
	"testing"
	"time"
)

func TestSlidingWindowLimiter_AllowsUpToMax(t *testing.T) {
	fakeNow := time.Now() // point de départ fixe, on avancera manuellement

	limiter := NewSlidingWindowLimiter(5*time.Second, 3)
	limiter.now = func() time.Time { return fakeNow } // horloge figée

	if !limiter.Allow("esp32-01") {
		t.Fatal("1st message should be allowed")
	}
	if !limiter.Allow("esp32-01") {
		t.Fatal("2nd message should be allowed")
	}
	if !limiter.Allow("esp32-01") {
		t.Fatal("3rd message should be allowed")
	}
	if limiter.Allow("esp32-01") {
		t.Fatal("4th message within window should be REJECTED")
	}
}

func TestSlidingWindowLimiter_ResetsAfterWindow(t *testing.T) {
	fakeNow := time.Now()

	limiter := NewSlidingWindowLimiter(5*time.Second, 3)
	limiter.now = func() time.Time { return fakeNow }

	limiter.Allow("esp32-01")
	limiter.Allow("esp32-01")
	limiter.Allow("esp32-01")
	if limiter.Allow("esp32-01") {
		t.Fatal("4th message within window should be REJECTED")
	}

	// Advance the fake clock past the window.
	fakeNow = fakeNow.Add(6 * time.Second)

	if !limiter.Allow("esp32-01") {
		t.Fatal("message after window expiry should be ALLOWED")
	}
}

func TestSlidingWindowLimiter_PerDeviceIsolation(t *testing.T) {
	fakeNow := time.Now()

	limiter := NewSlidingWindowLimiter(5*time.Second, 1)
	limiter.now = func() time.Time { return fakeNow }

	if !limiter.Allow("esp32-01") {
		t.Fatal("esp32-01 first message should be allowed")
	}
	if !limiter.Allow("esp32-02") {
		t.Fatal("esp32-02 should have its own independent counter")
	}
}

func TestCheckRateLimits_CascadeToFloodAlert(t *testing.T) {
	fakeNow := time.Now()

	// Real gateway config: flood threshold is intentionally higher than
	// ops, so ops always rejects first — flood is a stricter escalation,
	// never an isolated signal.
	ops := NewSlidingWindowLimiter(5*time.Second, 3)
	ops.now = func() time.Time { return fakeNow }

	flood := NewSlidingWindowLimiter(5*time.Second, 10)
	flood.now = func() time.Time { return fakeNow }

	// Messages 1-3: under both thresholds.
	for i := 1; i <= 3; i++ {
		allowed, floodDetected := checkRateLimits(ops, flood, "esp32-01")
		if !allowed {
			t.Fatalf("message %d: expected allowed=true", i)
		}
		if floodDetected {
			t.Fatalf("message %d: expected floodDetected=false", i)
		}
	}

	// Messages 4-10: ops rejects (>3), but flood's own count (4..10)
	// hasn't yet reached its threshold of 10 — no flood alert yet.
	for i := 4; i <= 10; i++ {
		allowed, floodDetected := checkRateLimits(ops, flood, "esp32-01")
		if allowed {
			t.Fatalf("message %d: expected allowed=false (ops threshold breached)", i)
		}
		if floodDetected {
			t.Fatalf("message %d: expected floodDetected=false (flood count=%d, threshold=10)", i, i)
		}
	}

	// Message 11: flood's count has now reached 10 on the PREVIOUS call,
	// so this 11th call is the first one flood itself rejects too.
	allowed, floodDetected := checkRateLimits(ops, flood, "esp32-01")
	if allowed {
		t.Fatal("message 11: expected allowed=false")
	}
	if !floodDetected {
		t.Fatal("message 11: expected floodDetected=true (flood threshold now breached)")
	}
}
