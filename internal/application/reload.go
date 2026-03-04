package application

import (
	"errors"

	"github.com/segmentation-service/segmentation/internal/domain/ports"
	"github.com/segmentation-service/segmentation/internal/domain/validation"
)

// ErrNoConfig is returned when no config has been loaded.
var ErrNoConfig = errors.New("no configuration loaded")

// ReloadUseCase handles config reload.
type ReloadUseCase struct {
	source ports.ConfigSource
	store  ports.SegmentStore
}

// NewReloadUseCase creates a new reload use case.
func NewReloadUseCase(source ports.ConfigSource, store ports.SegmentStore) *ReloadUseCase {
	return &ReloadUseCase{source: source, store: store}
}

// Execute loads, validates, and swaps the configuration.
func (uc *ReloadUseCase) Execute() error {
	snap, err := uc.source.Load()
	if err != nil {
		return err
	}
	if err := validation.ValidateSnapshot(snap); err != nil {
		return err
	}
	uc.store.Swap(snap)
	return nil
}
