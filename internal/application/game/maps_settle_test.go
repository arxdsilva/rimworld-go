package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/caravan"
	"rimworld-go/internal/domain/world/planet"
)

// Attacking a settlement (MAP-05, wiki: Faction base).

// besieged has ada's caravan attack a neutral settlement near home, of the
// faction typ, with enemy a second faction hostile to the colony. It
// returns the settlement map and the faction's index.
func besieged(t *testing.T, typ faction.Type) (g *Game, m *mapState, tile, fi int, ada entity.EntityID) {
	t.Helper()
	g = worldGame(t)
	tile, ada = plainNear(t, g), g.pawnIDs[0]
	fi, enemy := -1, -1
	for i := range g.ng.factions {
		f := &g.ng.factions[i]
		f.Settlements, f.SettlementNames = nil, nil
		switch {
		case fi < 0 && f.Type == typ:
			fi = i
			f.Relation, f.Goodwill, f.Settlements = faction.Neutral, 0, []int{tile}
		case enemy < 0 && f.Type == faction.Clans || enemy < 0 && f.Type == faction.Drifters:
			enemy = i
			f.Relation, f.Goodwill = faction.Hostile, -60
		}
	}
	if fi < 0 || enemy < 0 {
		t.Fatalf("no %v faction or no enemy in %d factions", typ, len(g.ng.factions))
	}
	cid := arriveAt(t, g, tile, ada)
	g.Enqueue(command.Command{Kind: command.AttackSettlement, Target: uint64(cid)})
	g.Step(1)
	if len(g.maps) != 2 {
		t.Fatalf("no settlement map opened: %d maps", len(g.maps))
	}
	m = g.maps[1]
	m.climate = biome.DefOf(biome.TemperateForest).Climate // the test planet is all sea ice: keep ada on her feet
	return g, m, tile, fi, ada
}

// settleMeta is settlement map m's row.
func settleMeta(g *Game, m *mapState) *maps.Meta { return g.worldSt.atlas.Meta(m.id) }

// defenders is map m's defenders, in spawn order.
func defenders(m *mapState) []entity.EntityID {
	var out []entity.EntityID
	for _, id := range m.raiderIDs {
		if m.raidGroups[m.raiders[id].Group].Plan == raid.Defend {
			out = append(out, id)
		}
	}
	return out
}

// nextHours steps g over the next n hour marks.
func nextHours(g *Game, n int) {
	r := int(uint64(g.clock.Now()) % clock.TicksPerHour)
	g.Step((clock.TicksPerHour-r)%clock.TicksPerHour + (n-1)*clock.TicksPerHour + 1)
}

