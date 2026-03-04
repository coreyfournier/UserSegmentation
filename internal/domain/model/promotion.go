package model

import "time"

// Promotion defines a time-bound window for a segment.
type Promotion struct {
	EffectiveFrom  *time.Time `json:"effective_from,omitempty"`
	EffectiveUntil *time.Time `json:"effective_until,omitempty"`
}

// IsActive returns true if the current time is within the promotion window.
func (p *Promotion) IsActive(now time.Time) bool {
	if p == nil {
		return true
	}
	if p.EffectiveFrom != nil && now.Before(*p.EffectiveFrom) {
		return false
	}
	if p.EffectiveUntil != nil && now.After(*p.EffectiveUntil) {
		return false
	}
	return true
}
