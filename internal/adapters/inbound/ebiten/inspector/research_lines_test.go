package inspector

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
)

// RSCH-2: a research bench shows the project under way and its speed; a
// hi-tech bench says whether an analyzer is linked; an analyzer counts
// its benches; other buildings have no research lines.
func TestResearchLinesForBenchesAndAnalyzer(t *testing.T) {
	t.Parallel()
	w := game.WorldView{
		Research: []game.ResearchView{{Label: "Smithing", Cost: 1000, CostNow: 2000, Progress: 500, Current: true}},
		Buildings: []game.BuildingView{
			{ID: 7, Kind: "multi-analyzer"},
			{ID: 8, Kind: "electric research bench", ResearchSpeed: 1100, Analyzer: 7},
			{ID: 9, Kind: "electric research bench", ResearchSpeed: 1000},
		},
	}
	got := strings.Join(ResearchLines(w.Buildings[1], w), "|")
	if got != "Researching Smithing: 50%|Research speed: 110%|Linked: multi-analyzer" {
		t.Fatalf("linked bench: %q", got)
	}
	if got := strings.Join(ResearchLines(w.Buildings[2], w), "|"); !strings.HasSuffix(got, "No multi-analyzer linked") {
		t.Fatalf("unlinked bench: %q", got)
	}
	if got := ResearchLines(w.Buildings[0], w); len(got) != 1 || got[0] != "Linked to 1 hi-tech research bench" {
		t.Fatalf("analyzer: %q", got)
	}
	if got := ResearchLines(game.BuildingView{Kind: "research bench"}, w); len(got) != 1 || !strings.HasPrefix(got[0], "Researching") {
		t.Fatalf("simple bench: %q", got)
	}
	if got := ResearchLines(game.BuildingView{Kind: "stool"}, w); got != nil {
		t.Fatalf("stool: %q", got)
	}
}
