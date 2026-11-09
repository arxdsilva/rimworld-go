package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// EAI-010 (sight and memory), EAI-012 (sparking turrets), EAI-021 and
// EAI-023 (raider look-ups through indexes) whole-game tests.

// eaiRaid sends one club raider per spot in pts, placed there.
func eaiRaid(t *testing.T, g *Game, pts ...Position) []entity.EntityID {
	t.Helper()
	kit := make([]item.Kind, len(pts))
	for i := range kit {
		kit[i] = item.Club
	}
	g.raidWith(kit, raid.Assault)
	ids := slices.Clone(g.raiderIDs)
	if len(ids) != len(pts) {
		t.Fatalf("raiders %d, want %d", len(ids), len(pts))
	}
	for i, id := range ids {
		g.positions.Set(id, pts[i])
	}
	return ids
}

// shootRaider wounds raider rid as a shot from by would.
func shootRaider(g *Game, rid, by entity.EntityID) {
	g.hurt(rid, 1, by)
	g.combat.Gear.Count(by, rid, 1, false)
	g.lastHit[rid] = hitRec{Tick: uint64(g.clock.Now()), Damage: 1}
}

func dist(g *Game, a, b entity.EntityID) int {
	pa, _ := g.positions.Get(a)
	pb, _ := g.positions.Get(b)
	return weapon.Distance(pa.X, pa.Y, pb.X, pb.Y)
}

// TestRaidersAdvanceUntilSeen (EAI-010): raiders far from a colonist walk
// on the colony knowing of no foe (the read model says so) and attack no
// one until one of them has the colonist in sight.
func TestRaidersAdvanceUntilSeen(t *testing.T) {
	t.Parallel()
	g, ada := def4Game(t, 120, 60, 60, 30)
	g.draft[ada] = true // she holds her ground
	ids := eaiRaid(t, g, Position{X: 5, Y: 28}, Position{X: 5, Y: 32})
	hx, hy := g.home()
	def4Step(g, 1, 1)
	for _, id := range ids {
		j, _ := g.jobs.Get(id)
		p, _ := g.positions.Get(id)
		if !g.raid.Raiders.Advancing(id) || j.Kind != job.Move || weapon.Distance(j.TX, j.TY, hx, hy) >= weapon.Distance(p.X, p.Y, hx, hy) ||
			weapon.Distance(j.TX, j.TY, p.X, p.Y) > 2*raid.SightRange+4 {
			t.Fatalf("raider %d advances on home (%d, %d) a leg at a time: %+v", id, hx, hy, j)
		}
	}
	seen, attacked := false, false
	for i := 0; i < 3000 && !attacked; i++ {
		def4Step(g, 1, 1)
		grp := g.raidGroups[0]
		for _, id := range ids {
			if j, _ := g.jobs.Get(id); j.Kind == job.Attack {
				if grp.Foe == 0 {
					t.Fatalf("tick %d: raider %d attacks before any saw a foe", i, id)
				}
				attacked = attacked || j.Target == ada
			}
		}
		if grp.Foe == 0 || seen {
			continue
		}
		seen = true
		close := false
		for _, id := range ids {
			close = close || dist(g, id, ada) <= raid.SightRange
		}
		if grp.Foe != ada || !close {
			t.Fatalf("tick %d: the group learnt of %d, none in sight of her", i, grp.Foe)
		}
		for _, id := range ids {
			if g.raid.Raiders.Advancing(id) && g.raiders[id].Foe == ada {
				t.Fatal("one that saw her is no longer advancing")
			}
		}
	}
	if !seen || !attacked {
		t.Fatalf("seen %v, attacked %v", seen, attacked)
	}
}

