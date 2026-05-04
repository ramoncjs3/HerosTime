package state

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"oldbeggar-refactor/internal/config"
)

type mysqlStore struct {
	db         *sql.DB
	stateTable string
	eventTable string
}

func openMySQLStore(cfg config.MySQLConfig, legacyStateFile string, autoMigrate bool) (*mysqlStore, error) {
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)

	store := &mysqlStore{
		db:         db,
		stateTable: quoteIdentifier(cfg.StateTable),
		eventTable: quoteIdentifier(cfg.EventTable),
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if autoMigrate {
		if err := store.migrate(); err != nil {
			_ = db.Close()
			return nil, err
		}
		if err := store.importFileSnapshot(legacyStateFile); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	return store, nil
}

func (s *mysqlStore) migrate() error {
	stateSQL := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
  state_key varchar(96) NOT NULL,
  source varchar(32) NOT NULL,
  server_code varchar(32) NOT NULL,
  state_date date NOT NULL,
  status varchar(32) NOT NULL,
  items_json longtext NULL,
  updated_at datetime(6) NOT NULL,
  created_at datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (state_key),
  KEY idx_state_date (state_date),
  KEY idx_source_server (source, server_code),
  KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`, s.stateTable)
	if _, err := s.db.Exec(stateSQL); err != nil {
		return err
	}

	eventSQL := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
  id bigint unsigned NOT NULL AUTO_INCREMENT,
  state_key varchar(96) NOT NULL,
  source varchar(32) NOT NULL,
  server_code varchar(32) NOT NULL,
  state_date date NOT NULL,
  status varchar(32) NOT NULL,
  items_json longtext NULL,
  created_at datetime(6) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_state_date (state_date),
  KEY idx_source_server (source, server_code),
  KEY idx_status (status),
  KEY idx_state_key_created (state_key, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`, s.eventTable)
	_, err := s.db.Exec(eventSQL)
	return err
}

func (s *mysqlStore) importFileSnapshot(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	count, err := s.stateRowCount()
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	data, found, err := readFileData(path)
	if err != nil {
		return err
	}
	if !found || len(data.Servers) == 0 {
		return nil
	}

	now := time.Now()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	upsertSQL := fmt.Sprintf(`
INSERT INTO %s (state_key, source, server_code, state_date, status, items_json, updated_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  source = VALUES(source),
  server_code = VALUES(server_code),
  state_date = VALUES(state_date),
  status = VALUES(status),
  items_json = VALUES(items_json),
  updated_at = VALUES(updated_at)`, s.stateTable)
	insertEventSQL := fmt.Sprintf(`
INSERT INTO %s (state_key, source, server_code, state_date, status, items_json, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`, s.eventTable)

	for key, value := range data.Servers {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value.Date) == "" || strings.TrimSpace(value.Status) == "" {
			continue
		}
		itemsJSON, encodeErr := encodeItems(value.Items)
		if encodeErr != nil {
			err = encodeErr
			return err
		}
		source, server := splitStateKey(key)
		updatedAt := value.UpdatedAt
		if updatedAt.IsZero() {
			updatedAt = now
		}
		if _, err = tx.Exec(upsertSQL, key, source, server, value.Date, value.Status, nullableItems(itemsJSON), updatedAt, updatedAt); err != nil {
			return err
		}
		if _, err = tx.Exec(insertEventSQL, key, source, server, value.Date, value.Status, nullableItems(itemsJSON), updatedAt); err != nil {
			return err
		}
	}

	err = tx.Commit()
	return err
}

func (s *mysqlStore) stateRowCount() (int, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", s.stateTable)
	var count int
	if err := s.db.QueryRow(query).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *mysqlStore) Done(serverCode, date string) bool {
	status, ok := s.Status(serverCode, date)
	return ok && isTerminal(status)
}

func (s *mysqlStore) Status(serverCode, date string) (string, bool) {
	query := fmt.Sprintf("SELECT status FROM %s WHERE state_key = ? AND state_date = ? LIMIT 1", s.stateTable)
	var status string
	if err := s.db.QueryRow(query, serverCode, date).Scan(&status); err != nil {
		return "", false
	}
	if !isTerminal(status) {
		return "", false
	}
	return status, true
}

