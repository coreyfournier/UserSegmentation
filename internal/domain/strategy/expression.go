package strategy

import (
	"fmt"
	"math"
	"sync"

	"github.com/expr-lang/expr"
	"github.com/segmentation-service/segmentation/internal/domain/model"
)

type runFn func(env interface{}) (interface{}, error)

// mathFn1 wraps a single-argument float64 → float64 math function for expr-lang.
// It reuses the package-level toFloat64 from operators.go for numeric coercion.
func mathFn1(name string, f func(float64) float64) expr.Option {
	return expr.Function(name, func(args ...any) (any, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("%s: expected 1 argument", name)
		}
		x, ok := toFloat64(args[0])
		if !ok {
			return nil, fmt.Errorf("%s: expected number, got %T", name, args[0])
		}
		return f(x), nil
	})
}

// mathOptions registers math functions not provided by expr-lang's built-in library.
// These are compiled into every expression program so callers can use exp(), ln(), etc.
var mathOptions = []expr.Option{
	mathFn1("exp",   math.Exp),
	mathFn1("ln",    math.Log),
	mathFn1("log2",  math.Log2),
	mathFn1("log10", math.Log10),
	mathFn1("sin",   math.Sin),
	mathFn1("cos",   math.Cos),
	expr.Function("pow", func(args ...any) (any, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("pow: expected 2 arguments")
		}
		base, ok1 := toFloat64(args[0])
		exp, ok2 := toFloat64(args[1])
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("pow: expected numbers")
		}
		return math.Pow(base, exp), nil
	}),
}

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
	program, err := expr.Compile(expression, mathOptions...)
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

	enrichedCtx := &EvalContext{
		SubjectKey:      ctx.SubjectKey,
		Context:         enriched,
		Languages:       ctx.Languages,
		RenderAll:       ctx.RenderAll,
		DefaultLanguage: ctx.DefaultLanguage,
	}
	res, ok := (&RuleStrategy{}).Evaluate(seg, enrichedCtx)
	if ok && len(computed) > 0 {
		res.Expressions = computed
	}
	return res, ok
}
