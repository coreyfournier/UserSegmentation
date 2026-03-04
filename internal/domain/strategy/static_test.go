package strategy

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func TestStaticStrategy_Mapping(t *testing.T) {
	seg := &model.Segment{
		Strategy: "static",
		Static: &model.StaticConfig{
			Mappings: map[string]string{"user-vip": "platinum"},
			Default:  "standard",
		},
	}
	s := &StaticStrategy{}

	res, ok := s.Evaluate(seg, &EvalContext{SubjectKey: "user-vip"})
	if !ok || res.Segment != "platinum" {
		t.Errorf("got %v %v, want platinum", res, ok)
	}

	res, ok = s.Evaluate(seg, &EvalContext{SubjectKey: "other"})
	if !ok || res.Segment != "standard" {
		t.Errorf("got %v %v, want standard", res, ok)
	}
}

func TestStaticStrategy_NoConfig(t *testing.T) {
	seg := &model.Segment{Strategy: "static"}
	s := &StaticStrategy{}
	_, ok := s.Evaluate(seg, &EvalContext{SubjectKey: "x"})
	if ok {
		t.Error("expected no match with nil static config")
	}
}
