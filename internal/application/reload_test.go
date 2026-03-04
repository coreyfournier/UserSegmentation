package application

import (
	"errors"
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
	"github.com/segmentation-service/segmentation/internal/infrastructure/store"
)

type mockConfigSource struct {
	snap *model.Snapshot
	err  error
}

func (m *mockConfigSource) Load() (*model.Snapshot, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.snap, nil
}

func TestReloadUseCase_Success(t *testing.T) {
	s := store.NewMemory()
	snap := &model.Snapshot{Version: 1, Layers: []model.Layer{
		{Name: "test", Order: 1, Segments: []model.Segment{}},
	}}
	src := &mockConfigSource{snap: snap}
	uc := NewReloadUseCase(src, s)

	if err := uc.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := s.Get()
	if got == nil || got.Version != 1 {
		t.Error("expected snapshot to be swapped into store")
	}
}

func TestReloadUseCase_LoadError(t *testing.T) {
	s := store.NewMemory()
	src := &mockConfigSource{err: errors.New("file not found")}
	uc := NewReloadUseCase(src, s)

	err := uc.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if s.Get() != nil {
		t.Error("store should remain nil on load error")
	}
}

func TestReloadUseCase_ValidationError(t *testing.T) {
	s := store.NewMemory()
	// Empty snapshot with no layers is valid, but a snapshot with invalid rules should fail.
	// Create a segment with an inputSchema that references a field with incompatible operator.
	snap := &model.Snapshot{
		Version: 1,
		Layers: []model.Layer{
			{
				Name:  "bad",
				Order: 1,
				Segments: []model.Segment{
					{
						ID:       "s1",
						Strategy: "rule",
						Rules: []model.Rule{
							{
								RuleName: "r1",
								Expression: &model.Expression{
									Field: "age", Operator: "gt", Value: 18,
								},
								SuccessEvent: "young",
							},
						},
						InputSchema: model.InputSchema{
							"age": {Type: model.FieldTypeString, Required: true}, // gt doesn't support string
						},
					},
				},
			},
		},
	}
	src := &mockConfigSource{snap: snap}
	uc := NewReloadUseCase(src, s)

	err := uc.Execute()
	if err == nil {
		t.Fatal("expected validation error for incompatible operator/type")
	}
	if s.Get() != nil {
		t.Error("store should remain nil on validation error")
	}
}
