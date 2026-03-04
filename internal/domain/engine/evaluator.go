package engine

import (
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

// Evaluate evaluates a user across the specified layers (or all if filterLayers is nil).
func (e *Evaluator) Evaluate(snap *model.Snapshot, userKey string, ctx map[string]interface{}, filterLayers []string, now time.Time) *EvalResult {
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

	for _, layer := range layers {
		lr := e.evaluateLayer(&layer, userKey, evalCtx, now)

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

func (e *Evaluator) evaluateLayer(layer *model.Layer, userKey string, ctx map[string]interface{}, now time.Time) *LayerResult {
	lr := &LayerResult{}

	for i := range layer.Segments {
		seg := &layer.Segments[i]

		// Promotion time gating
		if !seg.Promotion.IsActive(now) {
			continue
		}

		// Check required fields and collect warnings
		lr.Warnings = append(lr.Warnings, validation.CheckRequiredFields(seg, ctx)...)

		evalCtx := &strategy.EvalContext{
			UserKey: userKey,
			Context: ctx,
		}

		// Check overrides first
		if len(seg.Overrides) > 0 {
			if res, ok := strategy.EvalOverrides(seg.Overrides, evalCtx); ok {
				lr.Assignment = &model.Assignment{
					Segment:  res.Segment,
					Strategy: "override",
					Reason:   res.Reason,
				}
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
				Segment:  res.Segment,
				Strategy: seg.Strategy,
				Reason:   res.Reason,
			}
			return lr
		}
	}

	return lr
}
