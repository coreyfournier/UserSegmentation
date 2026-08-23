package model

import "strings"

// ResolveField looks up a field in the evaluation context.
//
// A flat key match wins outright, which preserves any existing key that
// literally contains a dot. Only when there is no flat match is the name
// treated as a dotted path and traversed segment by segment, so a context may
// nest — an employee alongside its parent company, for example.
//
// A missing intermediate segment resolves to absent, which is the same path as
// any other missing field: a required-field violation, not an error.
func ResolveField(ctx map[string]interface{}, field string) (interface{}, bool) {
	if v, ok := ctx[field]; ok {
		return v, true
	}
	if !strings.Contains(field, ".") {
		return nil, false
	}

	var cur interface{} = ctx
	for _, part := range strings.Split(field, ".") {
		m, ok := asStringMap(cur)
		if !ok {
			return nil, false
		}
		cur, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// asStringMap normalizes the map shapes a decoded JSON document can produce.
func asStringMap(v interface{}) (map[string]interface{}, bool) {
	switch m := v.(type) {
	case map[string]interface{}:
		return m, true
	case map[string]string:
		out := make(map[string]interface{}, len(m))
		for k, s := range m {
			out[k] = s
		}
		return out, true
	default:
		return nil, false
	}
}
