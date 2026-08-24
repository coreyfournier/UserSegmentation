package application

// EvaluateRequest is the input DTO for single-subject evaluation.
type EvaluateRequest struct {
	SubjectKey string                 `json:"subject_key"`
	Context    map[string]interface{} `json:"context"`
	Layers     []string               `json:"layers,omitempty"`
	// Languages requests localized messages for the winning rule/override/default
	// of each layer. RenderAll returns every defined locale (testing aid).
	Languages []string `json:"languages,omitempty"`
	RenderAll bool     `json:"render_all,omitempty"`
}

// EvaluateResponse is the output DTO for evaluation.
type EvaluateResponse struct {
	SubjectKey  string                    `json:"subject_key"`
	Layers      map[string]LayerResultDTO `json:"layers"`
	Warnings    []WarningDTO              `json:"warnings,omitempty"`
	EvaluatedAt string                    `json:"evaluated_at"`
	DurationUS  int64                     `json:"duration_us"`
}

// LayerResultDTO is a single layer's outcome in the response.
//
// Status is authoritative: a consumer never needs to inspect the length of
// Failures to learn whether anything is wrong. Checklist layers report
// satisfied/violated/unevaluable; every other strategy reports
// resolved/unresolved/skipped.
type LayerResultDTO struct {
	Status      string                 `json:"status"`
	Segment     string                 `json:"segment,omitempty"`
	Strategy    string                 `json:"strategy,omitempty"`
	Reason      string                 `json:"reason,omitempty"`
	Expressions map[string]interface{} `json:"expressions,omitempty"`
	Messages    map[string]string      `json:"messages,omitempty"`
	Failures    []FailureDTO           `json:"failures,omitempty"`
}

// FailureDTO is one itemised problem from a checklist layer. The rule name is the
// stable identifier; the message states the problem.
type FailureDTO struct {
	Rule     string            `json:"rule"`
	Message  string            `json:"message,omitempty"`
	Messages map[string]string `json:"messages,omitempty"`
}

// WarningDTO represents a validation warning.
type WarningDTO struct {
	Segment string `json:"segment"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// BatchEvaluateRequest is the input DTO for multi-subject evaluation.
type BatchEvaluateRequest struct {
	Subjects []EvaluateRequest `json:"subjects"`
}

// BatchEvaluateResponse is the output DTO for batch evaluation.
type BatchEvaluateResponse struct {
	Results    []EvaluateResponse `json:"results"`
	DurationUS int64              `json:"duration_us"`
}
