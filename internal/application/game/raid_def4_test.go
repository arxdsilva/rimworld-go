package game

import (
	"bytes"
	"strings"
	"testing"

	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	raiddef "rimworld-go/internal/domain/raid"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/roof"
)

// DEF-4 whole-game tests: drop pods, staged raids, sieges with mortars,
// thieves, burners, rival factions, retreat, drafted control, grenades,
// IEDs and the combat log.

func def4Game(t *testing.T, w, h, ax, ay int) (*Game, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 41, Width: w, Height: h})
	ada, err := g.SpawnNamedPawn(ax, ay, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	g.SetNeeds(ada, needs.Full, needs.Full)
	return g, ada
}

// stepFed steps n ticks in chunks, keeping the colonists fed and rested.
func def4Step(g *Game, n, chunk int) {
	for done := 0; done < n; done += chunk {
		g.Step(chunk)
		for _, id := range g.pawnIDs {
			g.SetNeeds(id, needs.Full, needs.Full)
		}
	}
}

func logHas(g *Game, sub string) int {
	n := 0
	for _, e := range g.World().CombatLog {
		if strings.Contains(e.Text, sub) {
			n++
		}
	}
	return n
}

func kill(g *Game, id entity.EntityID) {
	v, _ := g.vital.Get(id)
	v.Status = health.Dead
	g.vital.Set(id, v)
}

func TestDropPodRaidLandsAndLeavesShells(t *testing.T) {
	g, _ := def4Game(t, 40, 30, 20, 15)
	g.RaidDropPods(item.Club, item.Club)
	if len(g.raiderIDs) != 0 {
		t.Fatal("nobody steps out before the pods land")
	}
	falling := 0
	for _, a := range g.World().Arrivals {
		if a.Hostile && !a.Landed {
			falling++
		}
	}
	msgs := g.World().Messages
	if falling != 2 || !strings.HasPrefix(msgs[len(msgs)-1].Text, "Drop pods!") || logHas(g, "Drop pods!") != 1 {
		t.Fatalf("two hostile pods falling (%d), announced and logged: %+v", falling, msgs[len(msgs)-1])
	}
	def4Step(g, raiddef.PodFallTicks+raiddef.PodStagger+1, 1)
	if s, n := g.Raiders(); n != 2 || s != 2 {
		t.Fatalf("two raiders out of their pods, got %d/%d", s, n)
	}
	shells, landed := 0, 0
	for _, b := range g.World().Buildings {
		if b.Kind == construction.DefOf(construction.PodShell).Name {
			shells++
		}
	}
	for _, a := range g.World().Arrivals {
		if a.Hostile && a.Landed {
			landed++
		}
	}
	if shells != 2 || landed != 2 || logHas(g, "landing") != 1 {
		t.Fatalf("the shells stay (%d) and the pods show landed (%d)", shells, landed)
	}
	if g.raidGroups[0].Size != 2 || g.raidGroups[0].Arrival != raiddef.DropPodCentre {
		t.Fatalf("group %+v", g.raidGroups[0])
	}
	def4Step(g, clock.TicksPerHour+1, 100)
	if g.raidGroups[0].Pods != nil {
		t.Fatal("landed pods are forgotten after the hour")
	}
}

func TestPodBreaksRoofAndHurtsBeneath(t *testing.T) {
	g, _ := def4Game(t, 30, 20, 5, 5)
	bo, _ := g.SpawnNamedPawn(13, 10, "Bo")
	g.ensureRoofGrid()
	g.setRoof(10*g.grid.Width()+12, roof.Built)
	g.raidGroups = append(g.raidGroups, raid.Group{Size: 1})
	id := raidArrive{g}.Land(0, raid.Pod{X: 12, Y: 10, Kit: item.Club})
	if id == 0 || !g.isRaider(id) {
		t.Fatal("the raider steps out")
	}
	if g.roofAt(12, 10).Roofed() {
		t.Fatal("the pod broke the roof")
	}
	if v, _ := g.vital.Get(bo); v.Injury != raiddef.RoofCrush {
		t.Fatalf("the falling roof hurts whoever is beside it: injury %d", v.Injury)
	}
	if !(raidArrive{g}).PodSpot(20, 10) || (raidArrive{g}).PodSpot(-1, 0) {
		t.Fatal("pod spots")
	}
}

