package orders

import (
	"testing"

	"rimworld-go/internal/application/game"
)

func TestMatchRank(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		label, q string
		want     int
	}{{"Wood x 5", "wo", 0}, {"Steel wall", "wa", 1}, {"Granite", "ani", 2}, {"Wood", "z", -1}} {
		if got := matchRank(tc.label, tc.q); got != tc.want {
			t.Errorf("matchRank(%q, %q) = %d, want %d", tc.label, tc.q, got, tc.want)
		}
	}
}

func TestSearchResultsRankKindsAndCache(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.pawns = []game.PawnView{{ID: 1, Name: "Wolfgang"}}
	f.world = game.WorldView{
		Items:     []game.ItemView{{ID: 2, Kind: "wood", Qty: 5}},
		Buildings: []game.BuildingView{{ID: 3, Kind: "wall"}},
	}
	c := New(f)
	c.OpenSearch()
	c.Search.Text = "wo"
	hits := c.SearchHits()
	if len(hits) != 2 || hits[0].Kind != "colonist" || hits[1].Kind != "item" {
		t.Fatalf("hits %+v", hits)
	}
	c.Search.Text = "wall"
	if hits := c.SearchHits(); len(hits) != 1 || hits[0].Label != "B:wall" {
		t.Fatalf("hits %+v", hits)
	}
	r := c.SearchRect(1280)
	if r.W != searchW || len(c.SearchButtons(1280)) != 2 { // one row and the box
		t.Fatalf("rect %+v buttons %d", r, len(c.SearchButtons(1280)))
	}
	c.CloseSearch()
	if c.Search.Open || c.SearchButtons(1280) != nil {
		t.Fatal("closed")
	}
}
