package search

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
	"github.com/segmentation-service/segmentation/internal/infrastructure/store"
)

func testSearcher(t *testing.T, snap *model.Snapshot) *SnapshotSearcher {
	t.Helper()
	s := store.NewMemory()
	s.Swap(snap)
	return NewSnapshotSearcher(s)
}

func fixture() *model.Snapshot {
	return &model.Snapshot{Layers: []model.Layer{
		{Name: "ewa-risk", Segments: []model.Segment{
			{ID: "employee", Strategy: model.StrategyChecklist},
		}},
		{Name: "base-tier", Segments: []model.Segment{
			{ID: "user-tier", Strategy: model.StrategyStatic},
		}},
		{Name: "experiments", Segments: []model.Segment{
			{ID: "checkout-flow", Strategy: model.StrategyPercentage},
			{ID: "ewa-banner", Strategy: model.StrategyPercentage},
		}},
	}}
}

func hitKeys(r *model.SearchResult) []string {
	out := make([]string, 0, len(r.Hits))
	for _, h := range r.Hits {
		out = append(out, h.Kind+":"+h.Layer+"/"+h.Segment+" ("+h.Field+")")
	}
	return out
}

func TestSearch_MatchesLayersAndSegments(t *testing.T) {
	res, err := testSearcher(t, fixture()).Search("ewa", 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	// The layer ewa-risk by name, and the segment ewa-banner by id. Not
	// ewa-risk's own segment, whose id is "employee".
	got := hitKeys(res)
	want := []string{
		"layer:ewa-risk/ (name)",
		"segment:experiments/ewa-banner (id)",
	}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("hit %d: expected %q, got %q", i, want[i], got[i])
		}
	}
}

func TestSearch_IsCaseInsensitive(t *testing.T) {
	res, _ := testSearcher(t, fixture()).Search("EWA-Risk", 0)
	if len(res.Hits) != 1 || res.Hits[0].Layer != "ewa-risk" {
		t.Fatalf("expected the ewa-risk layer, got %v", hitKeys(res))
	}
}

// Strategy is searchable so "which segments are checklists?" is answerable.
func TestSearch_MatchesStrategy(t *testing.T) {
	res, _ := testSearcher(t, fixture()).Search("percentage", 0)
	if len(res.Hits) != 2 {
		t.Fatalf("expected both percentage segments, got %v", hitKeys(res))
	}
	for _, h := range res.Hits {
		if h.Field != "strategy" {
			t.Errorf("expected a strategy match, got %+v", h)
		}
	}
}

// Results are ordered by layer name, not config order, so the same query
// returns the same list after an unrelated layer is created or deleted.
func TestSearch_OrderIsStableAcrossConfigOrder(t *testing.T) {
	snap := fixture()
	first, _ := testSearcher(t, snap).Search("e", 0)

	reordered := &model.Snapshot{Layers: []model.Layer{
		snap.Layers[2], snap.Layers[0], snap.Layers[1],
	}}
	second, _ := testSearcher(t, reordered).Search("e", 0)

	a, b := hitKeys(first), hitKeys(second)
	if len(a) != len(b) {
		t.Fatalf("different hit counts: %v vs %v", a, b)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("order changed with config order:\n %v\n %v", a, b)
		}
	}
}

// An empty query matches nothing. Listing everything is what GET
// /v1/admin/layers is for, and "every row, unfiltered" through a search path
// is the shape of an accident once a database is behind it.
func TestSearch_EmptyQueryMatchesNothing(t *testing.T) {
	for _, q := range []string{"", "   "} {
		res, err := testSearcher(t, fixture()).Search(q, 0)
		if err != nil {
			t.Fatalf("search(%q): %v", q, err)
		}
		if len(res.Hits) != 0 {
			t.Errorf("query %q returned %v", q, hitKeys(res))
		}
		if res.Hits == nil {
			t.Errorf("query %q returned a nil slice; it must encode as [] not null", q)
		}
	}
}

func TestSearch_LimitTruncates(t *testing.T) {
	res, _ := testSearcher(t, fixture()).Search("e", 2)
	if len(res.Hits) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(res.Hits))
	}
	if !res.Truncated {
		t.Error("expected Truncated to be set when the limit stopped the scan")
	}

	// A result that fits is not marked truncated.
	full, _ := testSearcher(t, fixture()).Search("checkout", 0)
	if full.Truncated {
		t.Error("a complete result must not be marked truncated")
	}
}

func TestSearch_NoSnapshotIsEmptyNotAPanic(t *testing.T) {
	res, err := NewSnapshotSearcher(store.NewMemory()).Search("anything", 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res.Hits) != 0 {
		t.Errorf("expected no hits, got %v", hitKeys(res))
	}
}
