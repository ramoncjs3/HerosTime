package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type entry struct {
	Status string   `json:"status"`
	Items  []string `json:"items,omitempty"`
}

// Store persists per-day push status keyed by (source:serverCode, date).
type Store struct {
	mu       sync.Mutex
	filePath string
	data     map[string]map[string]entry // key -> date -> entry
}

// Open loads an existing state file or creates a new empty store.
func Open(filePath string) (*Store, error) {
	s := &Store{
		filePath: filePath,
		data:     make(map[string]map[string]entry),
	}
	raw, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		return nil, err
	}
	return s, nil
}

// Done reports whether any status entry exists for key on date.
func (s *Store) Done(key, date string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.data[key][date]
	return ok
}

// Status returns the recorded status for key on date, and whether it exists.
func (s *Store) Status(key, date string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.data[key][date]
	if !ok {
		return "", false
	}
	return e.Status, true
}

// Mark records status and items for key on date, then flushes to disk.
func (s *Store) Mark(key, date, status string, items []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data[key] == nil {
		s.data[key] = make(map[string]entry)
	}
	s.data[key][date] = entry{Status: status, Items: items}
	return s.flush()
}

// flush writes the current state to disk atomically via a temp file.
func (s *Store) flush() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.filePath)
}
