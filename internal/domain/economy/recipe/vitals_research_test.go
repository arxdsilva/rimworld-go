package recipe_test

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/research"
)

// TestVitalsMonitorNeedsFabrication (RSCH-1, wiki: Research): the vitals
// monitor is behind its own project, which needs the multi-analyzer and
// hospital beds.
func TestVitalsMonitorNeedsFabrication(t *testing.T) {
	key := "build:" + construction.VitalsMonitor.String()
	if p, ok := (research.Progress{}).Locks(key); !ok || p != research.VitalsMonitors {
		t.Fatalf("locked by %v %v", p, ok)
	}
	if d := research.DefOf(research.VitalsMonitors); len(d.Requires) != 2 || d.Requires[0] != research.MultiAnalysis ||
		d.Requires[1] != research.HospitalBeds {
		t.Fatalf("vitals monitors require %v", d.Requires)
	}
	var pr research.Progress
	pr.Add(research.VitalsMonitors, research.DefOf(research.VitalsMonitors).Cost)
	if _, ok := pr.Locks(key); ok {
		t.Fatal("still locked after the project")
	}
}
