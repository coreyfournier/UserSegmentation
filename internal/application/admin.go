package application

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/segmentation-service/segmentation/internal/domain/model"
	"github.com/segmentation-service/segmentation/internal/domain/ports"
	"github.com/segmentation-service/segmentation/internal/domain/validation"
)

// AdminUseCase handles CRUD operations on layers and segments.
type AdminUseCase struct {
	mu    sync.Mutex
	store ports.SegmentStore
	sink  ports.ConfigSink
}

// NewAdminUseCase creates a new admin use case.
func NewAdminUseCase(store ports.SegmentStore, sink ports.ConfigSink) *AdminUseCase {
	return &AdminUseCase{store: store, sink: sink}
}

// GetSnapshot returns the current snapshot.
func (uc *AdminUseCase) GetSnapshot() *model.Snapshot {
	return uc.store.Get()
}

// ReplaceSnapshot replaces the entire snapshot (import).
func (uc *AdminUseCase) ReplaceSnapshot(snap *model.Snapshot) error {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	if err := validation.ValidateSnapshot(snap); err != nil {
		return err
	}
	if err := uc.sink.Save(snap); err != nil {
		return err
	}
	uc.store.Swap(snap)
	return nil
}

// CreateLayer adds a new layer.
func (uc *AdminUseCase) CreateLayer(layer model.Layer) (*model.Snapshot, error) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	snap := uc.cloneSnapshot()
	for _, l := range snap.Layers {
		if l.Name == layer.Name {
			return nil, fmt.Errorf("layer %q already exists", layer.Name)
		}
	}
	if layer.Segments == nil {
		layer.Segments = []model.Segment{}
	}
	snap.Layers = append(snap.Layers, layer)
	return uc.commitSnapshot(snap)
}

// UpdateLayer updates an existing layer's order (preserving segments).
func (uc *AdminUseCase) UpdateLayer(name string, updated model.Layer) (*model.Snapshot, error) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	snap := uc.cloneSnapshot()
	idx := uc.findLayer(snap, name)
	if idx < 0 {
		return nil, fmt.Errorf("layer %q not found", name)
	}
	snap.Layers[idx].Name = updated.Name
	snap.Layers[idx].Order = updated.Order
	return uc.commitSnapshot(snap)
}

// DeleteLayer removes a layer by name.
func (uc *AdminUseCase) DeleteLayer(name string) (*model.Snapshot, error) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	snap := uc.cloneSnapshot()
	idx := uc.findLayer(snap, name)
	if idx < 0 {
		return nil, fmt.Errorf("layer %q not found", name)
	}
	snap.Layers = append(snap.Layers[:idx], snap.Layers[idx+1:]...)
	return uc.commitSnapshot(snap)
}

// CreateSegment adds a segment to a layer.
func (uc *AdminUseCase) CreateSegment(layerName string, seg model.Segment) (*model.Snapshot, error) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	snap := uc.cloneSnapshot()
	idx := uc.findLayer(snap, layerName)
	if idx < 0 {
		return nil, fmt.Errorf("layer %q not found", layerName)
	}
	for _, s := range snap.Layers[idx].Segments {
		if s.ID == seg.ID {
			return nil, fmt.Errorf("segment %q already exists in layer %q", seg.ID, layerName)
		}
	}
	snap.Layers[idx].Segments = append(snap.Layers[idx].Segments, seg)
	return uc.commitSnapshot(snap)
}

// UpdateSegment replaces a segment in a layer.
func (uc *AdminUseCase) UpdateSegment(layerName, segID string, seg model.Segment) (*model.Snapshot, error) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	snap := uc.cloneSnapshot()
	li := uc.findLayer(snap, layerName)
	if li < 0 {
		return nil, fmt.Errorf("layer %q not found", layerName)
	}
	si := uc.findSegment(snap, li, segID)
	if si < 0 {
		return nil, fmt.Errorf("segment %q not found in layer %q", segID, layerName)
	}
	seg.ID = segID // preserve original ID
	snap.Layers[li].Segments[si] = seg
	return uc.commitSnapshot(snap)
}

// DeleteSegment removes a segment from a layer.
func (uc *AdminUseCase) DeleteSegment(layerName, segID string) (*model.Snapshot, error) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	snap := uc.cloneSnapshot()
	li := uc.findLayer(snap, layerName)
	if li < 0 {
		return nil, fmt.Errorf("layer %q not found", layerName)
	}
	si := uc.findSegment(snap, li, segID)
	if si < 0 {
		return nil, fmt.Errorf("segment %q not found in layer %q", segID, layerName)
	}
	segs := snap.Layers[li].Segments
	snap.Layers[li].Segments = append(segs[:si], segs[si+1:]...)
	return uc.commitSnapshot(snap)
}

func (uc *AdminUseCase) cloneSnapshot() *model.Snapshot {
	orig := uc.store.Get()
	data, _ := json.Marshal(orig)
	var clone model.Snapshot
	json.Unmarshal(data, &clone)
	return &clone
}

func (uc *AdminUseCase) commitSnapshot(snap *model.Snapshot) (*model.Snapshot, error) {
	snap.Version++
	if err := validation.ValidateSnapshot(snap); err != nil {
		return nil, err
	}
	if err := uc.sink.Save(snap); err != nil {
		return nil, err
	}
	uc.store.Swap(snap)
	return snap, nil
}

func (uc *AdminUseCase) findLayer(snap *model.Snapshot, name string) int {
	for i, l := range snap.Layers {
		if l.Name == name {
			return i
		}
	}
	return -1
}

func (uc *AdminUseCase) findSegment(snap *model.Snapshot, layerIdx int, segID string) int {
	for i, s := range snap.Layers[layerIdx].Segments {
		if s.ID == segID {
			return i
		}
	}
	return -1
}
