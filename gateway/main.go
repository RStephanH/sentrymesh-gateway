package main

import (
	"encoding/json"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Telemetry matches the JSON payload published by the ESP32 firmware.
type Telemetry struct {
	DeviceID    string  `json:"device_id"`
	Timestamp   int64   `json:"timestamp"`
	Temperature float64 `json:"temperature"`
	Humidity    float64 `json:"humidity"`
}

const (
	brokerAddr     = "tcp://localhost:1883"
	topic          = "sentrymesh/esp32-01/telemetry"
	ClientID       = "sentrymesh-gateway"
	rateWindow     = 5 * time.Second
	rateMaxMessage = 3

	floodWindow      = 5 * time.Second
	floodMaxMessages = 10
)

func main() {
	logger := log.New(os.Stderr)
	validator := NewRangeValidator()
	opsLimiter := NewSlidingWindowLimiter(rateWindow, rateMaxMessage)
	floodDetector := NewSlidingWindowLimiter(floodWindow, floodMaxMessages)

	opts := mqtt.NewClientOptions().
		AddBroker(brokerAddr).
		SetClientID(ClientID)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		logger.Fatal("failed to connect to broker", "error", token.Error())
	}
	logger.Info("connected to broker", "broker", brokerAddr)

	handler := func(c mqtt.Client, msg mqtt.Message) {
		var t Telemetry
		if err := json.Unmarshal(msg.Payload(), &t); err != nil {
			logger.Error(
				"failed to parse telemetry payload", "error", err,
				"raw", string(msg.Payload()),
			)
			return
		}

		if err := validator.Validate(t); err != nil {
			logger.Warn("telemetry rejected: invalid payload",
				"device_id", t.DeviceID, "error", err)
			return
		}

		if allowed, floodDetected := checkRateLimits(opsLimiter, floodDetector, t.DeviceID); !allowed {
			logger.Warn("telemetry rejected: rate limit exceeded", "device_id", t.DeviceID)
			if floodDetected {
				logger.Error(
					"FLOOD ATTACK DETECTED",
					"device_id", t.DeviceID,
					"window", floodWindow,
					"threshold", floodMaxMessages,
				)
			}
			return
		}

		logger.Info(
			"telemetry received",
			"device_id", t.DeviceID,
			"timestamp", t.Timestamp,
			"temperature", t.Temperature,
			"humidity", t.Humidity,
		)
	}

	if token := client.Subscribe(topic, 1, handler); token.Wait() && token.Error() != nil {
		logger.Fatal("failed to subscribe", "error", token.Error())
	}

	logger.Info("subscribe", "topic", topic)

	// Keep the process alive until interrupted.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	logger.Info("shutting down")
	client.Disconnect(250)
}