func (s *mysqlStore) Snapshot() map[string]ServerState {
	query := fmt.Sprintf("SELECT state_key, state_date, status, items_json, updated_at FROM %s ORDER BY state_date DESC, source, server_code", s.stateTable)
	rows, err := s.db.Query(query)
	if err != nil {
		return map[string]ServerState{}
	}
	defer rows.Close()

	out := map[string]ServerState{}
	for rows.Next() {
		var key string
		var dateRaw interface{}
		var status string
		var itemsRaw sql.NullString
		var updatedRaw interface{}
		if err := rows.Scan(&key, &dateRaw, &status, &itemsRaw, &updatedRaw); err != nil {
			continue
		}
		out[key] = ServerState{
			Date:      normalizeDate(scanTimeString(dateRaw)),
			Status:    status,
			Items:     decodeItems(itemsRaw.String),
			UpdatedAt: scanTime(updatedRaw),
		}
	}
	return out
}

func (s *mysqlStore) Events(limit int) []Event {
	if limit <= 0 {
		limit = 1000
	}
	query := fmt.Sprintf("SELECT id, state_key, state_date, status, items_json, created_at FROM %s ORDER BY created_at DESC, id DESC LIMIT ?", s.eventTable)
	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	events := []Event{}
	for rows.Next() {
		var event Event
		var dateRaw interface{}
		var itemsRaw sql.NullString
		var createdRaw interface{}
		if err := rows.Scan(&event.ID, &event.Key, &dateRaw, &event.Status, &itemsRaw, &createdRaw); err != nil {
			continue
		}
		event.Date = normalizeDate(scanTimeString(dateRaw))
		event.Items = decodeItems(itemsRaw.String)
		event.CreatedAt = scanTime(createdRaw)
		events = append(events, event)
	}
	return events
}

func (s *mysqlStore) Mark(serverCode, date, status string, items []string) error {
	now := time.Now()
	source, server := splitStateKey(serverCode)
	itemsJSON, err := encodeItems(items)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	upsertSQL := fmt.Sprintf(`
INSERT INTO %s (state_key, source, server_code, state_date, status, items_json, updated_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  source = VALUES(source),
  server_code = VALUES(server_code),
  state_date = VALUES(state_date),
  status = VALUES(status),
  items_json = VALUES(items_json),
  updated_at = VALUES(updated_at)`, s.stateTable)
	if _, err = tx.Exec(upsertSQL, serverCode, source, server, date, status, nullableItems(itemsJSON), now, now); err != nil {
		return err
	}

	insertEventSQL := fmt.Sprintf(`
INSERT INTO %s (state_key, source, server_code, state_date, status, items_json, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`, s.eventTable)
	if _, err = tx.Exec(insertEventSQL, serverCode, source, server, date, status, nullableItems(itemsJSON), now); err != nil {
		return err
	}

	err = tx.Commit()
	return err
}

func encodeItems(items []string) (string, error) {
	if len(items) == 0 {
		return "", nil
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func decodeItems(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	return items
}

func nullableItems(value string) interface{} {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func normalizeDate(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= len("2006-01-02") {
		return value[:len("2006-01-02")]
	}
	return value
}

func parseMySQLTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	layouts := []string{
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
		time.RFC3339Nano,
		time.RFC3339,
	}
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func scanTimeString(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case time.Time:
		return v.Format("2006-01-02 15:04:05.999999")
	case []byte:
		return string(v)
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

func scanTime(value interface{}) time.Time {
	switch v := value.(type) {
	case nil:
		return time.Time{}
	case time.Time:
		return v
	case []byte:
		return parseMySQLTime(string(v))
	case string:
		return parseMySQLTime(v)
	default:
		return parseMySQLTime(fmt.Sprint(v))
	}
}

func quoteIdentifier(value string) string {
	return "`" + strings.ReplaceAll(value, "`", "``") + "`"
}
