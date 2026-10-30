package game

import (
	"testing"

	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	raiddef "rimworld-go/internal/domain/raid"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/terrain"
)

// CBT-4 raider and colonist tactics in the whole game (unit tests:
// combat/tactics_test.go, mind/tantrum_test.go, prison/escape_test.go).

// coverRig: Ada drafted with a rifle at (40, 15), holding and not
// firing; sandbags at x 20 (rows 13-17); a rifle raider at (12, 15).
func coverRig(t *testing.T) (*Game, entity.EntityID, entity.EntityID) {
	t.Helper()
	g, ada := def4Game(t, 50, 30, 40, 15)
	g.equip[ada] = item.Rifle
	g.draft[ada] = true
	g.combat.Drafts.SetFireAtWill(ada, false)
	g.combat.Drafts.SetHold(ada, true)
	for y := 13; y <= 17; y++ {
		if !g.BuildNow("sandbags", "wood", "south", 20, y) {
			t.Fatal("sandbags")
		}
	}
	rid := turretRaider(t, g, 12, 15)
	return g, ada, rid
}

// TestRaiderSeeksCoverUnderFire: a raider shooting at an armed colonist
// first moves behind the sandbags between them.
func TestRaiderSeeksCoverUnderFire(t *testing.T) {
	g, ada, rid := coverRig(t)
	behind := false
	for i := 0; i < 900 && !behind; i++ {
		def4Step(g, 1, 1)
		p, _ := g.positions.Get(rid)
		behind = p.X == 19 && p.Y >= 13 && p.Y <= 17
	}
	if !behind {
		p, _ := g.positions.Get(rid)
		t.Fatalf("the raider never stood behind the sandbags: at %+v, record %+v", p, g.raiders[rid])
	}
	p, _ := g.positions.Get(rid)
	ap, _ := g.positions.Get(ada)
	if c := g.combat.Sight.Cover(ap.X, ap.Y, p.X, p.Y); c <= 0 {
		t.Fatalf("its spot has cover against Ada: %d", c)
	}
}

// TestRaiderCoverDoesNotFlap: once in cover the raider holds it; it
// picks a new spot at most rarely and never steps back and forth.
func TestRaiderCoverDoesNotFlap(t *testing.T) {
	g, ada, rid := coverRig(t)
	var spots []Position
	for i := 0; i < 2400 && g.standingPawn(ada); i++ {
		def4Step(g, 1, 1)
		if r := g.raiders[rid]; r.CoverAt != 0 && (len(spots) == 0 || spots[len(spots)-1] != (Position{X: r.CoverX, Y: r.CoverY})) && r.CoverX != 0 {
			spots = append(spots, Position{X: r.CoverX, Y: r.CoverY})
		}
	}
	if len(spots) == 0 || len(spots) > 2 {
		t.Fatalf("cover spots chosen: %v", spots)
	}
	for i := 2; i < len(spots); i++ {
		if spots[i] == spots[i-2] {
			t.Fatalf("the raider flapped between cover spots: %v", spots)
		}
	}
}

// TestRaiderPrefersArmedColonist: threat-weighted, a raider goes for the
// rifleman a little further off over the unarmed colonist nearer.
func TestRaiderPrefersArmedColonist(t *testing.T) {
	g, ada := def4Game(t, 50, 30, 30, 15)
	bo, _ := g.SpawnNamedPawn(22, 15, "Bo")
	g.equip[ada] = item.Rifle
	for _, id := range []entity.EntityID{ada, bo} {
		g.draft[id] = true
		g.combat.Drafts.SetFireAtWill(id, false)
		g.combat.Drafts.SetHold(id, true)
	}
	rid := turretRaider(t, g, 14, 15)
	r := g.raiders[rid]
	r.Mode, r.CoverAt = raid.Assaulting, 1<<62 // no cover move: the target is the point
	g.raiders[rid] = r
	var target entity.EntityID
	for i := 0; i < 60 && target == 0; i++ {
		def4Step(g, 1, 1)
		if j, _ := g.jobs.Get(rid); j.Kind == job.Attack {
			target = j.Target
		}
	}
	if target != ada {
		t.Fatalf("the raider went for %d, not the rifleman %d (Bo %d)", target, ada, bo)
	}
}

