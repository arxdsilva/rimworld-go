package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/prisoner"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/mathx"
)

// MAP-13: a settlement's turrets fight. The whole game: a real attack on an
// industrial settlement, and an open field for the rules that the compound's
// walls would hide.

// settlementTurrets is map m's settlement turrets, ascending.
func settlementTurrets(m *mapState) []entity.EntityID {
	var out []entity.EntityID
	for _, id := range m.bldgIDs {
		if b, ok := m.bldgs.Get(id); ok && b.Hostile && !b.Kind.IsDoor() { // MAP-17: its doors are hostile too
			out = append(out, id)
		}
	}
	return out
}

// standInSight puts id on the nearest free tile in turret tid's sight, on
// map m, and returns it.
func standInSight(t *testing.T, g *Game, m *mapState, tid, id entity.EntityID) Position {
	t.Helper()
	cur := g.mapState
	g.mapState = m
	defer func() { g.mapState = cur }()
	b, _ := m.bldgs.Get(tid)
	for r := 2; r <= 12; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if max(mathx.Abs(dx), mathx.Abs(dy)) != r || !g.grid.InBounds(b.X+dx, b.Y+dy) {
					continue
				}
				x, y := b.X+dx, b.Y+dy
				if fx, fy, ok := g.combat.Tiles.WalkableNear(x, y, 0); !ok || fx != x || fy != y || !g.combat.Sight.Line(b.X, b.Y, x, y) {
					continue
				}
				p := Position{X: x, Y: y}
				g.positions.Set(id, p)
				g.invalidateCreatures()
				return p
			}
		}
	}
	t.Fatalf("no free tile in sight of the turret at %d,%d", b.X, b.Y)
	return Position{}
}

// shotBySentry reports a round of a turret having hit creature id.
func shotBySentry(g *Game, id entity.EntityID) bool {
	for _, e := range g.logs[id] {
		if e.Kind == logAttacked && e.Name == combat.TurretLogName {
			return true
		}
	}
	return false
}

// TestSettlementTurretHitsColonist: a real attack, no power device anywhere
// on the map, and a turret lands a round on a colonist in its sight.
func TestSettlementTurretHitsColonist(t *testing.T) {
	t.Parallel()
	g, m, _, _, ada := besieged(t, faction.Settlers)
	ts := settlementTurrets(m)
	if len(ts) != 2 {
		t.Fatalf("%d settlement turrets, want 2", len(ts))
	}
	fell(g, defenders(m)...) // the turret's shot, not a defender's
	p := standInSight(t, g, m, ts[0], ada)
	for i := 0; i < 2000 && !shotBySentry(g, ada); i++ {
		g.Step(1)
		m.positions.Set(ada, p)
	}
	if m.pw != nil {
		t.Fatal("the settlement map has a power device: this proves nothing")
	}
	if !shotBySentry(g, ada) {
		t.Fatal("no turret round hit ada in 2000 ticks")
	}
}

// hostileField is an open field with no power device and a settlement's
// sentry turret at (20, 5).
func hostileField(t *testing.T, seed uint64) (*Game, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: seed, Width: 40, Height: 40})
	for y := range 40 {
		for x := range 40 {
			g.setTerrain(x, y, terrain.Soil)
		}
	}
	tid := buildNow(t, g, construction.SentryTurret, item.Steel, construction.South, 20, 5)
	makeSettlementTurret(g, tid)
	return g, tid
}

// makeSettlementTurret turns a built turret into a settlement's.
func makeSettlementTurret(g *Game, tid entity.EntityID) {
	b, _ := g.bldgs.Get(tid)
	b.Hostile, b.Ancient = true, true
	g.bldgs.Set(tid, b)
	g.structVer++
}

