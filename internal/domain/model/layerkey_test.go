package model

import "testing"

func TestValidateLayerKey(t *testing.T) {
	ok := []string{"ewaRisk", "EwaRisk", "ewa_risk", "_private", "a", "layer2", "Value", "async"}
	for _, k := range ok {
		if msg := ValidateLayerKey(k); msg != "" {
			t.Errorf("expected %q to be valid, got %q", k, msg)
		}
	}

	bad := map[string]string{
		"":            "key is required",
		"ewa-risk":    "key may contain only letters, digits and underscores",
		"CT Rule":     "key may contain only letters, digits and underscores",
		"ewa.risk":    "key may contain only letters, digits and underscores",
		"2024rollout": "key cannot start with a digit",
		"class":       "key is a C# reserved word",
		"default":     "key is a C# reserved word",
		"string":      "key is a C# reserved word",
	}
	for k, want := range bad {
		if got := ValidateLayerKey(k); got != want {
			t.Errorf("key %q: expected %q, got %q", k, want, got)
		}
	}
}

func TestDeriveLayerKey(t *testing.T) {
	cases := map[string]string{
		"baseTier":         "baseTier",
		"CT Rule":           "ctRule",
		"Risk Rating":       "riskRating",
		"ct-fee-override":   "ctFeeOverride",
		"ewa-eligibility":   "ewaEligibility",
		"ewa-risk":          "ewaRisk",
		"experiments":       "experiments",
		"promotions":        "promotions",
		"features":          "features",
		"dSDsd":             "dSDsd",
		"RiskRating":        "riskRating",
		"T&A Gates":         "tAGates",
		"Balance Diagnosis": "balanceDiagnosis",
		// A digit-initial name is prefixed rather than handed back invalid.
		"2024 rollout": "_2024Rollout",
		// A derivation that lands on a keyword is pushed off it.
		"class": "classLayer",
		// Nothing usable in the name at all.
		"---": "",
		"":    "",
	}
	for name, want := range cases {
		if got := DeriveLayerKey(name); got != want {
			t.Errorf("DeriveLayerKey(%q) = %q, want %q", name, got, want)
		}
	}
}

// Whatever the derivation produces must be accepted by the validator, or the
// UI would suggest keys that cannot be saved.
func TestDeriveLayerKey_AlwaysValid(t *testing.T) {
	names := []string{
		"baseTier", "CT Rule", "2024 rollout", "class", "default", "  spaced  ",
		"a", "9", "___", "Ünïcödé name", "MiXeD-Case_Thing",
	}
	for _, n := range names {
		key := DeriveLayerKey(n)
		if key == "" {
			continue // no usable characters; the caller must ask for a key
		}
		if msg := ValidateLayerKey(key); msg != "" {
			t.Errorf("DeriveLayerKey(%q) = %q, which the validator rejects: %s", n, key, msg)
		}
	}
}
