package strategy

import (
	"sync"

	"github.com/expr-lang/expr"
	"github.com/segmentation-service/segmentation/internal/domain/model"
)

type runFn func(env interface{}) (interface{}, error)

// ExpressionStrategy evaluates named expr-lang expressions to enrich the context,
// then delegates to rule evaluation against the enriched context.
type ExpressionStrategy struct {
	mu    sync.Mutex
	cache map[string]runFn
}

func (s *ExpressionStrategy) compiled(expression string) (runFn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cache == nil {
		s.cache = make(map[string]runFn)
	}
	if fn, ok := s.cache[expression]; ok {
		return fn, nil
	}
	program, err := expr.Compile(expression)
	if err != nil {
		return nil, err
	}
	fn := func(env interface{}) (interface{}, error) {
		return expr.Run(program, env)
	}
	s.cache[expression] = fn
	return fn, nil
}

func (s *ExpressionStrategy) Evaluate(seg *model.Segment, ctx *EvalContext) (Result, bool) {
	// Copy caller's context, then overwrite with expression results in declaration order.
	enriched := make(map[string]interface{}, len(ctx.Context)+len(seg.Expressions))
	for k, v := range ctx.Context {
		enriched[k] = v
	}

	computed := make(map[string]interface{}, len(seg.Expressions))
	for _, def := range seg.Expressions {
		run, err := s.compiled(def.Expression)
		if err != nil {
			continue
		}
		val, err := run(enriched)
		if err != nil {
			continue
		}
		enriched[def.Name] = val
		computed[def.Name] = val
	}

	enrichedCtx := &EvalContext{SubjectKey: ctx.SubjectKey, Context: enriched}
	res, ok := (&RuleStrategy{}).Evaluate(seg, enrichedCtx)
	if ok && len(computed) > 0 {
		res.Expressions = computed
	}
	return res, ok
}
