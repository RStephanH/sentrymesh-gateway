# SentryMesh Gateway

A simulated IoT security gateway: intrusion detection (replay, flood)
across a fleet of simulated devices (ESP32 + DHT22 via Wokwi), backed
by a Go gateway for validation and anomaly detection.

## Structure
- `firmware/` — embedded ESP32 code (simulated on Wokwi)
- `gateway/` — Go service (auth, rate-limiting, anomaly detection)
- `docs/` — architecture and documentation

## Status
🚧 Work in progress
