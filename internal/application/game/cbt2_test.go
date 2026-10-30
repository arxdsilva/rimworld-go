package game

import (
	"testing"

	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
)

// CBT-2 whole-game tests: auto-undraft and dropping a weapon.

// TestAutoUndraftInPlay: a drafted colonist left idle with nothing
// hostile on the map is released after combat.AutoUndraft ticks, an order
// starts the quiet over, and nothing drafts it again.
func TestAutoUndraftInPlay(t *testing.T) {
	g := New(Config{Seed: 3, Width: 12, Height: 8})
	p, _ := g.SpawnNamedPawn(3, 4, "Ada")
	g.Apply(command.Command{Kind: command.DraftPawn, Pawn: uint64(p), Enabled: true})
	g.Step(combat.AutoUndraft / 2)
	g.Apply(command.Command{Kind: command.MovePawn, Pawn: uint64(p), X: 8, Y: 4}) // an order
	g.Step(combat.AutoUndraft - 100)
	if !g.drafted(p) {
		t.Fatal("the order started the quiet over")
	}
	g.Step(400)
	if g.drafted(p) {
		t.Fatal("released after the quiet")
	}
	g.Step(2 * combat.AutoUndraft)
	if g.drafted(p) {
		t.Fatal("nothing drafts it again")
	}
}

// TestDropWeaponThenReEquip: the command puts the rifle down where the
// colonist stands, the equip order takes it back; a stowed weapon is the
// spare and an unarmed drop puts it down.
func TestDropWeaponThenReEquip(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 8})
	p, _ := g.SpawnNamedPawn(3, 4, "Ada")
	g.equip[p] = item.Rifle
	g.Apply(command.Command{Kind: command.DropWeapon, Pawn: uint64(p)})
	g.Step(1)
	if g.Equipped(p) != "" || itemsOnGround(g, "rifle") != 1 {
		t.Fatalf("dropped: wields %q, %d on the ground", g.Equipped(p), itemsOnGround(g, "rifle"))
	}
	var rx, ry int
	for _, it := range g.World().Items {
		if it.Kind == "rifle" {
			rx, ry = it.X, it.Y
		}
	}
	if rx != 3 || ry != 4 {
		t.Fatalf("the rifle lies where Ada stood: (%d,%d)", rx, ry)
	}
	g.Apply(command.Command{Kind: command.Prioritize, Pawn: uint64(p), Order: command.OrderEquip, X: rx, Y: ry})
	g.Step(60)
	if g.Equipped(p) != "rifle" || itemsOnGround(g, "rifle") != 0 {
		t.Fatalf("re-equipped: %q", g.Equipped(p))
	}
	g.Apply(command.Command{Kind: command.DropWeapon, Pawn: uint64(p), Enabled: true}) // stow
	g.Step(1)
	if k, _, ok := g.combat.Drafts.Spare(p); !ok || k != item.Rifle || g.Equipped(p) != "" {
		t.Fatalf("stowed: %v %v, wields %q", k, ok, g.Equipped(p))
	}
	g.Apply(command.Command{Kind: command.DropWeapon, Pawn: uint64(p), Enabled: true}) // draw it back
	g.Step(1)
	if g.Equipped(p) != "rifle" {
		t.Fatal("drawn")
	}
}