// TestAttackSettlementOpensCompound: the attack costs 200 goodwill and
// turns the faction hostile; a settlement map opens at its tile with the
// compound and goods placed and the defenders at their posts; our people
// come in drafted at the edge; the attack is charged once.
func TestAttackSettlementOpensCompound(t *testing.T) {
	t.Parallel()
	g, m, tile, fi, ada := besieged(t, faction.Settlers)
	f := &g.ng.factions[fi]
	if f.Relation != faction.Hostile || f.Goodwill != -100 {
		t.Fatalf("relation %v goodwill %d after the attack", f.Relation, f.Goodwill)
	}
	if v := g.Maps()[1]; v.Kind != "settlement" || m.tile != tile || m.grid.Width() != maps.SiteSide {
		t.Fatalf("map view %+v on tile %d", v, m.tile)
	}
	if onMap(g, ada) != m || !g.drafted(ada) || fleetOf(g).Of(ada) != nil {
		t.Fatalf("ada did not come in drafted: on %v drafted %v", onMap(g, ada), g.drafted(ada))
	}
	if settleMeta(g, m).Faction != fi+1 {
		t.Fatalf("meta faction %d, want %d", settleMeta(g, m).Faction, fi+1)
	}
	count := map[construction.Kind]int{}
	for _, id := range m.bldgIDs {
		b, _ := m.bldgs.Get(id)
		count[b.Kind]++
		if b.Kind.Sleeps() && (!b.Forbidden || !b.Ancient) {
			t.Fatalf("bed at %d,%d: %+v", b.X, b.Y, b)
		}
	}
	if count[construction.Wall] < 40 || count[construction.Door] < 3 || count[construction.Bed] < 6 || count[construction.SentryTurret] != 2 || count[construction.MachiningTable] != 1 {
		t.Fatalf("compound %v", count)
	}
	goods := map[item.Kind]bool{}
	for _, id := range m.itemIDs {
		if st, ok := m.stacks.Get(id); ok && !st.Forbidden {
			goods[st.Kind] = true
		}
	}
	if !goods[item.Silver] || !goods[item.Components] || !goods[item.Meal] {
		t.Fatalf("store %v", goods)
	}
	ds := defenders(m)
	if len(ds) < 3 {
		t.Fatalf("%d defenders", len(ds))
	}
	for _, id := range ds {
		if m.raidGroups[m.raiders[id].Group].Over != true {
			t.Fatal("defenders are no raid: no goodwill to pay when they fall")
		}
	}
	// a second caravan joins without paying again
	g.ng.factions[fi].Goodwill = 0
	bo := g.pawnIDs[0]
	cid := arriveAt(t, g, tile, bo)
	g.Step(1)
	if fleetOf(g).At(cid) != nil || onMap(g, bo) != m {
		t.Fatal("a caravan reaching an open settlement map should enter it")
	}
	if g.ng.factions[fi].Goodwill != 0 {
		t.Fatalf("entering the open map cost goodwill: %d", g.ng.factions[fi].Goodwill)
	}
}

// TestDefendersHoldThenEngage: with nobody in sight the defenders hold
// inside the compound for hours; a colonist walking up within sight draws
// them.
func TestDefendersHoldThenEngage(t *testing.T) {
	t.Parallel()
	g, m, _, _, ada := besieged(t, faction.Settlers)
	g.Step(2 * clock.TicksPerHour)
	ds := defenders(m)
	camp := m.raidGroups[m.raiders[ds[0]].Group]
	for _, id := range ds {
		p, _ := m.positions.Get(id)
		if max(p.X-camp.CampX, camp.CampX-p.X, p.Y-camp.CampY, camp.CampY-p.Y) > 20 {
			t.Fatalf("defender %d wandered to %d,%d from %d,%d", id, p.X, p.Y, camp.CampX, camp.CampY)
		}
		if r := m.raiders[id]; r.Mode != raid.Arriving || r.Idle != 0 {
			t.Fatalf("defender %d left its post: %+v", id, r)
		}
	}
	ap, _ := m.positions.Get(ada)
	if d := max(ap.X-camp.CampX, camp.CampX-ap.X, ap.Y-camp.CampY, camp.CampY-ap.Y); d <= raid.SightRange {
		t.Fatalf("ada should have started out of sight: %d", d)
	}
	near := m.positions.Ref(ds[0])
	m.positions.Set(ada, Position{X: near.X + 3, Y: near.Y})
	engaged := false
	for i := 0; i < 30 && !engaged; i++ { // she may fall fast: watched look by look
		g.Step(1)
		engaged = slices.ContainsFunc(ds, func(id entity.EntityID) bool { return m.raiders[id].Mode == raid.Assaulting })
	}
	if !engaged {
		t.Fatal("no defender engaged a colonist in sight")
	}
}

// fell marks ids dead through the raid AI's own path.
func fell(g *Game, ids ...entity.EntityID) {
	for _, id := range ids {
		v, _ := g.vital.Get(id)
		v.Status = health.Dead
		g.vital.Set(id, v)
	}
}

