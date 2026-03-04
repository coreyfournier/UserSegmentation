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
	ID          string            `json:"id"`
	Strategy    string            `json:"strategy"`
	Static      *StaticConfig     `json:"static,omitempty"`
	Percentage  *PercentageConfig `json:"percentage,omitempty"`
	Rules       []Rule            `json:"rules,omitempty"`
	Overrides   []Rule            `json:"overrides,omitempty"`
	Default     string            `json:"default,omitempty"`
	Promotion   *Promotion        `json:"promotion,omitempty"`
	InputSchema InputSchema       `json:"inputSchema,omitempty"`
}
