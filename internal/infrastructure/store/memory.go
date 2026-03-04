package store

import (
	"sync/atomic"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// Memory is an in-memory segment store using atomic.Pointer for lock-free reads.
type Memory struct {
	ptr atomic.Pointer[model.Snapshot]
}

// NewMemory creates a new in-memory store.
func NewMemory() *Memory {
	return &Memory{}
}

// Get returns the current snapshot. May return nil if not yet loaded.
func (m *Memory) Get() *model.Snapshot {
	return m.ptr.Load()
}

// Swap atomically replaces the current snapshot.
func (m *Memory) Swap(snap *model.Snapshot) {
	m.ptr.Store(snap)
}
