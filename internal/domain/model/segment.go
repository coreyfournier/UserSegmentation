package model

// PercentageBucket defines a weighted segment in percentage-based allocation.
type PercentageBucket struct {
	Segment string `json:"segment"`
	Weight  int    `json:"weight"`
}

// PercentageConfig holds the salt and bucket definitions.
type PercentageConfig struct {
	Salt    string             `json:"salt"`
	Buckets []PercentageBucket `json:"buckets"`
}

// StaticConfig holds direct user-to-segment mappings and a default.
type StaticConfig struct {
	Mappings map[string]string `json:"mappings"`
	Default  string            `json:"default"`
}

// Segment is a single segment definition within a layer.
type Segment struct {
	ID string `json:"id"`
	// When is an optional dispatch predicate. When present and false, the
	// segment is passed over entirely and produces no output of any kind —
	// it is not a reported state. This is how one layer holds per-entity-type
	// variants: each segment declares the type it applies to.
	When       *Rule             `json:"when,omitempty"`
	Strategy   string            `json:"strategy"`
	Static     *StaticConfig     `json:"static,omitempty"`
	Percentage *PercentageConfig `json:"percentage,omitempty"`
	Computed   []ComputedField   `json:"computed,omitempty"`
	Rules      []Rule            `json:"rules,omitempty"`
	Overrides  []Rule            `json:"overrides,omitempty"`
	Default    string            `json:"default,omitempty"`
	// DefaultMessages are optional localized templates keyed by language code,
	// rendered when the segment falls back to Default.
	DefaultMessages map[string]string `json:"defaultMessages,omitempty"`
	// DefaultOutputs are the output values the default path authors, the way a
	// rule authors its own. Without them the default could only take the
	// segment-wide fallback in Outputs, so a field whose value depends on the
	// outcome — the fee charged, the tier landed on — had no way to differ
	// when nothing matched. Worse, a required field then forced a
	// segment-level value that was wrong for every rule that did match.
	//
	// Read only by the rule strategy's default branch. A checklist returns
	// before reaching it, and static and percentage author no outputs at all.
	DefaultOutputs map[string]string `json:"defaultOutputs,omitempty"`
	Promotion       *Promotion        `json:"promotion,omitempty"`
	Outputs         map[string]string `json:"outputs,omitempty"`
	// LegacyInputSchema and LegacyOutputSchema exist only to catch config
	// written before schemas moved to the layer. They carry no behaviour:
	// validation rejects any segment where either is non-empty, telling the
	// author to move it up.
	//
	// Without them Go's decoder would drop the old keys silently, and a dropped
	// input schema switches rule-field validation off rather than failing — so
	// the config would load, look fine, and check nothing. Delete both once no
	// config in flight still carries them.
	LegacyInputSchema  InputSchema  `json:"inputSchema,omitempty"`
	LegacyOutputSchema OutputSchema `json:"outputSchema,omitempty"`
}