// TestStagedRaidStormsOnceNoFlap: a staged raid gathers at its spot,
// holds, then storms exactly once; gathered raiders don't shuffle.
func TestStagedRaidStormsOnceNoFlap(t *testing.T) {
	g, _ := def4Game(t, 80, 80, 40, 40)
	g.raidWith(g.kitOf(3), raid.Stage)
	if len(g.raiderIDs) == 0 {
		t.Fatal("raiders")
	}
	storms, prev := 0, false
	walks := map[entity.EntityID]int{}
	moving := map[entity.EntityID]bool{}
	for i := 0; i < 16*clock.TicksPerHour/30 && !g.raidGroups[0].Stormed; i++ {
		def4Step(g, 30, 30)
		grp := g.raidGroups[0]
		if grp.Stormed != prev {
			storms, prev = storms+1, grp.Stormed
		}
		for _, id := range g.raiderIDs {
			j, _ := g.jobs.Get(id)
			m := j.Kind == job.Move && j.Active()
			if m && !moving[id] {
				walks[id]++
			}
			moving[id] = m
		}
	}
	grp := g.raidGroups[0]
	if !grp.Stormed || storms != 1 || !grp.Camped {
		t.Fatalf("the staged raid storms once: %+v (%d)", grp, storms)
	}
	for id, n := range walks {
		if n > 3 {
			t.Fatalf("raider %d walked off %d times while staging", id, n)
		}
	}
	def4Step(g, 600, 30)
	if !g.raidGroups[0].Stormed {
		t.Fatal("the storm is one way")
	}
}

// TestRetreatAtFleeShareNoFlap: the group flees at its share lost, logs
// it once, and no raider turns back.
func TestRetreatAtFleeShareNoFlap(t *testing.T) {
	g, _ := def4Game(t, 40, 20, 20, 10)
	g.Raid(4)
	g.raidGroups[0].FleeLoss = 50
	ids := append([]entity.EntityID(nil), g.raiderIDs...)
	kill(g, ids[0])
	g.Step(1)
	for _, id := range ids[1:] {
		if g.raiders[id].Mode == raid.Fleeing {
			t.Fatal("one of four lost is under the share")
		}
	}
	kill(g, ids[1])
	g.Step(1)
	for _, id := range ids[2:] {
		if g.raiders[id].Mode != raid.Fleeing {
			t.Fatal("half lost: the rest flee")
		}
	}
	for i := 0; i < 300; i++ {
		g.Step(1)
		for _, id := range ids[2:] {
			if r, ok := g.raiders[id]; ok && r.Mode != raid.Fleeing {
				t.Fatalf("raider %d turned back at tick %d", id, i)
			}
		}
	}
	if n := logHas(g, "retreating"); n != 1 || logHas(g, "was killed") != 2 {
		t.Fatalf("the retreat is logged once (%d) and the deaths too", n)
	}
}

func drafted(t *testing.T, g *Game, id entity.EntityID, k item.Kind) {
	t.Helper()
	g.equip[id] = k
	g.draft[id] = true
}

func send(g *Game, c command.Command) {
	g.Enqueue(c)
	g.ApplyPending()
}

// TestFireAtWillNoFlap: on, a drafted rifleman keeps its target (switching
// at most when one falls); off, it never starts shooting on its own; the
// switch shows on the read model.
func TestFireAtWillNoFlap(t *testing.T) {
	g, ada := def4Game(t, 40, 20, 20, 10)
	drafted(t, g, ada, item.Rifle)
	g.Raid(2)
	for i, id := range g.raiderIDs {
		g.positions.Set(id, Position{X: 27 + i, Y: 10})
	}
	switches, last := 0, entity.EntityID(0)
	for i := 0; i < 120; i++ {
		def4Step(g, 1, 1)
		if j, _ := g.jobs.Get(ada); j.Kind == job.Attack && j.Target != last {
			switches, last = switches+1, j.Target
		}
	}
	if switches == 0 || switches > 2 {
		t.Fatalf("fire at will: %d target switches", switches)
	}
	send(g, command.Command{Kind: command.SetFireAtWill, Pawn: uint64(ada), Enabled: false})
	if def4View(g, ada).FireAtWill {
		t.Fatal("PawnView.FireAtWill off")
	}
	for i := 0; i < 300; i++ {
		def4Step(g, 1, 1)
		if j, _ := g.jobs.Get(ada); j.Kind == job.Attack && !j.Forced && j.Active() {
			t.Fatalf("tick %d: shooting with fire at will off", i)
		}
	}
	send(g, command.Command{Kind: command.SetFireAtWill, Pawn: uint64(ada), Enabled: true})
	if !def4View(g, ada).FireAtWill {
		t.Fatal("PawnView.FireAtWill back on")
	}
}

