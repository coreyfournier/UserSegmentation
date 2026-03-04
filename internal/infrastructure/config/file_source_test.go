package config

import (
	"os"
	"path/filepath"
	"testing"
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