// TestRaiderDodgesGrenade: a raider steps out of a grenade's blast
// before it lands.
func TestRaiderDodgesGrenade(t *testing.T) {
	g, ada := def4Game(t, 50, 30, 40, 15)
	rid := turretRaider(t, g, 20, 15)
	g.combat.Shots.Lob(ada, weapon.FragGrenade, 30, 15, 20, 15, 20, 15)
	g.Step(1)
	j, _ := g.jobs.Get(rid)
	b := weapon.DefOf(weapon.FragGrenade).Blast
	if j.Kind != job.Move || b.Within(j.TX-20, j.TY-15) {
		t.Fatalf("the raider walks clear of the blast: %+v", j)
	}
}

// TestRetreatingRaiderGrabsLoot: a raider leaving picks up a stack worth
// taking on its way out, once.
func TestRetreatingRaiderGrabsLoot(t *testing.T) {
	g, _ := def4Game(t, 50, 30, 45, 15)
	sid, _ := g.SpawnStack(item.Rifle, 1, 24, 15)
	rid := turretRaider(t, g, 20, 15)
	r := g.raiders[rid]
	r.Mode = raid.Fleeing
	g.raiders[rid] = r
	for i := 0; i < 600 && g.raiders[rid].LootQty == 0 && g.isRaider(rid); i++ {
		def4Step(g, 1, 1)
	}
	if r := g.raiders[rid]; r.Loot != item.Rifle || r.LootQty != 1 {
		t.Fatalf("the raider grabbed the rifle on the way: %+v", r)
	}
	if _, ok := g.stacks.Get(sid); ok || logHas(g, "on the way out") != 1 {
		t.Fatal("the stack is gone and the grab logged")
	}
}

// TestBurnRaidBurnsCrops: a burner sets a sown crop alight.
func TestBurnRaidBurnsCrops(t *testing.T) {
	g, _ := def4Game(t, 40, 20, 35, 10)
	pid, err := g.SpawnCrop("corn", 20, 10, 60)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := g.plants.Get(pid)
	p.Zone = true
	g.plants.Set(pid, p)
	if !(raidTactics{g}).Crop(20, 10) || (raidTactics{g}).Crop(21, 10) {
		t.Fatal("the crop is a burner's mark")
	}
	g.raidWith([]item.Kind{item.Club}, raid.Burn)
	lit := false
	for i := 0; i < 3000 && !lit; i++ {
		def4Step(g, 1, 1)
		lit = g.fireAt(20, 10) > 0
	}
	if !lit {
		t.Fatal("the burner sets the crop alight")
	}
}

// TestSiegeResupply: a camped siege gets a pod of meals and full mortar
// racks once a day.
func TestSiegeResupply(t *testing.T) {
	g, _ := def4Game(t, 90, 90, 45, 45)
	g.Siege(3)
	for i := 0; i < 400 && !g.raidGroups[0].Camped; i++ {
		def4Step(g, 25, 25)
	}
	grp := &g.raidGroups[0]
	if !grp.Camped || grp.Supply == 0 {
		t.Fatalf("camped with a supply date: %+v", *grp)
	}
	if grp.Supply < uint64(g.clock.Now())+clock.TicksPerDay-100 {
		t.Fatal("the first supply comes a day after camping")
	}
	grp.Supply = uint64(g.clock.Now()) + 1
	meals := func() int {
		n := 0
		for _, id := range g.itemIDs {
			if st, _ := g.stacks.Get(id); st.Kind == item.SurvivalMeal {
				n += st.Qty
			}
		}
		return n
	}
	before := meals()
	def4Step(g, 3, 1)
	if meals() != before+12 || g.raidGroups[0].Supply <= uint64(g.clock.Now()) {
		t.Fatalf("12 meals landed (%d → %d) and the next supply is set", before, meals())
	}
}

