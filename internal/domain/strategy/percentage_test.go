package strategy

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

type mockHasher struct {
	bucket int
}

func (m *mockHasher) Bucket(subjectKey, salt string) int {
	return m.bucket
}

func TestPercentageStrategy(t *testing.T) {
	seg := &model.Segment{
		Strategy: "percentage",
		Percentage: &model.PercentageConfig{
			Salt: "test",
			Buckets: []model.PercentageBucket{
				{Segment: "control", Weight: 50},
				{Segment: "treatment", Weight: 50},
			},
		},
	}

	tests := []struct {
		bucket int
		want   string
	}{
		{0, "control"},
		{49, "control"},
		{50, "treatment"},
		{99, "treatment"},
	}

	for _, tt := range tests {
		s := &PercentageStrategy{Hasher: &mockHasher{bucket: tt.bucket}}
		res, ok := s.Evaluate(seg, &EvalContext{SubjectKey: "user"})
		if !ok || res.Segment != tt.want {
			t.Errorf("bucket=%d: got %v %v, want %s", tt.bucket, res, ok, tt.want)
		}
	}
}

func TestPercentageStrategy_NoBuckets(t *testing.T) {
	seg := &model.Segment{Strategy: "percentage"}
	s := &PercentageStrategy{Hasher: &mockHasher{}}
	_, ok := s.Evaluate(seg, &EvalContext{SubjectKey: "user"})
	if ok {
		t.Error("expected no match with nil percentage config")
	}
}
