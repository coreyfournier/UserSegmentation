package store

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func TestMemory_NilBeforeSwap(t *testing.T) {
	m := NewMemory()
	if m.Get() != nil {
		t.Error("expected nil before swap")
	}
}

func TestMemory_SwapAndGet(t *testing.T) {
	m := NewMemory()
	snap := &model.Snapshot{Version: 42}
	m.Swap(snap)
	got := m.Get()
	if got == nil || got.Version != 42 {
		t.Errorf("expected version 42, got %v", got)
	}
}

func TestMemory_SwapReplacesOld(t *testing.T) {
	m := NewMemory()
	m.Swap(&model.Snapshot{Version: 1})
	m.Swap(&model.Snapshot{Version: 2})
	got := m.Get()
	if got.Version != 2 {
		t.Errorf("expected version 2, got %d", got.Version)
	}
}
