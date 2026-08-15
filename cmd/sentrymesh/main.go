package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/RStephanH/sentrymesh-gateway/internal/dashboard"
	"github.com/RStephanH/sentrymesh-gateway/internal/security"
	"github.com/RStephanH/sentrymesh-gateway/internal/storage"
	"github.com/RStephanH/sentrymesh-gateway/internal/telemetry"
)

const (
	brokerAddr       = "tcp://localhost:1883"
	topic            = "sentrymesh/esp32-01/telemetry"
	clientID         = "sentrymesh-gateway"
	rateWindow       = 5 * time.Second
	rateMaxMessages  = 3
	floodWindow      = 5 * time.Second
	floodMaxMessages = 10
	dbPath           = "sentrymesh.db"
)

func main() {
	logFile, err := os.OpenFile("sentrymesh.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to open log file:", err)
		os.Exit(1)
	}
	defer logFile.Close()

	logger := log.New(logFile)
	logger.SetReportTimestamp(true)
	logger.SetTimeFormat("2006-01-02 15:04:05")

	validator := security.NewRangeValidator()
	replayDetector := security.NewReplayDetector()
	opsLimiter := security.NewSlidingWindowLimiter(rateWindow, rateMaxMessages)
	floodDetector := security.NewSlidingWindowLimiter(floodWindow, floodMaxMessages)

	store, err := storage.NewStore(dbPath)
	if err != nil {
		logger.Fatal("failed to open store", "err", err)
	}
	defer store.Close()

	events := make(chan dashboard.Event, 16) // buffered: handler never blocks on a busy UI

	opts := mqtt.NewClientOptions().AddBroker(brokerAddr).SetClientID(clientID)
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		logger.Fatal("failed to connect to broker", "err", token.Error())
	}
	logger.Info("connected to broker", "broker", brokerAddr)

	handler := func(c mqtt.Client, msg mqtt.Message) {
		var t telemetry.Telemetry
		if err := json.Unmarshal(msg.Payload(), &t); err != nil {
			logger.Error("failed to parse telemetry payload", "err", err, "raw", string(msg.Payload()))
			return
		}

		if err := validator.Validate(t); err != nil {
			logger.Warn("telemetry rejected: invalid payload", "device_id", t.DeviceID, "err", err)
			store.SaveAlert(t.DeviceID, "invalid_payload", err.Error())
			sendEvent(events, dashboard.InvalidPayloadEvent{
				DeviceID: t.DeviceID, Err: err.Error(), OccurredAt: time.Now(),
			})
			return
		}

		if !replayDetector.Check(t.DeviceID, t.Timestamp) {
			logger.Error("REPLAY ATTACK DETECTED", "device_id", t.DeviceID, "timestamp", t.Timestamp)
			store.SaveAlert(t.DeviceID, "replay", fmt.Sprintf("timestamp=%d", t.Timestamp))
			sendEvent(events, dashboard.ReplayEvent{
				DeviceID: t.DeviceID, Timestamp: t.Timestamp, OccurredAt: time.Now(),
			})
			return
		}

		if allowed, floodDetected := security.CheckRateLimits(opsLimiter, floodDetector, t.DeviceID); !allowed {
			logger.Warn("telemetry rejected: rate limit exceeded", "device_id", t.DeviceID)
			store.SaveAlert(t.DeviceID, "rate_limit", "")
			sendEvent(events, dashboard.RateLimitEvent{DeviceID: t.DeviceID, OccurredAt: time.Now()})

			if floodDetected {
				logger.Error("FLOOD ATTACK DETECTED", "device_id", t.DeviceID,
					"window", floodWindow, "threshold", floodMaxMessages)
				store.SaveAlert(t.DeviceID, "flood",
					fmt.Sprintf("window=%s threshold=%d", floodWindow, floodMaxMessages))
				sendEvent(events, dashboard.FloodEvent{
					DeviceID: t.DeviceID, Window: floodWindow, Threshold: floodMaxMessages,
					OccurredAt: time.Now(),
				})
			}
			return
		}

		logger.Info("telemetry received", "device_id", t.DeviceID, "timestamp", t.Timestamp,
			"temperature", t.Temperature, "humidity", t.Humidity)
		store.SaveTelemetry(t)
		sendEvent(events, dashboard.TelemetryEvent{
			DeviceID: t.DeviceID, Timestamp: t.Timestamp,
			Temperature: t.Temperature, Humidity: t.Humidity, ReceivedAt: time.Now(),
		})
	}

	if token := client.Subscribe(topic, 1, handler); token.Wait() && token.Error() != nil {
		logger.Fatal("failed to subscribe", "err", token.Error())
	}
	logger.Info("subscribed", "topic", topic)

	p := tea.NewProgram(dashboard.NewModel(events))
	if _, err := p.Run(); err != nil {
		logger.Error("dashboard exited with error", "err", err)
	}

	logger.Info("shutting down")
	client.Disconnect(250)
}

// sendEvent is non-blocking: if the dashboard is somehow behind and the
// buffer is full, we drop the UI event rather than stall MQTT ingestion.
// Persistence (store.SaveXxx above) is the source of truth regardless.
func sendEvent(events chan dashboard.Event, e dashboard.Event) {
	select {
	case events <- e:
	default:
	}
}