// TestSettlementDestroyedThenFactionGone: all defenders dead, three hourly
// results later the settlement falls: off the faction, the faction
// defeated with its last base, +20 goodwill from the factions hostile to
// it, the letter on the map, the map a site with its pack-up running; the
// map closes with the pack-up. A faction with another base lives on.
func TestSettlementDestroyedThenFactionGone(t *testing.T) {
	t.Parallel()
	for _, last := range []bool{true, false} {
		g, m, tile, fi, _ := besieged(t, faction.Settlers)
		if !last {
			g.ng.factions[fi].Settlements = append(g.ng.factions[fi].Settlements, tile+1000) // another base, far off
		}
		enemy := slices.IndexFunc(g.ng.factions, func(f faction.Faction) bool {
			return f.Relation == faction.Hostile && f.Type != faction.Settlers && !faction.DefOf(f.Type).Fixed
		})
		was := g.ng.factions[enemy].Goodwill
		fell(g, defenders(m)...)
		nextHours(g, 2)
		if meta := settleMeta(g, m); meta.Defeated || meta.PackBy != 0 {
			t.Fatalf("defeated after two results: %+v", meta)
		}
		nextHours(g, 2)
		meta := settleMeta(g, m)
		if !meta.Defeated || meta.Kind != maps.Site || meta.PackBy == 0 {
			t.Fatalf("not destroyed: %+v", meta)
		}
		f := g.ng.factions[fi]
		if slices.Contains(f.Settlements, tile) || f.Defeated != last {
			t.Fatalf("faction after the fall: %+v", f)
		}
		if got := g.ng.factions[enemy].Goodwill; got != was+20 {
			t.Fatalf("enemy goodwill %d, want %d", got, was+20)
		}
		i := slices.IndexFunc(g.letters.list, func(l savedLetter) bool { return l.Title == "Settlement destroyed" })
		if i < 0 || uint32(g.letters.list[i].Map) != m.id {
			t.Fatalf("no letter on the map: %d", i)
		}
		if v := g.Maps()[1]; v.Kind != "site" || v.Countdown <= 0 {
			t.Fatalf("map view %+v", v)
		}
		packBy := meta.PackBy
		nextHours(g, 1)
		if meta.PackBy != packBy {
			t.Fatal("the pack-up restarted")
		}
		meta.PackBy = uint64(g.clock.Now()) + 1
		nextHours(g, 1)
		if len(g.maps) != 1 {
			t.Fatalf("the map did not close after the pack-up: %d maps", len(g.maps))
		}
	}
}

// TestDefendersFleeAtHalfLosses: half the defenders down, the rest flee,
// and fleeing ones no longer defend it: the settlement falls with them
// still on the map.
func TestDefendersFleeAtHalfLosses(t *testing.T) {
	t.Parallel()
	g, m, _, _, _ := besieged(t, faction.Settlers)
	ds := defenders(m)
	fell(g, ds[:(len(ds)+1)/2]...)
	nextHours(g, 1)
	for _, id := range ds[(len(ds)+1)/2:] {
		if v, ok := g.vital.Get(id); ok && v.Status == health.Alive && m.positions.Has(id) && m.raiders[id].Mode != raid.Fleeing {
			t.Fatalf("defender %d did not flee: %+v", id, m.raiders[id])
		}
	}
	nextHours(g, 3)
	if !settleMeta(g, m).Defeated {
		t.Fatal("fled defenders should not hold the settlement")
	}
	for _, id := range ds[(len(ds)+1)/2:] { // their own gates let them out: off the map by now
		if v, ok := g.vital.Get(id); ok && v.Status == health.Alive && m.positions.Has(id) {
			p, _ := m.positions.Get(id)
			t.Errorf("defender %d is still walking at %d,%d", id, p.X, p.Y)
		}
	}
}

