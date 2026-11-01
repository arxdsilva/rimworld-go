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
	g.lab.Prog = research.Progress{} // a fresh colony
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
	if len(g.frameIDs) != 0 || g.lab.Set {
		t.Fatal("locked: no blueprint, and gunsmithing can't be picked before traps")
	}
	g.Enqueue(command.Command{Kind: command.SetResearch, Build: uint8(research.Traps)})
	g.lab.Prog[research.Traps] = research.DefOf(research.Traps).Cost - 5 // RSCH-1: wiki costs take days; the last points here
	studied := false
	for i := 0; i < 400 && !g.lab.Prog.Done(research.Traps); i++ {
		g.Step(50)
		g.SetNeeds(ada, 1<<20, 1<<20)
		if j, _ := g.jobs.Get(ada); j.Kind.String() == "research" {
			studied = true
		}
	}
	if !studied || !g.lab.Prog.Done(research.Traps) {
		t.Fatalf("Ada should research traps at the bench (progress %d)", g.lab.Prog[research.Traps])
	}
	found := false
	for _, m := range g.World().Messages {
		if m.Letter && m.Title == "Research complete: Traps" && strings.HasPrefix(m.Text, research.DefOf(research.Traps).Desc) {
			found = true
		}
	}
	if !found || g.lab.Set {
		t.Fatal("finishing posts a letter with the description and clears the current project")
	}
	g.Enqueue(command.Command{Kind: command.DesignateBuild, Build: trap, X: 12, Y: 2})
	g.Step(1)
	if len(g.frameIDs) != 1 {
		t.Fatal("researched: spike traps can be placed")
	}
	views := g.World().Research
	if len(views) != int(research.NumProjects) || !views[research.Traps].Done || !views[research.Bridges].Available {
		t.Fatalf("views %+v", views)
	}
	if _, locked := g.World().Locked["recipe:make_rifle"]; !locked {
		t.Fatal("rifles still need gunsmithing")
	}
}
