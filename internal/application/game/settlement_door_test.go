package game

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/door"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/grid"
	"rimworld-go/internal/domain/world/region"
	"rimworld-go/internal/domain/world/terrain"
)

// MAP-17 (wiki: Door): a settlement's doors are shut to colonists until
// claimed, its own people open them, and they can be smashed.

// The compound's layout: a walled box with one door on its south side.
const (
	cmpX0, cmpY0, cmpX1, cmpY1 = 20, 10, 30, 20
	cmpDoorX, cmpDoorY         = 25, 20
	cmpInX, cmpInY             = 25, 15
	cmpOutX, cmpOutY           = 25, 40
)

// enemyCompound is an open field with a walled compound, as Settle lays it,
// whose door is shut and hostile; defended adds a settlement defender inside.
func enemyCompound(t *testing.T, seed uint64, defended bool) (g *Game, did, def entity.EntityID) {
	t.Helper()
	g = New(Config{Seed: seed, Width: 60, Height: 60})
	for y := range 60 {
		for x := range 60 {
			g.setTerrain(x, y, terrain.Soil)
		}
	}
	for y := cmpY0; y <= cmpY1; y++ {
		for x := cmpX0; x <= cmpX1; x++ {
			if x != cmpX0 && x != cmpX1 && y != cmpY0 && y != cmpY1 || x == cmpDoorX && y == cmpDoorY {
				continue
			}
			w := buildNow(t, g, construction.Wall, item.Steel, construction.South, x, y)
			b, _ := g.bldgs.Get(w)
			b.Ancient = true
			g.bldgs.Set(w, b)
		}
	}
	did = buildNow(t, g, construction.Door, item.Steel, construction.South, cmpDoorX, cmpDoorY)
	b, _ := g.bldgs.Get(did)
	b.Hostile, b.Ancient = true, true
	g.bldgs.Set(did, b)
	g.markRoomsDirty()
	g.structVer++
	if defended {
		g.raidGroups = append(g.raidGroups, raid.Group{Plan: raid.Defend, Over: true, FleeLoss: raid.DefendFleeLoss, CampX: cmpInX, CampY: cmpInY, Camped: true, Size: 1})
		def = spawnRaider(g, 0, len(g.raidGroups)-1, item.Club, cmpInX, cmpInY)
	}
	return g, did, def
}

