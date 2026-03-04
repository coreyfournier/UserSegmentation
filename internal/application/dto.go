package application

// EvaluateRequest is the input DTO for single-user evaluation.
type EvaluateRequest struct {
	UserKey string                 `json:"user_key"`
	Context map[string]interface{} `json:"context"`
	Layers  []string               `json:"layers,omitempty"`
}

// EvaluateResponse is the output DTO for evaluation.
type EvaluateResponse struct {
	UserKey     string                    `json:"user_key"`
	Layers      map[string]LayerResultDTO `json:"layers"`
	Warnings    []WarningDTO              `json:"warnings,omitempty"`
	EvaluatedAt string                    `json:"evaluated_at"`
	DurationUS  int64                     `json:"duration_us"`
}

// LayerResultDTO is a single layer's assignment in the response.
type LayerResultDTO struct {
	Segment  string `json:"segment"`
	Strategy string `json:"strategy"`
	Reason   string `json:"reason"`
}

// WarningDTO represents a validation warning.
type WarningDTO struct {
	Segment string `json:"segment"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// BatchEvaluateRequest is the input DTO for multi-user evaluation.
type BatchEvaluateRequest struct {
	Users []EvaluateRequest `json:"users"`
}

// BatchEvaluateResponse is the output DTO for batch evaluation.
type BatchEvaluateResponse struct {
	Results    []EvaluateResponse `json:"results"`
	DurationUS int64              `json:"duration_us"`
}