// TestPrisonBreakFightsAndFlees: an escaping prisoner is hostile,
// flees, and bashes through a door that shuts it in.
func TestPrisonBreakFightsAndFlees(t *testing.T) {
	g, ada := def4Game(t, 40, 20, 5, 5)
	rid := turretRaider(t, g, 20, 10)
	g.makePrisoner(rid, ada)
	if !g.prison.Is(rid) || g.standingRaider(rid) {
		t.Fatal("a prisoner is no hostile")
	}
	g.prison.Drop(rid)
	(prisonEscapes{g}).Escape(rid)
	if !g.standingRaider(rid) || !g.raiders[rid].Escaping || g.raiders[rid].Mode != raid.Fleeing {
		t.Fatalf("an escapee is a fleeing hostile: %+v", g.raiders[rid])
	}
	gone := false
	for i := 0; i < 3000 && !gone; i++ {
		def4Step(g, 1, 1)
		gone = !g.isRaider(rid)
	}
	if !gone {
		t.Fatal("the escapee got away")
	}
}

// TestMechBashesShutDoor: a mechanoid shut out by walls and a door breaks
// its way in (a wall or the door on its breach route).
func TestMechBashesShutDoor(t *testing.T) {
	g, _ := def4Game(t, 30, 30, 12, 12)
	for x := 10; x <= 14; x++ {
		for _, y := range []int{10, 14} {
			if x != 12 || y != 10 {
				g.BuildNow("wall", "wood", "south", x, y)
			}
		}
	}
	for y := 11; y <= 13; y++ {
		g.BuildNow("wall", "wood", "south", 10, y)
		g.BuildNow("wall", "wood", "south", 14, y)
	}
	g.BuildNow("door", "wood", "south", 12, 10)
	door := g.layerBldgAt(12, 10, construction.LayerObject)
	g.setForbidden(door, true) // Ada stays shut in
	g.RaidMechs(1)
	hit := false
	for i := 0; i < 6000 && !hit; i++ {
		def4Step(g, 1, 1)
		for _, id := range g.bldgIDs {
			if b, _ := g.bldgs.Get(id); (b.Kind == construction.Wall || b.Kind == construction.Door) && b.HP() < construction.MaxHP(b.Kind, b.Stuff) {
				hit = true
			}
		}
		_, ok := g.bldgs.Get(door)
		hit = hit || !ok
	}
	if !hit {
		t.Fatal("the mechanoid never bashed its way in")
	}
}

// TestMechDropPods: a mechanoid raid by drop pod falls, then lands its
// machines beside their shells; the pods survive a save.
func TestMechDropPods(t *testing.T) {
	g, _ := def4Game(t, 50, 40, 25, 20)
	if n := g.raid.Mechs.Raid(raid.MechRaidMinPoints, raiddef.DropPodCentre, uint64(g.clock.Now())); n == 0 {
		t.Fatal("pods planned")
	}
	falling := 0
	for _, a := range g.World().Arrivals {
		if a.Hostile && !a.Landed {
			falling++
		}
	}
	if falling == 0 || len(g.animalIDs) != 0 {
		t.Fatalf("pods fall first: %d falling, %d animals", falling, len(g.animalIDs))
	}
	g = saveLoad(t, g)
	def4Step(g, raiddef.PodFallTicks+raiddef.PodStagger+1, 1)
	mechs := 0
	for _, id := range g.animalIDs {
		if a, _ := g.animals.Get(id); animal.IsMech(a.Species) {
			mechs++
		}
	}
	if mechs != falling {
		t.Fatalf("%d mechanoids out of %d pods", mechs, falling)
	}
}

// hiveRig is a dark, roofed cave by a rock face west of the colony.
func hiveRig(t *testing.T) (*Game, entity.EntityID) {
	t.Helper()
	g, ada := def4Game(t, 60, 30, 50, 15)
	for y := 0; y < 30; y++ {
		for x := 0; x < 5; x++ {
			g.setTerrain(x, y, terrain.Rock)
		}
	}
	roofOver(g, 0, 0, 19, 29)
	g.Step(1)
	return g, ada
}