// TestGroupMateSightShared (EAI-010): a raider far off learns of the foe a
// group mate sees, and walks on it instead of on home.
func TestGroupMateSightShared(t *testing.T) {
	t.Parallel()
	g, ada := def4Game(t, 120, 60, 100, 30)
	g.draft[ada] = true
	ids := eaiRaid(t, g, Position{X: 90, Y: 30}, Position{X: 20, Y: 50})
	far := ids[1]
	for i := 0; i < 2*15; i++ {
		def4Step(g, 1, 1)
	}
	if g.raidGroups[0].Foe != ada || g.raiders[ids[0]].Foe != ada {
		t.Fatalf("the near raider saw her: group %d, raider %d", g.raidGroups[0].Foe, g.raiders[ids[0]].Foe)
	}
	j, _ := g.jobs.Get(far)
	ap, _ := g.positions.Get(ada)
	if g.raid.Raiders.Advancing(far) || j.Kind != job.Move || weapon.Distance(j.TX, j.TY, ap.X, ap.Y) > 4 {
		t.Fatalf("the far raider walks on her, not home: advancing %v, %+v", g.raid.Raiders.Advancing(far), j)
	}
}

// TestShotRaiderTurnsOnShooter (EAI-010): a raider advancing on home that
// is shot learns of the shooter and walks on it.
func TestShotRaiderTurnsOnShooter(t *testing.T) {
	t.Parallel()
	g, ada := def4Game(t, 120, 60, 100, 30)
	g.draft[ada] = true
	rid := eaiRaid(t, g, Position{X: 20, Y: 30})[0]
	def4Step(g, 1, 1)
	if !g.raid.Raiders.Advancing(rid) {
		t.Fatal("advancing")
	}
	shootRaider(g, rid, ada)
	def4Step(g, 1, 1)
	j, _ := g.jobs.Get(rid)
	if r := g.raiders[rid]; r.Foe != ada || g.raidGroups[0].Foe != ada || g.raid.Raiders.Advancing(rid) ||
		j.Kind != job.Move || weapon.Distance(j.TX, j.TY, 100, 30) > 4 {
		t.Fatalf("shot, it turns on her: %+v, %+v", r, j)
	}
}

// TestRaiderTargetMemoryNoFlap (EAI-010): a raider fighting one colonist,
// shot again and again by her and by one farther off, keeps its target
// (it turns on a shooter only switchMargin closer), and switches at most
// when its target falls.
func TestRaiderTargetMemoryNoFlap(t *testing.T) {
	t.Parallel()
	g, ada := def4Game(t, 60, 40, 30, 20)
	bo, _ := g.SpawnNamedPawn(30, 26, "Bo")
	g.draft[ada], g.draft[bo] = true, true
	rid := eaiRaid(t, g, Position{X: 30, Y: 17})[0]
	var last entity.EntityID
	switches, fights := 0, 0
	for i := 0; i < 400; i++ {
		if i%20 == 10 {
			shootRaider(g, rid, []entity.EntityID{bo, ada}[i/20%2])
		}
		def4Step(g, 1, 1)
		j, _ := g.jobs.Get(rid)
		if j.Kind != job.Attack {
			continue
		}
		fights++
		if last != 0 && j.Target != last && g.standingPawn(last) {
			switches++
		}
		last = j.Target
	}
	if fights == 0 || switches != 0 {
		t.Fatalf("fought %d ticks, switched %d times with its target standing", fights, switches)
	}
}

// lightTurret lights turret id's fuse (combat/turret_fuse.go).
func lightTurret(g *Game, id entity.EntityID) {
	b, _ := g.bldgs.Get(id)
	s := g.combat.Turrets.Export()
	if s == nil {
		s = &combat.TurretSave{}
	}
	s.Sparks = append(s.Sparks, combat.Fuse{ID: id, Kind: b.Kind, X: b.X, Y: b.Y, At: uint64(g.clock.Now()) + combat.TurretFuseTicks})
	g.combat.Turrets.Import(s)
}

// sparkingTurret builds a sentry turret at (x, y) and lights its fuse.
func sparkingTurret(t *testing.T, g *Game, x, y int) entity.EntityID {
	t.Helper()
	if !g.BuildNow("sentry turret", "steel", "south", x, y) {
		t.Fatal("turret")
	}
	id := g.bldgIDs[len(g.bldgIDs)-1]
	lightTurret(g, id)
	shown := false
	for _, v := range g.buildingViews() {
		shown = shown || v.ID == id && v.Sparking
	}
	if !g.combat.Turrets.Sparking(id) || !shown {
		t.Fatal("sparking, on the read model too")
	}
	return id
}

