package application

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// A snapshot where "baseTier" is referenced from every place a layer key can
// appear, so a cascade that misses one is a failing test rather than a stale
// token discovered at runtime.
func rekeyFixture() *model.Snapshot {
	return &model.Snapshot{Layers: []model.Layer{
		{Key: "baseTier", Name: "Base Tier"},
		{
			Key:       "features",
			DependsOn: []string{"baseTier", "experiments"},
			Segments: []model.Segment{{
				ID:              "rollout",
				Strategy:        model.StrategyRule,
				DefaultMessages: map[string]string{"en": "tier is ${layer:baseTier}"},
				Outputs:         map[string]string{"tier": "${layer:baseTier}"},
				Rules: []model.Rule{{
					RuleName:  "topLevel",
					Operator:  model.CompositeAnd,
					Condition: nil,
					Rules: []model.Rule{{
						RuleName:  "isPlatinum",
						Condition: &model.Condition{Field: "layer:baseTier", Operator: model.OpEq, Value: "platinum"},
					}},
					Messages:     map[string]string{"en": "matched ${layer:baseTier}"},
					ErrorMessage: "expected ${layer:baseTier}",
					Outputs:      map[string]string{"note": "${ layer:baseTier }"},
				}},
				Overrides: []model.Rule{{
					RuleName:  "vip",
					Condition: &model.Condition{Field: "layer:baseTier", Operator: model.OpEq, Value: "gold"},
				}},
				When: &model.Rule{
					RuleName:  "applies",
					Condition: &model.Condition{Field: "layer:baseTier", Operator: model.OpIsNull},
				},
			}},
		},
	}}
}

func countWhere(refs []RekeyRef, where string) int {
	n := 0
	for _, r := range refs {
		if r.Where == where {
			n++
		}
	}
	return n
}

func TestRekeyLayer_RewritesEveryReferenceSite(t *testing.T) {
	snap := rekeyFixture()
	refs := rekeyLayer(snap, "baseTier", "baseTierV2")

	// dependsOn, three conditions (rule, override, when), two messages
	// (rule message + errorMessage), one defaultMessage, two outputs.
	for where, want := range map[string]int{
		"dependsOn":      1,
		"condition":      3,
		"message":        1,
		"errorMessage":   1,
		"defaultMessage": 1,
		"output":         2,
	} {
		if got := countWhere(refs, where); got != want {
			t.Errorf("%s: expected %d rewrites, got %d (all: %+v)", where, want, got, refs)
		}
	}

	seg := &snap.Layers[1].Segments[0]
	if snap.Layers[1].DependsOn[0] != "baseTierV2" {
		t.Errorf("dependsOn not rewritten: %v", snap.Layers[1].DependsOn)
	}
	// The unrelated edge is untouched.
	if snap.Layers[1].DependsOn[1] != "experiments" {
		t.Errorf("an unrelated dependency was rewritten: %v", snap.Layers[1].DependsOn)
	}
	if got := seg.Rules[0].Rules[0].Condition.Field; got != "layer:baseTierV2" {
		t.Errorf("nested condition not rewritten: %q", got)
	}
	if got := seg.Overrides[0].Condition.Field; got != "layer:baseTierV2" {
		t.Errorf("override condition not rewritten: %q", got)
	}
	if got := seg.When.Condition.Field; got != "layer:baseTierV2" {
		t.Errorf("when condition not rewritten: %q", got)
	}
	if got := seg.DefaultMessages["en"]; got != "tier is ${layer:baseTierV2}" {
		t.Errorf("defaultMessage not rewritten: %q", got)
	}
	if got := seg.Outputs["tier"]; got != "${layer:baseTierV2}" {
		t.Errorf("segment output not rewritten: %q", got)
	}
	// Whitespace inside the braces is live, because renderTemplate trims it.
	if got := seg.Rules[0].Outputs["note"]; got != "${layer:baseTierV2}" {
		t.Errorf("padded token not rewritten: %q", got)
	}
	if got := seg.Rules[0].ErrorMessage; got != "expected ${layer:baseTierV2}" {
		t.Errorf("errorMessage not rewritten: %q", got)
	}
}

// Renaming a key must not corrupt a longer key that merely starts with it.
func TestRekeyLayer_DoesNotMatchOnPrefix(t *testing.T) {
	snap := &model.Snapshot{Layers: []model.Layer{
		{Key: "ewa"},
		{Key: "ewaRisk"},
		{Key: "features", DependsOn: []string{"ewaRisk"}, Segments: []model.Segment{{
			ID:       "s",
			Strategy: model.StrategyRule,
			Rules: []model.Rule{{
				RuleName:  "r",
				Condition: &model.Condition{Field: "layer:ewaRisk", Operator: model.OpIsNull},
				Messages:  map[string]string{"en": "${layer:ewaRisk}"},
			}},
		}}},
	}}

	refs := rekeyLayer(snap, "ewa", "ewaBase")
	if len(refs) != 0 {
		t.Fatalf("renaming %q rewrote references to %q: %+v", "ewa", "ewaRisk", refs)
	}
	if got := snap.Layers[2].Segments[0].Rules[0].Condition.Field; got != "layer:ewaRisk" {
		t.Errorf("condition corrupted: %q", got)
	}
	if got := snap.Layers[2].Segments[0].Rules[0].Messages["en"]; got != "${layer:ewaRisk}" {
		t.Errorf("message corrupted: %q", got)
	}
}

func TestRekeyLayer_NoOpWhenKeyUnchanged(t *testing.T) {
	snap := rekeyFixture()
	if refs := rekeyLayer(snap, "baseTier", "baseTier"); refs != nil {
		t.Errorf("expected no rewrites, got %+v", refs)
	}
	if got := snap.Layers[1].Segments[0].DefaultMessages["en"]; got != "tier is ${layer:baseTier}" {
		t.Errorf("an unchanged key rewrote content: %q", got)
	}
}

// A token that is not a layer reference is left exactly as it was.
func TestRekeyTemplate_LeavesOtherTokensAlone(t *testing.T) {
	in := "hello ${company.name} and ${layer:other} and ${layer:baseTier}!"
	got, changed := rekeyTemplate(in, "baseTier", "bt")
	if !changed {
		t.Fatal("expected a change")
	}
	want := "hello ${company.name} and ${layer:other} and ${layer:bt}!"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}