func def4View(g *Game, id entity.EntityID) PawnView {
	for _, p := range g.Snapshot() {
		if p.ID == id {
			return p
		}
	}
	return PawnView{}
}

// TestHoldPositionNeverChases: holding, a drafted colonist shoots a
// fleeing raider in reach and never follows it.
func TestHoldPositionNeverChases(t *testing.T) {
	g, ada := def4Game(t, 60, 20, 30, 10)
	drafted(t, g, ada, item.Club)
	send(g, command.Command{Kind: command.SetHoldPosition, Pawn: uint64(ada), Enabled: true})
	if !def4View(g, ada).HoldPosition {
		t.Fatal("PawnView.HoldPosition on")
	}
	g.Raid(1)
	rid := g.raiderIDs[0]
	g.positions.Set(rid, Position{X: 31, Y: 10})
	r := g.raiders[rid]
	r.Mode = raid.Fleeing
	g.raiders[rid] = r
	start, _ := g.positions.Get(ada)
	for i := 0; i < 400; i++ {
		def4Step(g, 1, 1)
		if p, _ := g.positions.Get(ada); p != start {
			t.Fatalf("tick %d: the colonist left its post for %v", i, p)
		}
	}
	send(g, command.Command{Kind: command.SetHoldPosition, Pawn: uint64(ada), Enabled: false})
	if g.combat.Drafts.Holding(ada) {
		t.Fatal("hold off")
	}
}

// TestGrenadeBelt: equipping takes the whole stack, a throw bursts at the
// cell and spends one, the last leaves the colonist unarmed, and too far
// walks in first (CBT-2; the next order replaces the walk).
func TestGrenadeBelt(t *testing.T) {
	g, ada := def4Game(t, 40, 20, 10, 10)
	sid, _ := g.SpawnStack(item.FragGrenades, 2, 11, 10)
	g.doEquip(ada, job.Record{Kind: job.Equip, Target: sid})
	if n := def4View(g, ada).Grenades; n != 2 {
		t.Fatalf("the belt holds the stack: %d", n)
	}
	if _, ok := g.stacks.Get(sid); ok {
		t.Fatal("the stack was taken whole")
	}
	g.draft[ada] = true
	if !g.combat.Drafts.Throw(ada, 30, 10) { // CBT-2: it walks into range first
		t.Fatal("a throw too far walks in")
	}
	send(g, command.Command{Kind: command.ThrowGrenade, Pawn: uint64(ada), X: 16, Y: 10})
	burst := false
	for i := 0; i < 200 && !burst; i++ {
		def4Step(g, 1, 1)
		for _, b := range g.World().Bursts {
			burst = burst || b.X >= 15 && b.X <= 17
		}
	}
	if !burst || def4View(g, ada).Grenades != 1 {
		t.Fatalf("burst %v, grenades left %d", burst, def4View(g, ada).Grenades)
	}
	sid2, _ := g.SpawnStack(item.Rifle, 1, 10, 11)
	g.doEquip(ada, job.Record{Kind: job.Equip, Target: sid2})
	if g.equip[ada] != item.Rifle || def4View(g, ada).Grenades != 0 {
		t.Fatal("a rifle replaces the belt")
	}
	found := false
	for _, id := range g.itemIDs {
		if st, _ := g.stacks.Get(id); st.Kind == item.FragGrenades && st.Qty == 1 {
			found = true
		}
	}
	if !found {
		t.Fatal("the last grenade is put down with the belt")
	}
	g.equip[ada] = item.FragGrenades // one wielded from before belts
	g.combat.Drafts.Throw(ada, 14, 10)
	def4Step(g, 120, 1)
	if _, armed := g.equip[ada]; armed {
		t.Fatal("the last grenade thrown leaves it unarmed")
	}
}

