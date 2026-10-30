package game

import (
	"testing"

	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/vault"
)

// Sleeper caskets (MAPGEN-3): grouped opening, the occupants, the open
// order and its colonist job. Fixtures: vault_test.go.

func sealedIn(g *Game, group int) int {
	n := 0
	for _, id := range g.bldgIDs {
		if b, _ := g.bldgs.Get(id); b.Kind == construction.SleeperCasket && b.Occupant != 0 && b.CasketGroup == group {
			n++
		}
	}
	return n
}

func TestCasketOpensItsWholeGroup(t *testing.T) {
	t.Parallel()
	g := vaultGame(t, nil, nil, vault.Empty)
	breachV1(t, g)
	if sealedIn(g, 1) != 4 || sealedIn(g, 2) != 1 {
		t.Fatal("fixture caskets")
	}
	g.openCasketGroup(bldgIDAt(g, construction.SleeperCasket, 68, 69))
	if sealedIn(g, 1) != 0 {
		t.Fatalf("%d caskets of the group still sealed", sealedIn(g, 1))
	}
	if sealedIn(g, 2) != 1 {
		t.Fatal("another vault's casket opened")
	}
	if s := vaultByID(g, 1).State; s != vault.CasketsOpened {
		t.Fatalf("state %v", s)
	}
	n := 0
	for _, m := range g.messages {
		if m.Text == casketEmptyText {
			n++
		}
	}
	if n != 4 {
		t.Fatalf("%d empty-casket messages, want 4", n)
	}
	// opening an open group again does nothing
	g.openCasketGroup(bldgIDAt(g, construction.SleeperCasket, 65, 64))
	if historyKind(g, "vault") != 1 {
		t.Fatal("history moved on a second opening")
	}
}

func TestCasketDamageOpensGroup(t *testing.T) {
	t.Parallel()
	g := vaultGame(t, nil, nil, vault.Empty)
	breachV1(t, g)
	g.damageBuilding(bldgIDAt(g, construction.SleeperCasket, 65, 64), 1)
	if sealedIn(g, 1) != 0 {
		t.Fatal("harming a casket left its group sealed")
	}
	// the damage still lands on the (now open) shell
	if b, _ := bldgAt(g, construction.SleeperCasket, 65, 64); b.Damage != 1 {
		t.Fatalf("damage %d", b.Damage)
	}
}

func TestCasketOpenBeforeBreachWakesTheVault(t *testing.T) {
	t.Parallel()
	g := vaultGame(t, nil, nil, vault.Empty)
	g.damageBuilding(bldgIDAt(g, construction.SleeperCasket, 65, 64), 1)
	v := vaultByID(g, 1)
	if v.State != vault.CasketsOpened {
		t.Fatalf("state %v", v.State)
	}
	for _, d := range v.Defenders {
		if a, _ := g.animals.Get(entity.EntityID(d)); a.Dormant {
			t.Fatal("caskets opened under sleeping defenders")
		}
	}
}

// openOne opens vault 1 (one occupant kind in all four caskets) and
// returns the colonists and raiders that appeared.
func openOne(t *testing.T, occ vault.Occupant) (g *Game, pawns, raiders []entity.EntityID, beasts []animal.State) {
	t.Helper()
	g = vaultGame(t, func(s *Setup) {}, func(g *Game) {}, occ)
	breachV1(t, g)
	p0 := append([]entity.EntityID(nil), g.pawnIDs...)
	r0 := len(g.raiderIDs)
	a0 := map[entity.EntityID]bool{}
	for _, id := range g.animalIDs {
		a0[id] = true
	}
	g.openCasketGroup(bldgIDAt(g, construction.SleeperCasket, 65, 64))
	pawns = g.pawnIDs[len(p0):]
	raiders = g.raiderIDs[r0:]
	for _, id := range g.animalIDs {
		if !a0[id] {
			a, _ := g.animals.Get(id)
			beasts = append(beasts, a)
		}
	}
	return g, pawns, raiders, beasts
}

func insideV1(g *Game, id entity.EntityID) bool {
	p, _ := g.positions.Get(id)
	return p.X > v1X0 && p.X < v1X1 && p.Y > v1Y0 && p.Y < v1Y1
}

