package model

import (
	"errors"
	"fmt"
	"time"
)

// ConflictError reports that an aggregate was changed by someone else since the
// caller read it, so the write it is asking for would silently discard that
// change.
//
// It lives in the domain rather than in either the application or a store
// because both produce it and neither owns it. Today the application compares
// revisions under its own lock, which is correct for one process. A store that
// can enforce the predicate atomically — "UPDATE … WHERE revision = ?", a
// document store's compare-and-set, a rowversion check — should do so instead
// and return this same error, and nothing above has to change: the HTTP layer
// maps this type to 409 whoever raised it, and the client reads one shape.
//
// That is the whole point of a typed error here rather than a string: the
// concurrency contract is expressed once, in terms every storage technology
// already has a mechanism for.
type ConflictError struct {
	// Kind and Key identify the aggregate — "layer" and its key today.
	Kind string
	Key  string
	// Expected is the revision the caller believed it was updating; Actual is
	// what the store holds now.
	Expected int
	Actual   int
	// ChangedAt is when the stored revision was written, so a caller can say
	// how stale its copy is rather than only that it is stale.
	ChangedAt *time.Time
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf(
		"%s %q was changed by someone else: you have revision %d, the stored one is %d",
		e.Kind, e.Key, e.Expected, e.Actual)
}

// AsConflict reports whether err is a ConflictError, unwrapping as it goes so a
// store is free to wrap one on the way out.
func AsConflict(err error) (*ConflictError, bool) {
	var c *ConflictError
	if errors.As(err, &c) {
		return c, true
	}
	return nil, false
}
