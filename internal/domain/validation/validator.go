package validation

import (
	"fmt"
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

			// Validate expression syntax. Checklist carries expressions too, so
			// it must be included — otherwise a checklist segment loses
			// compile-time checking and a config typo becomes a runtime
			// unevaluable.
			if seg.Strategy == model.StrategyExpression || seg.Strategy == model.StrategyChecklist {
				for _, def := range seg.Expressions {
					if _, err := expr.Compile(def.Expression); err != nil {
						errs = append(errs, fmt.Sprintf("segment %q expression %q: %v", seg.ID, def.Name, err))
					}
				}
			}

			if seg.InputSchema == nil && len(seg.Expressions) == 0 {
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
	effective := make(model.InputSchema, len(seg.InputSchema)+len(seg.Expressions))
	for k, v := range seg.InputSchema {
		effective[k] = v
	}
	for _, def := range seg.Expressions {
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
		field := r.Expression.Field

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
		if !model.OperatorSupportsType(r.Expression.Operator, sf.Type) {
			errs = append(errs, fmt.Sprintf("segment %q rule %q: operator %q not compatible with type %q for field %q",
				vc.segment, r.RuleName, r.Expression.Operator, sf.Type, field))
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
	op := r.Expression.Operator
	if op != model.OpInLookup && op != model.OpNotInLookup {
		return nil
	}
	id, ok := r.Expression.Value.(string)
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
