package validation

import (
	"fmt"
	"sort"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// ValidateSnapshot validates all rules against their inputSchemas at config load time.
func ValidateSnapshot(snap *model.Snapshot) error {
	var errs []string

	// Validate lookup tables and build an id->table index for reference checks.
	lookups := make(map[string]model.LookupTable, len(snap.Lookups))
	errs = append(errs, validateLookups(snap.Lookups, lookups)...)

	// The dependency graph must be usable before anything that reads it.
	errs = append(errs, validateLayerGraph(snap.Layers)...)

	for _, layer := range snap.Layers {
		deps := make(map[string]struct{}, len(layer.DependsOn))
		for _, d := range layer.DependsOn {
			deps[d] = struct{}{}
		}

		for _, seg := range layer.Segments {
			// An unknown strategy is silently skipped by the evaluator, so the
			// segment would just never produce anything. Reject it at load.
			if !model.IsKnownStrategy(seg.Strategy) {
				errs = append(errs, fmt.Sprintf("segment %q: unknown strategy %q (expected one of %s)",
					seg.ID, seg.Strategy, strings.Join(model.KnownStrategies, ", ")))
			}

			// A checklist has no "nothing matched" outcome, so an override has
			// nothing to override. Worse, EvalOverrides resolves a segment value
			// and the evaluator reports StatusResolved — both outside the
			// checklist vocabulary, so a consumer switching on status meets a
			// case it was told could not happen. The segment editor already tells
			// authors a checklist has no overrides and renders no editor for them
			// (SegmentEditor.tsx); this makes the engine agree, closing the raw
			// JSON, admin API and hand-edited config paths that bypass the UI.
			if seg.Strategy == model.StrategyChecklist && len(seg.Overrides) > 0 {
				errs = append(errs, fmt.Sprintf(
					"segment %q: a checklist cannot declare overrides — an override resolves a "+
						"segment value and reports %q, which is not part of the checklist "+
						"vocabulary (%q, %q, %q)",
					seg.ID, model.StatusResolved,
					model.StatusSatisfied, model.StatusViolated, model.StatusUnevaluable))
			}

			errs = append(errs, validateOutputSchema(&seg, lookups)...)

			// Formulas are syntax-checked wherever they are declared. Gating
			// this on the strategy used to mean a typo on a segment that never
			// ran it was accepted, and a genuine typo on one that did became a
			// runtime unevaluable instead of a load failure.
			for _, def := range seg.Computed {
				if _, err := expr.Compile(def.Formula); err != nil {
					errs = append(errs, fmt.Sprintf("segment %q formula %q: %v", seg.ID, def.Name, err))
				}
			}

			if seg.InputSchema == nil && len(seg.Computed) == 0 {
				continue
			}

			// Build the effective schema: inputSchema fields + expression-defined fields.
			effective := buildEffectiveSchema(seg)
			vc := ruleContext{
				schema:  effective,
				layer:   layer.Name,
				segment: seg.ID,
				deps:    deps,
				lookups: lookups,
			}

			// Validate rules, overrides and the dispatch predicate.
			for _, r := range seg.Rules {
				errs = append(errs, validateRuleTree(&r, vc)...)
			}
			for _, r := range seg.Overrides {
				errs = append(errs, validateRuleTree(&r, vc)...)
			}
			if seg.When != nil {
				errs = append(errs, validateRuleTree(seg.When, vc)...)
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

// buildEffectiveSchema merges the segment's inputSchema with any expression-defined fields.
// Expression fields overwrite inputSchema entries with the same name.
func buildEffectiveSchema(seg model.Segment) model.InputSchema {
	effective := make(model.InputSchema, len(seg.InputSchema)+len(seg.Computed))
	for k, v := range seg.InputSchema {
		effective[k] = v
	}
	for _, def := range seg.Computed {
		effective[def.Name] = model.SchemaField{Type: def.Type}
	}
	return effective
}

// ruleContext carries everything a rule tree is validated against.
type ruleContext struct {
	schema  model.InputSchema
	layer   string
	segment string
	deps    map[string]struct{}
	lookups map[string]model.LookupTable
}

func validateRuleTree(r *model.Rule, vc ruleContext) []string {
	var errs []string
	if r.IsLeaf() {
		field := r.Condition.Field

		// A cross-layer reference must be declared as a dependency. Without
		// this, a typo or a reference to a layer that runs later passes config
		// validation and then evaluates false forever.
		if ref, ok := strings.CutPrefix(field, "layer:"); ok {
			if _, declared := vc.deps[ref]; !declared {
				errs = append(errs, fmt.Sprintf(
					"layer %q segment %q rule %q: references %q but %q is not declared in dependsOn",
					vc.layer, vc.segment, r.RuleName, field, ref))
			}
			return errs
		}

		sf, ok := vc.schema[field]
		if !ok {
			errs = append(errs, fmt.Sprintf("segment %q rule %q: field %q not in inputSchema", vc.segment, r.RuleName, field))
			return errs
		}
		if !model.OperatorSupportsType(r.Condition.Operator, sf.Type) {
			errs = append(errs, fmt.Sprintf("segment %q rule %q: operator %q not compatible with type %q for field %q",
				vc.segment, r.RuleName, r.Condition.Operator, sf.Type, field))
		}
		errs = append(errs, validateLookupRef(r, sf.Type, vc.segment, vc.lookups)...)
		return errs
	}
	for i := range r.Rules {
		errs = append(errs, validateRuleTree(&r.Rules[i], vc)...)
	}
	return errs
}

// validateLayerGraph checks that dependency edges form a usable DAG: unique
// layer names, every edge resolving to a real layer, and no cycles.
func validateLayerGraph(layers []model.Layer) []string {
	var errs []string

	byName := make(map[string]struct{}, len(layers))
	for _, l := range layers {
		if l.Name == "" {
			errs = append(errs, "layer with empty name")
			continue
		}
		if _, dup := byName[l.Name]; dup {
			errs = append(errs, fmt.Sprintf("duplicate layer name %q", l.Name))
			continue
		}
		byName[l.Name] = struct{}{}
	}

	for _, l := range layers {
		seen := make(map[string]struct{}, len(l.DependsOn))
		for _, dep := range l.DependsOn {
			switch {
			case dep == l.Name:
				errs = append(errs, fmt.Sprintf("layer %q depends on itself", l.Name))
			case !contains(byName, dep):
				errs = append(errs, fmt.Sprintf("layer %q depends on unknown layer %q", l.Name, dep))
			}
			if _, dup := seen[dep]; dup {
				errs = append(errs, fmt.Sprintf("layer %q declares duplicate dependency %q", l.Name, dep))
			}
			seen[dep] = struct{}{}
		}
	}

	// Only look for cycles once the edges are known to resolve.
	if len(errs) == 0 {
		if cycle := findCycle(layers); len(cycle) > 0 {
			errs = append(errs, fmt.Sprintf("layer dependency cycle: %s", strings.Join(cycle, " -> ")))
		}
	}
	return errs
}

func contains(set map[string]struct{}, key string) bool {
	_, ok := set[key]
	return ok
}

// findCycle returns one cycle in the dependency graph, as a readable path.
func findCycle(layers []model.Layer) []string {
	deps := make(map[string][]string, len(layers))
	for _, l := range layers {
		deps[l.Name] = l.DependsOn
	}

	const (
		white = iota // unvisited
		grey         // on the current path
		black        // fully explored
	)
	color := make(map[string]int, len(layers))
	var path, cycle []string

	var visit func(string) bool
	visit = func(name string) bool {
		color[name] = grey
		path = append(path, name)

		for _, dep := range deps[name] {
			switch color[dep] {
			case grey:
				for i, n := range path {
					if n == dep {
						cycle = append(append([]string{}, path[i:]...), dep)
						break
					}
				}
				return true
			case white:
				if visit(dep) {
					return true
				}
			}
		}

		path = path[:len(path)-1]
		color[name] = black
		return false
	}

	for _, l := range layers {
		if color[l.Name] == white && visit(l.Name) {
			return cycle
		}
	}
	return nil
}

// validateLookupRef checks a lookup-operator expression: its value must name an
// existing table, and the field type must match the table's key type.
func validateLookupRef(r *model.Rule, fieldType model.FieldType, segID string, lookups map[string]model.LookupTable) []string {
	op := r.Condition.Operator
	if op != model.OpInLookup && op != model.OpNotInLookup {
		return nil
	}
	id, ok := r.Condition.Value.(string)
	if !ok || id == "" {
		return []string{fmt.Sprintf("segment %q rule %q: operator %q requires a lookup table id as value",
			segID, r.RuleName, op)}
	}
	tbl, exists := lookups[id]
	if !exists {
		return []string{fmt.Sprintf("segment %q rule %q: references unknown lookup table %q",
			segID, r.RuleName, id)}
	}
	if fieldType != tbl.KeyType {
		return []string{fmt.Sprintf("segment %q rule %q: field type %q does not match lookup %q key type %q",
			segID, r.RuleName, fieldType, id, tbl.KeyType)}
	}
	return nil
}

// CheckRequiredFields returns warnings for required schema fields missing from context.
func CheckRequiredFields(seg *model.Segment, ctx map[string]interface{}) []model.Warning {
	if seg.InputSchema == nil {
		return nil
	}
	var warnings []model.Warning
	for field, sf := range seg.InputSchema {
		if sf.Required {
			if _, ok := model.ResolveField(ctx, field); !ok {
				warnings = append(warnings, model.Warning{
					Segment: seg.ID,
					Field:   field,
					Message: "required field missing from context",
				})
			}
		}
	}
	return warnings
}

// CheckRequiredOutputs returns warnings for required output fields absent from
// what a segment actually emitted.
//
// Config validation already rejects a required field that no authoring path
// supplies — including on an override rule — so reaching here means something
// ran, a value was authored for it, and it still did not arrive. In practice
// that means the value's expression or template failed and the field was
// dropped, which is deliberate degradation and not recoverable at load. So the
// caller is told and decides.
func CheckRequiredOutputs(seg *model.Segment, a *model.Assignment, failures []model.Failure) []model.Warning {
	// Output-schema enforcement does not apply to the static or percentage
	// strategies: neither ever populates Result.Outputs, so a required field
	// on one of them would warn on every evaluation with no config change
	// able to silence it (see requiredOutputErrors). The exception is an
	// override that fired — identified by a.Strategy == "override" — because
	// an override does resolve outputs even on a static/percentage segment,
	// and should still be checked like any other override.
	if (seg.Strategy == model.StrategyStatic || seg.Strategy == model.StrategyPercentage) &&
		(a == nil || a.Strategy != "override") {
		return nil
	}

	var required []string
	for name, f := range seg.OutputSchema {
		if f.Required {
			required = append(required, name)
		}
	}
	if len(required) == 0 {
		return nil
	}
	sort.Strings(required) // stable output; map iteration is not ordered

	var warnings []model.Warning
	missing := func(field, detail string) {
		warnings = append(warnings, model.Warning{
			Segment: seg.ID,
			Field:   field,
			Message: "required output field absent from emitted record: " + detail,
		})
	}

	// Where outputs live depends on the strategy, not on whether anything
	// happened to be reported. A checklist's ChecklistStrategy never populates
	// Result.Outputs — and so never populates Assignment.Outputs — because its
	// outputs live per-Failure instead; every other strategy sets them on the
	// Assignment. Branching on len(failures) > 0 got this backwards: a
	// satisfied checklist (nothing violated) or an unevaluable one (a computed
	// field's formula failed, so collectViolations never ran) both report zero
	// failures and would fall through to the a.Outputs[name] check below, which
	// is structurally always empty for a checklist — spuriously warning on
	// every healthy evaluation. And the "a == nil" guard below cannot catch
	// this: ChecklistStrategy.Evaluate always succeeds, so evaluateLayer always
	// builds a non-nil lr.Assignment for it, checklist or not.
	if seg.Strategy == model.StrategyChecklist {
		for _, f := range failures {
			for _, name := range required {
				if _, ok := f.Outputs[name]; !ok {
					missing(name, fmt.Sprintf("finding %q did not emit it", f.Rule))
				}
			}
		}
		return warnings
	}

	if a == nil {
		return nil // nothing reported, so no record is missing anything
	}
	for _, name := range required {
		if _, ok := a.Outputs[name]; !ok {
			detail := "the segment did not emit it"
			if a.Strategy == "override" {
				detail = "the override that resolved this segment did not emit it, " +
					"most likely because its value failed to resolve"
			}
			missing(name, detail)
		}
	}
	return warnings
}

// validateOutputSchema checks a referenced lookup table exists, the object type
// is confined to expression mode, and every Required field is actually authored.
// Lookup membership and order uniqueness remain the author's invariants and are
// deliberately not checked.
func validateOutputSchema(seg *model.Segment, lookups map[string]model.LookupTable) []string {
	var errs []string
	for name, f := range seg.OutputSchema {
		if f.Lookup != "" {
			if _, ok := lookups[f.Lookup]; !ok {
				errs = append(errs, fmt.Sprintf(
					"segment %q output %q: lookup %q does not exist",
					seg.ID, name, f.Lookup))
			}
		}
		if f.Type == model.FieldTypeObject && f.EvalMode() != model.EvalExpression {
			errs = append(errs, fmt.Sprintf(
				"segment %q output %q: the object type requires eval \"expression\"",
				seg.ID, name))
		}
		if f.EvalMode() == model.EvalExpression {
			errs = append(errs, validateOutputExpressionSyntax(seg, name)...)
		}
		if f.Required {
			errs = append(errs, requiredOutputErrors(seg, name)...)
		}
	}
	return errs
}

// outputAuthoringSiteKind identifies where an output field's value was (or
// could have been) authored.
type outputAuthoringSiteKind int

const (
	siteSegment outputAuthoringSiteKind = iota
	siteRule
	siteOverride
)

// outputAuthoringSite is one place a field's value can be authored: the
// segment-level fallback every rule shares, or one enabled top-level rule's
// or override's own value. name is the rule's/override's RuleName; it is
// empty for the segment-level site.
//
// ok mirrors evaluateOutputs' own presence check exactly (strategy/output.go):
// a value only counts as authored when the key is present *and* non-empty. A
// gate that tested key presence alone could be satisfied by a bare empty
// string, which the runtime treats as unauthored — silently exempting every
// rule and override in the segment from a required-field check that looks
// like it passed.
type outputAuthoringSite struct {
	kind  outputAuthoringSiteKind
	name  string
	value string
	ok    bool
}

// outputAuthoringSites enumerates every place field could be authored on seg,
// in a fixed order: the segment-level site first, then each enabled top-level
// rule, then each enabled override. Disabled rules and overrides are omitted
// entirely so a work-in-progress item cannot wedge an unrelated save — the
// same exemption requiredOutputErrors and validateOutputExpressionSyntax have
// always applied, now enforced in one place instead of twice.
func outputAuthoringSites(seg *model.Segment, field string) []outputAuthoringSite {
	sites := make([]outputAuthoringSite, 0, 1+len(seg.Rules)+len(seg.Overrides))

	v, ok := seg.Outputs[field]
	sites = append(sites, outputAuthoringSite{kind: siteSegment, value: v, ok: ok && v != ""})

	for i := range seg.Rules {
		r := &seg.Rules[i]
		if !r.IsEnabled() {
			continue
		}
		v, ok := r.Outputs[field]
		sites = append(sites, outputAuthoringSite{kind: siteRule, name: r.RuleName, value: v, ok: ok && v != ""})
	}

	for i := range seg.Overrides {
		r := &seg.Overrides[i]
		if !r.IsEnabled() {
			continue
		}
		v, ok := r.Outputs[field]
		sites = append(sites, outputAuthoringSite{kind: siteOverride, name: r.RuleName, value: v, ok: ok && v != ""})
	}

	return sites
}

// validateOutputExpressionSyntax syntax-checks every authored value for an
// expression-mode output field, mirroring the existing formula check: a
// segment-level value (the fallback every rule shares), each enabled
// top-level rule's own value, and each enabled override's own value, wherever
// one is authored.
func validateOutputExpressionSyntax(seg *model.Segment, name string) []string {
	var errs []string
	for _, s := range outputAuthoringSites(seg, name) {
		if !s.ok {
			continue
		}
		if _, err := expr.Compile(s.value); err != nil {
			switch s.kind {
			case siteSegment:
				errs = append(errs, fmt.Sprintf("segment %q output %q: %v", seg.ID, name, err))
			case siteRule:
				errs = append(errs, fmt.Sprintf("segment %q rule %q output %q: %v", seg.ID, s.name, name, err))
			case siteOverride:
				errs = append(errs, fmt.Sprintf("segment %q override %q output %q: %v", seg.ID, s.name, name, err))
			}
		}
	}
	return errs
}

// requiredOutputErrors reports every authoring path that would leave a required
// output field unset.
//
// A segment-level value covers every path at once, which is the intended way to
// satisfy a field that does not vary per item. Failing that, each enabled
// top-level rule must set it, and so must each enabled override — an override
// that fires replaces the strategy result entirely, so it carries the same
// reporting obligation as a rule. Disabled rules and overrides are both exempt
// so a work-in-progress item cannot wedge an unrelated save. A declared Default
// has no rule to read from at all (the default branch calls evaluateOutputs
// with nil item values), so only a segment-level value can satisfy it.
//
// Output-schema enforcement does not apply to the static or percentage
// strategies: neither ever populates Result.Outputs (only the rule and
// checklist paths do), so a required field on one of them is unsatisfiable no
// matter what is authored, and would warn on every evaluation with no config
// change able to silence it. Rather than reject config that can never be
// satisfied, these strategies are simply exempted from enforcement.
func requiredOutputErrors(seg *model.Segment, name string) []string {
	if seg.Strategy == model.StrategyStatic || seg.Strategy == model.StrategyPercentage {
		return nil
	}

	sites := outputAuthoringSites(seg, name)

	// sites[0] is always the segment-level site: a value there satisfies
	// every rule and override at once.
	if sites[0].ok {
		return nil
	}

	var errs []string
	// Only the rule strategy reads Segment.Default: a checklist delegates to
	// RuleStrategy but returns from collectViolations before the default
	// branch. So a checklist carrying a stray, inert Default must not be
	// gated here — it is never evaluated.
	if seg.Default != "" && seg.Strategy == model.StrategyRule {
		errs = append(errs, fmt.Sprintf(
			"segment %q output %q: required, and a default is declared, so it must be set in "+
				"the segment's outputs — the default path reads no rule values",
			seg.ID, name))
	}
	for _, s := range sites[1:] {
		if s.ok {
			continue
		}
		switch s.kind {
		case siteRule:
			errs = append(errs, fmt.Sprintf(
				"segment %q rule %q: required output %q has no value (set it on the rule, "+
					"or once in the segment's outputs)",
				seg.ID, s.name, name))
		case siteOverride:
			errs = append(errs, fmt.Sprintf(
				"segment %q override %q: required output %q has no value (set it on the "+
					"override, or once in the segment's outputs)",
				seg.ID, s.name, name))
		}
	}
	return errs
}
