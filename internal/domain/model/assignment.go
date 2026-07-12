package model

// Assignment represents the result of evaluating a single segment for a user.
type Assignment struct {
	Segment     string                 `json:"segment"`
	Strategy    string                 `json:"strategy"`
	Reason      string                 `json:"reason"`
	Expressions map[string]interface{} `json:"expressions,omitempty"`
	// Messages holds rendered localized messages keyed by language code for the
	// rule/override/default that produced this assignment.
	Messages map[string]string `json:"messages,omitempty"`
}
