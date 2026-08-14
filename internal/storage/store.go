package storage

import (
	"database/sql"
	"fmt"

	"github.com/RStephanH/sentrymesh-gateway/internal/telemetry"
	_ "modernc.org/sqlite" // driver registration via side-effect import
)

type Store struct {
	db *sql.DB
}

func NewStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1) // avoid "database is locked" on concurrent writes

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	return &Store{db: db}, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS telemetry (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id   TEXT NOT NULL,
    timestamp   INTEGER NOT NULL,
    temperature REAL NOT NULL,
    humidity    REAL NOT NULL,
    received_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS alerts (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id   TEXT NOT NULL,
    type        TEXT NOT NULL,
    details     TEXT,
    occurred_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

func (s *Store) SaveTelemetry(t telemetry.Telemetry) error {
	_, err := s.db.Exec(
		`INSERT INTO telemetry (device_id, timestamp, temperature, humidity)
		 VALUES (?, ?, ?, ?)`,
		t.DeviceID, t.Timestamp, t.Temperature, t.Humidity,
	)
	return err
}

func (s *Store) SaveAlert(deviceID, alertType, details string) error {
	_, err := s.db.Exec(
		`INSERT INTO alerts (device_id, type, details) VALUES (?, ?, ?)`,
		deviceID, alertType, details,
	)
	return err
}

func (s *Store) Close() error {
	return s.db.Close()
}
