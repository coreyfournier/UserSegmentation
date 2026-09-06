package application

import (
	"strings"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// RekeyRef is one reference rewritten by a key change, for reporting back what
// a rename touched.
type RekeyRef struct {
	// Layer is the key of the layer holding the reference — not the layer
	// being renamed.
	Layer string `json:"layer"`
	// Segment and Rule are empty for a dependsOn edge.
	Segment string `json:"segment,omitempty"`
	Rule    string `json:"rule,omitempty"`
	// Where says what kind of reference it was: "dependsOn", "condition",
	// "message", "errorMessage", "defaultMessage" or "output".
	Where string `json:"where"`
}

// rekeyLayer rewrites every reference to oldKey so it names newKey, returning
// what it touched.
//
// A layer's key reaches four places, and a rename that misses any of them
// leaves config that still loads:
//
//   - dependsOn edges on other layers
//   - "layer:<key>" as a condition field
//   - "${layer:<key>}" in a message, error message or default message
//   - "${layer:<key>}" in an authored output value
//
// The condition case is caught by validation if missed (a layer:x field must
// have a matching dependsOn edge), but the template cases are not — a stale
// token renders empty and says nothing. That asymmetry is why this rewrites
// all four rather than relying on validation to find the stragglers.
func rekeyLayer(snap *model.Snapshot, oldKey, newKey string) []RekeyRef {
	if oldKey == newKey {
		return nil
	}
	var refs []RekeyRef

	oldField := "layer:" + oldKey
	newField := "layer:" + newKey

	for i := range snap.Layers {
		layer := &snap.Layers[i]

		for j, dep := range layer.DependsOn {
			if dep == oldKey {
				layer.DependsOn[j] = newKey
				refs = append(refs, RekeyRef{Layer: layer.Key, Where: "dependsOn"})
			}
		}

		for s := range layer.Segments {
			seg := &layer.Segments[s]

			for lang, msg := range seg.DefaultMessages {
				if rewritten, ok := rekeyTemplate(msg, oldKey, newKey); ok {
					seg.DefaultMessages[lang] = rewritten
					refs = append(refs, RekeyRef{
						Layer: layer.Key, Segment: seg.ID, Where: "defaultMessage",
					})
				}
			}

			for name, val := range seg.Outputs {
				if rewritten, ok := rekeyTemplate(val, oldKey, newKey); ok {
					seg.Outputs[name] = rewritten
					refs = append(refs, RekeyRef{
						Layer: layer.Key, Segment: seg.ID, Where: "output",
					})
				}
			}

			for r := range seg.Rules {
				refs = append(refs, rekeyRule(&seg.Rules[r], layer.Key, seg.ID, oldKey, newKey, oldField, newField)...)
			}
			for r := range seg.Overrides {
				refs = append(refs, rekeyRule(&seg.Overrides[r], layer.Key, seg.ID, oldKey, newKey, oldField, newField)...)
			}
			if seg.When != nil {
				refs = append(refs, rekeyRule(seg.When, layer.Key, seg.ID, oldKey, newKey, oldField, newField)...)
			}
		}
	}

	return refs
}

// rekeyRule walks one rule tree. Conditions live on leaves, but messages and
// outputs can sit on any node, so both are checked at every level.
func rekeyRule(r *model.Rule, layerKey, segID, oldKey, newKey, oldField, newField string) []RekeyRef {
	var refs []RekeyRef

	if r.Condition != nil && r.Condition.Field == oldField {
		r.Condition.Field = newField
		refs = append(refs, RekeyRef{
			Layer: layerKey, Segment: segID, Rule: r.RuleName, Where: "condition",
		})
	}

	if rewritten, ok := rekeyTemplate(r.ErrorMessage, oldKey, newKey); ok {
		r.ErrorMessage = rewritten
		refs = append(refs, RekeyRef{
			Layer: layerKey, Segment: segID, Rule: r.RuleName, Where: "errorMessage",
		})
	}
	for lang, msg := range r.Messages {
		if rewritten, ok := rekeyTemplate(msg, oldKey, newKey); ok {
			r.Messages[lang] = rewritten
			refs = append(refs, RekeyRef{
				Layer: layerKey, Segment: segID, Rule: r.RuleName, Where: "message",
			})
		}
	}
	for name, val := range r.Outputs {
		if rewritten, ok := rekeyTemplate(val, oldKey, newKey); ok {
			r.Outputs[name] = rewritten
			refs = append(refs, RekeyRef{
				Layer: layerKey, Segment: segID, Rule: r.RuleName, Where: "output",
			})
		}
	}

	for i := range r.Rules {
		refs = append(refs, rekeyRule(&r.Rules[i], layerKey, segID, oldKey, newKey, oldField, newField)...)
	}
	return refs
}

// rekeyTemplate rewrites ${layer:<oldKey>} tokens, reporting whether it changed
// anything.
//
// Matching is on the whole token rather than on the substring "layer:oldKey",
// so renaming "ewa" does not corrupt a reference to "ewaRisk". Whitespace
// inside the braces is tolerated because renderTemplate trims it, so
// "${ layer:x }" is a live reference that a stricter match would miss.
func rekeyTemplate(s, oldKey, newKey string) (string, bool) {
	if s == "" || !strings.Contains(s, "layer:") {
		return s, false
	}

	var b strings.Builder
	changed := false
	rest := s
	for {
		open := strings.Index(rest, "${")
		if open < 0 {
			break
		}
		close := strings.Index(rest[open:], "}")
		if close < 0 {
			break
		}
		close += open

		b.WriteString(rest[:open])
		inner := rest[open+2 : close]
		if strings.TrimSpace(inner) == "layer:"+oldKey {
			b.WriteString("${layer:" + newKey + "}")
			changed = true
		} else {
			b.WriteString(rest[open : close+1])
		}
		rest = rest[close+1:]
	}
	b.WriteString(rest)

	if !changed {
		return s, false
	}
	return b.String(), true
}
