package pathstage

import (
	"testing"

	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/grid"
)

func TestBudgetDefersPastTheLimit(t *testing.T) {
	var b Budget
	for i := range 3 {
		if !b.Take(7, 3) || b.Deferred() {
			t.Fatalf("search %d of 3 refused", i)
		}
	}
	if b.Take(7, 3) || !b.Deferred() {
		t.Fatal("a fourth search in the tick ran")
	}
	if b.Clear(); b.Deferred() {
		t.Fatal("Clear kept the refusal")
	}
	if !b.Take(8, 3) {
		t.Fatal("the next tick starts afresh")
	}
}

// TestStageAnswersAsSearch: every kind answers as Search does, in request
// order, on any lane.
func TestStageAnswersAsSearch(t *testing.T) {
	g := grid.New(8, 8)
	var s Stage
	s.Open()
	for id, k := range []Kind{Exact, Corridor, Failed, Here} {
		s.Queue(Request{ID: entity.EntityID(1 + id), From: pathing.Point{X: 0, Y: 0}, Goal: pathing.Point{X: 7, Y: 5}})
		var allow []uint64
		if k == Corridor {
			allow = []uint64{0} // nothing allowed: the whole-map fallback answers
		}
		s.SetPlan(id, g, k, allow)
	}
	s.Close()
	var own pathing.Searcher
	s.Compute(&own)
	want, _ := pathing.Find(g, 0, 0, 7, 5)
	for i, r := range s.Requests() {
		got := s.Result(i)
		switch r.Kind {
		case Exact, Corridor:
			if !got.OK || len(got.Path) != len(want) || got.Path[len(want)-1] != want[len(want)-1] {
				t.Fatalf("kind %d: %v, want %v", r.Kind, got.Path, want)
			}
		case Failed:
			if got.OK {
				t.Fatal("a failed plan answered ok")
			}
		case Here:
			if !got.OK || got.Path != nil {
				t.Fatalf("already there: %v %v", got.Path, got.OK)
			}
		}
	}
	if runs, n := s.Stats(); runs != 1 || n != 4 {
		t.Fatalf("stats %d runs %d requests, want 1 and 4", runs, n)
	}
}
