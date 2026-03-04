package ports

import "github.com/segmentation-service/segmentation/internal/domain/model"

// ConfigSink persists segment configuration to an external store.
type ConfigSink interface {
	Save(snap *model.Snapshot) error
}
