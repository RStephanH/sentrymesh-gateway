package dashboard

import "time"

// Event is a marker interface — only types defined in this file can be
// sent on the events channel between main's MQTT handler and the
// dashboard's bubbletea model.
type Event interface {
	isEvent()
}

type TelemetryEvent struct {
	DeviceID    string
	Timestamp   int64
	Temperature float64
	Humidity    float64
	ReceivedAt  time.Time
}

func (TelemetryEvent) isEvent() {}

type InvalidPayloadEvent struct {
	DeviceID   string
	Err        string
	OccurredAt time.Time
}

func (InvalidPayloadEvent) isEvent() {}

type ReplayEvent struct {
	DeviceID   string
	Timestamp  int64
	OccurredAt time.Time
}

func (ReplayEvent) isEvent() {}

type RateLimitEvent struct {
	DeviceID   string
	OccurredAt time.Time
}

func (RateLimitEvent) isEvent() {}

type FloodEvent struct {
	DeviceID   string
	Window     time.Duration
	Threshold  int
	OccurredAt time.Time
}

func (FloodEvent) isEvent() {}
