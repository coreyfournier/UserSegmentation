package config

import (
	"os"
	"path/filepath"
	"strings"
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
			{"key": "test", "segments": []}
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
	if len(snap.Layers) != 1 || snap.Layers[0].Key != "test" {
		t.Errorf("unexpected layers: %v", snap.Layers)
	}
}

func TestFileSource_LoadReadsDependsOn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	data := []byte(`{
		"version": 1,
		"layers": [
			{"key": "b", "dependsOn": ["a"], "segments": []},
			{"key": "a", "segments": []}
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
	// Declaration order on disk is preserved; the evaluator topologically sorts.
	if snap.Layers[0].Key != "b" || len(snap.Layers[0].DependsOn) != 1 || snap.Layers[0].DependsOn[0] != "a" {
		t.Errorf("dependsOn not loaded: %+v", snap.Layers)
	}
}

// A config carrying the removed "order" field must fail loudly rather than be
// silently reinterpreted, which would leave it subtly misordered.
func TestFileSource_RejectsLegacyOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	data := []byte(`{
		"version": 1,
		"layers": [
			{"key": "stale", "order": 1, "segments": []}
		]
	}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := NewFileSource(path).Load(); err == nil {
		t.Fatal("expected load to fail on a config still declaring order")
	} else if !strings.Contains(err.Error(), "dependsOn") {
		t.Errorf("error should point at dependsOn, got: %v", err)
	}
}

// The old vocabulary must fail loudly. Unmarshalling ignores unknown fields, so
// a stale "expression" would leave a rule with no condition and no children,
// evaluating false forever.
func TestFileSource_RejectsLegacyExpressionKeys(t *testing.T) {
	cases := map[string]string{
		"rule condition": `{"version":1,"layers":[{"key":"l","segments":[
			{"id":"s","strategy":"rule","rules":[
				{"ruleName":"r","expression":{"field":"a","operator":"eq","value":1}}
			]}
		]}]}`,
		"computed field list": `{"version":1,"layers":[{"key":"l","segments":[
			{"id":"s","strategy":"computed","expressions":[{"name":"X","type":"number","formula":"1"}]}
		]}]}`,
	}

	for name, body := range cases {
		path := filepath.Join(t.TempDir(), "test.json")
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
		_, err := NewFileSource(path).Load()
		if err == nil {
			t.Errorf("%s: expected load to fail on the removed key", name)
			continue
		}
		if !strings.Contains(err.Error(), "condition") {
			t.Errorf("%s: error should name the replacement, got: %v", name, err)
		}
	}
}

// Only top-level rules report, so the branches of an And/Or need no name. A
// group's children are its condition, not separate checklist items.
func TestFileSource_AllowsUnnamedNestedRules(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	data := []byte(`{
		"version": 1,
		"layers": [
			{"key": "gates", "segments": [
				{"id": "g", "strategy": "checklist", "rules": [
					{"ruleName": "depositAccountMissing", "operator": "Or", "rules": [
						{"ruleName": "", "condition": {"field": "hasDeposit", "operator": "is_null"}},
						{"ruleName": "", "condition": {"field": "hasDeposit", "operator": "eq", "value": false}}
					]}
				]}
			]}
		]
	}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := NewFileSource(path).Load(); err != nil {
		t.Fatalf("unnamed condition branches should load, got: %v", err)
	}
}

// Rule names are the stable public identifier for a reported failure, so the
// store rejects collisions among the rules that actually report.
func TestFileSource_RejectsDuplicateChecklistRuleName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	data := []byte(`{
		"version": 1,
		"layers": [
			{"key": "gateOne", "segments": [
				{"id": "s", "strategy": "checklist", "rules": [
					{"ruleName": "sameName", "condition": {"field": "a", "operator": "eq", "value": 1}}
				]}
			]},
			{"key": "gateTwo", "segments": [
				{"id": "s", "strategy": "checklist", "rules": [
					{"ruleName": "sameName", "condition": {"field": "b", "operator": "eq", "value": 2}}
				]}
			]}
		]
	}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := NewFileSource(path).Load(); err == nil {
		t.Fatal("expected load to fail on duplicate checklist ruleName")
	} else if !strings.Contains(err.Error(), "sameName") {
		t.Errorf("error should name the colliding rule, got: %v", err)
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
			{Key: "saved", Segments: []model.Segment{}},
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
	if len(loaded.Layers) != 1 || loaded.Layers[0].Key != "saved" {
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
		{Key: "new", Segments: []model.Segment{}},
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