// TestSettlementTurretPicksTheColony: raiders, visitors, prisoners and wild
// animals stand in front of it for a long while unharmed; the colonist and
// the tame animal behind them are what it takes.
func TestSettlementTurretPicksTheColony(t *testing.T) {
	t.Parallel()
	g, tid := hostileField(t, 3)
	raider := turretRaider(t, g, 19, 7)
	visitor := turretRaider(t, g, 18, 9)
	r := g.raiders[visitor]
	r.Leaving = true
	g.raiders[visitor] = r
	pris := turretRaider(t, g, 22, 9)
	held := g.prison.Export()
	if held == nil {
		held = map[entity.EntityID]*prisoner.State{}
	}
	held[pris] = &prisoner.State{}
	g.prison.Import(held)
	wild, err := g.spawnAnimal(animal.Chicken, 21, 11)
	if err != nil {
		t.Fatal(err)
	}
	keep := map[entity.EntityID]Position{raider: {X: 19, Y: 7}, visitor: {X: 18, Y: 9}, pris: {X: 22, Y: 9}, wild: {X: 21, Y: 11}}
	place := func() {
		for id, p := range keep {
			g.positions.Set(id, p)
		}
		g.invalidateCreatures()
	}
	for i := 0; i < 120; i++ {
		g.Step(1)
		place()
		if tg := g.combat.Turrets.Aim(tid).Target; tg != 0 {
			t.Fatalf("tick %d: locked on %d (raider %d, visitor %d, prisoner %d, wild %d)", g.Tick(), tg, raider, visitor, pris, wild)
		}
	}
	if _, _, _, ok := g.combat.Turrets.ForRaider(19, 7, 40, 20); ok {
		t.Fatal("a raider turned on its own side's turret")
	}
	pet, err := g.spawnAnimal(animal.Chicken, 20, 14)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := g.animals.Get(pet)
	a.Tame = true
	g.animals.Set(pet, a)
	keep[pet] = Position{X: 20, Y: 14}
	for i := 0; i <= combat.TurretScanEvery && g.combat.Turrets.Aim(tid).Target == 0; i++ {
		g.Step(1)
		place()
	}
	if tg := g.combat.Turrets.Aim(tid).Target; tg != pet {
		t.Fatalf("locked on %d, want the tame animal %d", tg, pet)
	}
	ada, err := g.SpawnNamedPawn(20, 10, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	delete(keep, pet)
	g.animals.Delete(pet)
	g.positions.Delete(pet)
	g.invalidateCreatures()
	keep[ada] = Position{X: 20, Y: 10}
	for i := 0; i <= 2*combat.TurretScanEvery && g.combat.Turrets.Aim(tid).Target != ada; i++ {
		g.Step(1)
		place()
	}
	if tg := g.combat.Turrets.Aim(tid).Target; tg != ada {
		t.Fatalf("locked on %d, want the colonist %d", tg, ada)
	}
}

// TestSettlementTurretStaysOffGrid: a conduit and a generator touching it do
// not put it on the grid, and our turret on the same conduit runs as ever,
// each shooting its own enemy.
func TestSettlementTurretStaysOffGrid(t *testing.T) {
	t.Parallel()
	out := 1000
	g, ts := turretRig(t, 4, 40, 30, &out, 20, 30)
	tid, mine := ts[0], ts[1]
	makeSettlementTurret(g, tid)
	buildNow(t, g, construction.SolarPanel, item.Steel, construction.South, 21, 5)
	g.Step(powerInterval * 2)
	if _, onList := g.powerList().powerNode(tid); onList || g.netOf(tid) != 0 || g.powered(tid) {
		t.Fatalf("on the grid: net %d powered %v", g.netOf(tid), g.powered(tid))
	}
	if !g.powered(mine) {
		t.Fatal("our turret on the conduit lost its power")
	}
	if v := psView(t, g, tid); !v.Hostile || v.PowerRole != "" || v.NoPowerReason != "" {
		t.Fatalf("the view says it runs on power: role %q reason %q hostile %v", v.PowerRole, v.NoPowerReason, v.Hostile)
	}
	rid := turretRaider(t, g, 30, 9)
	ada, err := g.SpawnNamedPawn(20, 9, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i <= combat.TurretScanEvery; i++ {
		g.positions.Set(rid, Position{X: 30, Y: 9})
		g.positions.Set(ada, Position{X: 20, Y: 9})
		g.invalidateCreatures()
		g.Step(1)
		holdFire(g)
	}
	if got := g.combat.Turrets.Aim(tid).Target; got != ada {
		t.Fatalf("the settlement's turret holds %d, want colonist %d", got, ada)
	}
	if got := g.combat.Turrets.Aim(mine).Target; got == ada || got == 0 {
		t.Fatalf("our turret holds %d, want a raider", got)
	}
}

// TestSettlementTurretDoesNotFlapInGame: the real creature index, the same
// two ranges as TestHostileTurretDoesNotFlap: a colonist wandering between
// lock and release is kept; let go, she is not picked up from there.
func TestSettlementTurretDoesNotFlapInGame(t *testing.T) {
	t.Parallel()
	g, tid := hostileField(t, 5)
	ada, err := g.SpawnNamedPawn(20, 14, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	at := func(dist, ticks int) (flips int) {
		t.Helper()
		for i := 0; i < ticks; i++ {
			had := g.combat.Turrets.Aim(tid).Target
			g.positions.Set(ada, Position{X: 20, Y: 5 + dist})
			g.invalidateCreatures()
			g.Step(1)
			holdFire(g)
			if got := g.combat.Turrets.Aim(tid).Target; got != had {
				flips++
			}
		}
		return flips
	}
	at(9, combat.TurretScanEvery+1)
	if g.combat.Turrets.Aim(tid).Target != ada {
		t.Fatal("no lock on a colonist 9 tiles off")
	}
	for k := range 40 { // across the lock radius, inside release
		if at(combat.TurretLock+k%3, 5) != 0 {
			t.Fatalf("flipped at %d tiles", combat.TurretLock+k%3)
		}
	}
	if at(combat.TurretRelease+1, 1) != 1 {
		t.Fatal("past release: let go")
	}
	if at(combat.TurretRelease, 3*combat.TurretScanEvery) != 0 || at(combat.TurretLock+1, 3*combat.TurretScanEvery) != 0 {
		t.Fatal("a target in the band is not picked up again")
	}
	if at(combat.TurretLock, combat.TurretScanEvery) != 1 || g.combat.Turrets.Aim(tid).Target != ada {
		t.Fatal("back within the lock radius it is picked up, once")
	}
}

// TestSettlementFallsTurretsFight: defenders down, the settlement falls with
// its turrets up; they stay hostile and keep firing, and they keep the map
// open no longer than the pack-up.
func TestSettlementFallsTurretsFight(t *testing.T) {
	t.Parallel()
	g, m, _, _, ada := besieged(t, faction.Settlers)
	ts := settlementTurrets(m)
	fell(g, defenders(m)...)
	nextHours(g, 4)
	meta := settleMeta(g, m)
	if !meta.Defeated || meta.PackBy == 0 {
		t.Fatalf("the settlement did not fall with its turrets up: %+v", meta)
	}
	for _, tid := range ts {
		if b, ok := m.bldgs.Get(tid); !ok || !b.Hostile {
			t.Fatalf("turret %d after the fall: %+v %v", tid, b, ok)
		}
	}
	p := standInSight(t, g, m, ts[0], ada)
	for i := 0; i < 2000 && !shotBySentry(g, ada); i++ {
		g.Step(1)
		m.positions.Set(ada, p)
	}
	if !shotBySentry(g, ada) {
		t.Fatal("the turret stopped firing after the fall")
	}
	if c := g.worldSt.atlas.Meta(m.id); c.PackBy == 0 {
		t.Fatalf("the pack-up stopped: %+v", c)
	}
	meta.PackBy = uint64(g.clock.Now()) + 1
	nextHours(g, 1)
	if len(g.maps) != 1 {
		t.Fatalf("a turret kept the map open: %d maps", len(g.maps))
	}
}

// fieldPawn is a drafted colonist on hostileField's map at (x, y), armed
// with kind (0: her fists), and the turret holds its fire so she
// lives through the test.
func fieldPawn(t *testing.T, g *Game, tid entity.EntityID, x, y int, kind item.Kind) entity.EntityID {
	t.Helper()
	ada, err := g.SpawnNamedPawn(x, y, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	if kind != 0 {
		if g.equip == nil {
			g.equip = map[entity.EntityID]item.Kind{}
		}
		g.equip[ada] = kind
	}
	g.setDraft(ada, true)
	g.combat.Turrets.SetHold(tid, true) // the test's quiet: no player command does this
	return ada
}

// TestOrderedAttackOnSettlementTurret: the order menu offers it to a drafted
// colonist, the command path takes it, the round lands, and once destroyed
// it leaves no blueprint and no aim. An undrafted colonist is not offered it.
func TestOrderedAttackOnSettlementTurret(t *testing.T) {
	t.Parallel()
	g, tid := hostileField(t, 6)
	ada := fieldPawn(t, g, tid, 20, 14, item.Rifle)
	var cmd command.Command
	for _, o := range g.OrdersAt(ada, 20, 5) {
		if strings.HasSuffix(o.Label, "sentry turret") && o.Cmd.Order == command.OrderAttack {
			if o.Disabled {
				t.Fatalf("option disabled: %+v", o)
			}
			cmd = o.Cmd
		}
	}
	if cmd.Order != command.OrderAttack {
		t.Fatalf("no attack option: %+v", g.OrdersAt(ada, 20, 5))
	}
	g.Apply(cmd)
	g.Step(1)
	if j, _ := g.jobs.Get(ada); j.Kind != job.Attack || j.Target != tid || !j.Forced {
		t.Fatalf("the order made job %+v", j)
	}
	for i := 0; i < 20000; i++ {
		if _, ok := g.bldgs.Get(tid); !ok {
			break
		}
		g.Step(1)
	}
	if _, ok := g.bldgs.Get(tid); ok {
		t.Fatal("ordered fire did not bring the turret down")
	}
	if g.layerFrameAt(20, 5, construction.LayerObject) != 0 {
		t.Fatal("a destroyed settlement's turret left a blueprint")
	}
	if g.combat.Turrets.Aim(tid).Target != 0 {
		t.Fatal("it still aims")
	}
	if len(g.powerList().offGrid) != 0 {
		t.Fatal("still on the turret list")
	}

	g2, tid2 := hostileField(t, 7)
	ben, err := g2.SpawnNamedPawn(20, 7, "Ben")
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range g2.OrdersAt(ben, 20, 5) {
		if o.Cmd.Order == command.OrderAttack {
			t.Fatalf("offered undrafted: %+v", o)
		}
	}
	g2.setDraft(ben, true)
	g2.combat.Turrets.SetHold(tid2, true)
	found := false
	for _, o := range g2.OrdersAt(ben, 20, 5) {
		if o.Cmd.Order == command.OrderAttack {
			found = true
			if o.Label != "Melee attack sentry turret" || o.Disabled {
				t.Fatalf("melee option %+v", o)
			}
			g2.Apply(o.Cmd)
		}
	}
	if !found {
		t.Fatal("no melee option")
	}
	before, _ := g2.bldgs.Get(tid2)
	g2.Step(600)
	if b, _ := g2.bldgs.Get(tid2); b.Damage <= before.Damage {
		t.Fatalf("fists on the turret did nothing: %+v", b)
	}
}

// TestFireAtWillShootsSettlementTurret: drafted, in reach, nothing nearer:
// she shoots it unasked. Our own turret, in the same reach, she leaves alone.
func TestFireAtWillShootsSettlementTurret(t *testing.T) {
	t.Parallel()
	out := 1000
	g, ts := turretRig(t, 8, 40, 30, &out, 20, 22)
	tid, mine := ts[0], ts[1]
	makeSettlementTurret(g, tid)
	ada := fieldPawn(t, g, tid, 21, 12, item.Rifle)
	g.Step(5)
	j, _ := g.jobs.Get(ada)
	if j.Kind != job.Attack || j.Target != tid || j.Forced {
		t.Fatalf("job %+v, want an auto attack on %d", j, tid)
	}
	g.Step(600)
	if b, _ := g.bldgs.Get(mine); b.Damage != 0 {
		t.Fatalf("she shot our own turret: %+v", b)
	}
}

// TestSettlementTurretRefusesThePlayer: every command that runs a building
// on the colony's behalf leaves it alone; the compound's other pieces still
// claim.
func TestSettlementTurretRefusesThePlayer(t *testing.T) {
	t.Parallel()
	g, tid := hostileField(t, 9)
	g.raidGroups = append(g.raidGroups, raid.Group{Plan: raid.Defend, Over: true, FleeLoss: raid.DefendFleeLoss, CampX: 30, CampY: 30, Camped: true, Size: 1})
	spawnRaider(g, 0, len(g.raidGroups)-1, item.Club, 30, 30)
	wall := buildNow(t, g, construction.Wall, item.Steel, construction.South, 25, 5)
	w, _ := g.bldgs.Get(wall)
	w.Ancient = true
	g.bldgs.Set(wall, w)

	ax1Cmd(g, ax1Rect(command.DesignateDeconstruct, 20, 5, 20, 5, true))
	if b, _ := g.bldgs.Get(tid); b.Deconstruct {
		t.Fatal("marked for deconstruction")
	}
	ax1Cmd(g, ax1Rect(command.DesignateUninstall, 20, 5, 20, 5, true))
	if b, _ := g.bldgs.Get(tid); b.Uninstall {
		t.Fatal("marked to uninstall")
	}
	ax1Cmd(g, ax1Rect(command.DesignateClaim, 0, 0, 39, 39, true))
	if b, _ := g.bldgs.Get(tid); !b.Hostile || !b.Ancient {
		t.Fatalf("claimed: %+v", b)
	}
	if w, _ := g.bldgs.Get(wall); w.Ancient {
		t.Fatal("the compound's wall stays claimable")
	}
	frames := len(g.frameIDs)
	moveCmd(g, tid, construction.South, 30, 5)
	if len(g.frameIDs) != frames {
		t.Fatal("it took a move order")
	}
	ax1Cmd(g, command.Command{Kind: command.SetTurretHold, X: 20, Y: 5, Enabled: true})
	if g.combat.Turrets.Held(tid) {
		t.Fatal("it holds fire on our word")
	}
	if b, _ := g.bldgs.Get(tid); !notColonyPiece(b) {
		t.Fatal("repair would take it up")
	}
}

// turretFightRun is a colonist under fire from a settlement's turret on a
// real attack: the state hash, with a save and load mid-fight when reload is
// set. It reports the turret had fired before the reload.
func turretFightRun(t *testing.T, reload bool) uint64 {
	t.Helper()
	g, m, _, _, ada := besieged(t, faction.Settlers)
	ts := settlementTurrets(m)
	standInSight(t, g, m, ts[0], ada)
	g.Step(clock.TicksPerHour / 8)
	cur := g.mapState
	g.mapState = m
	shot := g.combat.Turrets.Aim(ts[0]).ShotAt
	g.mapState = cur
	if shot == 0 {
		t.Fatal("no shot before the save point")
	}
	if reload {
		h := saveLoad(t, g)
		if h.StateHash() != g.StateHash() {
			t.Fatal("a save mid-fight changed the state hash")
		}
		var n int
		for _, hm := range h.maps {
			n += len(settlementTurrets(hm))
		}
		if n != len(ts) {
			t.Fatalf("%d settlement turrets after the load, want %d", n, len(ts))
		}
		h.worldS().planet, h.worldS().planetTried = testPlanet().Fork(), true
		g = h
	}
	g.Step(clock.TicksPerHour / 4)
	return g.StateHash()
}

// TestSettlementTurretSameSeedAndSaveHash: two runs of a turret fight hash
// the same, and so does one saved and loaded mid-fight, its turrets still
// hostile.
func TestSettlementTurretSameSeedAndSaveHash(t *testing.T) {
	t.Parallel()
	a := turretFightRun(t, false)
	if b := turretFightRun(t, false); a != b {
		t.Fatalf("same seed, hash %x and %x", a, b)
	}
	if c := turretFightRun(t, true); a != c {
		t.Fatalf("a save mid-fight moved the hash: %x and %x", a, c)
	}
}

// spendTurret fires off turret tid's whole barrel.
func spendTurret(g *Game, tid entity.EntityID) {
	s := g.combat.Turrets.Export()
	if s == nil {
		s = &combat.TurretSave{Version: combat.TurretSaveVersion}
	}
	for i := range s.Arms {
		if s.Arms[i].ID == tid {
			s.Arms = append(s.Arms[:i], s.Arms[i+1:]...)
			break
		}
	}
	s.Arms = append(s.Arms, combat.ArmSave{ID: tid, Fired: combat.SentryBarrel})
	g.combat.Turrets.Import(s)
}

// TestSpentSettlementTurretStaysSilent (MAP-20): the wiki's numbers hold (60
// rounds a barrel; 80 steel scaled by 9 as construction costs are). Spent, a
// settlement's turret never fires or rearms, however much steel lies by it,
// and it's no threat: a drafted colonist it could have shot goes off the
// draft by the usual quiet stretch.
func TestSpentSettlementTurretStaysSilent(t *testing.T) {
	t.Parallel()
	if combat.SentryBarrel != 60 {
		t.Fatalf("a barrel lasts %d rounds, the wiki says 60", combat.SentryBarrel)
	}
	if k, n := combat.BarrelCost(construction.SentryTurret); k != item.Steel || n != 9 {
		t.Fatalf("a barrel costs %d of %v, want 9 steel (80 scaled)", n, k)
	}
	g, tid := hostileField(t, 7)
	spendTurret(g, tid)
	ada, err := g.SpawnNamedPawn(20, 14, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	g.dropStackOf(item.Stack{Kind: item.Steel, Qty: 60}, 21, 6)
	g.setDraft(ada, true)
	if g.combat.Turrets.Barrel(tid, construction.SentryTurret) != 0 {
		t.Fatal("the barrel isn't spent")
	}
	for i := 0; i < combat.AutoUndraft+200 && g.drafted(ada); i++ {
		g.Step(1)
	}
	if g.drafted(ada) {
		t.Fatal("a spent turret kept the colonist drafted")
	}
	g.Step(2 * clock.TicksPerHour) // on its work now: the steel is in reach
	if shotBySentry(g, ada) {
		t.Fatal("a spent turret fired")
	}
	if g.combat.Turrets.Barrel(tid, construction.SentryTurret) != 0 {
		t.Fatal("somebody rearmed a settlement's turret")
	}
}
