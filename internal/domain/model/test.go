package model

// SavedTest is a named evaluation input, kept against the layer it exercises.
//
// Inputs only: it records what to send, not what should come back. Running one
// shows the result to read rather than a pass or a fail. That is a deliberate
// limit — an expectation would have to be maintained alongside the config, and
// a stale expectation that fails for the wrong reason is worse than no
// expectation at all. This is a fixture you can replay in one click, which is
// the thing that was actually missing.
//
// It lives in the snapshot beside lookup tables: the same store, the same save
// path, the same export. That does mean saving a test bumps the config version
// and trips the watcher — harmless, since nothing about evaluation changes, and
// the alternative was a second file with its own source, sink and watcher.
type SavedTest struct {
	// ID is the stable identity, slugged from the name on create.
	ID string `json:"id"`
	// Layer is the key of the layer this test exercises. A test is filed under
	// exactly one layer, which is what makes "run this layer's tests" a
	// question with an answer.
	Layer string `json:"layer"`
	// Segment is the id of the segment within that layer the test is aimed at,
	// and the run is scoped to it (see EvaluateRequest.Segments).
	//
	// Filing by layer alone was not enough once a layer held more than one
	// segment. The panel is shown inside a segment's editor, so every test in
	// the layer appeared to belong to whichever segment was open — and because
	// names only had to be unique per layer, saving a test for the second
	// segment could rename and overwrite the first segment's test of the same
	// name.
	//
	// Empty means the test predates this field and is filed against the layer
	// as a whole: it runs every segment, which is what it did when it was
	// written. Nothing rewrites those, because guessing which segment an old
	// test meant would change what it exercises.
	Segment string `json:"segment,omitempty"`
	// Name is the author's label, unique within the layer and segment.
	Name string `json:"name"`
	// Context is the evaluation context to send, verbatim.
	//
	// Deliberately not validated against the layer's input schema. A test whose
	// whole point is a missing required field — to see the warning, or the
	// unevaluable a missing subjectKey produces — must be storable, and
	// rejecting it would make the interesting cases the ones you cannot save.
	Context map[string]interface{} `json:"context"`
	// Languages and RenderAll mirror the evaluate request, so a test that
	// exercises message rendering replays faithfully.
	Languages []string `json:"languages,omitempty"`
	RenderAll bool     `json:"renderAll,omitempty"`
}
