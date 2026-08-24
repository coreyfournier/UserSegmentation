package strategy

import (
	"sort"
	"strings"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// RuleStrategy evaluates composite rule trees, optionally deriving computed
// fields first.
//
// Computed fields are part of this strategy rather than a separate one, because
// a strategy that computes nothing is indistinguishable from plain rule
// evaluation. Keeping them apart meant declaring `computed` on a `rule` segment
// was silently dead config: the formulas never ran, yet validation accepted
// rules referencing them because it merged their names into the schema anyway.
//
// Default mode: the first matching rule's successEvent wins, and evaluation
// short-circuits. This is the segmentation hot path — a fifty-rule layer that
// matches on rule three does no further work, and a segment with no computed
// fields copies no maps.
//
// Collect mode (ctx.CollectFailures, set by ChecklistStrategy): every rule is
// evaluated instead of stopping at the first match, and each one that matches
// is reported. A rule means the same thing in both modes — it fires when its
// condition holds — only what happens on a match differs.
type RuleStrategy struct{}

func (s *RuleStrategy) Evaluate(seg *model.Segment, ctx *EvalContext) (Result, bool) {
	enriched, computed, failed := enrichWithComputed(seg.Computed, ctx.Context)

	// Under collection a failed computation must not fall through to rule
	// evaluation: the rules consuming that field would fire and be reported as
	// real problems, when in truth the value could not be computed. Plain
	// segmentation keeps the older, quieter behaviour — the field is absent.
	if ctx.CollectFailures && len(failed) > 0 {
		return Result{
			Reason:   "formula error: " + strings.Join(failed, ", "),
			Status:   model.StatusUnevaluable,
			Computed: computed,
		}, true
	}

	evalCtx := ctx
	if len(seg.Computed) > 0 {
		derived := *ctx
		derived.Context = enriched
		evalCtx = &derived
	}

	res, ok := s.evaluateRules(seg, evalCtx)
	if ok && len(computed) > 0 {
		res.Computed = computed
	}
	return res, ok
}

func (s *RuleStrategy) evaluateRules(seg *model.Segment, ctx *EvalContext) (Result, bool) {
	if ctx.CollectFailures {
		return collectViolations(seg, ctx), true
	}

	for i := range seg.Rules {
		r := &seg.Rules[i]
		if !r.IsEnabled() {
			continue
		}
		if evaluateRule(r, ctx.Context, ctx.Lookups) {
			event := r.SuccessEvent
			if event == "" {
				event = r.RuleName
			}
			res := Result{Segment: event, Reason: "rule:" + r.RuleName}
			applyMessages(&res, r.Messages, ctx)
			return res, true
		}
	}
	if seg.Default != "" {
		res := Result{Segment: seg.Default, Reason: "rule:default"}
		applyMessages(&res, seg.DefaultMessages, ctx)
		return res, true
	}
	return Result{}, false
}

// collectViolations evaluates every rule in the segment and reports each one
// that matches. Each rule is one checklist item: its condition describes a
// problem, and its message states it.
//
// A rule fires on a match here exactly as it does under first-match evaluation,
// so the same config means the same thing under either strategy. And/Or build
// one item's condition — they are not a reporting structure — which is why this
// does not recurse: the tree below a rule decides whether that one item fires.
//
// It always succeeds; a checklist has no notion of "no rule matched".
func collectViolations(seg *model.Segment, ctx *EvalContext) Result {
	res := Result{Reason: "checklist:" + seg.ID}
	for i := range seg.Rules {
		r := &seg.Rules[i]
		if !r.IsEnabled() {
			continue
		}
		if evaluateRule(r, ctx.Context, ctx.Lookups) {
			appendFailure(&res, r, ctx)
		}
	}
	return res
}

// appendFailure renders the rule's message templates and records the failure.
func appendFailure(res *Result, r *model.Rule, ctx *EvalContext) {
	f := model.Failure{Rule: r.RuleName}

	// The message is the payload of a failure, so it is always populated. When
	// no plain errorMessage is set, fall back to the default-language template —
	// otherwise a rule that carries only localized messages would report a
	// failure with no readable text unless the caller happened to ask for a
	// language. Localized rendering below stays opt-in.
	template := r.ErrorMessage
	if template == "" {
		template = defaultLanguageMessage(r.Messages, ctx.DefaultLanguage)
	}

	if template != "" {
		rendered, bad := renderTemplate(template, ctx.Context)
		f.Message = rendered
		for _, te := range bad {
			res.RenderErrors = append(res.RenderErrors, RenderError{
				Language: ctx.DefaultLanguage,
				Token:    te.token,
				Err:      te.err,
			})
		}
	}

	if len(r.Messages) > 0 {
		rr := RenderMessages(r.Messages, ctx.Context, ctx.Languages, ctx.RenderAll, ctx.DefaultLanguage)
		if len(rr.Rendered) > 0 {
			f.Messages = rr.Rendered
		}
		res.RenderErrors = append(res.RenderErrors, rr.Errors...)
	}

	res.Failures = append(res.Failures, f)
}

// defaultLanguageMessage picks the template that stands in for a missing plain
// errorMessage: the layer's default language, then English, then whichever
// locale sorts first so the choice is stable across runs.
func defaultLanguageMessage(messages map[string]string, defaultLang string) string {
	if len(messages) == 0 {
		return ""
	}
	if defaultLang == "" {
		defaultLang = "en"
	}
	if t, ok := messages[defaultLang]; ok {
		return t
	}
	if t, ok := messages["en"]; ok {
		return t
	}

	langs := make([]string, 0, len(messages))
	for lang := range messages {
		langs = append(langs, lang)
	}
	sort.Strings(langs)
	return messages[langs[0]]
}

// applyMessages renders the raw localized templates against the eval context and
// attaches the rendered messages and any render errors to res.
func applyMessages(res *Result, raw map[string]string, ctx *EvalContext) {
	if len(raw) == 0 {
		return
	}
	rr := RenderMessages(raw, ctx.Context, ctx.Languages, ctx.RenderAll, ctx.DefaultLanguage)
	if len(rr.Rendered) > 0 {
		res.Messages = rr.Rendered
	}
	res.RenderErrors = append(res.RenderErrors, rr.Errors...)
}

// EvalRule evaluates a single rule tree with short-circuiting. Exposed for
// dispatch predicates (Segment.When) evaluated outside of strategy selection.
func EvalRule(r *model.Rule, ctx map[string]interface{}, lookups map[string]model.LookupTable) bool {
	return evaluateRule(r, ctx, lookups)
}

// evaluateRule recursively evaluates a rule node.
func evaluateRule(r *model.Rule, ctx map[string]interface{}, lookups map[string]model.LookupTable) bool {
	if !r.IsEnabled() {
		return false
	}
	if r.IsLeaf() {
		return EvalCondition(r.Condition, ctx, lookups)
	}
	// Composite rule
	switch r.Operator {
	case model.CompositeAnd:
		for i := range r.Rules {
			child := &r.Rules[i]
			if !child.IsEnabled() {
				continue
			}
			if !evaluateRule(child, ctx, lookups) {
				return false // short-circuit
			}
		}
		return true
	case model.CompositeOr:
		for i := range r.Rules {
			child := &r.Rules[i]
			if !child.IsEnabled() {
				continue
			}
			if evaluateRule(child, ctx, lookups) {
				return true // short-circuit
			}
		}
		return false
	default:
		return false
	}
}
