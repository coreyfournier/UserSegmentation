package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// FileSource loads and saves segment configuration from/to a JSON file.
type FileSource struct {
	path string
}

// NewFileSource creates a config source from the given file path.
func NewFileSource(path string) *FileSource {
	return &FileSource{path: path}
}

// Load reads and parses the config file into a Snapshot.
func (fs *FileSource) Load() (*model.Snapshot, error) {
	data, err := os.ReadFile(fs.path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var snap model.Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	// Sort layers by order for deterministic evaluation
	sort.Slice(snap.Layers, func(i, j int) bool {
		return snap.Layers[i].Order < snap.Layers[j].Order
	})

	return &snap, nil
}

// Save atomically writes the snapshot to disk (write tmp then rename).
func (fs *FileSource) Save(snap *model.Snapshot) error {
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	dir := filepath.Dir(fs.path)
	tmp, err := os.CreateTemp(dir, "segments-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, fs.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}
