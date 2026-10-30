package ebiten

// journey_wave_e_test.go: the wave E journeys through the real UI loop:
// PRD-4 stockpile filters, shelves, the fine meal; DEF-4 drop-pod raids,
// fire at will, hold position, grenades and mechanoids under EMP. The new
// pawn buttons have no hotkey (inspector/cmds_combat.go), so no input
// action is added for them.

import (
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/command"
)

var _ = owe("journey_wave_e_test.go", []string{}, []string{})

// weForbidWood opens the selected stockpile's settings and switches the
// wood category off; it returns the command the click sent.
func weForbidWood(j *journey, id uint16) command.Command {
	j.t.Helper()
	if wbFind(j, "storage.filter.") == "" {
		j.clickID(weButton(j, "stockpile settings button", "cmd.zone.storage"))
	}
	if wbFind(j, "storage.filter.", "wood") == "" { // Wood sits under Materials
		j.clickID(weButton(j, "materials expander", "storage.expand.", "materials"))
	}
	j.clickID(weButton(j, "wood filter row", "storage.filter.", "wood"))
	c, ok := j.lastCmd(command.SetStockpileFilter)
	if !ok || c.Target != uint64(id) || c.Enabled || !strings.Contains(strings.ToLower(c.Name), "wood") {
		j.t.Fatalf("SetStockpileFilter %+v (sent %v), want zone %d, a wood node, Enabled false", c, ok, id)
	}
	return c
}

// TestJourneyStockpileFilterEdit (PRD-4): the player opens a stockpile's
// settings and forbids wood; ZoneView.Filter loses it and haulers bring
// stone but not wood. The settings paste onto a second stockpile.
func TestJourneyStockpileFilterEdit(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	sp := j.layZone("zones", "stockpile", 8, 1, 9, 2)
	before := weStrings(j, sp, "Filter")
	if !weHas(before, "wood") {
		t.Fatalf("a new stockpile's Filter %q does not allow wood", before)
	}
	weForbidWood(j, sp.ID)
	j.update(2)
	z1 := mustZone(t, j, 8, 1)
	if after := weStrings(j, z1, "Filter"); weHas(after, "wood") || len(after) == 0 {
		t.Fatalf("ZoneView.Filter after forbidding wood: %q (was %q)", after, before)
	}

	// copy the settings, lay a second stockpile and paste onto it
	j.clickID(weButton(j, "copy button", "copy"))
	j.key("Escape", 0) // close the Storage window before the Architect
	j.update(1)
	sp2 := j.layZone("zones", "stockpile", 12, 4, 13, 5)
	if !weHas(weStrings(j, mustZone(t, j, 12, 4), "Filter"), "wood") {
		t.Fatal("the second stockpile does not start allowing wood")
	}
	if wbFind(j, "storage.paste") == "" { // open the second stockpile's settings
		j.clickID(weButton(j, "stockpile settings button", "cmd.zone.storage"))
	}
	j.clickID(weButton(j, "paste button", "paste"))
	c, ok := j.lastCmd(command.PasteStockpile)
	if !ok || c.Target != uint64(sp2.ID) || uint64(c.Zone) != uint64(sp.ID) {
		t.Fatalf("PasteStockpile %+v (sent %v), want zone %d onto %d", c, ok, sp.ID, sp2.ID)
	}
	j.update(2)
	if f := weStrings(j, mustZone(t, j, 12, 4), "Filter"); weHas(f, "wood") {
		t.Fatalf("the pasted stockpile still allows wood: %q", f)
	}

	// a special filter row toggles through SetStockpileSpecial
	j.clickID(weButton(j, "special filter row", "special"))
	if c, ok := j.lastCmd(command.SetStockpileSpecial); !ok || c.Target != uint64(sp2.ID) {
		t.Fatalf("SetStockpileSpecial %+v (sent %v), want zone %d", c, ok, sp2.ID)
	}
	j.update(2)
	if len(weStrings(j, mustZone(t, j, 12, 4), "Special")) == 0 && !weHas(weStrings(j, mustZone(t, j, 12, 4), "Special"), "") {
		t.Log("ZoneView.Special is empty after the toggle (the row may have switched a default off)")
	}

	// haulers: stone goes into a stockpile, wood stays where it lies
	wood, err := j.g.SpawnStack(item.Wood, 20, 15, 7)
	if err != nil {
		t.Fatal(err)
	}
	stone, err := j.g.SpawnStack(item.Stone, 20, 15, 8)
	if err != nil {
		t.Fatal(err)
	}
	inZone := func(x, y int) bool {
		for _, z := range []int{8, 12} {
			zv := mustZone(t, j, z, map[int]int{8: 1, 12: 4}[z])
			for _, c := range zv.Cells {
				if c[0] == x && c[1] == y {
					return true
				}
			}
		}
		return false
	}
	stored, woodIn := false, false
	j.key("Digit3", 0)
	for i := 0; i < 5000 && !stored; i++ {
		careFeed(j)
		j.update(1)
		for _, it := range j.world().Items {
			if it.Carried {
				continue
			}
			if it.ID == stone && inZone(it.X, it.Y) {
				stored = true
			}
			if it.ID == wood && inZone(it.X, it.Y) {
				woodIn = true
			}
		}
	}
	j.key("Space", 0)
	if !stored {
		t.Fatal("nobody hauled the stone into a stockpile in 5000 updates (the control for the wood check)")
	}
	if woodIn {
		t.Fatal("wood reached a stockpile that forbids it")
	}
}

