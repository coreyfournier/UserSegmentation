package model

// Assignment represents the result of evaluating a single segment for a user.
type Assignment struct {
	Segment  string `json:"segment"`
	Strategy string `json:"strategy"`
	Reason   string `json:"reason"`
}