// TestSiegeMortarsAndStorm: a siege builds two mortars its raiders crew
// and fire at the colony; a besieger hit may storm at once (8 %).
func TestSiegeMortarsAndStorm(t *testing.T) {
	g, _ := def4Game(t, 90, 90, 45, 45)
	g.Siege(3)
	for i := 0; i < 400 && !g.raidGroups[0].Camped; i++ {
		def4Step(g, 25, 25)
	}
	grp := g.raidGroups[0]
	if !grp.Camped || len(grp.Mortars) != raiddef.SiegeMortars {
		t.Fatalf("camped %v, mortars %v", grp.Camped, grp.Mortars)
	}
	for _, m := range grp.Mortars {
		if !g.combat.Mortars.State(m).Hostile {
			t.Fatal("the raiders' mortars")
		}
	}
	if grp.Until < grp.Started+raiddef.SiegeAssaultMin*clock.TicksPerHour {
		t.Fatal("the storm waits 1.5 days at least")
	}
	fired := false
	for i := 0; i < 3000 && !fired; i++ {
		def4Step(g, 1, 1)
		for _, s := range g.World().Shots {
			fired = fired || g.isRaider(s.Owner) && strings.Contains(s.Weapon, "shell")
		}
		for _, m := range grp.Mortars {
			fired = fired || g.combat.Mortars.State(m).Fired > 0
		}
	}
	if !fired {
		t.Fatal("the crews fire their mortars")
	}
	crews := 0
	for _, id := range g.raiderIDs {
		if j, _ := g.jobs.Get(id); j.Kind == job.ManMortar {
			crews++
		}
	}
	if crews == 0 {
		t.Fatal("raiders crew the mortars")
	}
	hits := 0
	for ; hits < 400 && !g.raidGroups[0].Stormed; hits++ {
		for _, id := range g.raiderIDs {
			if g.standingRaider(id) {
				v, _ := g.vital.Get(id)
				v.Injury++
				g.vital.Set(id, v)
				break
			}
		}
		g.Step(1)
	}
	if !g.raidGroups[0].Stormed {
		t.Fatal("hits set the storm off")
	}
	for _, id := range g.raiderIDs {
		if j, _ := g.jobs.Get(id); j.Kind == job.ManMortar && j.Active() {
			t.Fatal("the crews leave their guns to storm")
		}
	}
}

func TestStealRaidTakesLootAndDropsIt(t *testing.T) {
	g, _ := def4Game(t, 40, 20, 35, 10)
	sid, _ := g.SpawnStack(item.Rifle, 1, 20, 10)
	g.raidWith([]item.Kind{item.Club}, raid.Steal)
	rid := g.raiderIDs[0]
	for i := 0; i < 2000 && g.raiders[rid].LootQty == 0; i++ {
		def4Step(g, 1, 1)
	}
	r := g.raiders[rid]
	if r.Loot != item.Rifle || r.Mode != raid.Fleeing {
		t.Fatalf("the thief runs with the rifle: %+v", r)
	}
	if _, ok := g.stacks.Get(sid); ok {
		t.Fatal("the stack is gone")
	}
	kill(g, rid)
	g.Step(1)
	found := false
	for _, id := range g.itemIDs {
		if st, _ := g.stacks.Get(id); st.Kind == item.Rifle {
			found = true
		}
	}
	if !found || logHas(g, "stole") != 1 {
		t.Fatal("the haul is dropped where the thief fell, and the theft logged")
	}
}

