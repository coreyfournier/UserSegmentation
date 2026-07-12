package model

import "testing"

func TestStripNestedMessages(t *testing.T) {
	snap := &Snapshot{
		Layers: []Layer{{
			Name: "l",
			Segments: []Segment{{
				ID: "s",
				Rules: []Rule{{
					RuleName: "top", Operator: CompositeAnd,
					Messages: map[string]string{"en": "kept"}, // top-level: keep
					Rules: []Rule{
						{RuleName: "child", Messages: map[string]string{"en": "dead"}, // nested: strip
							Rules: []Rule{
								{RuleName: "grandchild", Messages: map[string]string{"en": "dead"}}, // deeper: strip
							}},
					},
				}},
				Overrides: []Rule{{
					RuleName: "topOv",
					Messages: map[string]string{"en": "kept"}, // top-level override: keep
					Rules: []Rule{
						{RuleName: "ovChild", Messages: map[string]string{"en": "dead"}}, // nested: strip
					},
				}},
			}},
		}},
	}

	snap.StripNestedMessages()

	seg := snap.Layers[0].Segments[0]
	if seg.Rules[0].Messages["en"] != "kept" {
		t.Fatalf("top-level rule message should be kept, got %v", seg.Rules[0].Messages)
	}
	if seg.Rules[0].Rules[0].Messages != nil {
		t.Fatalf("nested child message should be stripped, got %v", seg.Rules[0].Rules[0].Messages)
	}
	if seg.Rules[0].Rules[0].Rules[0].Messages != nil {
		t.Fatalf("grandchild message should be stripped, got %v", seg.Rules[0].Rules[0].Rules[0].Messages)
	}
	if seg.Overrides[0].Messages["en"] != "kept" {
		t.Fatalf("top-level override message should be kept, got %v", seg.Overrides[0].Messages)
	}
	if seg.Overrides[0].Rules[0].Messages != nil {
		t.Fatalf("nested override child message should be stripped, got %v", seg.Overrides[0].Rules[0].Messages)
	}
}

func TestStripNestedMessages_NilSafe(t *testing.T) {
	var snap *Snapshot
	snap.StripNestedMessages() // must not panic
}
