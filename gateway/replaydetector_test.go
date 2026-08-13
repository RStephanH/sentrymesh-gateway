package main

import "testing"

func TestReplayDetector_FirstMessageAlwaysAllowed(t *testing.T) {
	rd := NewReplayDetector()

	if !rd.Check("esp32-01", 100) {
		t.Fatal("first message for a device should always be allowed")
	}
}

func TestReplayDetector_StrictlyIncreasingIsAllowed(t *testing.T) {
	rd := NewReplayDetector()

	rd.Check("esp32-01", 100)
	if !rd.Check("esp32-01", 150) {
		t.Fatal("a later, greater timestamp should be allowed")
	}
	if !rd.Check("esp32-01", 200) {
		t.Fatal("a later, greater timestamp should be allowed")
	}
}

func TestReplayDetector_ExactReplayIsRejected(t *testing.T) {
	rd := NewReplayDetector()

	rd.Check("esp32-01", 100)
	if rd.Check("esp32-01", 100) {
		t.Fatal("replaying the exact same timestamp should be rejected")
	}
}

func TestReplayDetector_OlderTimestampIsRejected(t *testing.T) {
	rd := NewReplayDetector()

	rd.Check("esp32-01", 200)
	if rd.Check("esp32-01", 100) {
		t.Fatal("a timestamp older than the last seen one should be rejected")
	}
}

func TestReplayDetector_RejectedTimestampDoesNotAdvanceState(t *testing.T) {
	rd := NewReplayDetector()

	rd.Check("esp32-01", 200)
	rd.Check("esp32-01", 100) // rejected, must not overwrite lastSeen=200

	if !rd.Check("esp32-01", 201) {
		t.Fatal("a message greater than the true last-seen (200) should still be allowed")
	}
}

func TestReplayDetector_PerDeviceIsolation(t *testing.T) {
	rd := NewReplayDetector()

	rd.Check("esp32-01", 500)

	// A different device starting at a low timestamp must not be
	// penalized by esp32-01's history.
	if !rd.Check("esp32-02", 10) {
		t.Fatal("esp32-02 should have its own independent timestamp history")
	}
}
