package main

import (
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()

	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory store: %v", err)
	}
	t.Cleanup(func() {
		store.Close()
	})
	return store
}

func TestStore_SaveTelemetry(t *testing.T) {
	store := newTestStore(t)

	tel := Telemetry{
		DeviceID:    "esp32-01",
		Timestamp:   26694,
		Temperature: 24.5,
		Humidity:    55.0,
	}

	if err := store.SaveTelemetry(tel); err != nil {
		t.Fatalf("SaveTelemetry failed: %v", err)
	}

	var deviceID string
	var timestamp int64
	var temperature, humidity float64

	row := store.db.QueryRow(
		`SELECT device_id, timestamp, temperature, humidity FROM telemetry WHERE device_id = ?`,
		"esp32-01",
	)
	if err := row.Scan(&deviceID, &timestamp, &temperature, &humidity); err != nil {
		t.Fatalf("failed to read back inserted row: %v", err)
	}

	if deviceID != tel.DeviceID {
		t.Errorf("device_id = %q, want %q", deviceID, tel.DeviceID)
	}
	if timestamp != tel.Timestamp {
		t.Errorf("timestamp = %d, want %d", timestamp, tel.Timestamp)
	}
	if temperature != tel.Temperature {
		t.Errorf("temperature = %f, want %f", temperature, tel.Temperature)
	}
	if humidity != tel.Humidity {
		t.Errorf("humidity = %f, want %f", humidity, tel.Humidity)
	}
}

func TestStore_SaveAlert(t *testing.T) {
	store := newTestStore(t)

	if err := store.SaveAlert("esp32-01", "replay", "timestamp=10173"); err != nil {
		t.Fatalf("SaveAlert failed: %v", err)
	}

	var deviceID, alertType, details string

	row := store.db.QueryRow(
		`SELECT device_id, type, details FROM alerts WHERE device_id = ?`,
		"esp32-01",
	)
	if err := row.Scan(&deviceID, &alertType, &details); err != nil {
		t.Fatalf("failed to read back inserted row: %v", err)
	}

	if deviceID != "esp32-01" {
		t.Errorf("device_id = %q, want %q", deviceID, "esp32-01")
	}
	if alertType != "replay" {
		t.Errorf("type = %q, want %q", alertType, "replay")
	}
	if details != "timestamp=10173" {
		t.Errorf("details = %q, want %q", details, "timestamp=10173")
	}
}

func TestStore_MultipleAlertsAreAllPersisted(t *testing.T) {
	store := newTestStore(t)

	store.SaveAlert("esp32-01", "replay", "")
	store.SaveAlert("esp32-01", "flood", "")
	store.SaveAlert("esp32-02", "rate_limit", "")

	var count int
	row := store.db.QueryRow(`SELECT COUNT(*) FROM alerts`)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("failed to count alerts: %v", err)
	}
	if count != 3 {
		t.Errorf("alert count = %d, want 3", count)
	}
}
