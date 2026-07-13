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

	for _, layer := range snap.Layers {
		for _, seg := range layer.Segments {
			// Validate expression syntax for expression-strategy segments.
			if seg.Strategy == "expression" {
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

			// Validate rules and overrides against the effective schema.
			for _, r := range seg.Rules {
				if err := validateRuleTree(&r, effective, seg.ID, lookups); err != nil {
					errs = append(errs, err...)
				}
			}
			for _, r := range seg.Overrides {
				if err := validateRuleTree(&r, effective, seg.ID, lookups); err != nil {
					errs = append(errs, err...)
				}
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

func validateRuleTree(r *model.Rule, schema model.InputSchema, segID string, lookups map[string]model.LookupTable) []string {
	var errs []string
	if r.IsLeaf() {
		field := r.Expression.Field
		// Cross-layer refs are always valid at config time
		if strings.HasPrefix(field, "layer:") {
			return nil
		}
		sf, ok := schema[field]
		if !ok {
			errs = append(errs, fmt.Sprintf("segment %q rule %q: field %q not in inputSchema", segID, r.RuleName, field))
			return errs
		}
		if !model.OperatorSupportsType(r.Expression.Operator, sf.Type) {
			errs = append(errs, fmt.Sprintf("segment %q rule %q: operator %q not compatible with type %q for field %q",
				segID, r.RuleName, r.Expression.Operator, sf.Type, field))
		}
		errs = append(errs, validateLookupRef(r, sf.Type, segID, lookups)...)
		return errs
	}
	for i := range r.Rules {
		errs = append(errs, validateRuleTree(&r.Rules[i], schema, segID, lookups)...)
	}
	return errs
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
			if _, ok := ctx[field]; !ok {
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