func TestCasketOccupants(t *testing.T) {
	t.Parallel()
	t.Run("grateful joins", func(t *testing.T) {
		g, pawns, raiders, beasts := openOne(t, vault.Grateful)
		if len(pawns) != 4 || len(raiders) != 0 || len(beasts) != 0 {
			t.Fatalf("pawns %d raiders %d beasts %d", len(pawns), len(raiders), len(beasts))
		}
		for _, id := range pawns {
			if g.isRaider(id) || !g.standingPawn(id) || !insideV1(g, id) {
				t.Fatalf("sleeper %d is not a standing colonist in the vault", id)
			}
			if _, ok := g.bioFor(id); !ok {
				t.Fatal("a sleeper without a bio")
			}
		}
		if lettersTitled(g, casketJoinTitle) != 4 {
			t.Fatal("no join letters")
		}
	})
	t.Run("hostile attacks", func(t *testing.T) {
		g, pawns, raiders, _ := openOne(t, vault.Hostile)
		if len(pawns) != 0 || len(raiders) != 4 {
			t.Fatalf("pawns %d raiders %d", len(pawns), len(raiders))
		}
		for _, id := range raiders {
			if !g.standingRaider(id) || !insideV1(g, id) {
				t.Fatalf("hostile %d not standing in the vault", id)
			}
			if k, ok := g.equip[id]; !ok || k == 0 {
				t.Fatal("an unarmed hostile")
			}
			if g.name(id) == "" || g.prison.Is(id) {
				t.Fatal("hostile name or status")
			}
		}
		if len(vaultByID(g, 1).Hostiles) != 4 {
			t.Fatal("hostiles not tracked for clearing")
		}
		if !messageWith(g, "bursts from a sleeper casket") {
			t.Fatal("no threat message")
		}
	})
	t.Run("sleeper and crawler", func(t *testing.T) {
		g, pawns, raiders, beasts := openOne(t, vault.SleeperAndCrawler)
		if len(pawns) != 4 || len(raiders) != 0 || len(beasts) != 4 {
			t.Fatalf("pawns %d raiders %d beasts %d", len(pawns), len(raiders), len(beasts))
		}
		for _, a := range beasts {
			if a.Species != animal.HiveCrawler || a.Manhunter != raid.HiveForever || a.Dormant {
				t.Fatalf("crawler %+v", a)
			}
		}
		if len(vaultByID(g, 1).Hostiles) != 4 {
			t.Fatal("crawlers not tracked")
		}
	})
	t.Run("beetles", func(t *testing.T) {
		_, pawns, raiders, beasts := openOne(t, vault.Beetles)
		if len(pawns) != 0 || len(raiders) != 0 || len(beasts) != 8 {
			t.Fatalf("pawns %d raiders %d beasts %d", len(pawns), len(raiders), len(beasts))
		}
		for _, a := range beasts {
			if a.Species != animal.HiveBeetle || a.Manhunter != raid.HiveForever || a.Mode != animal.Attack {
				t.Fatalf("beetle %+v", a)
			}
		}
	})
	t.Run("empty", func(t *testing.T) {
		g, pawns, raiders, beasts := openOne(t, vault.Empty)
		if len(pawns)+len(raiders)+len(beasts) != 0 {
			t.Fatal("something came out of an empty casket")
		}
		if !messageWith(g, casketEmptyText) {
			t.Fatal("no empty message")
		}
	})
	t.Run("remains", func(t *testing.T) {
		g, pawns, raiders, beasts := openOne(t, vault.Remains)
		if len(pawns) != 0 || len(beasts) != 0 || len(raiders) != 4 {
			t.Fatalf("pawns %d raiders %d beasts %d", len(pawns), len(raiders), len(beasts))
		}
		weapons := 0
		for _, id := range raiders {
			if v, _ := g.vital.Get(id); v.Status != health.Dead {
				t.Fatal("the remains are alive")
			}
			if _, st, ok := g.bodyOf(id); !ok || st.Kind != item.Bones {
				t.Fatalf("remains body %+v %v", st, ok)
			}
		}
		for _, sid := range g.itemIDs {
			if st, _ := g.stacks.Get(sid); st.Kind.IsWeapon() && st.X > v1X0 && st.X < v1X1 && st.Y > v1Y0 && st.Y < v1Y1 {
				weapons += st.Qty
			}
		}
		if weapons != 4 {
			t.Fatalf("%d weapons by the remains, want 4", weapons)
		}
		if len(vaultByID(g, 1).Hostiles) != 0 {
			t.Fatal("remains counted as hostiles")
		}
	})
}

