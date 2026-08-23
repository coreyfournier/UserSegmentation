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
	When        *Rule             `json:"when,omitempty"`
	Strategy    string            `json:"strategy"`
	Static      *StaticConfig     `json:"static,omitempty"`
	Percentage  *PercentageConfig `json:"percentage,omitempty"`
	Expressions []ExpressionDef   `json:"expressions,omitempty"`
	Rules       []Rule            `json:"rules,omitempty"`
	Overrides   []Rule            `json:"overrides,omitempty"`
	Default     string            `json:"default,omitempty"`
	// DefaultMessages are optional localized templates keyed by language code,
	// rendered when the segment falls back to Default.
	DefaultMessages map[string]string `json:"defaultMessages,omitempty"`
	Promotion       *Promotion        `json:"promotion,omitempty"`
	InputSchema     InputSchema       `json:"inputSchema,omitempty"`
}