// TestJourneyShelfStoresStacks (PRD-4): a shelf (2x1 at (10,6)-(11,6))
// holds more than one stack in one of its cells.
func TestJourneyShelfStoresStacks(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	wdBuild(j, "shelf", 10, 6)
	for i, k := range []item.Kind{item.Wood, item.Stone, item.Berries} {
		if _, err := j.g.SpawnStack(k, 20, 15, 5+i); err != nil {
			t.Fatal(err)
		}
	}
	j.key("Digit3", 0)
	most := func() int { return max(weItemsAt(j, 10, 6), weItemsAt(j, 11, 6)) }
	for i := 0; i < 8000 && most() < 2; i++ {
		careFeedHostileFree(j)
		j.update(1)
	}
	j.key("Space", 0)
	if n := most(); n < 2 {
		t.Fatalf("the fullest shelf cell holds %d stacks, want at least 2; items %+v", n, j.world().Items)
	}
}

// TestJourneyCraftFineMeal (PRD-4): a cook makes a fine meal at a stove.
func TestJourneyCraftFineMeal(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	wdBuild(j, "stove", 10, 5)
	var def recipe.Def
	found := false
	for _, d := range recipe.For(construction.Stove) {
		if d.Output == item.FineMeal {
			def, found = d, true
		}
	}
	if !found {
		t.Fatal("no stove recipe yields a fine meal (PRD-4)")
	}
	j.g.Enqueue(command.Command{Kind: command.AddBill, X: 10, Y: 5, Recipe: uint8(def.ID), Mode: 0, Count: 1})
	j.g.ApplyPending()
	for i, k := range def.InputKinds() {
		if _, err := j.g.SpawnStack(k, 40, 13, 6+i); err != nil {
			t.Fatal(err)
		}
	}
	made := false
	j.key("Digit3", 0)
	for i := 0; i < 15000 && !made; i++ {
		careFeed(j)
		j.update(1)
		for _, it := range j.world().Items {
			if it.Kind == item.FineMeal.String() {
				made = true
			}
		}
	}
	j.key("Space", 0)
	if !made {
		t.Fatalf("the %q bill produced no fine meal", def.Label)
	}
}

// weRaidOver reports that no raider is still a threat: dead, downed or
// retreating.
func weRaidOver(j *journey) bool {
	for _, p := range weRaiders(j) {
		if p.Status == "alive" && !p.Fleeing {
			return false
		}
	}
	return true
}