// TestDefeatNoFlap: two results with no defender, then one back on its
// feet, resets the count; only three in a row destroy it.
func TestDefeatNoFlap(t *testing.T) {
	t.Parallel()
	g, m, _, fi, _ := besieged(t, faction.Settlers)
	ds := defenders(m)
	nextHours(g, 1)
	fell(g, ds...)
	nextHours(g, 2)
	if meta := settleMeta(g, m); meta.Routed != 2 || meta.Defeated {
		t.Fatalf("two results: %+v", meta)
	}
	for _, id := range ds { // they stand up again (one alone would count its group as broken and flee)
		v, _ := g.vital.Get(id)
		v.Status = health.Alive
		g.vital.Set(id, v)
		r := m.raiders[id]
		r.Dead, r.Mode = 0, raid.Arriving
		m.raiders[id] = r
	}
	nextHours(g, 1)
	if meta := settleMeta(g, m); meta.Routed != 0 || meta.Defeated {
		t.Fatalf("a defender stood up: %+v", meta)
	}
	fell(g, ds...)
	nextHours(g, 2)
	if meta := settleMeta(g, m); meta.Defeated || g.ng.factions[fi].Defeated {
		t.Fatalf("the count should have restarted: %+v", meta)
	}
	nextHours(g, 1)
	if !settleMeta(g, m).Defeated {
		t.Fatal("three results in a row should destroy it")
	}
}

// TestAttackReasons: refusals are readable and cost nothing.
func TestAttackReasons(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	if why := g.AttackReason(1 << 40); why != "No such caravan." {
		t.Fatalf("no caravan: %q", why)
	}
	tile, ada := plainNear(t, g), g.pawnIDs[0]
	cid := arriveAt(t, g, tile, ada)
	if why := g.AttackReason(uint64(cid)); why != "There is no settlement here to attack." {
		t.Fatalf("plain tile: %q", why)
	}
	g.Enqueue(command.Command{Kind: command.AttackSettlement, Target: uint64(cid)})
	g.Step(1)
	if len(g.maps) != 1 {
		t.Fatal("a refused attack opened a map")
	}
	friendlySettlement(t, g, tile)
	if why := g.AttackReason(uint64(cid)); why != "" {
		t.Fatalf("waiting at a settlement: %q", why)
	}
	fleetOf(g).At(cid).C.State = caravan.Travelling
	if why := g.AttackReason(uint64(cid)); why != "Only a caravan waiting at a settlement can attack it." {
		t.Fatalf("on the road: %q", why)
	}
}

// TestPodsAttackCentre: a pod group launched to attack the centre is
// refused nowhere near a settlement; over one it opens the map, charges
// the attack and sets its people down in the compound's middle.
func TestPodsAttackCentre(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	tile, ada := plainNear(t, g), g.pawnIDs[0]
	x, y := ax9Free(t, g)
	pod, _ := ax9PodRig(t, g, x+1, y, 150)
	g.Enqueue(command.Command{Kind: command.LoadPods, Target: uint64(pod), Enabled: true, Name: entityStr(ada)})
	for i := 0; i < 8000 && !g.isStashed(ada); i++ {
		g.Step(1)
	}
	if !g.isStashed(ada) {
		t.Fatal("ada never boarded")
	}
	if why, _ := travelOf(g).LaunchReason(pod, tile, command.PodArriveAttackCentre); why != "There is no settlement there to attack." {
		t.Fatalf("no settlement: %q", why)
	}
	friendlySettlement(t, g, tile)
	if why, _ := travelOf(g).LaunchReason(pod, tile, command.PodArriveAttackCentre); why != "" {
		t.Fatalf("over a settlement: %q", why)
	}
	g.Enqueue(command.Command{Kind: command.LaunchPods, Target: uint64(pod), Count: tile, Mode: command.PodArriveAttackCentre})
	g.Step(2*caravan.PodTicksPerTile + 260)
	if len(g.maps) != 2 {
		t.Fatalf("the pods opened no map: %d maps, pods %+v", len(g.maps), g.World().AX.Pods)
	}
	fi := g.settlementFaction(tile) - 1
	if f := g.ng.factions[fi]; f.Relation != faction.Hostile {
		t.Fatalf("faction after the pods' attack: %+v", f)
	}
	m := onMap(g, ada)
	if m == nil || m == g.maps[0] {
		t.Fatal("ada is not on the settlement map")
	}
	p, _ := m.positions.Get(ada)
	if d := max(p.X-maps.SiteSide/2, maps.SiteSide/2-p.X, p.Y-maps.SiteSide/2, maps.SiteSide/2-p.Y); d > 30 {
		t.Fatalf("ada landed %d cells from the centre", d)
	}
}

