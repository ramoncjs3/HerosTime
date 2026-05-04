package state

import (
	"fmt"
	"time"

	"oldbeggar-refactor/internal/config"
)

type Store struct {
	backend backend
}

type backend interface {
	Done(serverCode, date string) bool
	Status(serverCode, date string) (string, bool)
	Snapshot() map[string]ServerState
	Events(limit int) []Event
	Mark(serverCode, date, status string, items []string) error
}

type ServerState struct {
	Date      string    `json:"date"`
	Status    string    `json:"status"`
	Items     []string  `json:"items,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Event struct {
	ID        int64     `json:"id,omitempty"`
	Key       string    `json:"key"`
	Date      string    `json:"date"`
	Status    string    `json:"status"`
	Items     []string  `json:"items,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func Open(cfg *config.Config) (*Store, error) {
	switch cfg.Storage.Type {
	case "mysql":
		autoMigrate := true
		if cfg.Storage.AutoMigrate != nil {
			autoMigrate = *cfg.Storage.AutoMigrate
		}
		backend, err := openMySQLStore(cfg.Storage.MySQL, cfg.App.StateFile, autoMigrate)
		if err != nil {
			return nil, err
		}
		return &Store{backend: backend}, nil
	case "json", "file", "":
		backend, err := openFileStore(cfg.App.StateFile)
		if err != nil {
			return nil, err
		}
		return &Store{backend: backend}, nil
	default:
		return nil, fmt.Errorf("unsupported state store %q", cfg.Storage.Type)
	}
}

func (s *Store) Done(serverCode, date string) bool {
	return s.backend.Done(serverCode, date)
}

func (s *Store) Status(serverCode, date string) (string, bool) {
	return s.backend.Status(serverCode, date)
}

func (s *Store) Snapshot() map[string]ServerState {
	return s.backend.Snapshot()
}

func (s *Store) Events(limit int) []Event {
	return s.backend.Events(limit)
}

func (s *Store) Mark(serverCode, date, status string, items []string) error {
	return s.backend.Mark(serverCode, date, status, items)
}

func isTerminal(status string) bool {
	switch status {
	case "notified", "watch_notified", "event_over", "dry_run":
		return true
	default:
		return false
	}
}

func splitStateKey(key string) (string, string) {
	for i, r := range key {
		if r == ':' {
			return key[:i], key[i+1:]
		}
	}
	return "oldbeggar", key
}