func TestCasketOpenShellSalvage(t *testing.T) {
	t.Parallel()
	g := vaultGame(t, nil, nil, vault.Empty)
	breachV1(t, g)
	id := bldgIDAt(g, construction.SleeperCasket, 65, 64)
	g.openCasketGroup(id)
	g.removeBuilding(id)
	if k, q := stackAt(g, 65, 64); k != item.Steel || q != 20 {
		t.Fatalf("salvage %v x%d", k, q)
	}
}

func TestCasketOrderRefusals(t *testing.T) {
	t.Parallel()
	g := vaultGame(t, nil, nil, vault.Empty)
	id := bldgIDAt(g, construction.SleeperCasket, 65, 64)
	// sealed in the dark: nothing to order
	g.Apply(command.Command{Kind: command.OpenCasket, Target: uint64(id)})
	g.Step(1)
	if b, _ := g.bldgs.Get(id); b.Deconstruct {
		t.Fatal("an unseen casket was ordered open")
	}
	// revealed, but the way in walled up again
	breachV1(t, g)
	if !g.BuildNow("wall", "wood", "south", 61, v1DoorY) {
		t.Fatal("wall the corridor up")
	}
	if r := g.casketOpenReason(id); r != ReasonCasketUnreachable {
		t.Fatalf("reason %q", r)
	}
	g.orderOpenCasket(id)
	if b, _ := g.bldgs.Get(id); b.Deconstruct {
		t.Fatal("an unreachable casket was ordered open")
	}
	if !messageWith(g, ReasonCasketUnreachable) {
		t.Fatal("no refusal message")
	}
	sealedView := false
	for _, bv := range g.World().Buildings {
		if bv.ID == id {
			sealedView = bv.Sealed && bv.OpenReason == ReasonCasketUnreachable
		}
	}
	if !sealedView {
		t.Fatal("the view does not show the sealed casket and its reason")
	}
	// not a casket, or already open: ignored
	g.orderOpenCasket(bldgIDAt(g, construction.Wall, v1X0, v1Y0))
	g.orderOpenCasket(entity.EntityID(1 << 40))
}

func TestCasketOrderColonistOpensIt(t *testing.T) {
	t.Parallel()
	g := vaultGame(t, nil, nil, vault.Empty)
	breachV1(t, g)
	for _, d := range vaultByID(g, 1).Defenders {
		g.killAnimal(entity.EntityID(d), 0) // nothing in the way of the work
	}
	id := bldgIDAt(g, construction.SleeperCasket, 68, 64)
	if r := g.casketOpenReason(id); r != "" {
		t.Fatalf("reason %q", r)
	}
	g.Apply(command.Command{Kind: command.OpenCasket, Target: uint64(id)})
	g.Step(1)
	b, _ := g.bldgs.Get(id)
	if !b.Deconstruct || b.WorkLeft != casketOpenWork {
		t.Fatalf("order not marked: %+v", b)
	}
	for i := 0; i < 40 && sealedIn(g, 1) > 0; i++ {
		g.Step(250)
	}
	if sealedIn(g, 1) != 0 {
		t.Fatal("no colonist opened the casket")
	}
	if b, ok := g.bldgs.Get(id); !ok || b.Deconstruct {
		t.Fatalf("the opened casket was taken down or stays marked: %+v %v", b, ok)
	}
}

func TestCasketOccupantsLive(t *testing.T) {
	t.Parallel()
	g := vaultGame(t, nil, nil, vault.Grateful, vault.Hostile, vault.SleeperAndCrawler, vault.Remains)
	breachV1(t, g)
	g.openCasketGroup(bldgIDAt(g, construction.SleeperCasket, 65, 64))
	hostiles := vaultByID(g, 1).Hostiles
	if len(hostiles) != 2 {
		t.Fatalf("%d hostiles, want the ancient and the crawler", len(hostiles))
	}
	start := map[entity.EntityID]Position{}
	for _, h := range hostiles {
		start[entity.EntityID(h)], _ = g.positions.Get(entity.EntityID(h))
	}
	g.Step(600)
	for id, p0 := range start {
		p, ok := g.positions.Get(id)
		if ok && p == p0 {
			t.Fatalf("hostile occupant %d never moved", id)
		}
	}
}
