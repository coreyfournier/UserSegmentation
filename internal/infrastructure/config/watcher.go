package config

import (
	"log"
	"os"
	"time"

	"github.com/segmentation-service/segmentation/internal/domain/model"
	"github.com/segmentation-service/segmentation/internal/domain/ports"
	"github.com/segmentation-service/segmentation/internal/domain/validation"
)

// Watcher polls the config source and triggers reload on changes.
type Watcher struct {
	source   ports.ConfigSource
	store    ports.SegmentStore
	interval time.Duration
	stop     chan struct{}
	lastMod  time.Time
	filePath string
}

// NewWatcher creates a config watcher.
func NewWatcher(source ports.ConfigSource, store ports.SegmentStore, filePath string, interval time.Duration) *Watcher {
	return &Watcher{
		source:   source,
		store:    store,
		interval: interval,
		stop:     make(chan struct{}),
		filePath: filePath,
	}
}

// Start begins polling in the background.
func (w *Watcher) Start() {
	go w.poll()
}

// Stop terminates the watcher.
func (w *Watcher) Stop() {
	close(w.stop)
}

func (w *Watcher) poll() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			w.check()
		}
	}
}

func (w *Watcher) check() {
	info, err := os.Stat(w.filePath)
	if err != nil {
		return
	}
	if !info.ModTime().After(w.lastMod) {
		return
	}
	w.lastMod = info.ModTime()

	snap, err := w.source.Load()
	if err != nil {
		log.Printf("[watcher] error loading config: %v", err)
		return
	}
	if err := validation.ValidateSnapshot(snap); err != nil {
		log.Printf("[watcher] config validation failed: %v", err)
		return
	}
	w.store.Swap(snap)
	log.Printf("[watcher] config reloaded (version %d)", snap.Version)
}

// Reload performs an immediate load, validate, and swap. Returns the new snapshot or error.
func Reload(source ports.ConfigSource, store ports.SegmentStore) (*model.Snapshot, error) {
	snap, err := source.Load()
	if err != nil {
		return nil, err
	}
	if err := validation.ValidateSnapshot(snap); err != nil {
		return nil, err
	}
	store.Swap(snap)
	return snap, nil
}
