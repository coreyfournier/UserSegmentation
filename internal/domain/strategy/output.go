package strategy

import (
	"fmt"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// evaluateOutputs resolves a segment's declared output fields for one reported
// item. A value authored on the item wins; otherwise the segment-level value
// stands in, which is how fields that do not vary per item are declared once.
//
// A field whose value fails to render or evaluate is omitted and an error is
// recorded — the item still reports, because evidence failing must not take the
// finding with it.
func evaluateOutputs(seg *model.Segment, itemOutputs map[string]string, ctx *EvalContext) (map[string]interface{}, []RenderError) {
	if len(seg.OutputSchema) == 0 {
		return nil, nil
	}

	out := make(map[string]interface{}, len(seg.OutputSchema))
	var errs []RenderError

	for name, decl := range seg.OutputSchema {
		raw, ok := itemOutputs[name]
		if !ok || raw == "" {
			raw, ok = seg.Outputs[name]
		}
		if !ok || raw == "" {
			continue
		}

		var value interface{}
		switch decl.EvalMode() {
		case model.EvalExpression:
			fn, err := compileFormula(raw)
			if err != nil {
				errs = append(errs, RenderError{Token: raw, Err: err.Error()})
				continue
			}
			v, err := fn(ctx.Context)
			if err != nil {
				errs = append(errs, RenderError{Token: raw, Err: err.Error()})
				continue
			}
			value = v
		case model.EvalTemplate:
			rendered, bad := renderTemplate(raw, ctx.Context)
			for _, te := range bad {
				errs = append(errs, RenderError{
					Language: ctx.DefaultLanguage,
					Token:    te.token,
					Err:      te.err,
				})
			}
			value = rendered
		default:
			value = raw
		}

		if decl.Lookup != "" {
			value = enrichLookupValue(decl.Lookup, value, ctx)
		}
		out[name] = value
	}

	if len(out) == 0 {
		return nil, errs
	}
	return out, errs
}

// enrichLookupValue turns an authored key into the {key, value} shape a consumer
// displays with, adding order when the table emits it. An unknown key — or an
// unknown table — passes through unchanged: membership is the author's
// invariant, recorded in the table description, not enforced here.
func enrichLookupValue(tableID string, key interface{}, ctx *EvalContext) interface{} {
	table, ok := ctx.Lookups[tableID]
	if !ok {
		return key
	}
	for _, e := range table.Entries {
		if fmt.Sprint(e.Key) != fmt.Sprint(key) {
			continue
		}
		enriched := map[string]interface{}{"key": e.Key, "value": e.Value}
		if table.EmitOrder {
			enriched["order"] = e.Order
		}
		return enriched
	}
	return key
}
