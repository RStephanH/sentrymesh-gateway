package security

import (
	"fmt"

	"github.com/RStephanH/sentrymesh-gateway/internal/telemetry"
)

type Validator interface {
	Validate(t telemetry.Telemetry) error
}

// RangeValidator checks that telemetry fields fall within plausible bounds
// for a DHT22 sensor. It holds no state — each call is independent.
type RangeValidator struct{}

func NewRangeValidator() *RangeValidator {
	return &RangeValidator{}
}

func (v *RangeValidator) Validate(t telemetry.Telemetry) error {
	if t.DeviceID == "" {
		return fmt.Errorf("device_id is empty")
	}
	if t.Timestamp <= 0 {
		return fmt.Errorf("invalid timestamp: %d", t.Timestamp)
	}
	if t.Temperature < -40 || t.Temperature > 80 {
		return fmt.Errorf("temperature out of range: %.1f", t.Temperature)
	}
	if t.Humidity < 0 || t.Humidity > 100 {
		return fmt.Errorf("humidity out of range: %.1f", t.Humidity)
	}
	return nil
}