// outsider is a colonist standing outside the compound.
func outsider(t *testing.T, g *Game, name string) entity.EntityID {
	t.Helper()
	id, err := g.SpawnNamedPawn(cmpOutX, cmpOutY, name)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// settlementDoors is map m's settlement doors.
func settlementDoors(m *mapState) []entity.EntityID {
	var out []entity.EntityID
	for _, id := range m.bldgIDs {
		if b, ok := m.bldgs.Get(id); ok && b.Hostile && b.Kind.IsDoor() {
			out = append(out, id)
		}
	}
	return out
}

// goHere is the "Go here" option of the order menu.
func goHere(t *testing.T, g *Game, id entity.EntityID, x, y int) OrderOption {
	t.Helper()
	i := slices.IndexFunc(g.OrdersAt(id, x, y), func(o OrderOption) bool { return o.Label == "Go here" })
	if i < 0 {
		t.Fatalf("no Go here at %d,%d", x, y)
	}
	return g.OrdersAt(id, x, y)[i]
}

// TestSettlementLaysItsDoorsShut: a real attack: every compound door is shut,
// hostile and not held, and none of them passes for a turret.
func TestSettlementLaysItsDoorsShut(t *testing.T) {
	t.Parallel()
	g, m, _, _, _ := besieged(t, faction.Settlers)
	ds := settlementDoors(m)
	if len(ds) == 0 {
		t.Fatal("the compound has no hostile door")
	}
	cur := g.mapState
	g.mapState = m
	defer func() { g.mapState = cur }()
	for _, id := range ds {
		b, _ := m.bldgs.Get(id)
		if !b.Ancient || b.Forbidden {
			t.Fatalf("door %+v", b)
		}
		if g.combat.Doors.Held(id) || g.combat.Doors.IsOpen(id) {
			t.Fatalf("door at %d,%d is held open", b.X, b.Y)
		}
		if g.combat.Turrets.HostileAt(b.X, b.Y) != 0 {
			t.Fatalf("door at %d,%d counts as a turret", b.X, b.Y)
		}
	}
	if n := len(settlementTurrets(m)); n != 2 {
		t.Fatalf("%d settlement turrets, want 2 (no door among them)", n)
	}
	if colonistQ := colonistGrid(g); colonistQ == pathing.Query(g.grid) {
		t.Fatal("colonists plan with the plain grid on a map of enemy doors")
	}
}

// TestEnemyDoorShutsColonistsAndAnimals: a colonist and every animal plan
// around it, in the region layers and in the search alike; its own people
// plan through it; a map with none plans on the plain grid.
func TestEnemyDoorShutsColonistsAndAnimals(t *testing.T) {
	t.Parallel()
	g, did, def := enemyCompound(t, 1, true)
	ada := outsider(t, g, "Ada")

	if g.reachable(ada, cmpInX, cmpInY) {
		t.Fatal("a colonist can reach behind an enemy door")
	}
	if _, ok := g.pathTo(ada, cmpInX, cmpInY); ok {
		t.Fatal("a colonist has a path behind an enemy door")
	}
	if p, ok := g.pathTo(def, cmpOutX, cmpOutY); !ok || !slices.Contains(p, pathing.Point{X: cmpDoorX, Y: cmpDoorY}) {
		t.Fatalf("a defender plans out through its door: ok %v path %v", ok, p)
	}
	if g.doorAt(cmpDoorX, cmpDoorY) {
		t.Fatal("the hostile step check says its own door is shut to it")
	}
	if !g.reachable(def, cmpOutX, cmpOutY) {
		t.Fatal("regions say a defender is shut in")
	}
	for _, tc := range []struct {
		name string
		tame bool
	}{{"tame", true}, {"wild", false}} {
		if _, ok := pathing.Find(g.animalPathQuery(tc.tame, false), cmpOutX, cmpOutY, cmpInX, cmpInY); ok {
			t.Fatalf("a %s animal has a path behind an enemy door", tc.name)
		}
		if !g.fenceBars(tc.tame, cmpDoorX, cmpDoorY) {
			t.Fatalf("the %s animal's step check lets it through", tc.name)
		}
	}
	m := g.regionsNow()
	for l, want := range map[region.Layer]bool{region.Colonist: false, region.Tame: false, region.Wild: false, region.Hostile: true, region.Sapper: true} {
		if got := m.Reach(l, cmpOutX, cmpOutY, cmpInX, cmpInY); got != want {
			t.Fatalf("layer %d reaches inside: %v, want %v", l, got, want)
		}
	}

	g.removeBuilding(did)
	if q, ok := colonistGrid(g).(*grid.Grid); !ok || q != g.grid {
		t.Fatalf("with no enemy door colonists should plan on the grid: %T", colonistGrid(g))
	}
	if !g.reachable(ada, cmpInX, cmpInY) {
		t.Fatal("the door is gone and the way still shut")
	}
}

// TestMoveOrderRefusesEnemyDoor: the order menu and the order queue refuse
// the door and what is behind it, and a sent order moves nobody in.
func TestMoveOrderRefusesEnemyDoor(t *testing.T) {
	t.Parallel()
	g, _, _ := enemyCompound(t, 2, false)
	ada := outsider(t, g, "Ada")
	g.setDraft(ada, true)
	if o := goHere(t, g, ada, cmpInX, cmpInY); !o.Disabled || o.Reason != ReasonUnreachable {
		t.Fatalf("behind the door: %+v", o)
	}
	if o := goHere(t, g, ada, cmpDoorX, cmpDoorY); !o.Disabled || o.Reason != ReasonBlocked {
		t.Fatalf("on the door: %+v", o)
	}
	g.Apply(command.Command{Kind: command.MovePawn, Pawn: uint64(ada), X: cmpInX, Y: cmpInY})
	g.Step(600)
	if p, _ := g.positions.Get(ada); p.Y <= cmpDoorY {
		t.Fatalf("she went inside: %+v", p)
	}
}

// TestDefenderOpensItsDoorAndItCloses: a colonist near the wall draws a
// defender out: the door opens for it through the usual wait and shuts behind
// it. Nobody stays stuck, and the colonist stays outside.
func TestDefenderOpensItsDoorAndItCloses(t *testing.T) {
	t.Parallel()
	g, did, def := enemyCompound(t, 3, true)
	ada, err := g.SpawnNamedPawn(cmpDoorX, cmpDoorY+8, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	at := Position{X: cmpDoorX, Y: cmpDoorY + 8}
	opened, through := false, false
	for i := 0; i < 4000 && !through; i++ {
		g.Step(1)
		g.positions.Set(ada, at)
		opened = opened || g.combat.Doors.IsOpen(did)
		p, _ := g.positions.Get(def)
		through = p.Y > cmpDoorY
	}
	if !through {
		t.Fatal("the defender never came out")
	}
	if !opened {
		t.Fatal("it walked through a door that never opened")
	}
	if r := g.raiders[def]; r.Mode != raid.Assaulting {
		t.Fatalf("mode %v, want assaulting", r.Mode)
	}
	for i := 0; i < door.StayOpen+40; i++ {
		g.Step(1)
		g.positions.Set(ada, at)
	}
	if g.combat.Doors.IsOpen(did) {
		t.Fatal("the door stayed open behind it")
	}
	if v := g.combat.Doors.State(did); v.Hold {
		t.Fatal("the door is held")
	}
}

// TestDefendersAnswerABrokenCompound: a hit on the compound brings a defender
// out even with no colonist in sight of it.
func TestDefendersAnswerABrokenCompound(t *testing.T) {
	t.Parallel()
	g, did, def := enemyCompound(t, 4, true)
	ada := outsider(t, g, "Ada")
	g.positions.Set(ada, Position{X: 58, Y: 58})
	g.invalidateCreatures()
	g.Step(60)
	if r := g.raiders[def]; r.Mode == raid.Assaulting {
		t.Fatal("it engaged with nobody near")
	}
	g.damageBuilding(did, 1)
	g.Step(2)
	if r := g.raiders[def]; r.Mode != raid.Assaulting {
		t.Fatalf("mode %v after the door was hit, want assaulting", r.Mode)
	}
}

// TestOrderedAttackBreaksEnemyDoor: the menu offers it to a drafted colonist
// only, she smashes it, it leaves no blueprint, and the way is open to the
// region layers and the path search at once.
func TestOrderedAttackBreaksEnemyDoor(t *testing.T) {
	t.Parallel()
	g, did, _ := enemyCompound(t, 5, false)
	ada := outsider(t, g, "Ada")
	for _, o := range g.OrdersAt(ada, cmpDoorX, cmpDoorY) {
		if o.Cmd.Order == command.OrderAttack {
			t.Fatalf("offered undrafted: %+v", o)
		}
	}
	g.setDraft(ada, true)
	g.equip = map[entity.EntityID]item.Kind{ada: item.Rifle}
	var cmd command.Command
	for _, o := range g.OrdersAt(ada, cmpDoorX, cmpDoorY) {
		if o.Cmd.Order == command.OrderAttack && strings.Contains(o.Label, "door") {
			if o.Disabled {
				t.Fatalf("disabled: %+v", o)
			}
			cmd = o.Cmd
		}
	}
	if cmd.Order != command.OrderAttack {
		t.Fatalf("no attack option: %+v", g.OrdersAt(ada, cmpDoorX, cmpDoorY))
	}
	g.Apply(cmd)
	g.Step(1)
	if j, _ := g.jobs.Get(ada); j.Kind != job.Attack || j.Target != did || !j.Forced {
		t.Fatalf("the order made job %+v", j)
	}
	for i := 0; i < 60000; i++ {
		if _, ok := g.bldgs.Get(did); !ok {
			break
		}
		g.Step(1)
	}
	if _, ok := g.bldgs.Get(did); ok {
		t.Fatal("ordered fire did not bring the door down")
	}
	if g.layerFrameAt(cmpDoorX, cmpDoorY, construction.LayerObject) != 0 {
		t.Fatal("a smashed enemy door left a blueprint")
	}
	if !g.reachable(ada, cmpInX, cmpInY) {
		t.Fatal("regions still shut the way")
	}
	if _, ok := g.pathTo(ada, cmpInX, cmpInY); !ok {
		t.Fatal("no path through the smashed door")
	}
	g.Step(5)
	if j, _ := g.jobs.Get(ada); j.Kind == job.Attack {
		t.Fatalf("still on it: %+v", j)
	}
}

// TestMeleeBreaksEnemyDoorFromInside: a colonist dropped inside by pods walks
// to the door and clubs her way out: she does not hang, has no way to the
// outside until then, and is not sent for anything out there.
func TestMeleeBreaksEnemyDoorFromInside(t *testing.T) {
	t.Parallel()
	g, did, _ := enemyCompound(t, 6, false)
	ada, err := g.SpawnNamedPawn(cmpInX, cmpInY, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	if g.reachable(ada, cmpOutX, cmpOutY) {
		t.Fatal("from inside she can reach the outside")
	}
	if o := goHere(t, g, ada, cmpOutX, cmpOutY); !o.Disabled {
		t.Fatalf("Go here outside: %+v", o)
	}
	g.dropStackOf(item.Stack{Kind: item.Steel, Qty: 5}, cmpOutX, cmpOutY)
	st := g.itemAt(cmpOutX, cmpOutY)
	g.Step(1200)
	if p, _ := g.positions.Get(ada); p.Y >= cmpDoorY {
		t.Fatalf("she got out: %+v", p)
	}
	if j, _ := g.jobs.Get(ada); st == 0 || j.Target == st {
		t.Fatalf("sent for a stack outside: %+v", j)
	}
	g.setDraft(ada, true)
	g.prioritize(ada, command.OrderAttack, cmpDoorX, cmpDoorY, construction.LayerObject)
	for i := 0; i < 60000; i++ {
		if _, ok := g.bldgs.Get(did); !ok {
			break
		}
		g.Step(1)
	}
	if _, ok := g.bldgs.Get(did); ok {
		t.Fatal("she could not break out")
	}
	if !g.reachable(ada, cmpOutX, cmpOutY) {
		t.Fatal("still shut in")
	}
}

// TestColonistOnEnemyDoorWalksOff: standing on the door's own cell (a pod
// landed there) she can still step off, and has a plan.
func TestColonistOnEnemyDoorWalksOff(t *testing.T) {
	t.Parallel()
	g, _, _ := enemyCompound(t, 7, false)
	ada, err := g.SpawnNamedPawn(cmpDoorX, cmpDoorY, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	if !g.reachable(ada, cmpOutX, cmpOutY) || !g.reachable(ada, cmpInX, cmpInY) {
		t.Fatal("no way off the door's cell")
	}
	g.Apply(command.Command{Kind: command.MovePawn, Pawn: uint64(ada), X: cmpOutX, Y: cmpOutY})
	g.Step(1200)
	if p, _ := g.positions.Get(ada); p.X == cmpDoorX && p.Y == cmpDoorY {
		t.Fatal("she hung on the door's cell")
	}
}

// TestAutoEngageLeavesEnemyDoorAlone: a drafted colonist with fire at will
// beside it never picks the door by herself.
func TestAutoEngageLeavesEnemyDoorAlone(t *testing.T) {
	t.Parallel()
	g, did, _ := enemyCompound(t, 8, false)
	ada, err := g.SpawnNamedPawn(cmpDoorX, cmpDoorY+3, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	g.equip = map[entity.EntityID]item.Kind{ada: item.Rifle}
	g.setDraft(ada, true)
	for range 600 {
		g.Step(1)
		if j, _ := g.jobs.Get(ada); j.Kind == job.Attack {
			t.Fatalf("she picked %+v unasked", j)
		}
	}
	if b, _ := g.bldgs.Get(did); b.Damage != 0 {
		t.Fatalf("door damaged: %+v", b)
	}
}

// TestWorkSkipsBehindEnemyDoor: a stack behind the door is no job, while the
// one outside gets hauled, so the colonist is not merely idle.
func TestWorkSkipsBehindEnemyDoor(t *testing.T) {
	t.Parallel()
	g, _, _ := enemyCompound(t, 9, false)
	ada := outsider(t, g, "Ada")
	k, proto := stockpileProto(0)
	g.paintZone(k, proto, cmpOutX-3, cmpOutY-3, cmpOutX-1, cmpOutY-1)
	g.dropStackOf(item.Stack{Kind: item.Steel, Qty: 5}, cmpInX, cmpInY)
	g.dropStackOf(item.Stack{Kind: item.Steel, Qty: 5}, cmpOutX+3, cmpOutY)
	inside, outside := g.itemAt(cmpInX, cmpInY), g.itemAt(cmpOutX+3, cmpOutY)
	if inside == 0 || outside == 0 {
		t.Fatal("no stacks")
	}
	for range 1800 {
		g.Step(1)
		if j, _ := g.jobs.Get(ada); j.Target == inside {
			t.Fatalf("sent for the stack behind the door: %+v", j)
		}
	}
	if st, _ := g.stacks.Get(inside); st.X != cmpInX || st.Y != cmpInY {
		t.Fatalf("the stack behind the door moved: %+v", st)
	}
	if st, ok := g.stacks.Get(outside); ok && st.X == cmpOutX+3 && st.Y == cmpOutY {
		t.Fatalf("the stack outside was never hauled: %+v", st)
	}
}

// TestClaimEnemyDoorOnlyAfterDefeat: refused while a defender stands, then
// claimed: not hostile, ruin or ancient, the colonist walks through, and it is
// shut to raiders as any of ours is. A turret stays refused.
func TestClaimEnemyDoorOnlyAfterDefeat(t *testing.T) {
	t.Parallel()
	g, did, def := enemyCompound(t, 10, true)
	ada := outsider(t, g, "Ada")
	tur := buildNow(t, g, construction.SentryTurret, item.Steel, construction.South, 40, 5)
	makeSettlementTurret(g, tur)
	g.combat.Turrets.SetHold(tur, true)
	whole := ax1Rect(command.DesignateClaim, cmpDoorX, cmpDoorY, cmpDoorX, cmpDoorY, true)
	ax1Cmd(g, whole)
	if b, _ := g.bldgs.Get(did); !b.Hostile || !b.Ancient {
		t.Fatalf("claimed with a defender standing: %+v", b)
	}
	if g.reachable(ada, cmpInX, cmpInY) {
		t.Fatal("the refused claim opened the way")
	}
	fell(g, def)
	g.Step(1)
	ax1Cmd(g, whole)
	b, _ := g.bldgs.Get(did)
	if b.Hostile || b.Ancient || b.Ruin {
		t.Fatalf("not claimed after the defeat: %+v", b)
	}
	if !g.reachable(ada, cmpInX, cmpInY) {
		t.Fatal("claimed, it still shuts the colonist out")
	}
	if !g.doorAt(cmpDoorX, cmpDoorY) {
		t.Fatal("claimed, it is not shut to hostiles")
	}
	if colonistQ := colonistGrid(g); colonistQ != pathing.Query(g.grid) {
		t.Fatalf("a claimed door costs a query: %T", colonistQ)
	}
	ax1Cmd(g, ax1Rect(command.DesignateClaim, 40, 5, 40, 5, true))
	if tb, _ := g.bldgs.Get(tur); !tb.Hostile {
		t.Fatalf("the turret was claimed: %+v", tb)
	}
}

// TestEnemyDoorRefusesThePlayer: hold open, forbid, deconstruct, uninstall
// and move don't touch it; forbidding can't shut a defender in.
func TestEnemyDoorRefusesThePlayer(t *testing.T) {
	t.Parallel()
	g, did, def := enemyCompound(t, 11, true)
	g.Apply(command.Command{Kind: command.SetDoorHoldOpen, X: cmpDoorX, Y: cmpDoorY, Enabled: true})
	if g.combat.Doors.Held(did) {
		t.Fatal("held open on our word")
	}
	g.Apply(command.Command{Kind: command.SetForbidden, Target: uint64(did), Enabled: true})
	if !g.grid.Walkable(cmpDoorX, cmpDoorY) || !g.reachable(def, cmpOutX, cmpOutY) {
		t.Fatal("forbidding it shut the defenders in")
	}
	ax1Cmd(g, ax1Rect(command.DesignateDeconstruct, cmpDoorX, cmpDoorY, cmpDoorX, cmpDoorY, true))
	if b, _ := g.bldgs.Get(did); b.Deconstruct {
		t.Fatal("marked for deconstruction")
	}
	ax1Cmd(g, ax1Rect(command.DesignateUninstall, cmpDoorX, cmpDoorY, cmpDoorX, cmpDoorY, true))
	if b, _ := g.bldgs.Get(did); b.Uninstall {
		t.Fatal("marked to uninstall")
	}
	g.Step(120)
	if g.combat.Turrets.Aim(did).Target != 0 {
		t.Fatal("a door aims")
	}
}

// TestRealCompoundKeepsColonistOut: on the real map a colonist at the gate
// has no way inside, and a defender has a way out. No animal goes through.
func TestRealCompoundKeepsColonistOut(t *testing.T) {
	t.Parallel()
	g, m, _, _, ada := besieged(t, faction.Settlers)
	cur := g.mapState
	g.mapState = m
	defer func() { g.mapState = cur }()
	ds := defenders(m)
	if len(ds) == 0 {
		t.Fatal("no defenders")
	}
	dp, _ := g.positions.Get(ds[0])
	p, _ := g.positions.Get(ada)
	if g.reachable(ada, dp.X, dp.Y) {
		t.Fatalf("ada at %+v reaches a defender at %+v through shut doors", p, dp)
	}
	if !g.reachable(ds[0], p.X, p.Y) {
		t.Fatalf("defender at %+v has no way to ada at %+v", dp, p)
	}
	if _, ok := pathing.Find(g.animalPathQuery(true, false), p.X, p.Y, dp.X, dp.Y); ok {
		t.Fatal("a tame animal gets inside")
	}
}
