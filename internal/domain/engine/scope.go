package engine

// EvalOption narrows what an evaluation runs. Options exist rather than more
// parameters because Evaluate already takes six, and every caller but one
// wants the default for all of them.
type EvalOption func(*evalOptions)

type evalOptions struct {
	// segments, when non-nil, is the set of segment ids to consider — every
	// other segment in a requested layer is passed over as if it were not
	// there.
	segments map[string]struct{}
}

// OnlySegments restricts evaluation to the named segments within the layers
// the caller asked for.
//
// This exists for authoring, not for serving: a layer resolves to the first
// segment that applies, so a segment behind another one that always resolves
// can never be reached in a real evaluation — and could not be exercised at
// all without this. Testing it is exactly when you need to see what it would
// have produced.
//
// Deliberately scoped to the *requested* layers. A layer being evaluated only
// because something depends on it must run in full: narrowing it would change
// what the layer under test depends on, and the answer would be about a
// configuration that does not exist.
//
// An empty or nil list is no restriction, so a caller can pass through
// whatever it has without a conditional.
func OnlySegments(ids []string) EvalOption {
	return func(o *evalOptions) {
		if len(ids) == 0 {
			return
		}
		o.segments = make(map[string]struct{}, len(ids))
		for _, id := range ids {
			o.segments[id] = struct{}{}
		}
	}
}
