package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/segmentation-service/segmentation/internal/domain/model"
	"github.com/segmentation-service/segmentation/internal/infrastructure/store"
)

func writeConfig(t *testing.T, path string, snap *model.Snapshot) {
	t.Helper()
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func validSnapshot(version int) *model.Snapshot {
	return &model.Snapshot{
		Version: version,
		Layers: []model.Layer{
			{Name: "test", Order: 1, Segments: []model.Segment{}},
		},
	}
}

func TestWatcher_DetectsChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, validSnapshot(1))

	memStore := store.NewMemory()
	src := NewFileSource(path)
	w := NewWatcher(src, memStore, path, 50*time.Millisecond)
	w.Start()
	defer w.Stop()

	// Wait for initial detection
	time.Sleep(200 * time.Millisecond)

	if memStore.Get() == nil {
		t.Fatal("expected watcher to load initial config")
	}
	if memStore.Get().Version != 1 {
		t.Errorf("expected version 1, got %d", memStore.Get().Version)
	}

	// Update config
	writeConfig(t, path, validSnapshot(2))
	time.Sleep(200 * time.Millisecond)

	if memStore.Get().Version != 2 {
		t.Errorf("expected version 2 after change, got %d", memStore.Get().Version)
	}
}

func TestWatcher_StopTerminates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, validSnapshot(1))

	memStore := store.NewMemory()
	src := NewFileSource(path)
	w := NewWatcher(src, memStore, path, 50*time.Millisecond)
	w.Start()
	w.Stop()

	// Should not panic or hang — just return
	time.Sleep(100 * time.Millisecond)
}

func TestWatcher_IgnoresSameModTime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, validSnapshot(1))

	memStore := store.NewMemory()
	src := NewFileSource(path)
	w := NewWatcher(src, memStore, path, 50*time.Millisecond)
	w.Start()
	defer w.Stop()

	time.Sleep(200 * time.Millisecond)
	v1 := memStore.Get()

	// Don't modify the file — wait another cycle
	time.Sleep(200 * time.Millisecond)
	v2 := memStore.Get()

	// Should be the same pointer (no reload)
	if v1 != v2 {
		t.Error("expected no reload when file hasn't changed")
	}
}

func TestWatcher_InvalidConfigIgnored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, validSnapshot(1))

	memStore := store.NewMemory()
	src := NewFileSource(path)
	w := NewWatcher(src, memStore, path, 50*time.Millisecond)
	w.Start()
	defer w.Stop()

	time.Sleep(200 * time.Millisecond)
	if memStore.Get() == nil || memStore.Get().Version != 1 {
		t.Fatal("expected initial config to be loaded")
	}

	// Write invalid JSON
	os.WriteFile(path, []byte("{invalid"), 0644)
	time.Sleep(200 * time.Millisecond)

	// Should still have the old config
	if memStore.Get().Version != 1 {
		t.Error("expected old config to be preserved on invalid reload")
	}
}

func TestWatcher_MissingFileIgnored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")

	memStore := store.NewMemory()
	src := NewFileSource(path)
	w := NewWatcher(src, memStore, path, 50*time.Millisecond)
	w.Start()
	defer w.Stop()

	time.Sleep(200 * time.Millisecond)

	if memStore.Get() != nil {
		t.Error("expected nil store when file doesn't exist")
	}
}

func TestReload_Function(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, validSnapshot(3))

	memStore := store.NewMemory()
	src := NewFileSource(path)

	snap, err := Reload(src, memStore)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Version != 3 {
		t.Errorf("expected version 3, got %d", snap.Version)
	}
	if memStore.Get().Version != 3 {
		t.Error("expected store to be updated")
	}
}

func TestReload_LoadError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")

	memStore := store.NewMemory()
	src := NewFileSource(path)

	_, err := Reload(src, memStore)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReload_ValidationError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	// Write a config with incompatible operator/type
	bad := &model.Snapshot{
		Version: 1,
		Layers: []model.Layer{{
			Name: "bad", Order: 1,
			Segments: []model.Segment{{
				ID: "s", Strategy: "rule",
				Rules: []model.Rule{{
					RuleName:     "r",
					Expression:   &model.Expression{Field: "f", Operator: "gt", Value: 1},
					SuccessEvent: "x",
				}},
				InputSchema: model.InputSchema{
					"f": {Type: model.FieldTypeString, Required: true},
				},
			}},
		}},
	}
	writeConfig(t, path, bad)

	memStore := store.NewMemory()
	src := NewFileSource(path)

	_, err := Reload(src, memStore)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if memStore.Get() != nil {
		t.Error("store should not be updated on validation error")
	}
}
