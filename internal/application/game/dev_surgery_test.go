package game

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/mood"
)

// SIM-049 dev hooks: lose a part, spawn implants.

func TestDevLosePart(t *testing.T) {
	g, a, _ := socialPair(t)
	gen := g.viewGen
	if !g.DevLosePart(a, "left leg") {
		t.Fatal("a leg can be lost")
	}
	if g.viewGen == gen {
		t.Fatal("the hook bumps the view generation")
	}
	b := g.bodies[a]
	if b.Missing&(1<<body.LegL) == 0 || b.Missing&(1<<body.FootL) == 0 {
		t.Fatalf("the leg and the foot below it are gone: %b", b.Missing)
	}
	if m := b.Capacity(body.Moving); m >= 1000 {
		t.Fatalf("moving %d", m)
	}
	if fx := g.sk.stats[a]; fx == nil || fx.body != keyOf(b) {
		t.Fatal("the stat sheet follows the body")
	}
	if g.VitalOf(a).Injury != b.Damage() {
		t.Fatal("Injury is re-derived")
	}
	hv, _ := g.Health(a)
	if hv.Parts[body.LegL].Status != PartMissing {
		t.Fatalf("leg row %+v", hv.Parts[body.LegL])
	}
	thought := false
	for _, th := range g.bodyThoughts(a) {
		thought = thought || th.Kind == mood.MissingPart
	}
	if !thought {
		t.Fatal("a lost leg weighs on the mood")
	}
	gen = g.viewGen
	for _, bad := range []string{"left leg", "left foot", "heart", "brain", "tail"} {
		if g.DevLosePart(a, bad) {
			t.Fatalf("%q: already gone, vital or unknown", bad)
		}
	}
	if g.DevLosePart(9999, "nose") || g.viewGen != gen {
		t.Fatal("bad input changes nothing")
	}
	if !g.DevLosePart(a, "nose") || !b.Disfigured() {
		t.Fatal("a lost nose disfigures")
	}
}

func TestDevSpawnImplants(t *testing.T) {
	g, a, _ := socialPair(t)
	if g.DevSpawnImplants(9999) {
		t.Fatal("unknown pawn")
	}
	gen := g.viewGen
	if !g.DevSpawnImplants(a) || g.viewGen == gen {
		t.Fatal("spawned (and the view generation bumped)")
	}
	if len(implantKinds) < 9 {
		t.Fatalf("implant kinds %v", implantKinds)
	}
	for _, k := range implantKinds {
		if n := g.stockOf(k); n != 1 {
			t.Fatalf("%s: %d in stock", k, n)
		}
	}
	if g.stockOf(item.GlassEye) != 1 {
		t.Fatal("the new implants are among them")
	}
}