// TestJourneyDropPodRaidDefended (DEF-4): pods announce a hostile arrival,
// land, raiders step out, drafted colonists fight and the raid ends.
func TestJourneyDropPodRaidDefended(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{9, 4}, [2]int{10, 4}))
	for i, id := range ordColonists(j) {
		j.g.Arm(weID(id), item.Rifle)
		weDraft(j, i)
	}
	weDropPodRaid(j, item.Club, item.Club, item.Club)
	j.update(1) // the raid alert pauses the game; unpause after it
	sawArrival, sawLanded, sawRaiders, hurt := false, false, false, false
	j.key("Digit3", 0)
	for i := 0; i < 12000; i++ {
		careFeed(j)
		j.update(1)
		landed, any := weHostileArrival(j)
		sawArrival, sawLanded = sawArrival || any, sawLanded || landed
		rs := weRaiders(j)
		sawRaiders = sawRaiders || len(rs) > 0
		for _, p := range rs {
			hurt = hurt || p.Health < 100
		}
		if sawRaiders && sawLanded && weRaidOver(j) {
			break
		}
	}
	j.key("Space", 0)
	switch {
	case !sawArrival:
		t.Fatalf("no hostile ArrivalView on WorldView.Arrivals: %+v", j.world().Arrivals)
	case !sawLanded:
		t.Fatal("the hostile pods never landed")
	case !sawRaiders:
		t.Fatal("no raider came out of the pods")
	case !hurt:
		t.Fatal("the drafted colonists never hurt a raider")
	case !weRaidOver(j):
		t.Fatalf("the raid never ended: %+v", weRaiders(j))
	}
}

// weFireSetup is a rifle colonist, drafted and selected, and a lone club
// raider on the map; the game is paused.
func weFireSetup(t *testing.T) (*journey, uint64) {
	t.Helper()
	j := newJourney(t, jPawns([2]int{8, 4}))
	id := ordColonists(j)[0]
	j.g.Arm(weID(id), item.Rifle)
	weDraft(j, 0)
	j.g.RaidArmed(item.Club)
	j.update(1) // the raid alert pauses the game
	return j, id
}

// weDist is the Chebyshev distance between colonist id and the nearest raider.
func weDist(j *journey, id uint64) int {
	p := j.pawn(id)
	best := 1 << 20
	for _, r := range weRaiders(j) {
		best = min(best, max(abs(r.X-p.X), abs(r.Y-p.Y)))
	}
	return best
}

// weFired reports a shot or a ranged trace by colonist id this update.
func weFired(j *journey, id uint64) bool {
	if weShotsBy(j, id) > 0 {
		return true
	}
	for _, tr := range j.world().Traces {
		if uint64(wcNum(j, tr, "Attacker")) == id && !tr.Melee && !tr.FromBuilding {
			return true
		}
	}
	return false
}

// TestJourneyFireAtWillOffHoldsFire (DEF-4): with Fire at will off a
// drafted rifleman keeps its fire with a raider in range; turning it back
// on lets it shoot.
func TestJourneyFireAtWillOffHoldsFire(t *testing.T) {
	t.Parallel()
	// a 48-tile map with the colonist in the middle: the club raider walks
	// a long way through rifle range before it can reach melee, so a held
	// fire shows over many updates and the colonist is still up to fire
	// once the toggle comes back on
	j := newJourney(t, jSize(48, 48), jPawns([2]int{24, 24}))
	id := ordColonists(j)[0]
	j.g.Arm(weID(id), item.Rifle)
	j.lookAt(24, 24)
	j.update(1)
	j.settle()
	weDraft(j, 0)
	j.g.RaidArmed(item.Club)
	j.update(1) // the raid alert pauses the game
	if !j.pawn(id).FireAtWill {
		t.Fatal("a fresh colonist does not fire at will")
	}
	j.clickID(inspector.FireAtWillID)
	if c, ok := j.lastCmd(command.SetFireAtWill); !ok || c.Pawn != id || c.Enabled {
		t.Fatalf("SetFireAtWill %+v (sent %v), want colonist %d, Enabled false", c, ok, id)
	}
	j.update(1)
	if j.pawn(id).FireAtWill {
		t.Fatal("PawnView.FireAtWill is still true after the button")
	}
	j.key("Digit3", 0)
	// count updates with the raider inside rifle range but short of melee;
	// stop before it closes in
	inRange, shots := 0, 0
	for i := 0; i < 4000 && inRange < 60; i++ {
		careFeed(j)
		j.update(1)
		if weFired(j, id) {
			shots++
		}
		d := weDist(j, id)
		if d <= 3 {
			break
		}
		if d <= 20 {
			inRange++
		}
	}
	j.key("Space", 0)
	if inRange < 30 {
		t.Fatalf("the raider spent only %d updates in rifle range (dist %d)", inRange, weDist(j, id))
	}
	if shots > 0 {
		t.Fatalf("the colonist fired %d times with Fire at will off", shots)
	}
	j.clickID(inspector.FireAtWillID)
	if c, ok := j.lastCmd(command.SetFireAtWill); !ok || !c.Enabled {
		t.Fatalf("second press sent %+v (%v), want Enabled true", c, ok)
	}
	j.key("Digit3", 0)
	for i := 0; i < 1500 && shots == 0; i++ {
		careFeed(j)
		j.update(1)
		if weFired(j, id) {
			shots++
		}
	}
	j.key("Space", 0)
	if shots == 0 {
		t.Fatalf("no shot after Fire at will came back on; colonist %+v", j.pawn(id))
	}
}

