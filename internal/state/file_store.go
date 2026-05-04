package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type fileStore struct {
	path string
	mu   sync.Mutex
	data fileData
}

type fileData struct {
	Servers map[string]ServerState `json:"servers"`
}

func openFileStore(path string) (*fileStore, error) {
	store := &fileStore{
		path: path,
		data: fileData{Servers: map[string]ServerState{}},
	}
	data, _, err := readFileData(path)
	if err != nil {
		return nil, err
	}
	store.data = data
	return store, nil
}

func readFileData(path string) (fileData, bool, error) {
	data := fileData{Servers: map[string]ServerState{}}
	if path == "" {
		return data, false, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return data, false, nil
		}
		return data, false, err
	}
	if len(raw) == 0 {
		return data, true, nil
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return data, true, err
	}
	if data.Servers == nil {
		data.Servers = map[string]ServerState{}
	}
	return data, true, nil
}

func (s *fileStore) Done(serverCode, date string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.data.Servers[serverCode]
	return ok && current.Date == date && isTerminal(current.Status)
}

func (s *fileStore) Status(serverCode, date string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.data.Servers[serverCode]
	if !ok || current.Date != date || !isTerminal(current.Status) {
		return "", false
	}
	return current.Status, true
}

func (s *fileStore) Snapshot() map[string]ServerState {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]ServerState, len(s.data.Servers))
	for key, value := range s.data.Servers {
		value.Items = append([]string(nil), value.Items...)
		out[key] = value
	}
	return out
}

func (s *fileStore) Events(limit int) []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	events := make([]Event, 0, len(s.data.Servers))
	for key, value := range s.data.Servers {
		events = append(events, Event{
			Key:       key,
			Date:      value.Date,
			Status:    value.Status,
			Items:     append([]string(nil), value.Items...),
			CreatedAt: value.UpdatedAt,
		})
	}
	if limit > 0 && len(events) > limit {
		events = events[:limit]
	}
	return events
}

func (s *fileStore) Mark(serverCode, date, status string, items []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Servers[serverCode] = ServerState{
		Date:      date,
		Status:    status,
		Items:     append([]string(nil), items...),
		UpdatedAt: time.Now(),
	}
	return s.saveLocked()
}

func (s *fileStore) saveLocked() error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
