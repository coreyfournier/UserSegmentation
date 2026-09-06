package validation

import (
	"fmt"
	"sort"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/segmentation-service/segmentation/internal/domain/model"
	"github.com/segmentation-service/segmentation/internal/domain/strategy"
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

		// An input field may bind to a lookup, declaring its domain so the
		// condition editor can offer that table's keys. The two checks mirror
		// the output side exactly: the table must exist, and its key type must
		// agree with the field's, because a binding that cannot agree is a
		// mistake rather than a risk an author is deliberately taking. Whether
		// an incoming value is actually one of the keys stays unchecked, as it
		// is everywhere else lookups are used.
		for _, name := range sortedSchemaFields(layer.InputSchema) {
			f := layer.InputSchema[name]
			if f.Lookup == "" {
				continue
			}
			tbl, ok := lookups[f.Lookup]
			if !ok {
				errs = append(errs, fmt.Sprintf(
					"layer %q input %q: lookup %q does not exist", layer.Key, name, f.Lookup))
			} else if f.Type != tbl.KeyType {
				errs = append(errs, fmt.Sprintf(
					"layer %q input %q: field type %q does not match lookup %q key type %q",
					layer.Key, name, f.Type, tbl.Name, tbl.KeyType))
			}
		}

		// A legacy "eval" key lives on the layer's OutputSchema field, not on
		// any one segment, so it is checked here, once per field — not inside
		// the segment loop below, which would name the wrong owner (a
		// segment, N times over for a layer with N segments) and would miss
		// it entirely for a layer with zero segments (CreateLayer starts a
		// new layer with none, which the UI reaches before any segment is
		// added).
		for name, f := range layer.OutputSchema {
			if f.LegacyEval != "" {
				errs = append(errs, fmt.Sprintf(
					"layer %q output %q: \"eval\" is no longer declared — the mode is derived "+
						"from the type, so a string is a template and everything else is an "+
						"expression; remove it",
					layer.Key, name))
			}
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

			// Schemas live on the layer. A segment still carrying one is config
			// written against the old shape; rejecting it beats decoding it to
			// nothing and silently disabling rule-field validation.
			if len(seg.LegacyInputSchema) > 0 {
				errs = append(errs, fmt.Sprintf(
					"segment %q: inputSchema is declared on the layer now, not the segment — "+
						"move these %d field(s) to layer %q",
					seg.ID, len(seg.LegacyInputSchema), layer.Key))
			}
			if len(seg.LegacyOutputSchema) > 0 {
				errs = append(errs, fmt.Sprintf(
					"segment %q: outputSchema is declared on the layer now, not the segment — "+
						"move these %d field(s) to layer %q",
					seg.ID, len(seg.LegacyOutputSchema), layer.Key))
			}

			// Build the effective schema: layer's inputSchema fields + expression-defined fields.
			// This happens unconditionally — validateOutputSchema needs it too,
			// below — but env (the expr compile-time environment) is built only
			// when the layer actually declares an inputSchema. With no declared
			// fields there is nothing to check a token against, so the escape
			// hatch documented on Layer.InputSchema applies to token validation
			// the same way it applies to rule-field validation: skip it entirely
			// rather than reject every token in the layer.
			effective := buildEffectiveSchema(layer.InputSchema, seg.Computed)
			var env map[string]interface{}
			if layer.InputSchema != nil {
				env = envFromSchema(effective)
			}

			errs = append(errs, validateOutputSchema(&seg, layer.OutputSchema, lookups, effective, env)...)

			// Formulas are syntax-checked wherever they are declared. Gating
			// this on the strategy used to mean a typo on a segment that never
			// ran it was accepted, and a genuine typo on one that did became a
			// runtime unevaluable instead of a load failure.
			for _, def := range seg.Computed {
				if _, err := expr.Compile(def.Formula); err != nil {
					errs = append(errs, fmt.Sprintf("segment %q formula %q: %v", seg.ID, def.Name, err))
				}
			}

			// The segment's fallback default-language messages, checked the same
			// way a rule's own messages are — see validateRuleTree.
			if env != nil {
				for _, lang := range sortedKeys(seg.DefaultMessages) {
					where := fmt.Sprintf("layer %q segment %q defaultMessages[%s]", layer.Key, seg.ID, lang)
					errs = append(errs, validateTemplateTokens(where, seg.DefaultMessages[lang], effective, env)...)
				}
			}

			if layer.InputSchema == nil && len(seg.Computed) == 0 {
				continue
			}

			vc := ruleContext{
				schema:  effective,
				layer:   layer.Key,
				segment: seg.ID,
				deps:    deps,
				lookups: lookups,
				env:     env,
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

// WarnMissingInputSchemas returns one advisory line per layer whose segments'
// rule fields go unvalidated because the layer declares no inputSchema. This
// is deliberately not an error — the escape hatch stays (several shipped
// layers rely on it) — but a config author or operator should be able to see
// that it is in effect rather than discover it the hard way when a typoed
// field silently passes.
//
// A segment that supplies its own Computed fields is still validated against
// those (buildEffectiveSchema works with an empty inputSchema), so it is not
// counted here even when its layer has no inputSchema.
func WarnMissingInputSchemas(snap *model.Snapshot) []string {
	var warnings []string
	for _, layer := range snap.Layers {
		if layer.InputSchema != nil {
			continue
		}
		var affected int
		for _, seg := range layer.Segments {
			if len(seg.Computed) > 0 {
				continue
			}
			if len(seg.Rules) > 0 || len(seg.Overrides) > 0 || seg.When != nil {
				affected++
			}
		}
		if affected > 0 {
			warnings = append(warnings, fmt.Sprintf(
				"layer %q: no inputSchema — rule fields are not validated for its %d segment(s)",
				layer.Key, affected))
		}
	}
	return warnings
}

// buildEffectiveSchema merges the layer's inputSchema with any expression-defined
// fields declared on a segment. Expression fields overwrite inputSchema entries
// with the same name.
func buildEffectiveSchema(inSchema model.InputSchema, computed []model.ComputedField) model.InputSchema {
	effective := make(model.InputSchema, len(inSchema)+len(computed))
	for k, v := range inSchema {
		effective[k] = v
	}
	for _, def := range computed {
		effective[def.Name] = model.SchemaField{Type: def.Type}
	}
	return effective
}

// envFromSchema builds a compile-time environment from the effective schema so
// expr can report an unknown identifier. Values are zero values of the declared
// type; only the names and shapes matter here.
//
// TODO: an array or object field carries no declared element or member shape —
// SchemaField is {Type, Required} and nothing more — so a token that reaches
// inside one cannot be checked past its top-level name. ${employees} validates,
// ${employees[0].name} and ${payload.nested} do not, and both are accepted
// unchecked rather than falsely rejected. Closing this needs a nested schema
// type, which is a larger change than this validation.
func envFromSchema(schema model.InputSchema) map[string]interface{} {
	env := make(map[string]interface{}, len(schema))
	for name, f := range schema {
		env[name] = zeroValueForFieldType(f.Type)
	}
	return env
}

// zeroValueForFieldType returns a representative zero value for a declared
// field type — enough for expr to type-check identifier usage, nothing more.
func zeroValueForFieldType(t model.FieldType) interface{} {
	switch t {
	case model.FieldTypeNumber:
		return float64(0)
	case model.FieldTypeBoolean:
		return false
	case model.FieldTypeArray:
		return []interface{}{}
	case model.FieldTypeObject:
		return map[string]interface{}{}
	default: // FieldTypeString, and anything unrecognized
		return ""
	}
}

// templateToken is one ${ ... } span found by scanTemplateTokens.
type templateToken struct {
	// expr is the trimmed text between the delimiters — what gets resolved as
	// a declared field first and evaluated as an expr-lang expression on
	// fallback, mirroring resolveTokenValue in internal/domain/strategy/message.go.
	expr string
}

// scanTemplateTokens mirrors renderTemplate's own scan in
// internal/domain/strategy/message.go byte for byte: a token spans from "${"
// to the next "}", and an unterminated "${" (no closing "}") ends the scan
// with the remainder left out entirely — renderTemplate treats it as literal
// text to emit as-is, not a token, and this must agree. The two are not
// shared code (validation and strategy have no import relationship that would
// make one a natural home for the other without either exporting rendering
// internals or making validation depend on the runtime package for a few
// lines of string scanning), so if renderTemplate's scanning rule ever
// changes, this must change with it.
func scanTemplateTokens(tmpl string) []templateToken {
	var toks []templateToken
	i := 0
	for i < len(tmpl) {
		rel := strings.Index(tmpl[i:], "${")
		if rel < 0 {
			break
		}
		start := i + rel
		relEnd := strings.Index(tmpl[start+2:], "}")
		if relEnd < 0 {
			// Unterminated token: the rest is literal text, not a token.
			break
		}
		end := start + 2 + relEnd
		toks = append(toks, templateToken{expr: strings.TrimSpace(tmpl[start+2 : end])})
		i = end + 1
	}
	return toks
}

// validateTemplateTokens reports any ${…} token that names something the schema
// does not declare, or that does not compile.
//
// A declared field wins first, exactly as it does at evaluation: the context is
// a flat map whose keys may contain dots, so "company.ein" is one key rather
// than member access, and rejecting it here would forbid the naming convention
// this config uses throughout.
//
// Compilation uses strategy.ExprOptions() — the runtime's own set. An
// env-constrained compile rejects pow(2, 3) without them, and a validator that
// rejects working config is worse than the gap it closes.
//
// where is a caller-formatted description of the token's location, used as
// the error prefix; schema and env must be the same effective schema and its
// derived environment (see envFromSchema) used to validate the rest of the
// segment, so a declared field is recognized the same way everywhere.
func validateTemplateTokens(where, tmpl string, schema model.InputSchema, env map[string]interface{}) []string {
	if tmpl == "" {
		return nil
	}
	var errs []string
	opts := append([]expr.Option{expr.Env(env)}, strategy.ExprOptions()...)
	for _, tok := range scanTemplateTokens(tmpl) {
		if _, declared := schema[tok.expr]; declared {
			continue
		}
		// A cross-layer reference resolves at evaluation the same way a field
		// does: the evaluator injects "layer:<name>" as a flat context key, and
		// ResolveField finds it before expr is ever consulted. Without this the
		// check would reject "${layer:base-tier}" on a colon parse error while
		// the runtime renders it perfectly — the false-rejection direction, and
		// worse than the gap this validation closes. Whether the dependency is
		// actually declared is validateRuleTree's job, which already reports it
		// against dependsOn.
		if strings.HasPrefix(tok.expr, "layer:") {
			continue
		}
		if _, err := expr.Compile(tok.expr, opts...); err != nil {
			errs = append(errs, fmt.Sprintf("%s: token %q: %v", where, "${"+tok.expr+"}", err))
		}
	}
	return errs
}

// sortedKeys returns m's keys in sorted order, for stable diagnostic output —
// map iteration is not ordered.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ruleContext carries everything a rule tree is validated against.
type ruleContext struct {
	schema  model.InputSchema
	layer   string
	segment string
	deps    map[string]struct{}
	lookups map[string]model.LookupTable
	// env is the expr compile-time environment derived from schema, or nil
	// when the layer declares no inputSchema. Message-token validation is
	// skipped entirely when env is nil — see validateTemplateTokens' escape
	// hatch — while field/lookup validation below still runs whenever there
	// is an effective schema to check against (including a computed-only one).
	env map[string]interface{}
}

func validateRuleTree(r *model.Rule, vc ruleContext) []string {
	var errs []string

	// Message templates are checked at every node, leaf or composite — a
	// composite carries the same ErrorMessage/Messages fields a leaf does, and
	// both are validated regardless of whether every current runtime path
	// renders them, so a nested rule's typo cannot hide behind depth.
	if vc.env != nil {
		where := fmt.Sprintf("layer %q segment %q rule %q errorMessage", vc.layer, vc.segment, r.RuleName)
		errs = append(errs, validateTemplateTokens(where, r.ErrorMessage, vc.schema, vc.env)...)
		for _, lang := range sortedKeys(r.Messages) {
			where := fmt.Sprintf("layer %q segment %q rule %q messages[%s]", vc.layer, vc.segment, r.RuleName, lang)
			errs = append(errs, validateTemplateTokens(where, r.Messages[lang], vc.schema, vc.env)...)
		}
	}

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
		// Config written before the key existed carries only a name. Say so
		// outright, with the key it would derive, rather than reporting the
		// generic "key is required" and leaving the author to guess the
		// convention.
		if l.Key == "" && l.Name != "" {
			if derived := model.DeriveLayerKey(l.Name); derived != "" {
				errs = append(errs, fmt.Sprintf(
					"layer %q has no key — add \"key\": %q (the name is now the friendly label)",
					l.Name, derived))
				continue
			}
		}
		if msg := model.ValidateLayerKey(l.Key); msg != "" {
			errs = append(errs, fmt.Sprintf("layer %q: %s", l.Key, msg))
			continue
		}
		if _, dup := byName[l.Key]; dup {
			errs = append(errs, fmt.Sprintf("duplicate layer key %q", l.Key))
			continue
		}
		byName[l.Key] = struct{}{}
	}

	for _, l := range layers {
		seen := make(map[string]struct{}, len(l.DependsOn))
		for _, dep := range l.DependsOn {
			switch {
			case dep == l.Key:
				errs = append(errs, fmt.Sprintf("layer %q depends on itself", l.Key))
			case !contains(byName, dep):
				errs = append(errs, fmt.Sprintf("layer %q depends on unknown layer %q", l.Key, dep))
			}
			if _, dup := seen[dep]; dup {
				errs = append(errs, fmt.Sprintf("layer %q declares duplicate dependency %q", l.Key, dep))
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
		deps[l.Key] = l.DependsOn
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
		if color[l.Key] == white && visit(l.Key) {
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

// CheckRequiredFields returns warnings for required schema fields missing from
// context. schema is the enclosing layer's InputSchema — the only place it is
// declared.
func CheckRequiredFields(seg *model.Segment, schema model.InputSchema, ctx map[string]interface{}) []model.Warning {
	if schema == nil {
		return nil
	}
	var warnings []model.Warning
	for field, sf := range schema {
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
// what a segment actually emitted. schema is the enclosing layer's
// OutputSchema — the only place it is declared.
//
// Config validation already rejects a required field that no authoring path
// supplies — including on an override rule — so reaching here means something
// ran, a value was authored for it, and it still did not arrive. In practice
// that means the value's expression or template failed and the field was
// dropped, which is deliberate degradation and not recoverable at load. So the
// caller is told and decides.
func CheckRequiredOutputs(seg *model.Segment, schema model.OutputSchema, a *model.Assignment, failures []model.Failure) []model.Warning {
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
	for name, f := range schema {
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

// outputEnforcementExempt reports whether seg's own strategy exempts it from
// output-schema binding beyond the lookup-existence and expression-syntax
// checks: static and percentage never populate Result.Outputs (only the rule
// and checklist paths do), so nothing they declare on the segment itself is
// binding — the same reasoning requiredOutputErrors already applies to
// Required. An override firing on one of these segments is the deliberate
// exception (EvalOverrides calls evaluateOutputs regardless of the segment's
// own strategy), but that distinction lives at runtime (CheckRequiredOutputs);
// at load time there is no fired-or-not to key off of, so — matching
// requiredOutputErrors' own unconditional early return — the exemption here
// is unconditional too.
func outputEnforcementExempt(seg *model.Segment) bool {
	return seg.Strategy == model.StrategyStatic || seg.Strategy == model.StrategyPercentage
}

// validateOutputSchema checks a referenced lookup table exists, a lookup-bound
// field's declared type matches the table's key type, every authored key is
// actually declared, every non-string (i.e. expression-mode) field's authored
// value compiles as a syntactically valid expression, every string (i.e.
// template-mode) field's authored value has only tokens the schema declares,
// and every Required field is actually authored. (A legacy "eval" key is a
// layer-level concern, checked once per field in ValidateSnapshot's layer loop
// rather than here.) Lookup membership and order uniqueness remain the
// author's invariants and are deliberately not checked. schema is the
// enclosing layer's OutputSchema — the only place it is declared. inSchema
// and env are the segment's effective input schema and its derived expr
// environment (nil when the layer declares no inputSchema — see
// envFromSchema and the escape hatch it defers to); output values render
// against the same context a rule condition does, so they are checked
// against the same schema.
func validateOutputSchema(seg *model.Segment, schema model.OutputSchema, lookups map[string]model.LookupTable, inSchema model.InputSchema, env map[string]interface{}) []string {
	var errs []string

	// Name-checking is not per-field — it is what tells us an authored key has
	// no field to check against in the first place — so it runs once up front
	// rather than inside the per-field loop below.
	errs = append(errs, unknownOutputNameErrors(seg, schema)...)

	for name, f := range schema {
		if f.Lookup != "" {
			tbl, ok := lookups[f.Lookup]
			if !ok {
				errs = append(errs, fmt.Sprintf(
					"segment %q output %q: lookup %q does not exist",
					seg.ID, name, f.Lookup))
			} else if !outputEnforcementExempt(seg) && f.Type != tbl.KeyType {
				// Mirrors validateLookupRef's own condition-side check: a
				// declared type that cannot agree with the table it is bound
				// to is caught at load rather than emitting a bare key with
				// no value or order at runtime (enrichLookupValue).
				errs = append(errs, fmt.Sprintf(
					"segment %q output %q: field type %q does not match lookup %q key type %q",
					seg.ID, name, f.Type, f.Lookup, tbl.KeyType))
			}
		}
		if f.IsTemplate() {
			// Same escape hatch as everywhere else a token is checked: with no
			// declared inputSchema there is nothing to check a token against.
			if env != nil {
				errs = append(errs, validateOutputTemplateTokens(seg, name, inSchema, env)...)
			}
		} else {
			errs = append(errs, validateOutputExpressionSyntax(seg, name, env)...)
		}
		if f.Required {
			errs = append(errs, requiredOutputErrors(seg, name)...)
		}
	}
	return errs
}

// unknownOutputNameErrors rejects an authored output key that the layer's
// outputSchema does not declare, at every authoring site. This is the same
// class as the unknown-strategy rejection at the top of this file: config
// that can never do anything, because evaluateOutputs iterates the schema —
// not what was authored — so an undeclared key is silently dropped with no
// diagnostic at runtime. Lookup-membership-style checks these are not: a
// declared field with an unlisted value is the author's invariant and stays
// unchecked; this is about a key that has no field to check against at all.
func unknownOutputNameErrors(seg *model.Segment, schema model.OutputSchema) []string {
	if outputEnforcementExempt(seg) {
		return nil
	}
	var errs []string
	for _, m := range outputAuthoringMaps(seg) {
		names := make([]string, 0, len(m.values))
		for name := range m.values {
			names = append(names, name)
		}
		sort.Strings(names) // stable output; map iteration is not ordered

		for _, name := range names {
			if _, declared := schema[name]; declared {
				continue
			}
			switch m.kind {
			case siteSegment:
				errs = append(errs, fmt.Sprintf(
					"segment %q: output %q is not declared in outputSchema",
					seg.ID, name))
			case siteRule:
				errs = append(errs, fmt.Sprintf(
					"segment %q rule %q: output %q is not declared in outputSchema",
					seg.ID, m.name, name))
			case siteOverride:
				errs = append(errs, fmt.Sprintf(
					"segment %q override %q: output %q is not declared in outputSchema",
					seg.ID, m.name, name))
			}
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

// outputAuthoringMap is one place output values are authored on a segment:
// the segment-level fallback every rule and override shares, or one enabled
// top-level rule's or override's own values. name is the rule's/override's
// RuleName; it is empty for the segment-level site.
type outputAuthoringMap struct {
	kind   outputAuthoringSiteKind
	name   string
	values map[string]string
}

// outputAuthoringMaps enumerates every place values are authored on seg, in a
// fixed order: the segment-level fallback every rule shares, then each
// enabled top-level rule, then each enabled override. Disabled rules and
// overrides are omitted entirely so a work-in-progress item cannot wedge an
// unrelated save — the same exemption requiredOutputErrors and
// validateOutputExpressionSyntax have always applied, now enforced in one
// place instead of twice, and shared by the name check as well.
func outputAuthoringMaps(seg *model.Segment) []outputAuthoringMap {
	maps := make([]outputAuthoringMap, 0, 1+len(seg.Rules)+len(seg.Overrides))

	maps = append(maps, outputAuthoringMap{kind: siteSegment, values: seg.Outputs})

	for i := range seg.Rules {
		r := &seg.Rules[i]
		if !r.IsEnabled() {
			continue
		}
		maps = append(maps, outputAuthoringMap{kind: siteRule, name: r.RuleName, values: r.Outputs})
	}

	for i := range seg.Overrides {
		r := &seg.Overrides[i]
		if !r.IsEnabled() {
			continue
		}
		maps = append(maps, outputAuthoringMap{kind: siteOverride, name: r.RuleName, values: r.Outputs})
	}

	return maps
}

// outputAuthoringSites enumerates every place field could be authored on seg,
// in the same fixed order as outputAuthoringMaps, reading each map for just
// this one field.
//
// ok mirrors evaluateOutputs' own presence check exactly (strategy/output.go):
// a value only counts as authored when the key is present *and* non-empty. A
// gate that tested key presence alone could be satisfied by a bare empty
// string, which the runtime treats as unauthored — silently exempting every
// rule and override in the segment from a required-field check that looks
// like it passed.
func outputAuthoringSites(seg *model.Segment, field string) []outputAuthoringSite {
	maps := outputAuthoringMaps(seg)
	sites := make([]outputAuthoringSite, 0, len(maps))
	for _, m := range maps {
		v, ok := m.values[field]
		sites = append(sites, outputAuthoringSite{kind: m.kind, name: m.name, value: v, ok: ok && v != ""})
	}
	return sites
}

// validateOutputExpressionSyntax syntax-checks every authored value for an
// expression-mode output field, mirroring the existing formula check: a
// segment-level value (the fallback every rule shares), each enabled
// top-level rule's own value, and each enabled override's own value, wherever
// one is authored.
//
// env constrains the compile to the effective input schema's declared names,
// exactly like validateTemplateTokens — so a typo such as MaxAllowd (for a
// declared MaxAllowed) is caught here instead of resolving to nil at runtime
// (see evaluateOutputs). env is nil when the layer declares no inputSchema;
// compiling without expr.Env then accepts any identifier, the same escape
// hatch token validation applies, so an output expression in that layer is
// not falsely rejected for referencing a context field nothing declares.
func validateOutputExpressionSyntax(seg *model.Segment, name string, env map[string]interface{}) []string {
	var errs []string
	opts := strategy.ExprOptions()
	if env != nil {
		opts = append(append([]expr.Option{}, opts...), expr.Env(env))
	}
	for _, s := range outputAuthoringSites(seg, name) {
		if !s.ok {
			continue
		}
		if _, err := expr.Compile(s.value, opts...); err != nil {
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

// validateOutputTemplateTokens checks every authored value for a
// template-mode (string) output field against the effective input schema,
// using the same site enumeration validateOutputExpressionSyntax does: a
// segment-level value (the fallback every rule shares), each enabled
// top-level rule's own value, and each enabled override's own value, wherever
// one is authored. Called only when env is non-nil (see validateOutputSchema).
func validateOutputTemplateTokens(seg *model.Segment, name string, schema model.InputSchema, env map[string]interface{}) []string {
	var errs []string
	for _, s := range outputAuthoringSites(seg, name) {
		if !s.ok {
			continue
		}
		var where string
		switch s.kind {
		case siteSegment:
			where = fmt.Sprintf("segment %q output %q", seg.ID, name)
		case siteRule:
			where = fmt.Sprintf("segment %q rule %q output %q", seg.ID, s.name, name)
		case siteOverride:
			where = fmt.Sprintf("segment %q override %q output %q", seg.ID, s.name, name)
		}
		errs = append(errs, validateTemplateTokens(where, s.value, schema, env)...)
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
	if outputEnforcementExempt(seg) {
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

// sortedSchemaFields returns an input schema's field names in sorted order, so
// diagnostics come out the same way twice — map iteration does not.
func sortedSchemaFields(s model.InputSchema) []string {
	names := make([]string, 0, len(s))
	for n := range s {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
