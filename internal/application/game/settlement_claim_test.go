package game

import (
	"testing"

	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// MAP-21 (wiki: Faction base): "Once defeated, a base's turrets can be safely
// claimed to disable firing at friendly pawns and even turn them to your
// side ... These can be uninstalled and brought back to your colony."

// claimedTurret is a settlement's turret in an open field, claimed.
func claimedTurret(t *testing.T, seed uint64) (*Game, entity.EntityID) {
	t.Helper()
	g, tid := hostileField(t, seed)
	ax1Cmd(g, ax1Rect(command.DesignateClaim, 20, 5, 20, 5, true))
	if b, _ := g.bldgs.Get(tid); b.Hostile || b.Ancient || !b.OffGrid {
		t.Fatalf("the claim took nothing: %+v", b)
	}
	return g, tid
}

// turretView is building id's read model.
func turretView(g *Game, id entity.EntityID) BuildingView {
	for _, v := range g.World().Buildings {
		if v.ID == id {
			return v
		}
	}
	return BuildingView{}
}

// TestClaimedTurretFightsForUsOffTheGrid: with no power device on the map, it
// shoots an enemy and leaves the colonist in its sight alone, and the pane
// reads it as on.
func TestClaimedTurretFightsForUsOffTheGrid(t *testing.T) {
	t.Parallel()
	g, tid := claimedTurret(t, 21)
	ada, err := g.SpawnNamedPawn(20, 12, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	g.raidGroups = append(g.raidGroups, raid.Group{Size: 1})
	rid := spawnRaider(g, 0, len(g.raidGroups)-1, item.Club, 24, 12)
	standInSight(t, g, g.mapState, tid, rid)
	if g.powered(tid) {
		t.Fatal("fixture: it has a grid")
	}
	if v := turretView(g, tid); !v.OffGrid || !v.Powered || v.Hostile {
		t.Fatalf("view %+v", v)
	}
	for i := 0; i < 400 && !shotBySentry(g, rid); i++ {
		g.Step(1)
	}
	if !shotBySentry(g, rid) {
		t.Fatal("it never fired on the enemy")
	}
	if shotBySentry(g, ada) {
		t.Fatal("it fired on a colonist")
	}
}

// TestClaimedTurretDrawsRaiderFire: raiders see it as a live turret even with
// no device on the map; before the claim they don't.
func TestClaimedTurretDrawsRaiderFire(t *testing.T) {
	t.Parallel()
	g, tid := hostileField(t, 22)
	if _, _, _, ok := g.combat.Turrets.ForRaider(20, 10, 100, 20); ok {
		t.Fatal("raiders go for their own side's turret")
	}
	ax1Cmd(g, ax1Rect(command.DesignateClaim, 20, 5, 20, 5, true))
	if id, _, _, ok := g.combat.Turrets.ForRaider(20, 10, 100, 20); !ok || id != tid {
		t.Fatalf("no fire drawn: %d %v", id, ok)
	}
}

// TestClaimedTurretIsReloadedByAHauler: spent, ours now, it takes the steel a
// hauler brings (wiki: Mini-turret), as the colony's own does.
func TestClaimedTurretIsReloadedByAHauler(t *testing.T) {
	t.Parallel()
	g, tid := claimedTurret(t, 23)
	spendTurret(g, tid)
	if _, err := g.SpawnNamedPawn(20, 14, "Ada"); err != nil {
		t.Fatal(err)
	}
	g.dropStackOf(item.Stack{Kind: item.Steel, Qty: 60}, 21, 8)
	for i := 0; i < 40 && g.combat.Turrets.Barrel(tid, construction.SentryTurret) == 0; i++ {
		g.Step(clock.TicksPerHour / 4)
	}
	if n := g.combat.Turrets.Barrel(tid, construction.SentryTurret); n != 1000 { // per mille
		t.Fatalf("barrel %d after ten hours", n)
	}
}

// TestClaimedTurretCratedLosesItsOwnSupply: uninstalled (wiki: Faction base)
// and set up at home, it wants power like any other.
func TestClaimedTurretCratedLosesItsOwnSupply(t *testing.T) {
	t.Parallel()
	g, _ := claimedTurret(t, 24)
	bob := builder(t, g, 18, 8)
	ax1Cmd(g, ax1Rect(command.DesignateUninstall, 20, 5, 20, 5, true))
	if !runCare(g, 2000, func() bool { c, _ := crateOf(g); return c != 0 }, bob) {
		t.Fatal("no crate")
	}
	cid, st := crateOf(g)
	if construction.Kind(st.Building) != construction.SentryTurret {
		t.Fatalf("crate %+v", st)
	}
	ax1Cmd(g, command.Command{Kind: command.Reinstall, Target: uint64(cid), X: 12, Y: 7, Facing: uint8(construction.South)})
	for i := 0; i < 6000 && g.layerBldgAt(12, 7, construction.LayerObject) == 0; i++ {
		g.Step(1)
		if i%100 == 0 {
			g.SetNeeds(bob, needs.Full, needs.Full)
		}
	}
	id := g.layerBldgAt(12, 7, construction.LayerObject)
	if id == 0 {
		t.Fatal("never set up again")
	}
	b, _ := g.bldgs.Get(id)
	v := turretView(g, id)
	if b.OffGrid || v.OffGrid || v.Powered {
		t.Fatalf("set up at home: %+v, view %+v", b, v)
	}
}

// TestClaimedTurretMovedLosesItsOwnSupply: moved, the same.
func TestClaimedTurretMovedLosesItsOwnSupply(t *testing.T) {
	t.Parallel()
	g, tid := claimedTurret(t, 25)
	bob := builder(t, g, 18, 8)
	moveCmd(g, tid, construction.South, 12, 7)
	for i := 0; i < 6000; i++ {
		if b, _ := g.bldgs.Get(tid); b.X == 12 {
			break
		}
		g.Step(1)
		if i%100 == 0 {
			g.SetNeeds(bob, needs.Full, needs.Full)
		}
	}
	b, _ := g.bldgs.Get(tid)
	if b.X != 12 || b.OffGrid {
		t.Fatalf("moved: %+v", b)
	}
	if v := turretView(g, tid); v.Powered {
		t.Fatalf("view %+v", v)
	}
}

// TestHeavyTurretsStayWhereTheyStand (wiki: Autocannon turret, Uranium slug
// turret: "cannot be re-installed"): claimed, neither moves nor crates.
func TestHeavyTurretsStayWhereTheyStand(t *testing.T) {
	t.Parallel()
	for _, k := range []construction.Kind{construction.Autocannon, construction.UraniumSlugTurret} {
		g, _ := hostileField(t, 26)
		id := buildNow(t, g, k, item.Steel, construction.South, 30, 20)
		makeSettlementTurret(g, id)
		ax1Cmd(g, ax1Rect(command.DesignateClaim, 30, 20, 30, 20, true))
		if b, _ := g.bldgs.Get(id); b.Hostile || !b.OffGrid {
			t.Fatalf("%v unclaimed: %+v", k, b)
		}
		frames := len(g.frameIDs)
		moveCmd(g, id, construction.South, 10, 20)
		ax1Cmd(g, ax1Rect(command.DesignateUninstall, 30, 20, 30, 20, true))
		if b, _ := g.bldgs.Get(id); b.Uninstall || len(g.frameIDs) != frames {
			t.Fatalf("%v took the order: %+v", k, b)
		}
	}
}

// TestClaimedTurretSaved: the claim survives a save with the hash.
func TestClaimedTurretSaved(t *testing.T) {
	t.Parallel()
	g, tid := claimedTurret(t, 27)
	h := saveLoad(t, g)
	if h.StateHash() != g.StateHash() {
		t.Fatal("the save moved the hash")
	}
	if b, _ := h.bldgs.Get(tid); !b.OffGrid || b.Hostile {
		t.Fatalf("loaded: %+v", b)
	}
}

// TestTurretFuseInGame: a real blow below the mark lights the fuse on the defence
// stream, the view says so, a save keeps it, and in 240 ticks the turret is
// gone with a message. Which seed lights is the stream's business.
func TestTurretFuseInGame(t *testing.T) {
	t.Parallel()
	lit := false
	for seed := uint64(30); seed < 60 && !lit; seed++ {
		g, tid := claimedTurret(t, seed)
		b, _ := g.bldgs.Get(tid)
		g.damageBuilding(tid, b.HP()-1)
		g.damageBuilding(tid, 0)
		if !g.combat.Turrets.Sparking(tid) {
			if !g.bldgs.Has(tid) {
				t.Fatal("one hit point left, it fell")
			}
			continue
		}
		lit = true
		if !turretView(g, tid).Sparking {
			t.Fatal("the view doesn't say")
		}
		if !messageHas(g, "sparking") {
			t.Fatal("no warning")
		}
		h := saveLoad(t, g)
		if h.StateHash() != g.StateHash() || !h.combat.Turrets.Sparking(tid) {
			t.Fatal("the save lost the fuse")
		}
		g.Step(combat.TurretFuseTicks - 1)
		if !g.bldgs.Has(tid) {
			t.Fatal("blew up early")
		}
		g.Step(2)
		if g.bldgs.Has(tid) || !messageHas(g, "blew up") {
			t.Fatalf("still there %v", g.bldgs.Has(tid))
		}
	}
	if !lit {
		t.Fatal("30 turrets, not one lit: the roll is not 50 %")
	}
}
