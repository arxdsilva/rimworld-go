package recipe_test

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/research"
)

// TestVitalsMonitorNeedsFabrication: the vitals monitor is behind
// Fabrication like the hospital bed.
func TestVitalsMonitorNeedsFabrication(t *testing.T) {
	key := "build:" + construction.VitalsMonitor.String()
	if p, ok := (research.Progress{}).Locks(key); !ok || p != research.Fabrication {
		t.Fatalf("locked by %v %v", p, ok)
	}
	var pr research.Progress
	pr.Add(research.Traps, research.DefOf(research.Traps).Cost)
	pr.Add(research.Gunsmithing, research.DefOf(research.Gunsmithing).Cost)
	pr.Add(research.Fabrication, research.DefOf(research.Fabrication).Cost)
	if _, ok := pr.Locks(key); ok {
		t.Fatal("still locked after fabrication")
	}
}
