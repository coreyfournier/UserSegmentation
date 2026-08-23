package strategy

import (
	"sort"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// RuleStrategy evaluates composite rule trees.
//
// Default mode: the first matching rule's successEvent wins, and evaluation
// short-circuits. This is the segmentation hot path — a fifty-rule layer that
// matches on rule three does no further work.
//
// Collect mode (ctx.CollectFailures, set by AssertStrategy): the semantics
// invert. Every rule is an assertion that must hold, the whole tree is walked,
// and each assertion that does not hold is itemised.
type RuleStrategy struct{}

func (s *RuleStrategy) Evaluate(seg *model.Segment, ctx *EvalContext) (Result, bool) {
	if ctx.CollectFailures {
		return collectAssertions(seg, ctx), true
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

// collectAssertions treats every rule in the segment as an assertion that must
// hold, and reports each one that does not. It always succeeds — an assert
// segment has no notion of "no rule matched".
func collectAssertions(seg *model.Segment, ctx *EvalContext) Result {
	res := Result{Reason: "assert:" + seg.ID}
	for i := range seg.Rules {
		collectRuleFailures(&seg.Rules[i], ctx, &res)
	}
	return res
}

// collectRuleFailures walks the entire rule tree, appending a failure for every
// assertion that does not hold. Unlike evaluateRule it never short-circuits —
// itemising all of a gate's problems at once is the point.
func collectRuleFailures(r *model.Rule, ctx *EvalContext, res *Result) {
	// A rule that is switched off, or whose When predicate does not hold,
	// contributes nothing — no failure, and no effect on its parent. This is how
	// one condition governs a whole block of checks: gate the group, and every
	// check inside it is still reported individually when the group does apply.
	if !active(r, ctx.Context, ctx.Lookups) {
		return
	}

	if r.IsLeaf() {
		if !EvalExpression(r.Expression, ctx.Context, ctx.Lookups) {
			appendFailure(res, r, ctx)
		}
		return
	}

	switch r.Operator {
	case model.CompositeAnd:
		// Every child must hold; report each one that does not.
		for i := range r.Rules {
			collectRuleFailures(&r.Rules[i], ctx, res)
		}

	case model.CompositeOr:
		// Any branch holding satisfies the node. When none do, report the Or
		// node itself rather than each branch — listing every branch would tell
		// the resolver to set all of them when any one would have done.
		// evaluateRule already skips branches that are off or inapplicable.
		for i := range r.Rules {
			if evaluateRule(&r.Rules[i], ctx.Context, ctx.Lookups) {
				return
			}
		}
		appendFailure(res, r, ctx)

	default:
		appendFailure(res, r, ctx)
	}
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

// ruleApplies reports whether a rule's When predicate holds. A rule with no
// predicate always applies.
//
// This is the data-dependent counterpart to Enabled, and is skipped in the same
// places: a rule that does not apply neither satisfies nor fails its parent.
func ruleApplies(r *model.Rule, ctx map[string]interface{}, lookups map[string]model.LookupTable) bool {
	if r.When == nil {
		return true
	}
	return evaluateRule(r.When, ctx, lookups)
}

// active reports whether a child should take part in its parent's outcome.
func active(r *model.Rule, ctx map[string]interface{}, lookups map[string]model.LookupTable) bool {
	return r.IsEnabled() && ruleApplies(r, ctx, lookups)
}

// evaluateRule recursively evaluates a rule node.
func evaluateRule(r *model.Rule, ctx map[string]interface{}, lookups map[string]model.LookupTable) bool {
	if !active(r, ctx, lookups) {
		return false
	}
	if r.IsLeaf() {
		return EvalExpression(r.Expression, ctx, lookups)
	}
	// Composite rule
	switch r.Operator {
	case model.CompositeAnd:
		for i := range r.Rules {
			child := &r.Rules[i]
			if !active(child, ctx, lookups) {
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
			if !active(child, ctx, lookups) {
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