func TestBurnRaidSetsFires(t *testing.T) {
	g, _ := def4Game(t, 40, 20, 35, 10)
	if !g.BuildNow("wall", "wood", "south", 20, 5) {
		t.Fatal("wall")
	}
	g.raidWith([]item.Kind{item.Club}, raid.Burn)
	lit := false
	for i := 0; i < 3000 && !lit; i++ {
		def4Step(g, 1, 1)
		lit = g.fireAt(20, 5) > 0
	}
	if !lit || logHas(g, "on fire") != 1 {
		t.Fatal("the burner sets the wooden wall alight")
	}
}

func TestRivalFactionsFight(t *testing.T) {
	g, _ := def4Game(t, 50, 30, 45, 25)
	g.ng.factions = []faction.Faction{{Type: faction.Drifters, Name: "Ash", Relation: faction.Hostile},
		{Type: faction.Clans, Name: "Fern", Relation: faction.Hostile}}
	g.raidFrom([]item.Kind{item.Club}, raid.Assault, 1)
	g.raidFrom([]item.Kind{item.Club}, raid.Assault, 2)
	if len(g.raiderIDs) != 2 {
		t.Fatalf("raiders %d", len(g.raiderIDs))
	}
	a, b := g.raiderIDs[0], g.raiderIDs[1]
	g.positions.Set(a, Position{X: 5, Y: 5})
	g.positions.Set(b, Position{X: 8, Y: 5})
	g.Step(1)
	if j, _ := g.jobs.Get(a); j.Kind != job.Attack || j.Target != b {
		t.Fatalf("raiders of rival factions fight: %+v", j)
	}
	if (raidMischief{g}).Rival(1, 1) || !(raidMischief{g}).Rival(1, 2) {
		t.Fatal("rivalry")
	}
}

func TestIEDChainsAndBulletsAndSappersKnow(t *testing.T) {
	g, _ := def4Game(t, 40, 20, 35, 18)
	for _, x := range []int{10, 12} {
		if !g.BuildNow("blast charge", "steel", "south", x, 5) {
			t.Fatal("ied")
		}
	}
	first := g.layerBldgAt(10, 5, construction.LayerObject)
	second := g.layerBldgAt(12, 5, construction.LayerObject)
	g.combat.Traps.Struck(10, 5) // a bullet lands on it
	if !g.combat.Traps.Burning(first) {
		t.Fatal("a bullet lights the charge")
	}
	g.Step(20)
	if _, ok := g.bldgs.Get(first); ok {
		t.Fatal("the first charge went off")
	}
	if !g.combat.Traps.Burning(second) {
		if _, ok := g.bldgs.Get(second); ok {
			t.Fatal("the blast lights the charge in its radius")
		}
	}
	// sappers know about 30 % of the charges, the same ones every time
	g2, _ := def4Game(t, 60, 20, 55, 18)
	for x := 2; x < 52; x += 2 {
		g2.BuildNow("blast charge", "steel", "south", x, 3)
	}
	known := 0
	set := sapperTraps(g2)
	for x := 2; x < 52; x += 2 {
		if set.has(3*g2.grid.Width() + x) {
			known++
		}
	}
	if known < 2 || known > 15 {
		t.Fatalf("sappers know %d of 25 charges", known)
	}
}

func TestCombatLogBoundedAndSaved(t *testing.T) {
	g, ada := def4Game(t, 30, 20, 15, 10)
	for i := 0; i < 100; i++ {
		g.combat.Log.Add("line")
	}
	g.combat.Log.Add("last")
	l := g.World().CombatLog
	if len(l) != 64 || l[len(l)-1].Text != "last" {
		t.Fatalf("bounded, newest last: %d", len(l))
	}
	g.combat.Drafts.SetFireAtWill(ada, false)
	g.combat.Drafts.SetHold(ada, true)
	g.RaidDropPods(item.Club)
	g.raidGroups[0].FleeLoss = 61
	var buf bytes.Buffer
	if err := g.Save(&buf, "t"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.World().CombatLog) != 64 || h.combat.Drafts.FireAtWill(ada) || !h.combat.Drafts.Holding(ada) {
		t.Fatal("log and drafted switches survive a save")
	}
	if grp := h.raidGroups[0]; len(grp.Pods) != 1 || grp.FleeLoss != 61 || grp.Arrival != raiddef.DropPodCentre {
		t.Fatalf("pods survive a save: %+v", grp)
	}
}