// TestHiveMoundBreedsSpreadsAndStops: an infestation in a dark, warm,
// roofed cave raises a hive mound whose swarm is its brood; it breeds and
// spreads; destroyed, it breeds no more; it survives a save.
func TestHiveMoundBreedsSpreadsAndStops(t *testing.T) {
	g, _ := hiveRig(t)
	if !(raidNest{g}).HiveSpot(8, 10) || (raidNest{g}).HiveSpot(30, 10) {
		t.Fatal("hive spots: under the roof only")
	}
	g.HiveSwarm()
	hives := g.raid.Hives.List()
	if len(hives) != 1 || len(hives[0].Brood) == 0 {
		t.Fatalf("one hive mound with the swarm as its brood: %+v", hives)
	}
	if b, ok := g.bldgs.Get(hives[0].ID); !ok || b.Kind != construction.HiveMound || !b.Ancient {
		t.Fatal("the hive mound stands, no one's")
	}
	g = saveLoad(t, g)
	if len(g.raid.Hives.List()) != 1 {
		t.Fatal("hives are saved")
	}
	for _, id := range g.raid.Hives.List()[0].Brood {
		g.removeAnimal(id) // the swarm is gone: the hive breeds anew
	}
	def4Step(g, 4*clock.TicksPerDay, 100)
	hives = g.raid.Hives.List()
	if len(hives) < 2 {
		t.Fatalf("the hive spread within four days: %+v", hives)
	}
	bred := 0
	for _, h := range hives {
		bred += len(h.Brood)
	}
	if bred == 0 {
		t.Fatal("the hives bred new insects")
	}
	first := hives[0]
	g.removeBuilding(first.ID)
	def4Step(g, 120, 1)
	for _, h := range g.raid.Hives.List() {
		if h.ID == first.ID {
			t.Fatal("a destroyed hive is forgotten: no more insects from it")
		}
	}
}

// TestThreatResponse: an unarmed colonist flees a raider that is not even
// shooting at it; one set to ignore carries on; an armed one set to
// attack shoots the raider on sight (CBT-4, wiki: Threat response).
func TestThreatResponse(t *testing.T) {
	g, ada := def4Game(t, 50, 30, 30, 15)
	rid := turretRaider(t, g, 20, 15)
	r := g.raiders[rid]
	r.Mode, r.RetryAt = raid.Arriving, 1<<62 // it stands about
	g.raiders[rid] = r
	g.Step(combat.FleeCalm / 60)
	if g.calm[ada] == 0 {
		t.Fatal("unarmed, Ada flees the raider in sight")
	}
	g2, bo := def4Game(t, 50, 30, 30, 15)
	g2.combat.Drafts.SetResponse(bo, combat.RespondIgnore)
	rid2 := turretRaider(t, g2, 20, 15)
	r2 := g2.raiders[rid2]
	r2.Mode, r2.RetryAt = raid.Arriving, 1<<62
	g2.raiders[rid2] = r2
	g2.Step(20)
	if g2.calm[bo] != 0 {
		t.Fatal("set to ignore, Bo carries on")
	}
	g3, cy := def4Game(t, 50, 30, 30, 15)
	g3.equip[cy] = item.Rifle
	rid3 := turretRaider(t, g3, 20, 15)
	r3 := g3.raiders[rid3]
	r3.Mode, r3.RetryAt = raid.Arriving, 1<<62
	g3.raiders[rid3] = r3
	attacked := false
	for i := 0; i < 30 && !attacked; i++ {
		g3.Step(1)
		j, _ := g3.jobs.Get(cy)
		attacked = j.Kind == job.Attack && j.Target == rid3 || g3.combat.Tactics.Covering(cy, j)
	}
	if !attacked {
		t.Fatal("armed and set to attack, Cy goes for the raider")
	}
	if g3.combat.Drafts.SetResponse(cy, combat.RespondFlee); g3.fightsBack(cy) {
		t.Fatal("set to flee, an armed colonist runs")
	}
	if v, _ := g3.vital.Get(cy); v.Status != health.Alive {
		t.Fatal("alive")
	}
}

// TestThreatResponseDoesNotFlap: a fleeing colonist keeps fleeing (and
// stays calm) while the raider lingers, never switching back to work and
// to flight every few ticks.
func TestThreatResponseDoesNotFlap(t *testing.T) {
	g, ada := def4Game(t, 60, 30, 30, 15)
	rid := turretRaider(t, g, 20, 15)
	r := g.raiders[rid]
	r.Mode, r.RetryAt = raid.Arriving, 1<<62
	g.raiders[rid] = r
	switches, prev := 0, false
	for i := 0; i < 1200; i++ {
		def4Step(g, 1, 1)
		calm := g.calm[ada] > uint64(g.clock.Now())
		if calm != prev {
			switches, prev = switches+1, calm
		}
	}
	if switches > 2 {
		t.Fatalf("Ada's flight switched on and off %d times", switches)
	}
}