// weHoldRun runs the fleeing-raider scenario with hold position on or off
// and returns how far the colonist moved.
func weHoldRun(t *testing.T, hold bool) int {
	t.Helper()
	j, id := weFireSetup(t)
	raider := weRaiders(j)[0].ID
	for i := 0; i < 80 && !j.pawn(uint64(raider)).Fleeing; i++ {
		j.g.Injure(raider, 4)
		j.tick(1)
		if j.pawn(uint64(raider)).Status != "alive" {
			break
		}
	}
	if !j.pawn(uint64(raider)).Fleeing {
		t.Fatalf("could not make the raider flee: %+v", j.pawn(uint64(raider)))
	}
	if hold {
		j.clickID(inspector.HoldPositionID)
		if c, ok := j.lastCmd(command.SetHoldPosition); !ok || c.Pawn != id || !c.Enabled {
			t.Fatalf("SetHoldPosition %+v (sent %v), want colonist %d, Enabled true", c, ok, id)
		}
		j.update(1)
		if !j.pawn(id).HoldPosition {
			t.Fatal("PawnView.HoldPosition is false after the button")
		}
	}
	x0, y0 := j.pawn(id).X, j.pawn(id).Y
	far := 0
	j.key("Digit3", 0)
	for i := 0; i < 900; i++ {
		careFeed(j)
		j.update(1)
		p := j.pawn(id)
		far = max(far, max(abs(p.X-x0), abs(p.Y-y0)))
	}
	j.key("Space", 0)
	return far
}

// TestJourneyHoldPosition (DEF-4): a drafted colonist holding position
// stays on its tile while a hostile flees; without it the control run may
// move (logged, not asserted).
func TestJourneyHoldPosition(t *testing.T) {
	t.Parallel()
	if far := weHoldRun(t, true); far != 0 {
		t.Fatalf("a colonist holding position moved %d tiles after a fleeing hostile", far)
	}
	t.Logf("control (no hold) moved %d tiles", weHoldRun(t, false))
}

// weGrenadier gives colonist id grenades of kind k through the real
// equip order and waits for PawnView.Grenades.
func weGrenadier(j *journey, id uint64, k item.Kind) {
	j.t.Helper()
	if _, err := j.g.SpawnStack(k, 3, 8, 5); err != nil {
		j.t.Fatal(err)
	}
	wcPrioritize(j, id, 8, 5)
	j.key("Digit3", 0)
	for i := 0; i < 1500 && j.pawn(id).Grenades == 0; i++ {
		j.update(1)
	}
	j.key("Space", 0)
	if j.pawn(id).Grenades == 0 {
		j.t.Fatalf("colonist %d never picked up the %s: %+v", id, k, j.pawn(id))
	}
}

// weThrow drafts the colonist, presses Throw grenade and clicks (x, y).
func weThrow(j *journey, id uint64, x, y int) {
	j.t.Helper()
	weDraft(j, 0)
	j.clickID(inspector.ThrowGrenadeID)
	j.lookAt(x, y)
	j.update(1)
	j.settle()
	j.clickTile(x, y, 0, 0)
	if c, ok := j.lastCmd(command.ThrowGrenade); !ok || c.Pawn != id || c.X != x || c.Y != y {
		j.t.Fatalf("ThrowGrenade %+v (sent %v), want colonist %d at (%d, %d)", c, ok, id, x, y)
	}
}

