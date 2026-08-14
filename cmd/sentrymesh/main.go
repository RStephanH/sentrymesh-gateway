package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/RStephanH/sentrymesh-gateway/internal/security"
	"github.com/RStephanH/sentrymesh-gateway/internal/storage"
	"github.com/RStephanH/sentrymesh-gateway/internal/telemetry"
)

const (
	brokerAddr      = "tcp://localhost:1883"
	topic           = "sentrymesh/esp32-01/telemetry"
	clientID        = "sentrymesh-gateway"
	rateWindow      = 5 * time.Second
	rateMaxMessages = 3

	floodWindow      = 5 * time.Second
	floodMaxMessages = 10

	dbPath = "sentrymesh.db"
)

func main() {
	logger := log.New(os.Stderr)
	validator := security.NewRangeValidator()
	replayDetector := security.NewReplayDetector()
	opsLimiter := security.NewSlidingWindowLimiter(rateWindow, rateMaxMessages)
	floodDetector := security.NewSlidingWindowLimiter(floodWindow, floodMaxMessages)

	store, err := storage.NewStore(dbPath)
	if err != nil {
		logger.Fatal("failed to open store", "err", err)
	}
	defer store.Close()

	opts := mqtt.NewClientOptions().
		AddBroker(brokerAddr).
		SetClientID(clientID)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		logger.Fatal("failed to connect to broker", "err", token.Error())
	}
	logger.Info("connected to broker", "broker", brokerAddr)

	handler := func(c mqtt.Client, msg mqtt.Message) {
		var t telemetry.Telemetry
		if err := json.Unmarshal(msg.Payload(), &t); err != nil {
			logger.Error("failed to parse telemetry payload",
				"err", err, "raw", string(msg.Payload()))
			return
		}

		if err := validator.Validate(t); err != nil {
			logger.Warn("telemetry rejected: invalid payload",
				"device_id", t.DeviceID, "err", err)
			if saveErr := store.SaveAlert(t.DeviceID, "invalid_payload", err.Error()); saveErr != nil {
				logger.Error("failed to save alert", "err", saveErr)
			}
			return
		}

		if !replayDetector.Check(t.DeviceID, t.Timestamp) {
			logger.Error("REPLAY ATTACK DETECTED",
				"device_id", t.DeviceID, "timestamp", t.Timestamp)
			if saveErr := store.SaveAlert(t.DeviceID, "replay",
				fmt.Sprintf("timestamp=%d", t.Timestamp)); saveErr != nil {
				logger.Error("failed to save alert", "err", saveErr)
			}
			return
		}

		if allowed, floodDetected := security.CheckRateLimits(opsLimiter, floodDetector, t.DeviceID); !allowed {
			logger.Warn("telemetry rejected: rate limit exceeded", "device_id", t.DeviceID)
			if saveErr := store.SaveAlert(t.DeviceID, "rate_limit", ""); saveErr != nil {
				logger.Error("failed to save alert", "err", saveErr)
			}
			if floodDetected {
				logger.Error("FLOOD ATTACK DETECTED",
					"device_id", t.DeviceID,
					"window", floodWindow, "threshold", floodMaxMessages)
				if saveErr := store.SaveAlert(t.DeviceID, "flood",
					fmt.Sprintf("window=%s threshold=%d", floodWindow, floodMaxMessages)); saveErr != nil {
					logger.Error("failed to save alert", "err", saveErr)
				}
			}
			return
		}

		logger.Info("telemetry received",
			"device_id", t.DeviceID, "timestamp", t.Timestamp,
			"temperature", t.Temperature, "humidity", t.Humidity)
		if saveErr := store.SaveTelemetry(t); saveErr != nil {
			logger.Error("failed to save telemetry", "err", saveErr)
		}
	}

	if token := client.Subscribe(topic, 1, handler); token.Wait() && token.Error() != nil {
		logger.Fatal("failed to subscribe", "err", token.Error())
	}
	logger.Info("subscribed", "topic", topic)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	logger.Info("shutting down")
	client.Disconnect(250)
}
