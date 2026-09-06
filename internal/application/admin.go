package application

import (
	"encoding/json"
	"fmt"
	"strings"
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

	snap.StripNestedMessages()
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
		if l.Key == layer.Key {
			return nil, fmt.Errorf("layer %q already exists", layer.Key)
		}
	}
	if layer.Segments == nil {
		layer.Segments = []model.Segment{}
	}
	snap.Layers = append(snap.Layers, layer)
	return uc.commitSnapshot(snap)
}

// UpdateLayer updates an existing layer's key, friendly name, dependencies,
// default language, input schema and output schema (preserving segments).
// Every named field is replaced wholesale with the incoming value, including
// the two schemas: a layer whose inputSchema or outputSchema is absent from
// the request has that schema cleared, not preserved. This matches how the
// other fields here are already treated — a PUT replaces the whole object
// rather than merging — and it is the shape the layer editor UI (which always
// sends a complete layer) relies on.
//
// Changing the key cascades into every internal reference — dependsOn edges,
// layer:<key> condition fields, and ${layer:<key>} tokens in messages and
// output values — in this one transaction. What it cannot reach is an external
// consumer reading the old key out of the response; that break is real and is
// the caller's to weigh, which is why the UI states it before saving.
//
// Renaming the friendly name cascades into nothing, because nothing references
// it. That is the entire point of the split.
//
// Replacing a layer's schemas can invalidate its own segments — a rule reading
// a field the new schema no longer declares now fails validation.
// commitSnapshot validates the whole snapshot before saving, so such an update
// is rejected and the stored snapshot is left unchanged. The same is true of a
// cascade: if rewriting the references produces invalid config, nothing is
// saved and the key change is refused whole.
func (uc *AdminUseCase) UpdateLayer(key string, updated model.Layer) (*model.Snapshot, error) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	snap := uc.cloneSnapshot()
	idx := uc.findLayer(snap, key)
	if idx < 0 {
		return nil, fmt.Errorf("layer %q not found", key)
	}

	// An incoming layer with no key keeps the one it is addressed by, so a
	// caller updating only the schemas does not have to restate it.
	newKey := updated.Key
	if newKey == "" {
		newKey = key
	}
	if newKey != key {
		if msg := model.ValidateLayerKey(newKey); msg != "" {
			return nil, fmt.Errorf("layer %q: %s", newKey, msg)
		}
		if uc.findLayer(snap, newKey) >= 0 {
			return nil, fmt.Errorf("layer %q already exists", newKey)
		}
	}

	snap.Layers[idx].Key = newKey
	snap.Layers[idx].Name = updated.Name
	snap.Layers[idx].DependsOn = updated.DependsOn
	snap.Layers[idx].DefaultLanguage = updated.DefaultLanguage
	snap.Layers[idx].InputSchema = updated.InputSchema
	snap.Layers[idx].OutputSchema = updated.OutputSchema

	// Cascade after the layer's own fields are in place, so a dependsOn edge
	// the caller just sent naming the old key is rewritten too rather than
	// being left behind by a cascade that ran first.
	rekeyLayer(snap, key, newKey)

	return uc.commitSnapshot(snap)
}

// PreviewRekey reports what changing a layer's key would rewrite, without
// changing anything. The editor shows this before saving so a key change is
// made with its blast radius visible rather than discovered afterwards.
func (uc *AdminUseCase) PreviewRekey(oldKey, newKey string) ([]RekeyRef, error) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	if msg := model.ValidateLayerKey(newKey); msg != "" {
		return nil, fmt.Errorf("layer %q: %s", newKey, msg)
	}
	// Run against a clone: the cascade rewrites in place, and this must not.
	snap := uc.cloneSnapshot()
	if uc.findLayer(snap, oldKey) < 0 {
		return nil, fmt.Errorf("layer %q not found", oldKey)
	}
	return rekeyLayer(snap, oldKey, newKey), nil
}

// DeleteLayer removes a layer by name. A layer other layers depend on cannot be
// deleted — silently dropping those edges would change gating semantics for
// layers the caller did not mention.
func (uc *AdminUseCase) DeleteLayer(name string) (*model.Snapshot, error) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	snap := uc.cloneSnapshot()
	idx := uc.findLayer(snap, name)
	if idx < 0 {
		return nil, fmt.Errorf("layer %q not found", name)
	}

	var dependents []string
	for i := range snap.Layers {
		if i == idx {
			continue
		}
		for _, dep := range snap.Layers[i].DependsOn {
			if dep == name {
				dependents = append(dependents, snap.Layers[i].Key)
				break
			}
		}
	}
	if len(dependents) > 0 {
		return nil, fmt.Errorf("layer %q cannot be deleted: %s depend on it",
			name, strings.Join(dependents, ", "))
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
	snap.StripNestedMessages()
	if err := validation.ValidateSnapshot(snap); err != nil {
		return nil, err
	}
	if err := uc.sink.Save(snap); err != nil {
		return nil, err
	}
	uc.store.Swap(snap)
	return snap, nil
}

func (uc *AdminUseCase) findLayer(snap *model.Snapshot, key string) int {
	for i, l := range snap.Layers {
		if l.Key == key {
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
