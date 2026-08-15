package telemetry

// Telemetry matches the JSON payload published by the ESP32 firmware.
type Telemetry struct {
	DeviceID    string  `json:"device_id"`
	Timestamp   int64   `json:"timestamp"`
	Temperature float64 `json:"temperature"`
	Humidity    float64 `json:"humidity"`
}
