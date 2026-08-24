package engine

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/segmentation-service/segmentation/internal/domain/model"
	"github.com/segmentation-service/segmentation/internal/domain/strategy"
	"github.com/segmentation-service/segmentation/internal/domain/validation"
)

// Evaluator is the core domain service that evaluates layers as a dependency graph.
type Evaluator struct {
	strategies map[string]strategy.Strategy
}

// NewEvaluator creates an evaluator with the given strategy implementations.
func NewEvaluator(strategies map[string]strategy.Strategy) *Evaluator {
	return &Evaluator{strategies: strategies}
}

// LayerResult holds the outcome for a single layer.
type LayerResult struct {
	Status     model.LayerStatus
	Assignment *model.Assignment
	Failures   []model.Failure
	Warnings   []model.Warning
}

// EvalResult holds the full evaluation result across all layers.
type EvalResult struct {
	Layers   map[string]*LayerResult
	Warnings []model.Warning
}

// Evaluate evaluates a subject across the layer graph.
//
// Execution order comes from each layer's DependsOn edges, not from any ordinal
// field. When filterLayers is set, only those layers and everything they
// transitively depend on are evaluated; the rest never run.
func (e *Evaluator) Evaluate(snap *model.Snapshot, subjectKey string, ctx map[string]interface{}, filterLayers []string, languages []string, renderAll bool, now time.Time) *EvalResult {
	result := &EvalResult{
		Layers: make(map[string]*LayerResult, len(snap.Layers)),
	}

	ordered, err := topoSort(snap.Layers)
	if err != nil {
		// Cycles and dangling edges are rejected at config load. If one reaches
		// here the snapshot is unusable, so report rather than evaluate a
		// partial graph.
		result.Warnings = append(result.Warnings, model.Warning{Message: err.Error()})
		return result
	}

	// Build the output filter, and the wider execution scope it implies.
	var filterSet map[string]struct{}
	if len(filterLayers) > 0 {
		filterSet = make(map[string]struct{}, len(filterLayers))
		for _, name := range filterLayers {
			filterSet[name] = struct{}{}
		}
	}
	scope := dependencyClosure(ordered, filterLayers)

	// Copy context to avoid mutating the caller's map
	evalCtx := make(map[string]interface{}, len(ctx)+len(ordered))
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

	statuses := make(map[string]model.LayerStatus, len(ordered))

	for i := range ordered {
		layer := &ordered[i]

		if scope != nil {
			if _, ok := scope[layer.Name]; !ok {
				continue
			}
		}

		var lr *LayerResult
		if blocker, blocked := blockedBy(layer, statuses); blocked {
			lr = &LayerResult{Status: skippedStatus(layer)}
			lr.Warnings = append(lr.Warnings, model.Warning{
				Segment: layer.Name,
				Field:   blocker,
				Message: fmt.Sprintf("layer skipped: dependency %q did not resolve", blocker),
			})
		} else {
			lr = e.evaluateLayer(layer, subjectKey, evalCtx, languages, renderAll, lookups, now)
		}
		statuses[layer.Name] = lr.Status

		// Inject the resolved value for downstream layers. Assert layers resolve
		// no value — dependents gate on status instead.
		if lr.Assignment != nil && lr.Assignment.Segment != "" {
			evalCtx["layer:"+layer.Name] = lr.Assignment.Segment
		}

		// Only include in output if it passes the filter
		if filterSet != nil {
			if _, ok := filterSet[layer.Name]; !ok {
				continue
			}
		}

		result.Layers[layer.Name] = lr
		result.Warnings = append(result.Warnings, lr.Warnings...)
	}

	return result
}

func (e *Evaluator) evaluateLayer(layer *model.Layer, subjectKey string, ctx map[string]interface{}, languages []string, renderAll bool, lookups map[string]model.LookupTable, now time.Time) *LayerResult {
	lr := &LayerResult{Status: unresolvedStatus(layer)}

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

		// Dispatch predicate. A segment that does not apply is passed over
		// entirely and produces no output of any kind — it is not a state.
		if seg.When != nil && !strategy.EvalRule(seg.When, ctx, lookups) {
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
				lr.Status = model.StatusResolved
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
		// A strategy named in config but absent from the composition root would
		// otherwise make the segment produce nothing at all, silently. Config
		// validation cannot catch this — the name is legal, the wiring is not.
		strat, ok := e.strategies[seg.Strategy]
		if !ok {
			lr.Warnings = append(lr.Warnings, model.Warning{
				Segment: seg.ID,
				Field:   seg.Strategy,
				Message: fmt.Sprintf("strategy %q is not registered; segment skipped", seg.Strategy),
			})
			continue
		}
		if res, ok := strat.Evaluate(seg, evalCtx); ok {
			lr.Status = res.Status
			if lr.Status == "" {
				lr.Status = model.StatusResolved
			}
			lr.Failures = res.Failures
			lr.Assignment = &model.Assignment{
				Segment:     res.Segment,
				Strategy:    seg.Strategy,
				Reason:      res.Reason,
				Computed: res.Computed,
				Messages:    res.Messages,
			}
			lr.Warnings = append(lr.Warnings, renderWarnings(seg.ID, res.RenderErrors)...)
			return lr
		}
	}

	return lr
}

