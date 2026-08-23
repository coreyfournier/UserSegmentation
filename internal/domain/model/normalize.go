package model

// StripNestedMessages removes Messages from rules whose messages could never be
// rendered, keeping persisted config free of dead configuration.
//
// For first-match strategies only the top-level rule wins, so messages on its
// descendants are dead. Assert segments are the exception: every failing rule is
// itemised with its own message, so nested messages there are live config and
// must be preserved.
//
// When predicates are stripped everywhere — a predicate decides applicability
// and is never itself reported.
func (s *Snapshot) StripNestedMessages() {
	if s == nil {
		return
	}
	for li := range s.Layers {
		for si := range s.Layers[li].Segments {
			seg := &s.Layers[li].Segments[si]

			if seg.Strategy != StrategyAssert {
				stripDescendantMessages(seg.Rules)
			}
			stripDescendantMessages(seg.Overrides)

			stripPredicateMessages(seg.When)
			clearPredicateMessages(seg.Rules)
			clearPredicateMessages(seg.Overrides)
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

// clearPredicateMessages walks a rule tree and strips messages from every When
// predicate it finds, at any depth.
func clearPredicateMessages(rules []Rule) {
	for i := range rules {
		stripPredicateMessages(rules[i].When)
		clearPredicateMessages(rules[i].Rules)
	}
}

func stripPredicateMessages(predicate *Rule) {
	if predicate == nil {
		return
	}
	predicate.Messages = nil
	stripPredicateMessages(predicate.When)
	clearMessages(predicate.Rules)
	clearPredicateMessages(predicate.Rules)
}