// inBlast: (x, y) is within a sentry turret's blast (3.9 tiles) of (tx, ty).
func inBlast(x, y, tx, ty int) bool { dx, dy := x-tx, y-ty; return dx*dx+dy*dy <= 15 }

// TestRaidersStepFromSparkingTurret (EAI-012): a raider beside a sparking
// turret walks out of its blast before it goes off.
func TestRaidersStepFromSparkingTurret(t *testing.T) {
	t.Parallel()
	g, _ := def4Game(t, 80, 40, 75, 35)
	sparkingTurret(t, g, 20, 20)
	rid := eaiRaid(t, g, Position{X: 21, Y: 20})[0]
	def4Step(g, 1, 1)
	if j, _ := g.jobs.Get(rid); j.Kind != job.Move || weapon.Distance(j.TX, j.TY, 20, 20) < 5 {
		t.Fatalf("out to reach+2: %+v", j)
	}
	for i := 0; i < 60; i++ {
		def4Step(g, 1, 1)
	}
	if p, _ := g.positions.Get(rid); inBlast(p.X, p.Y, 20, 20) {
		t.Fatalf("still in the blast at %+v", p)
	}
}

// TestColonistsStepFromSparkingTurret (EAI-012): an idle colonist beside a
// sparking turret walks out of its blast and stays out until it goes off.
func TestColonistsStepFromSparkingTurret(t *testing.T) {
	t.Parallel()
	g, ada := def4Game(t, 60, 40, 21, 20)
	sparkingTurret(t, g, 20, 20)
	out, back, moves := false, 0, 0
	var was job.Record
	for i := 0; i < combat.TurretFuseTicks-5; i++ {
		def4Step(g, 1, 1)
		p, _ := g.positions.Get(ada)
		in := inBlast(p.X, p.Y, 20, 20)
		if out && in {
			back++
		}
		out = out || !in
		if j, _ := g.jobs.Get(ada); j.Kind == job.Move && j.Active() && (was.Kind != job.Move || !was.Active()) {
			moves++
		}
		was, _ = g.jobs.Get(ada)
	}
	if !out || back != 0 || moves > 2 {
		t.Fatalf("out %v, back in %d ticks, %d walks", out, back, moves)
	}
}

// TestNearestColonistRingsMatchScan (EAI-021): the nearest standing
// colonist found ring by ring through the creature index is the list
// scan's (ties to the lower id), near and far, with some colonists down.
func TestNearestColonistRingsMatchScan(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 5, Width: 160, Height: 120})
	seed := uint64(3)
	next := func(n int) int {
		seed = seed*6364136223846793005 + 1442695040888963407
		return int(seed>>33) % n
	}
	for i := 0; i < 40; i++ {
		id, err := g.SpawnPawn(next(50)+10, next(40)+10)
		if err != nil {
			t.Fatal(err)
		}
		if i%7 == 0 {
			kill(g, id)
		}
	}
	g.Step(1)
	for q := 0; q < 400; q++ {
		p := Position{X: next(160), Y: next(120)}
		a, ap, aok := nearestColonistRings(g, p)
		b, bp, bok := nearestColonistScan(g, p)
		if a != b || ap != bp || aok != bok {
			t.Fatalf("at %+v: rings %d %+v, scan %d %+v", p, a, ap, b, bp)
		}
	}
	if len(g.pawnIDs) <= ringPawns {
		t.Fatalf("%d colonists: the rings are not in use", len(g.pawnIDs))
	}
}