// weBurstNear reports a burst of that blast within r tiles of (x, y).
func weBurstNear(j *journey, blast string, x, y, r int) bool {
	for _, b := range j.world().Bursts {
		if strings.EqualFold(b.Blast, blast) && abs(b.X-x) <= r && abs(b.Y-y) <= r {
			return true
		}
	}
	return false
}

// TestJourneyThrowGrenade (DEF-4): the Throw grenade button arms a map
// click and a bomb burst appears at the clicked cell.
func TestJourneyThrowGrenade(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	id := ordColonists(j)[0]
	weGrenadier(j, id, item.FragGrenades)
	const tx, ty = 14, 4
	weThrow(j, id, tx, ty)
	hit := false
	j.key("Digit3", 0)
	for i := 0; i < 1500 && !hit; i++ {
		careFeed(j)
		j.update(1)
		hit = weBurstNear(j, "bomb", tx, ty, 1)
	}
	j.key("Space", 0)
	if !hit {
		t.Fatalf("no bomb burst near (%d, %d): %+v", tx, ty, j.world().Bursts)
	}
	if n := j.pawn(id).Grenades; n != 2 {
		t.Fatalf("Grenades after one throw is %d, want 2", n)
	}
}

// weStunned reads a stun flag or counter off the animal view, if the read
// model has one.
func weStunned(j *journey, a any) (stunned, known bool) {
	v := reflectStruct(a)
	for i := 0; i < v.NumField(); i++ {
		if !strings.Contains(strings.ToLower(v.Type().Field(i).Name), "stun") {
			continue
		}
		f := v.Field(i)
		switch {
		case f.Kind() == reflect.Bool:
			return f.Bool(), true
		case f.CanInt():
			return f.Int() > 0, true
		case f.CanUint():
			return f.Uint() > 0, true
		}
	}
	return false, false
}

// TestJourneyMechEMP (DEF-4): an EMP grenade stuns a mechanoid; killed, it
// leaves a wreck with steel and no meat.
func TestJourneyMechEMP(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	id := ordColonists(j)[0]
	weGrenadier(j, id, item.EMPGrenades) // armed before the mech lands: it attacks at once
	var mech, mx, my = uint64(0), 0, 0
	for _, sp := range []string{"harrow", "spindle", "bulwark", "quill"} {
		if m, err := j.g.SpawnAnimal(sp, 16, 4); err == nil {
			mech, mx, my = uint64(m), 16, 4
			break
		}
	}
	if mech == 0 {
		weHook(j, []string{"SpawnMech"}, nil, 0, 16, 4)
		t.Fatal("SpawnAnimal knows no mechanoid species yet (harrow, spindle, bulwark, quill)")
	}
	view := func() (v any, ok bool) {
		for _, a := range j.world().Animals {
			if uint64(a.ID) == mech {
				return a, true
			}
		}
		return nil, false
	}
	if v, ok := view(); ok {
		mx, my = wcIntOf(j, v, "X"), wcIntOf(j, v, "Y")
	}
	weThrow(j, id, mx, my)
	stunned, known := false, false
	burst := false
	j.key("Digit3", 0)
	for i := 0; i < 1500 && !(stunned && known); i++ {
		j.update(1)
		burst = burst || weBurstNear(j, "emp", mx, my, 3)
		if v, ok := view(); ok && burst {
			s, k := weStunned(j, v)
			stunned, known = stunned || s, known || k
		}
	}
	j.key("Space", 0)
	if !burst {
		t.Fatalf("no EMP burst near the mech at (%d, %d): %+v", mx, my, j.world().Bursts)
	}
	if !known {
		t.Fatalf("AnimalView has no stun field (a read-model flag is needed): %+v", mustView(t, view))
	}
	if !stunned {
		t.Fatal("the mech was never stunned by the EMP burst")
	}
	j.g.KillAnimal(weID(mech))
	j.update(2)
	// a dead mech leaves a wreck (shredded for steel at the machining
	// table, wiki), never meat
	wreck, meat := false, false
	for _, it := range j.world().Items {
		wreck = wreck || it.Wreck
		meat = meat || strings.EqualFold(it.Kind, "meat")
	}
	if !wreck || meat {
		t.Fatalf("the dead mech left wreck=%v meat=%v, want a wreck and no meat; items %+v", wreck, meat, j.world().Items)
	}
}
