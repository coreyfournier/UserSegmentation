package model

// StripNestedMessages removes Messages from every non-top-level rule in each
// segment's Rules and Overrides trees. Only top-level rules (whose successEvent
// can become the assignment) have their messages rendered at evaluation time, so
// messages on nested child rules are dead config. This keeps persisted config clean.
func (s *Snapshot) StripNestedMessages() {
	if s == nil {
		return
	}
	for li := range s.Layers {
		for si := range s.Layers[li].Segments {
			seg := &s.Layers[li].Segments[si]
			stripDescendantMessages(seg.Rules)
			stripDescendantMessages(seg.Overrides)
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
