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
