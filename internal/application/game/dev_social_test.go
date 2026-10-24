package game

import (
	"testing"

	"rimworld-go/internal/domain/pawn/social"
)

func TestDevSetPair(t *testing.T) {
	g, a, b := socialPair(t)
	for name, want := range devPairKinds {
		if !g.DevSetPair(a, b, name) || g.pairOf(a, b).Kind != want {
			t.Fatalf("%s: pair %v", name, g.pairOf(a, b).Kind)
		}
	}
	g.DevSetPair(a, b, "spouse")
	if len(g.soc.spouses[a]) != 1 {
		t.Error("spouse index not updated")
	}
	if g.DevSetPair(a, b, "nope") || g.DevSetPair(a, a, "lover") {
		t.Error("bad input accepted")
	}
}

func TestDevAddOpinionMemory(t *testing.T) {
	g, a, b := socialPair(t)
	before := g.opinion(a, b)
	if !g.DevAddOpinionMemory(a, b, "insulted") || g.opinion(a, b) >= before {
		t.Errorf("insult did not lower opinion: %d -> %d", before, g.opinion(a, b))
	}
	if !g.hasOpinionMemory(a, b, social.MemInsulted, socNow(g)) {
		t.Error("memory missing")
	}
	if g.DevAddOpinionMemory(a, b, "bogus") {
		t.Error("unknown kind accepted")
	}
}

func TestDevBond(t *testing.T) {
	g, a, _ := socialPair(t)
	id, err := g.SpawnAnimal("husky", 8, 5)
	if err != nil {
		t.Fatal(err)
	}
	if g.DevBond(id, a) {
		t.Error("bonded a wild animal")
	}
	g.TameAnimal(id, "Rex")
	if !g.DevBond(id, a) {
		t.Fatal("bond refused")
	}
	for _, av := range g.World().Animals {
		if av.ID == id && (av.BondID != a || av.MasterID != a) {
			t.Errorf("view bond %d master %d, want %d", av.BondID, av.MasterID, a)
		}
	}
}

func TestDevScar(t *testing.T) {
	g, a, _ := socialPair(t)
	if g.DevScar(a, "elbow") {
		t.Error("unknown part accepted")
	}
	if !g.DevScar(a, "jaw") || !g.bodies[a].Disfigured() {
		t.Fatal("jaw scar did not disfigure")
	}
	g.Step(5)
	for _, v := range g.Snapshot() {
		if v.ID == a && !v.Disfigured {
			t.Error("view not disfigured after a step")
		}
	}
}
