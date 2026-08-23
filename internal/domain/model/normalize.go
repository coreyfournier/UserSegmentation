package model

// StripNestedMessages removes Messages from rules whose messages could never be
// rendered, keeping persisted config free of dead configuration.
//
// For first-match strategies only the top-level rule wins, so messages on its
// descendants are dead. Checklists are the exception: every rule that fires is
// itemised with its own message, so nested messages there are live config and
// must be preserved.
//
// A segment's Applies When predicate is stripped everywhere — it decides
// applicability and is never itself reported.
func (s *Snapshot) StripNestedMessages() {
	if s == nil {
		return
	}
	for li := range s.Layers {
		for si := range s.Layers[li].Segments {
			seg := &s.Layers[li].Segments[si]

			if seg.Strategy != StrategyChecklist {
				stripDescendantMessages(seg.Rules)
			}
			stripDescendantMessages(seg.Overrides)
			stripPredicateMessages(seg.When)
		}
	}
}

// stripDescendantMessages clears Messages on all descendants of the given
// top-level rules; the top-level rules themselves retain their messages.
func stripDescendantMessages(topLevel []Rule) {
	for i := range topLevel {
		clearMessages(topLevel[i].Rules)
	}
}

func clearMessages(rules []Rule) {
	for i := range rules {
		rules[i].Messages = nil
		clearMessages(rules[i].Rules)
	}
}

// stripPredicateMessages clears messages throughout an Applies When predicate.
func stripPredicateMessages(predicate *Rule) {
	if predicate == nil {
		return
	}
	predicate.Messages = nil
	clearMessages(predicate.Rules)
}
