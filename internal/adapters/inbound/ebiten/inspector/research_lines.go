package inspector

import (
	"strconv"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
)

// Research bench and Multi-analyzer lines (RSCH-2; wiki: Research bench,
// Multi-analyzer): a bench shows the project under way, its research
// speed factor (BuildingView.ResearchSpeed, permille: bench tier, linked
// analyzer, room, cleanliness, temperature) and, for a hi-tech bench,
// whether a Multi-analyzer is linked. An analyzer counts the hi-tech
// benches that link it.

// ResearchLines are the building's research lines (nil for other kinds).
func ResearchLines(b game.BuildingView, w game.WorldView) []string {
	k, ok := construction.Parse(b.Kind)
	if !ok {
		return nil
	}
	switch {
	case k == construction.MultiAnalyzer:
		n := 0
		for _, o := range w.Buildings {
			if o.Analyzer != 0 && o.Analyzer == uint64(b.ID) {
				n++
			}
		}
		switch n {
		case 0:
			return []string{"No hi-tech research bench linked"}
		case 1:
			return []string{"Linked to 1 hi-tech research bench"}
		}
		return []string{"Linked to " + strconv.Itoa(n) + " hi-tech research benches"}
	case construction.RecipeBench(k) != construction.ResearchBench:
		return nil
	}
	var out []string
	for _, p := range w.Research {
		if p.Current && !p.Done {
			pct := 0
			if p.Cost > 0 { // Progress and Cost are both at the base cost
				pct = min(100, 100*p.Progress/p.Cost)
			}
			out = append(out, "Researching "+p.Label+": "+strconv.Itoa(pct)+"%")
			break
		}
	}
	if b.ResearchSpeed > 0 {
		out = append(out, "Research speed: "+strconv.Itoa((b.ResearchSpeed+5)/10)+"%")
	}
	if k == construction.ElectricResearchBench {
		if b.Analyzer != 0 {
			out = append(out, "Linked: multi-analyzer")
		} else {
			out = append(out, "No multi-analyzer linked")
		}
	}
	return out
}
