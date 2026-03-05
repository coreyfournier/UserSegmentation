package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func TestFileSource_Load(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	data := []byte(`{
		"version": 5,
		"layers": [
			{"name": "test", "order": 1, "segments": []}
		]
	}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	fs := NewFileSource(path)
	snap, err := fs.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if snap.Version != 5 {
		t.Errorf("expected version 5, got %d", snap.Version)
	}
	if len(snap.Layers) != 1 || snap.Layers[0].Name != "test" {
		t.Errorf("unexpected layers: %v", snap.Layers)
	}
}

func TestFileSource_LoadSortsLayers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	data := []byte(`{
		"version": 1,
		"layers": [
			{"name": "b", "order": 2, "segments": []},
			{"name": "a", "order": 1, "segments": []}
		]
	}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	fs := NewFileSource(path)
	snap, err := fs.Load()
	if err != nil {
		t.Fatal(err)
	}
	if snap.Layers[0].Name != "a" || snap.Layers[1].Name != "b" {
		t.Errorf("layers not sorted by order: %v", snap.Layers)
	}
}

func TestFileSource_MissingFile(t *testing.T) {
	fs := NewFileSource("/nonexistent/file.json")
	_, err := fs.Load()
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestFileSource_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("{invalid"), 0644)
	fs := NewFileSource(path)
	_, err := fs.Load()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestFileSource_Save(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	fs := NewFileSource(path)

	snap := &model.Snapshot{
		Version: 10,
		Layers: []model.Layer{
			{Name: "saved", Order: 1, Segments: []model.Segment{}},
		},
	}
	if err := fs.Save(snap); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify by loading back
	loaded, err := fs.Load()
	if err != nil {
		t.Fatalf("Load after Save failed: %v", err)
	}
	if loaded.Version != 10 {
		t.Errorf("expected version 10, got %d", loaded.Version)
	}
	if len(loaded.Layers) != 1 || loaded.Layers[0].Name != "saved" {
		t.Errorf("unexpected layers after Save: %v", loaded.Layers)
	}
	if loaded.LastModified == nil {
		t.Fatal("expected last_modified to be set after Save")
	}
	if time.Since(*loaded.LastModified) > 5*time.Second {
		t.Errorf("last_modified too old: %v", loaded.LastModified)
	}
}

func TestFileSource_SaveAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "atomic.json")

	// Write initial file
	fs := NewFileSource(path)
	initial := &model.Snapshot{Version: 1, Layers: []model.Layer{}}
	fs.Save(initial)

	// Overwrite with new version
	updated := &model.Snapshot{Version: 2, Layers: []model.Layer{
		{Name: "new", Order: 1, Segments: []model.Segment{}},
	}}
	if err := fs.Save(updated); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, _ := fs.Load()
	if loaded.Version != 2 {
		t.Errorf("expected version 2, got %d", loaded.Version)
	}

	// No temp files should remain
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "atomic.json" {
			t.Errorf("unexpected file left behind: %s", e.Name())
		}
	}
}
