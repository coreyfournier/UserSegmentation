package model

import "testing"

func msg(text string) map[string]string { return map[string]string{"en": text} }

func snapshotWith(strategy string) *Snapshot {
	return &Snapshot{Layers: []Layer{{
		Name: "l",
		Segments: []Segment{{
			ID:       "s",
			Strategy: strategy,
			When: &Rule{
				RuleName: "isPrecision",
				Messages: msg("predicate message"),
			},
			Rules: []Rule{{
				RuleName: "topLevel",
				Messages: msg("top level"),
				When: &Rule{
					RuleName: "gate",
					Messages: msg("gate message"),
				},
				Rules: []Rule{{
					RuleName: "child",
					Messages: msg("child message"),
				}},
			}},
		}},
	}}}
}

// Assert reports every failing rule with its own message, so nested messages
// there are live config — stripping them would silently delete the text a
// resolver reads.
func TestStripNestedMessages_KeepsNestedAssertMessages(t *testing.T) {
	snap := snapshotWith(StrategyAssert)
	snap.StripNestedMessages()

	rule := snap.Layers[0].Segments[0].Rules[0]
	if rule.Messages["en"] != "top level" {
		t.Error("top-level message should survive")
	}
	if rule.Rules[0].Messages["en"] != "child message" {
		t.Error("a nested assert message is live config and must survive")
	}
}

// First-match strategies only ever render the winning top-level rule's message,
// so descendants stay dead config and are cleaned up.
func TestStripNestedMessages_StripsNestedForFirstMatch(t *testing.T) {
	snap := snapshotWith(StrategyRule)
	snap.StripNestedMessages()

	rule := snap.Layers[0].Segments[0].Rules[0]
	if rule.Messages["en"] != "top level" {
		t.Error("top-level message should survive")
	}
	if rule.Rules[0].Messages != nil {
		t.Errorf("nested message should be stripped, got %v", rule.Rules[0].Messages)
	}
}

// A predicate decides applicability and is never itself reported, so its
// messages are dead config for every strategy.
func TestStripNestedMessages_StripsPredicateMessages(t *testing.T) {
	for _, strategy := range []string{StrategyAssert, StrategyRule} {
		snap := snapshotWith(strategy)
		snap.StripNestedMessages()

		seg := snap.Layers[0].Segments[0]
		if seg.When.Messages != nil {
			t.Errorf("%s: segment predicate message should be stripped", strategy)
		}
		if seg.Rules[0].When.Messages != nil {
			t.Errorf("%s: rule predicate message should be stripped", strategy)
		}
	}
}
