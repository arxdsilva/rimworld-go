package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/command"
)

func TestResearchUnlocksSpikeTraps(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	g.research = research.Progress{} // a fresh colony
	if !g.BuildNow("research bench", "wood", "south", 4, 4) {
		t.Fatal("build research bench")
	}
	ada, _ := g.SpawnNamedPawn(10, 8, "Ada")
	trap := uint8(construction.SpikeTrap)
	if ok, why := g.CanPlaceAt("spike trap", "wood", "south", 12, 2); ok || !strings.Contains(why, "Traps") {
		t.Fatalf("spike traps need research first: %v %q", ok, why)
	}
	g.Enqueue(command.Command{Kind: command.DesignateBuild, Build: trap, X: 12, Y: 2})
	g.Enqueue(command.Command{Kind: command.SetResearch, Build: uint8(research.Gunsmithing)}) // not available yet
	g.Step(1)
	if len(g.frameIDs) != 0 || g.researchSet {
		t.Fatal("locked: no blueprint, and gunsmithing can't be picked before traps")
	}
	g.Enqueue(command.Command{Kind: command.SetResearch, Build: uint8(research.Traps)})
	studied := false
	for i := 0; i < 400 && !g.research.Done(research.Traps); i++ {
		g.Step(50)
		g.SetNeeds(ada, 1<<20, 1<<20)
		if j, _ := g.jobs.Get(ada); j.Kind.String() == "research" {
			studied = true
		}
	}
	if !studied || !g.research.Done(research.Traps) {
		t.Fatalf("Ada should research traps at the bench (progress %d)", g.research[research.Traps])
	}
	found := false
	for _, m := range g.World().Messages {
		if strings.Contains(m.Text, "Research finished: Traps") {
			found = true
		}
	}
	if !found || g.researchSet {
		t.Fatal("finishing posts a message and clears the current project")
	}
	g.Enqueue(command.Command{Kind: command.DesignateBuild, Build: trap, X: 12, Y: 2})
	g.Step(1)
	if len(g.frameIDs) != 1 {
		t.Fatal("researched: spike traps can be placed")
	}
	views := g.World().Research
	if len(views) != int(research.NumProjects) || !views[research.Traps].Done || !views[research.Gunsmithing].Available {
		t.Fatalf("views %+v", views)
	}
	if _, locked := g.World().Locked["recipe:make_rifle"]; !locked {
		t.Fatal("rifles still need gunsmithing")
	}
}
