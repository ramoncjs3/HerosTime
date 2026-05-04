package state

import (
	"path/filepath"
	"testing"

	"oldbeggar-refactor/internal/config"
)

func TestFileStoreMarkSnapshotAndDone(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{
			StateFile: filepath.Join(t.TempDir(), "state.json"),
		},
		Storage: config.StorageConfig{Type: "json"},
	}
	store, err := Open(cfg)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if store.Done("notice_shop:g1", "2026-05-02") {
		t.Fatalf("Done()=true before mark")
	}
	if err := store.Mark("notice_shop:g1", "2026-05-02", "watch_notified", []string{"iron-sword"}); err != nil {
		t.Fatalf("mark: %v", err)
	}
	if !store.Done("notice_shop:g1", "2026-05-02") {
		t.Fatalf("Done()=false after mark")
	}
	status, ok := store.Status("notice_shop:g1", "2026-05-02")
	if !ok || status != "watch_notified" {
		t.Fatalf("Status()=(%q,%v), want watch_notified,true", status, ok)
	}
	snapshot := store.Snapshot()
	got := snapshot["notice_shop:g1"]
	if got.Date != "2026-05-02" || got.Status != "watch_notified" || len(got.Items) != 1 || got.Items[0] != "iron-sword" {
		t.Fatalf("Snapshot()=%+v", got)
	}
	events := store.Events(10)
	if len(events) != 1 || events[0].Key != "notice_shop:g1" || events[0].Status != "watch_notified" {
		t.Fatalf("Events()=%+v", events)
	}
}

func TestSplitStateKey(t *testing.T) {
	source, server := splitStateKey("notice_shop:h10")
	if source != "notice_shop" || server != "h10" {
		t.Fatalf("splitStateKey()=(%q,%q)", source, server)
	}
	source, server = splitStateKey("g1")
	if source != "oldbeggar" || server != "g1" {
		t.Fatalf("legacy splitStateKey()=(%q,%q)", source, server)
	}
}

func TestEncodeDecodeItems(t *testing.T) {
	raw, err := encodeItems([]string{"iron-sword", "treasure-map"})
	if err != nil {
		t.Fatalf("encodeItems: %v", err)
	}
	items := decodeItems(raw)
	if len(items) != 2 || items[0] != "iron-sword" || items[1] != "treasure-map" {
		t.Fatalf("decodeItems()=%v", items)
	}
}
