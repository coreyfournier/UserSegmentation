package ports

import "github.com/segmentation-service/segmentation/internal/domain/model"

// SegmentStore provides lock-free read access to the current config snapshot.
type SegmentStore interface {
	Get() *model.Snapshot
	Swap(snap *model.Snapshot)
}
