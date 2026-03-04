package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// FileSource loads segment configuration from a JSON file.
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
