package ports

import "github.com/segmentation-service/segmentation/internal/domain/model"

// ConfigSource loads segment configuration from an external source.
type ConfigSource interface {
	Load() (*model.Snapshot, error)
}
