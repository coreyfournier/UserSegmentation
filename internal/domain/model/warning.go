package model

// Warning is returned when a required field is missing from context at request time.
type Warning struct {
	Segment string `json:"segment"`
	Field   string `json:"field"`
	Message string `json:"message"`
}
