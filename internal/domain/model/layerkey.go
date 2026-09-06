package model

import "strings"

// csharpKeywords are C#'s reserved words: identifiers a generated client
// cannot use for a property without the @ escape. Contextual keywords (var,
// async, record, value…) are deliberately absent — those are legal
// identifiers in C#, and rejecting them would forbid perfectly good keys like
// "value" for no gain.
var csharpKeywords = map[string]bool{
	"abstract": true, "as": true, "base": true, "bool": true, "break": true,
	"byte": true, "case": true, "catch": true, "char": true, "checked": true,
	"class": true, "const": true, "continue": true, "decimal": true, "default": true,
	"delegate": true, "do": true, "double": true, "else": true, "enum": true,
	"event": true, "explicit": true, "extern": true, "false": true, "finally": true,
	"fixed": true, "float": true, "for": true, "foreach": true, "goto": true,
	"if": true, "implicit": true, "in": true, "int": true, "interface": true,
	"internal": true, "is": true, "lock": true, "long": true, "namespace": true,
	"new": true, "null": true, "object": true, "operator": true, "out": true,
	"override": true, "params": true, "private": true, "protected": true, "public": true,
	"readonly": true, "ref": true, "return": true, "sbyte": true, "sealed": true,
	"short": true, "sizeof": true, "stackalloc": true, "static": true, "string": true,
	"struct": true, "switch": true, "this": true, "throw": true, "true": true,
	"try": true, "typeof": true, "uint": true, "ulong": true, "unchecked": true,
	"unsafe": true, "ushort": true, "using": true, "virtual": true, "void": true,
	"volatile": true, "while": true,
}

// ValidateLayerKey reports why key is unusable, or "" when it is fine.
//
// The rule is a C# identifier: an ASCII letter or underscore, then letters,
// digits or underscores. camelCase, PascalCase and snake_case all pass; the
// separators that today's layer names actually use — spaces and hyphens — do
// not, which is the point.
//
// ASCII only. C# permits Unicode identifiers, but a key travels through JSON
// keys, URL path segments and generated code, and the failure modes of a
// non-ASCII key across those three are not worth the expressiveness.
func ValidateLayerKey(key string) string {
	if key == "" {
		return "key is required"
	}
	for i, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
			// Always allowed.
		case r >= '0' && r <= '9':
			if i == 0 {
				return "key cannot start with a digit"
			}
		default:
			return "key may contain only letters, digits and underscores"
		}
	}
	if csharpKeywords[key] {
		return "key is a C# reserved word"
	}
	return ""
}

// DeriveLayerKey turns a display name into a camelCase key: "ewa-eligibility"
// becomes "ewaEligibility", "CT Rule" becomes "ctRule".
//
// A suggestion, not a rule — a key is authored, and this is what the UI offers
// while one is being typed and what a migration error message names. It always
// returns something ValidateLayerKey accepts, or "" when the name holds no
// usable characters at all (the caller then has to ask for a key outright).
func DeriveLayerKey(name string) string {
	// Split on anything that cannot appear in a key, and on case boundaries
	// only insofar as the source already provides them — "RiskRating" is one
	// word here and survives as "riskRating" rather than being torn apart.
	var words []string
	var cur strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			cur.WriteRune(r)
		default:
			if cur.Len() > 0 {
				words = append(words, cur.String())
				cur.Reset()
			}
		}
	}
	if cur.Len() > 0 {
		words = append(words, cur.String())
	}
	if len(words) == 0 {
		return ""
	}

	var b strings.Builder
	for i, w := range words {
		if i == 0 {
			// An all-caps first word reads badly kept as-is ("CT Rule" ->
			// "CTRule"), so it is lowered whole; a mixed-case one keeps its
			// shape with only the first letter lowered ("RiskRating" ->
			// "riskRating").
			if isAllUpper(w) {
				b.WriteString(strings.ToLower(w))
			} else {
				b.WriteString(strings.ToLower(w[:1]))
				b.WriteString(w[1:])
			}
			continue
		}
		b.WriteString(strings.ToUpper(w[:1]))
		if isAllUpper(w) && len(w) > 1 {
			b.WriteString(strings.ToLower(w[1:]))
		} else {
			b.WriteString(w[1:])
		}
	}

	key := b.String()
	// A name starting with a digit ("2024 rollout") derives something legal by
	// prefixing, rather than returning a key the author would have to fix.
	if key[0] >= '0' && key[0] <= '9' {
		key = "_" + key
	}
	if csharpKeywords[key] {
		key += "Layer"
	}
	return key
}

func isAllUpper(s string) bool {
	hasLetter := false
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			return false
		}
		if r >= 'A' && r <= 'Z' {
			hasLetter = true
		}
	}
	return hasLetter
}