// lootScan is raidMischief.Loot by a scan of every item (ties to the lower
// id).
func lootScan(g *Game, x, y, r int) (entity.EntityID, int, int, bool) {
	var best entity.EntityID
	bx, by, bestD := 0, 0, r+1
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && worthStealing(st) {
			if d := weapon.Distance(x, y, st.X, st.Y); d < bestD || d == bestD && id < best {
				best, bx, by, bestD = id, st.X, st.Y, d
			}
		}
	}
	return best, bx, by, best != 0
}

// TestThiefPicksSameAsScan (EAI-023): a thief's loot, read from the item
// grid, is what a scan of every item picks.
func TestThiefPicksSameAsScan(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 9, Width: 100, Height: 80})
	seed := uint64(11)
	next := func(n int) int {
		seed = seed*6364136223846793005 + 1442695040888963407
		return int(seed>>33) % n
	}
	kinds := []item.Kind{item.Silver, item.Rifle, item.Wood, item.Steel, item.Club}
	for i := 0; i < 120; i++ {
		if _, err := g.SpawnStack(kinds[next(len(kinds))], 1+next(60), next(100), next(80)); err != nil {
			t.Fatal(err)
		}
	}
	m := raidMischief{g: g}
	for q := 0; q < 300; q++ {
		x, y, r := next(100), next(80), 5+next(60)
		a, ax, ay, aok := m.Loot(x, y, r)
		b, bx, by, bok := lootScan(g, x, y, r)
		if a != b || ax != bx || ay != by || aok != bok {
			t.Fatalf("at (%d, %d) r %d: grid %d, scan %d", x, y, r, a, b)
		}
	}
}

// flammableScan is raidMischief.Flammable by a scan of every building.
func flammableScan(g *Game, x, y, r int) (int, int, bool) {
	var best entity.EntityID
	bx, by, bestD := 0, 0, r+1
	for _, id := range g.bldgIDs {
		b, ok := g.bldgs.Get(id)
		if !ok || b.Stuff != item.Wood || notColonyPiece(b) || g.fireAt(b.X, b.Y) > 0 {
			continue
		}
		if d := weapon.Distance(x, y, b.X, b.Y); d < bestD || d == bestD && id < best {
			best, bx, by, bestD = id, b.X, b.Y, d
		}
	}
	return bx, by, best != 0
}

// TestBurnerIndex (EAI-023): a burner's mark, read from the wooden
// building grid, is what a scan of every building picks; the grid is
// rebuilt only when a building comes or goes, and sees one built or
// burning since.
func TestBurnerIndex(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 13, Width: 100, Height: 80})
	seed := uint64(17)
	next := func(n int) int {
		seed = seed*6364136223846793005 + 1442695040888963407
		return int(seed>>33) % n
	}
	stuff := []string{"wood", "steel"}
	for i := 0; i < 60; i++ {
		g.BuildNow("wall", stuff[next(2)], "south", next(100), next(80))
	}
	wi := &woodIndex{}
	m := raidMischief{g: g, wood: wi}
	check := func(label string) {
		t.Helper()
		for q := 0; q < 200; q++ {
			x, y, r := next(100), next(80), 5+next(50)
			ax, ay, aok := m.Flammable(x, y, r)
			bx, by, bok := flammableScan(g, x, y, r)
			if ax != bx || ay != by || aok != bok {
				t.Fatalf("%s: at (%d, %d) r %d: grid (%d, %d) %v, scan (%d, %d) %v", label, x, y, r, ax, ay, aok, bx, by, bok)
			}
		}
	}
	check("built")
	builds := wi.builds
	check("again")
	if wi.builds != builds {
		t.Fatal("rebuilt with nothing changed")
	}
	for _, id := range g.bldgIDs {
		if b, _ := g.bldgs.Get(id); b.Stuff == item.Wood {
			g.ignite(b.X, b.Y, burstFireSize)
			break
		}
	}
	check("one alight")
	if !g.BuildNow("wall", "wood", "south", 50, 40) && !g.BuildNow("wall", "wood", "south", 51, 41) {
		t.Fatal("wall")
	}
	check("one more")
	if wi.builds == builds {
		t.Fatal("a new building rebuilds the grid")
	}
}
