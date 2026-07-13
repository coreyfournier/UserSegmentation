package engine

import (
	"fmt"
	"sort"
	"time"

	"github.com/segmentation-service/segmentation/internal/domain/model"
	"github.com/segmentation-service/segmentation/internal/domain/strategy"
	"github.com/segmentation-service/segmentation/internal/domain/validation"
)

// Evaluator is the core domain service that evaluates layers in order.
type Evaluator struct {
	strategies map[string]strategy.Strategy
}

// NewEvaluator creates an evaluator with the given strategy implementations.
func NewEvaluator(strategies map[string]strategy.Strategy) *Evaluator {
	return &Evaluator{strategies: strategies}
}

// LayerResult holds the assignment for a single layer.
type LayerResult struct {
	Assignment *model.Assignment
	Warnings   []model.Warning
}

// EvalResult holds the full evaluation result across all layers.
type EvalResult struct {
	Layers   map[string]*model.Assignment
	Warnings []model.Warning
}

// Evaluate evaluates a subject across the specified layers (or all if filterLayers is nil).
// languages and renderAll control localized message rendering on the winning
// rule/override/default of each layer.
func (e *Evaluator) Evaluate(snap *model.Snapshot, subjectKey string, ctx map[string]interface{}, filterLayers []string, languages []string, renderAll bool, now time.Time) *EvalResult {
	result := &EvalResult{
		Layers: make(map[string]*model.Assignment, len(snap.Layers)),
	}

	// Sort layers by order
	layers := make([]model.Layer, len(snap.Layers))
	copy(layers, snap.Layers)
	sort.Slice(layers, func(i, j int) bool {
		return layers[i].Order < layers[j].Order
	})

	// Build filter set
	var filterSet map[string]struct{}
	if len(filterLayers) > 0 {
		filterSet = make(map[string]struct{}, len(filterLayers))
		for _, name := range filterLayers {
			filterSet[name] = struct{}{}
		}
	}

	// Copy context to avoid mutating the caller's map
	evalCtx := make(map[string]interface{}, len(ctx)+len(layers))
	for k, v := range ctx {
		evalCtx[k] = v
	}

	// Index lookup tables by id once for the whole evaluation.
	var lookups map[string]model.LookupTable
	if len(snap.Lookups) > 0 {
		lookups = make(map[string]model.LookupTable, len(snap.Lookups))
		for _, t := range snap.Lookups {
			lookups[t.ID] = t
		}
	}

	for _, layer := range layers {
		lr := e.evaluateLayer(&layer, subjectKey, evalCtx, languages, renderAll, lookups, now)

		// Inject cross-layer result regardless of filter
		if lr.Assignment != nil {
			evalCtx["layer:"+layer.Name] = lr.Assignment.Segment
		}

		// Only include in output if it passes the filter
		if filterSet != nil {
			if _, ok := filterSet[layer.Name]; !ok {
				continue
			}
		}

		if lr.Assignment != nil {
			result.Layers[layer.Name] = lr.Assignment
		}
		result.Warnings = append(result.Warnings, lr.Warnings...)
	}

	return result
}

func (e *Evaluator) evaluateLayer(layer *model.Layer, subjectKey string, ctx map[string]interface{}, languages []string, renderAll bool, lookups map[string]model.LookupTable, now time.Time) *LayerResult {
	lr := &LayerResult{}

	// Layer default language for message fallback; empty means English.
	defaultLang := layer.DefaultLanguage
	if defaultLang == "" {
		defaultLang = "en"
	}

	for i := range layer.Segments {
		seg := &layer.Segments[i]

		// Promotion time gating
		if !seg.Promotion.IsActive(now) {
			continue
		}

		// Check required fields and collect warnings
		lr.Warnings = append(lr.Warnings, validation.CheckRequiredFields(seg, ctx)...)

		evalCtx := &strategy.EvalContext{
			SubjectKey:      subjectKey,
			Context:         ctx,
			Languages:       languages,
			RenderAll:       renderAll,
			DefaultLanguage: defaultLang,
			Lookups:         lookups,
		}

		// Check overrides first
		if len(seg.Overrides) > 0 {
			if res, ok := strategy.EvalOverrides(seg.Overrides, evalCtx); ok {
				lr.Assignment = &model.Assignment{
					Segment:  res.Segment,
					Strategy: "override",
					Reason:   res.Reason,
					Messages: res.Messages,
				}
				lr.Warnings = append(lr.Warnings, renderWarnings(seg.ID, res.RenderErrors)...)
				return lr
			}
		}

		// Evaluate primary strategy
		strat, ok := e.strategies[seg.Strategy]
		if !ok {
			continue
		}
		if res, ok := strat.Evaluate(seg, evalCtx); ok {
			lr.Assignment = &model.Assignment{
				Segment:     res.Segment,
				Strategy:    seg.Strategy,
				Reason:      res.Reason,
				Expressions: res.Expressions,
				Messages:    res.Messages,
			}
			lr.Warnings = append(lr.Warnings, renderWarnings(seg.ID, res.RenderErrors)...)
			return lr
		}
	}

	return lr
}

// renderWarnings converts message render errors into layer warnings.
func renderWarnings(segmentID string, errs []strategy.RenderError) []model.Warning {
	if len(errs) == 0 {
		return nil
	}
	warnings := make([]model.Warning, 0, len(errs))
	for _, re := range errs {
		warnings = append(warnings, model.Warning{
			Segment: segmentID,
			Field:   re.Language,
			Message: fmt.Sprintf("message render error in %q: %s", re.Token, re.Err),
		})
	}
	return warnings
}
