package game

import (
	"testing"

	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// dressIn puts garments ks on id (their own slots), as wear does.
func dressIn(g *Game, id entity.EntityID, ks ...item.Kind) {
	w := g.apparel[id]
	for _, k := range ks {
		w.On[k.Slot()] = craft.Piece{Kind: k}
	}
	g.apparel[id] = w
}

// TestArmorSlowsMoveSpeed (GEAR-009): the wiki's cells/s penalties land in
// the MoveSpeed stat as permille of 4.6 cells/s, add up, go when the armour
// comes off, and the sheet is rebuilt once per change, not per step.
func TestArmorSlowsMoveSpeed(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id := spawnGen(t, g, 2, 2, genPawn("Plated", func(p *pawngen.Pawn) { p.Traits = nil }))
	if g.paceStep(id, 1000) != 1000 || g.statOf(id, stat.MoveSpeed) != 1000 {
		t.Fatalf("unarmoured: move %d", g.statOf(id, stat.MoveSpeed))
	}
	for _, c := range []struct {
		wear []item.Kind
		move int32
		step int
	}{
		{[]item.Kind{item.PlateArmor}, 826, 1211},               // -0.8 cells/s
		{[]item.Kind{item.MarineArmor}, 946, 1058},              // -0.25
		{[]item.Kind{item.FlakVest, item.FlakPants}, 948, 1055}, // -0.12 twice
		{[]item.Kind{item.FlakJacket, item.MarineHelmet}, 1000, 1000},
		{nil, 1000, 1000},
	} {
		g.apparel[id] = craft.Outfit{}
		dressIn(g, id, c.wear...)
		if got := g.paceStep(id, 1000); got != c.step || g.statOf(id, stat.MoveSpeed) != c.move {
			t.Errorf("%v: step %d move %d, want %d and %d", c.wear, got, g.statOf(id, stat.MoveSpeed), c.step, c.move)
		}
		fx := g.sk.stats[id]
		for range 5 {
			g.paceStep(id, 1000)
		}
		if g.sk.stats[id] != fx {
			t.Errorf("%v: the sheet was rebuilt with nothing changed", c.wear)
		}
	}
}

// TestArmorSlowsPawnWithoutBio: a plain pawn gets a sheet for its armour
// and loses it again when the armour comes off.
func TestArmorSlowsPawnWithoutBio(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id, err := g.SpawnNamedPawn(2, 2, "Plain")
	if err != nil {
		t.Fatal(err)
	}
	delete(g.sk.bios, id)
	delete(g.sk.stats, id)
	dressIn(g, id, item.PlateArmor)
	if got := g.paceStep(id, 1000); got != 1211 {
		t.Fatalf("plate: step %d", got)
	}
	g.apparel[id] = craft.Outfit{}
	if got := g.paceStep(id, 1000); got != 1000 || g.sk.stats[id] != nil {
		t.Errorf("bare: step %d, sheet %v", got, g.sk.stats[id])
	}
}

// TestArmorSlowsTheWalk: the same walk in the same world takes longer in
// plate armour (1 / 0.826 of the time, give or take the last step).
func TestArmorSlowsTheWalk(t *testing.T) {
	t.Parallel()
	walk := func(plate bool) int {
		g := New(Config{Seed: 3, Width: 40, Height: 8})
		id, err := g.SpawnNamedPawn(2, 4, "Walker")
		if err != nil {
			t.Fatal(err)
		}
		if plate {
			dressIn(g, id, item.PlateArmor)
		}
		g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(id), X: 34, Y: 4})
		for i := 1; i <= 3000; i++ {
			g.Step(1)
			if p := posOf(g, id); p.X == 34 && p.Y == 4 {
				return i
			}
		}
		t.Fatalf("plate %v: never arrived, at %v", plate, posOf(g, id))
		return 0
	}
	off, on := walk(false), walk(true)
	if want := off * 1000 / 826; on < want-TicksPerCell || on > want+2*TicksPerCell {
		t.Errorf("walk: %d ticks bare, %d in plate, want about %d", off, on, want)
	}
}
