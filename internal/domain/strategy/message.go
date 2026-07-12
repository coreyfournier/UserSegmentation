package strategy

import (
	"fmt"
	"strings"
	"sync"

	"github.com/expr-lang/expr"
)

// RenderError describes a failed ${ ... } token interpolation in a message.
type RenderError struct {
	Language string `json:"language"`
	Token    string `json:"token"`
	Err      string `json:"err"`
}

// RenderResult holds rendered messages keyed by language and any interpolation errors.
type RenderResult struct {
	Rendered map[string]string
	Errors   []RenderError
}

var (
	msgCacheMu sync.Mutex
	msgCache   = map[string]runFn{}
)

// compileMessageExpr compiles (and caches) an expr-lang expression used inside a
// ${ ... } token. Reuses mathOptions so exp()/pow()/etc. are available.
func compileMessageExpr(expression string) (runFn, error) {
	msgCacheMu.Lock()
	defer msgCacheMu.Unlock()
	if fn, ok := msgCache[expression]; ok {
		return fn, nil
	}
	program, err := expr.Compile(expression, mathOptions...)
	if err != nil {
		return nil, err
	}
	fn := func(env interface{}) (interface{}, error) {
		return expr.Run(program, env)
	}
	msgCache[expression] = fn
	return fn, nil
}

// RenderMessages renders localized message templates against env.
//
//   - renderAll: render every locale defined in raw.
//   - otherwise: render each requested language, falling back to defaultLang's
//     message when the requested locale is absent; omit if neither exists.
//
// Each ${ ... } token is evaluated as an expr-lang expression. On error the raw
// token is left in place and a RenderError is recorded. defaultLang defaults to "en".
func RenderMessages(raw map[string]string, env map[string]interface{}, languages []string, renderAll bool, defaultLang string) RenderResult {
	res := RenderResult{}
	if len(raw) == 0 {
		return res
	}
	if defaultLang == "" {
		defaultLang = "en"
	}

	res.Rendered = make(map[string]string)

	if renderAll {
		for lang, tmpl := range raw {
			res.renderInto(lang, tmpl, env)
		}
		return res
	}

	seen := make(map[string]bool, len(languages))
	for _, lang := range languages {
		if lang == "" || seen[lang] {
			continue
		}
		seen[lang] = true
		tmpl, ok := raw[lang]
		if !ok {
			// Fall back to the layer default language's message.
			tmpl, ok = raw[defaultLang]
			if !ok {
				continue // no translation and no fallback: omit
			}
		}
		res.renderInto(lang, tmpl, env)
	}
	return res
}

// renderInto renders one template and records it under lang, collecting errors.
func (r *RenderResult) renderInto(lang, tmpl string, env map[string]interface{}) {
	rendered, bad := renderTemplate(tmpl, env)
	r.Rendered[lang] = rendered
	for _, te := range bad {
		r.Errors = append(r.Errors, RenderError{Language: lang, Token: te.token, Err: te.err})
	}
}

type tokenErr struct {
	token string
	err   string
}

// renderTemplate substitutes ${ ... } tokens. A token spans from "${" to the next
// "}". On evaluation error the literal token is preserved and reported.
func renderTemplate(tmpl string, env map[string]interface{}) (string, []tokenErr) {
	var sb strings.Builder
	var bad []tokenErr
	i := 0
	for i < len(tmpl) {
		rel := strings.Index(tmpl[i:], "${")
		if rel < 0 {
			sb.WriteString(tmpl[i:])
			break
		}
		start := i + rel
		sb.WriteString(tmpl[i:start])
		relEnd := strings.Index(tmpl[start+2:], "}")
		if relEnd < 0 {
			// Unterminated token: emit the rest literally.
			sb.WriteString(tmpl[start:])
			break
		}
		end := start + 2 + relEnd
		token := tmpl[start : end+1] // includes ${ ... }
		exprStr := strings.TrimSpace(tmpl[start+2 : end])

		val, err := evalMessageExpr(exprStr, env)
		if err != nil {
			sb.WriteString(token)
			bad = append(bad, tokenErr{token: token, err: err.Error()})
		} else {
			sb.WriteString(stringify(val))
		}
		i = end + 1
	}
	return sb.String(), bad
}

func evalMessageExpr(expression string, env map[string]interface{}) (interface{}, error) {
	fn, err := compileMessageExpr(expression)
	if err != nil {
		return nil, err
	}
	if env == nil {
		env = map[string]interface{}{}
	}
	return fn(env)
}

func stringify(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}
