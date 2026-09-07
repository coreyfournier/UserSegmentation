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

// ExprOptions returns the option set every expression in this engine compiles
// with. Validation compiles with these too: an env-constrained compile rejects
// pow(2, 3) without them, so a validator using a different set would reject
// config that runs perfectly well.
func ExprOptions() []expr.Option { return mathOptions }

// Formulas are compiled once and cached for the process. The cache is
// package-level so any RuleStrategy value benefits, including the throwaway
// ones created inline.
var (
	formulaCacheMu sync.Mutex
	formulaCache   = map[string]runFn{}
)

func compileFormula(source string) (runFn, error) {
	formulaCacheMu.Lock()
	defer formulaCacheMu.Unlock()
	if fn, ok := formulaCache[source]; ok {
		return fn, nil
	}
	program, err := expr.Compile(source, mathOptions...)
	if err != nil {
		return nil, err
	}
	fn := func(env interface{}) (interface{}, error) {
		return expr.Run(program, env)
	}
	formulaCache[source] = fn
	return fn, nil
}

// enrichWithComputed derives each named field in declaration order, so a later
// formula can reference an earlier result. Computed values shadow context fields
// of the same name.
//
// With no fields declared it returns the caller's map untouched — a rule segment
// that computes nothing pays no copy, which keeps the segmentation hot path as
// cheap as it was before computed fields folded into this strategy.
func enrichWithComputed(fields []model.ComputedField, base map[string]interface{}) (
	enriched map[string]interface{}, computed map[string]interface{}, failed []string,
) {
	if len(fields) == 0 {
		return base, nil, nil
	}

	enriched = make(map[string]interface{}, len(base)+len(fields))
	for k, v := range base {
		enriched[k] = v
	}

	computed = make(map[string]interface{}, len(fields))
	for _, def := range fields {
		run, err := compileFormula(def.Formula)
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
	return enriched, computed, failed
}