// attackRun is one attack with hours of holding: the state hash, with a
// save and load mid-attack when reload is set.
func attackRun(t *testing.T, reload bool) uint64 {
	g, m, _, _, _ := besieged(t, faction.Settlers)
	g.Step(clock.TicksPerHour)
	g.mapState = m
	g.dieBy(defenders(m)[0], health.CauseBloodLoss, 0) // the real death: its corpse, as a load expects
	g.mapState = g.view
	g.Step(clock.TicksPerHour / 2)
	if reload {
		h := saveLoad(t, g)
		if h.StateHash() != g.StateHash() {
			t.Fatal("a save mid-attack changed the state hash")
		}
		h.worldS().planet, h.worldS().planetTried = testPlanet(), true
		g = h
	}
	g.Step(clock.TicksPerHour)
	return g.StateHash()
}

// TestAttackSettlementSameSeedAndSaveHash: two runs of an attack hash the
// same, and so does a run saved and loaded halfway.
func TestAttackSettlementSameSeedAndSaveHash(t *testing.T) {
	t.Parallel()
	a := attackRun(t, false)
	if b := attackRun(t, false); a != b {
		t.Fatalf("same seed, hash %x and %x", a, b)
	}
	if c := attackRun(t, true); a != c {
		t.Fatalf("a save mid-attack moved the hash: %x and %x", a, c)
	}
}

// TestDestroyedSettlementLeavesPlanet: a real settlement falls off the
// game's own planet, and a planet rebuilt from the same world after a load
// stays without it while keeping the others.
func TestDestroyedSettlementLeavesPlanet(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	p := planet.Build(worldTestParams) // this game's own: the shared test planet stays read-only
	g.worldS().planet = p
	tile, other := -1, -1
	for _, st := range p.SettlementTiles() {
		switch {
		case g.settlementFaction(int(st)) == 0:
		case tile < 0:
			tile = int(st)
		case other < 0:
			other = int(st)
		}
	}
	if tile < 0 || other < 0 {
		t.Fatal("the test planet has too few settlements")
	}
	ada := g.pawnIDs[0]
	cid := arriveAt(t, g, tile, ada)
	g.Enqueue(command.Command{Kind: command.AttackSettlement, Target: uint64(cid)})
	g.Step(1)
	if len(g.maps) != 2 {
		t.Fatalf("no settlement map: %d maps", len(g.maps))
	}
	m := g.maps[1]
	fell(g, defenders(m)...)
	nextHours(g, 4)
	if !settleMeta(g, m).Defeated {
		t.Fatal("not destroyed")
	}
	if p.Settlement[tile] >= 0 || p.SettlementName(tile) != "" || g.settlementFaction(tile) != 0 {
		t.Fatalf("the settlement is still on the planet: %d %q", p.Settlement[tile], p.SettlementName(tile))
	}
	rebuilt := planet.Build(worldTestParams)
	maps.Prune(rebuilt, g.ng.factions)
	if rebuilt.Settlement[tile] >= 0 || rebuilt.Settlement[other] < 0 {
		t.Fatalf("after a rebuild: fallen %d, other %d", rebuilt.Settlement[tile], rebuilt.Settlement[other])
	}
}
