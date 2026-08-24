package strategy

import (
	"fmt"
	"math"
	"strings"
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

// ComputedStrategy evaluates named expr-lang expressions to enrich the context,
// then delegates to rule evaluation against the enriched context.
type ComputedStrategy struct {
	mu    sync.Mutex
	cache map[string]runFn
}

func (s *ComputedStrategy) compiled(expression string) (runFn, error) {
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

func (s *ComputedStrategy) Evaluate(seg *model.Segment, ctx *EvalContext) (Result, bool) {
	// Copy caller's context, then overwrite with expression results in declaration order.
	enriched := make(map[string]interface{}, len(ctx.Context)+len(seg.Computed))
	for k, v := range ctx.Context {
		enriched[k] = v
	}

	computed := make(map[string]interface{}, len(seg.Computed))
	var failed []string
	for _, def := range seg.Computed {
		run, err := s.compiled(def.Formula)
		if err != nil {
			failed = append(failed, def.Name)
			continue
		}
		val, err := run(enriched)
		if err != nil {
			failed = append(failed, def.Name)
			continue
		}
		enriched[def.Name] = val
		computed[def.Name] = val
	}

	// Under collection a failed computation must not fall through to rule
	// evaluation. The rules consuming that field would evaluate false and be
	// reported as violations, telling the resolver a value is wrong when it
	// could not in fact be computed. Segmentation keeps the old behavior — the
	// computed field is simply absent.
	if ctx.CollectFailures && len(failed) > 0 {
		return Result{
			Reason:      "expression error: " + strings.Join(failed, ", "),
			Status:      model.StatusUnevaluable,
			Computed: computed,
		}, true
	}

	// Copy the struct rather than rebuilding it field by field, so fields added
	// later (Lookups, CollectFailures) cannot be silently dropped here.
	derived := *ctx
	derived.Context = enriched

	res, ok := (&RuleStrategy{}).Evaluate(seg, &derived)
	if ok && len(computed) > 0 {
		res.Computed = computed
	}
	return res, ok
}
