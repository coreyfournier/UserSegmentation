package application

import (
	"time"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// Optimistic concurrency for a layer. The layer is the aggregate — it owns its
// segments and the schemas they share — so one revision guards all of it, and
// any write that touches any part advances it.

// checkLayerRevision reports a conflict when the caller's expected revision is
// not the one the store holds.
//
// A nil expectation skips the check. That is deliberate rather than lax: the
// admin API is also driven by scripts and imports that have no revision to
// offer, and forcing one on them would mean inventing a read-then-write dance
// for callers that are the only writer anyway. The editor always sends one, so
// the case the check exists for — two people in the same layer — is covered.
//
// Compared under AdminUseCase's mutex, which serialises writes within this
// process. A deployment with several writers should push the predicate into the
// store, which can enforce it atomically; the error type is in the domain so
// that swap changes nothing above it (see model.ConflictError).
func checkLayerRevision(stored *model.Layer, expected *int) error {
	if expected == nil || stored.Revision == *expected {
		return nil
	}
	return &model.ConflictError{
		Kind:      "layer",
		Key:       stored.Key,
		Expected:  *expected,
		Actual:    stored.Revision,
		ChangedAt: stored.UpdatedAt,
	}
}

// stampLayer advances the layer's revision and records when.
//
// Called for every write that touches the layer, including one aimed at a
// segment: a segment belongs to the layer, so changing it changes the layer,
// and a revision that ignored that would let a stale schema edit land on top.
func stampLayer(l *model.Layer) {
	now := time.Now().UTC()
	l.Revision++
	l.UpdatedAt = &now
}
