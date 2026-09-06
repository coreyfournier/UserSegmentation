package application

// EvaluateRequest is the input DTO for single-subject evaluation.
//
// There is no subject_key field. The subject key is an ordinary context field
// named "subjectKey", declared in the input schema of any layer whose segments
// actually read it — which is only the static and percentage strategies. It was
// once required of every request, including the many that had no strategy able
// to use it.
type EvaluateRequest struct {
	Context map[string]interface{} `json:"context"`
	Layers  []string               `json:"layers,omitempty"`
	// Languages requests localized messages for the winning rule/override/default
	// of each layer. RenderAll returns every defined locale (testing aid).
	Languages []string `json:"languages,omitempty"`
	RenderAll bool     `json:"render_all,omitempty"`
}

// EvaluateResponse is the output DTO for evaluation.
type EvaluateResponse struct {
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
//
// With one deliberate crossing: "unevaluable" is also reported by any strategy
// that could not run for want of a required input — today, a static or
// percentage segment whose subjectKey is absent from context. It means the same
// thing in both places (nothing could be decided here, and the accompanying
// warning says why), which is why it was widened rather than given a second
// name. A consumer switching on status must accept it from any layer.
type LayerResultDTO struct {
	// Name is the layer's friendly label, inside the object its stable key
	// addresses. Omitted when the layer has none — the key is then the only
	// name it has, and an empty string would read as one it does not.
	Name     string                 `json:"name,omitempty"`
	Status   string                 `json:"status"`
	Segment  string                 `json:"segment,omitempty"`
	Strategy string                 `json:"strategy,omitempty"`
	Reason   string                 `json:"reason,omitempty"`
	Computed map[string]interface{} `json:"computed,omitempty"`
	Messages map[string]string      `json:"messages,omitempty"`
	Outputs  map[string]interface{} `json:"outputs,omitempty"`
	Failures []FailureDTO           `json:"failures,omitempty"`
}

// FailureDTO is one itemised problem from a checklist layer. The rule name is the
// stable identifier; the message states the problem.
type FailureDTO struct {
	Rule     string                 `json:"rule"`
	Message  string                 `json:"message,omitempty"`
	Messages map[string]string      `json:"messages,omitempty"`
	Outputs  map[string]interface{} `json:"outputs,omitempty"`
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