// topoSort orders layers so every layer follows the layers it depends on.
// Ready layers are taken in name order so execution is reproducible.
func topoSort(layers []model.Layer) ([]model.Layer, error) {
	byName := make(map[string]*model.Layer, len(layers))
	indegree := make(map[string]int, len(layers))
	dependents := make(map[string][]string, len(layers))

	for i := range layers {
		l := &layers[i]
		if _, dup := byName[l.Name]; dup {
			return nil, fmt.Errorf("duplicate layer name %q", l.Name)
		}
		byName[l.Name] = l
		indegree[l.Name] = 0
	}

	for i := range layers {
		l := &layers[i]
		for _, dep := range l.DependsOn {
			if _, ok := byName[dep]; !ok {
				return nil, fmt.Errorf("layer %q depends on unknown layer %q", l.Name, dep)
			}
			indegree[l.Name]++
			dependents[dep] = append(dependents[dep], l.Name)
		}
	}

	ready := make([]string, 0, len(layers))
	for name, deg := range indegree {
		if deg == 0 {
			ready = append(ready, name)
		}
	}
	sort.Strings(ready)

	out := make([]model.Layer, 0, len(layers))
	for len(ready) > 0 {
		name := ready[0]
		ready = ready[1:]
		out = append(out, *byName[name])

		freed := false
		for _, dependent := range dependents[name] {
			indegree[dependent]--
			if indegree[dependent] == 0 {
				ready = append(ready, dependent)
				freed = true
			}
		}
		if freed {
			sort.Strings(ready)
		}
	}

	if len(out) != len(layers) {
		return nil, errors.New("layer dependency cycle detected")
	}
	return out, nil
}

// dependencyClosure returns the set of layers that must execute to satisfy the
// requested ones — the requests plus everything they transitively depend on.
// A nil result means "no filter: evaluate everything".
func dependencyClosure(layers []model.Layer, requested []string) map[string]struct{} {
	if len(requested) == 0 {
		return nil
	}

	byName := make(map[string]*model.Layer, len(layers))
	for i := range layers {
		byName[layers[i].Name] = &layers[i]
	}

	scope := make(map[string]struct{}, len(requested))
	var visit func(string)
	visit = func(name string) {
		if _, seen := scope[name]; seen {
			return
		}
		l, ok := byName[name]
		if !ok {
			return // unknown layer requested: contributes nothing, as before
		}
		scope[name] = struct{}{}
		for _, dep := range l.DependsOn {
			visit(dep)
		}
	}
	for _, name := range requested {
		visit(name)
	}
	return scope
}

// blockedBy reports the first dependency that did not resolve successfully.
// A dependency succeeds when a checklist layer is satisfied, or any other layer
// produced an assignment. Anything else — violated, unevaluable, unresolved,
// skipped — blocks, so a gate never runs against state an earlier gate failed
// to establish.
func blockedBy(layer *model.Layer, statuses map[string]model.LayerStatus) (string, bool) {
	for _, dep := range layer.DependsOn {
		switch statuses[dep] {
		case model.StatusSatisfied, model.StatusResolved:
			continue
		default:
			return dep, true
		}
	}
	return "", false
}

// isChecklistLayer reports whether a layer speaks the checklist vocabulary. It
// is determined from config alone so a skipped layer still reports the right
// status without being evaluated.
func isChecklistLayer(layer *model.Layer) bool {
	for i := range layer.Segments {
		if layer.Segments[i].Strategy == model.StrategyChecklist {
			return true
		}
	}
	return false
}

// skippedStatus is for a layer that never ran because a dependency did not
// resolve. Its outcome is genuinely unknown, so it blocks readiness.
func skippedStatus(layer *model.Layer) model.LayerStatus {
	if isChecklistLayer(layer) {
		return model.StatusUnevaluable
	}
	return model.StatusSkipped
}

// unresolvedStatus is for a layer that ran but matched no segment — every
// segment's Applies When excluded this subject, or their promotion windows are
// closed.
//
// For a checklist that is satisfied, not unevaluable: no check applied, so no
// problem was found. Reporting unevaluable would block readiness for every
// subject a conditional layer simply does not cover, which is indistinguishable
// from a gate that could not be judged.
func unresolvedStatus(layer *model.Layer) model.LayerStatus {
	if isChecklistLayer(layer) {
		return model.StatusSatisfied
	}
	return model.StatusUnresolved
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
