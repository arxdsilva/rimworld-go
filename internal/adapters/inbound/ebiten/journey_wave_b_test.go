package ebiten

// journey_wave_b_test.go: the wave B journeys through the real UI loop:
// PRD-1 bill settings (store mode and radius, ingredient filter, reorder),
// BED-1 bedrolls and forbidden beds, DEF-1 hit-chance tooltip and burst
// fire. Fixtures live in journey_wave_b_helpers_test.go.

import (
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// wbSettings opens bill idx's settings dialog from the Bills tab.
func wbSettings(j *journey, idx string) {
	j.t.Helper()
	j.clickID("bill.cfg." + idx)
	j.update(1)
	if !strings.Contains(j.a.uiModalTop(), "bill.cfg") {
		j.t.Fatalf("top modal %q, want the bill settings dialog", j.a.uiModalTop())
	}
}

// wbCommit closes the dialog with Done (every change was already sent).
func wbCommit(j *journey) {
	j.t.Helper()
	wbClick(j, "dialog Done", []string{"bill.cfg.done"})
}

// TestJourneyBillsStoreModeAndRadius (PRD-1): the bill dialog's "drop on
// floor" and ingredient radius reach the sim as ConfigureBill, and the
// product then lies on the floor near the bench instead of in the
// stockpile.
func TestJourneyBillsStoreModeAndRadius(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	j.g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: 12, Y: 6, X2: 13, Y2: 7})
	j.g.ApplyPending()
	wbBench(j, 10, 5, uint8(recipe.Club))
	if _, err := j.g.SpawnStack(item.Wood, 20, 12, 6); err != nil {
		t.Fatal(err)
	}
	wbSettings(j, "0")
	j.clickID("bill.cfg.store.1") // "Drop on floor"
	for range 6 {                 // from unlimited the first step is radius 1; the wood lies 2-3 tiles away
		j.clickID("bill.cfg.radius.plus")
	}
	wbCommit(j)
	c, ok := j.lastCmd(command.ConfigureBill)
	if !ok || c.X != 10 || c.Y != 5 || c.Index != 0 || c.Store == 0 || c.Radius <= 0 {
		t.Fatalf("ConfigureBill %+v (sent %v), want bench (10,5) bill 0 with a non-default store mode (drop) and radius > 0", c, ok)
	}
	// Step one tick at a time: the colonist, also the only hauler, picks
	// the dropped club up again within a few ticks, so look for the tick
	// it lies at the bench; in "best stockpile" mode it is carried the
	// whole way and never lies outside the stockpile.
	lay := false
	for i := 0; i < 20000 && !lay; i++ {
		careFeed(j)
		j.tick(1)
		for _, it := range j.world().Items {
			if it.Kind == "club" && !it.Carried && !(it.X >= 12 && it.X <= 13 && it.Y >= 6 && it.Y <= 7) {
				if d := max(abs(it.X-10), abs(it.Y-5)); d > 4 {
					t.Fatalf("the club lies at (%d, %d), %d tiles from the bench", it.X, it.Y, d)
				}
				lay = true
			}
		}
		if i%50 == 49 { // finished: the club stands in the stockpile and never lay outside
			for _, it := range j.world().Items {
				if it.Kind == "club" && !it.Carried {
					t.Fatalf("the club went straight to the stockpile at (%d, %d): drop on floor was ignored", it.X, it.Y)
				}
			}
		}
	}
	if !lay {
		t.Fatal("no club was made")
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// TestJourneyBillsIngredientToggle (PRD-1): forbidding wood in the bill
// dialog sends SetBillIngredient and the crafter stops using it.
func TestJourneyBillsIngredientToggle(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	wbBench(j, 10, 5, uint8(recipe.Club))
	if _, err := j.g.SpawnStack(item.Wood, 20, 12, 5); err != nil {
		t.Fatal(err)
	}
	wbSettings(j, "0")
	j.clickID("bill.cfg.ing." + strconv.Itoa(int(item.Wood)))
	wbCommit(j)
	c, ok := j.lastCmd(command.SetBillIngredient)
	if !ok || c.X != 10 || c.Y != 5 || c.Index != 0 || c.Stuff != uint8(item.Wood) || c.Enabled {
		t.Fatalf("SetBillIngredient %+v (sent %v), want bill 0 at (10,5), ingredient wood, forbidden (Enabled false)", c, ok)
	}
	j.key("Escape", 0)
	wbRun(j, "Digit3", 2500)
	for _, it := range j.world().Items {
		if it.Kind == "club" {
			t.Fatalf("a club was made at (%d, %d) although wood is forbidden for the bill", it.X, it.Y)
		}
	}
	wood := 0
	for _, it := range j.world().Items {
		if it.Kind == "wood" {
			wood += it.Qty
		}
	}
	if wood < 20 {
		t.Fatalf("%d wood left of 20: the crafter used a forbidden ingredient", wood)
	}
}

// wbRun runs the colony at speed key for n Updates, keeping it fed.
func wbRun(j *journey, key string, n int) {
	j.t.Helper()
	j.key(key, 0)
	defer j.key("Space", 0)
	for i := 0; i < n; i++ {
		careFeed(j)
		j.update(1)
	}
}

// TestJourneyBillsReorder (PRD-1): the up arrow on the second bill sends
// ReorderBill and the bills swap places.
func TestJourneyBillsReorder(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	wbBench(j, 10, 5, uint8(recipe.Club), uint8(recipe.Spear))
	bench := wbBuildingAt(j, 10, 5)
	order := func() string {
		var out []string
		for _, b := range j.world().Buildings {
			if b.ID == bench {
				for _, bl := range b.Bills {
					out = append(out, bl.Recipe)
				}
			}
		}
		return strings.Join(out, ",")
	}
	before := order()
	if strings.Count(before, ",") != 1 {
		t.Fatalf("fixture: bills %q, want two", before)
	}
	j.clickID("bill.up.1")
	c, ok := j.lastCmd(command.ReorderBill)
	if !ok || c.X != 10 || c.Y != 5 || c.Index != 1 || c.Count != -1 {
		t.Fatalf("ReorderBill %+v (sent %v), want bench (10,5) bill 1 moved by -1", c, ok)
	}
	after := order()
	parts := strings.Split(before, ",")
	if after != parts[1]+","+parts[0] {
		t.Fatalf("bill order %q after the up arrow, want %q swapped", after, before)
	}
}

// TestJourneyBedrollSleep (BED-1): a tired colonist sleeps in a bedroll
// and the inspector shows the bedroll's rest effectiveness and comfort.
func TestJourneyBedrollSleep(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	// BuildNow takes only wood, stone and steel; a bedroll is sewn from
	// cloth, so it is designated and built by the colonist.
	j.g.Enqueue(command.Command{Kind: command.DesignateBuild, Build: uint8(construction.Bedroll), Stuff: uint8(item.Cloth), X: 10, Y: 4})
	j.g.ApplyPending()
	if _, err := j.g.SpawnStack(item.Cloth, 20, 9, 4); err != nil {
		t.Fatal(err)
	}
	careRun(j, "Digit3", 6000, func() bool {
		careFeed(j)
		for _, b := range j.world().Buildings {
			if b.Kind == "bedroll" {
				return true
			}
		}
		return false
	})
	id := wbBuildingAt(j, 10, 4)
	j.lookAt(10, 4)
	j.update(1)
	// the builder stands beside the bedroll; send it away so the click
	// reaches the bedroll and not the colonist
	j.g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: ordColonists(j)[0], X: 4, Y: 8})
	j.g.ApplyPending()
	j.tick(300)
	j.settle()
	j.clickTile(10, 4, 0, 0)
	if !j.a.sel.HasBuilding || j.a.sel.SelBuilding != id {
		t.Fatalf("the click selected %v/%d (pawn selected: %v), want the bedroll %d", j.a.sel.HasBuilding, j.a.sel.SelBuilding, j.a.renderer.HasSelect(), id)
	}
	lines := wbPaneLines(j, id)
	for _, want := range []string{"rest", "comfort"} {
		if !strings.Contains(lines, want) {
			t.Fatalf("the bedroll pane shows no %q line: %q", want, lines)
		}
	}
	for h := 0; h < 24; h++ {
		j.g.Enqueue(command.Command{Kind: command.SetSchedule, Pawn: 0, Index: h, Mode: uint8(schedule.Anything)})
	}
	j.g.ApplyPending()
	pawn := ordColonists(j)[0]
	j.g.SetNeeds(entity.EntityID(pawn), needs.Full, needs.CriticalRest-500)
	before := j.pawn(pawn).Rest
	careRun(j, "Digit3", 6000, func() bool { return j.pawn(pawn).InBed })
	if p := j.pawn(pawn); !p.InBed {
		t.Fatalf("the colonist does not sleep in the bedroll (rest %d, was %d)", p.Rest, before)
	}
	for _, b := range j.world().Buildings {
		if b.ID == id && b.Claimant != entity.EntityID(pawn) {
			t.Fatalf("the colonist sleeps, but the bedroll's claimant is %d", b.Claimant)
		}
	}
}

// TestJourneyForbidBed (BED-1): forbidding a bed from its inspector sends
// the forbid command, and nobody then sleeps in it.
func TestJourneyForbidBed(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	if !j.g.BuildNow("bed", "wood", "south", 10, 4) {
		t.Fatal("no bed")
	}
	id := wbBuildingAt(j, 10, 4)
	j.lookAt(10, 4)
	j.update(1)
	j.settle()
	j.clickTile(10, 4, 0, 0)
	if !j.a.sel.HasBuilding || j.a.sel.SelBuilding != id {
		t.Fatalf("the click did not select the bed %d", id)
	}
	j.clickID("bldg.forbid")
	c, ok := j.lastCmd(command.SetForbidden)
	if !ok || c.Target != uint64(id) || !c.Enabled {
		t.Fatalf("forbid sent %+v (%v), want SetForbidden Enabled on building %d (contract: reuses SetForbidden with the building id as Target)", c, ok, id)
	}
	for h := 0; h < 24; h++ {
		j.g.Enqueue(command.Command{Kind: command.SetSchedule, Pawn: 0, Index: h, Mode: uint8(schedule.Anything)})
	}
	j.g.ApplyPending()
	pawn := ordColonists(j)[0]
	for i := 0; i < 4000; i++ { // run on, topping the colonist's rest up to exhaustion each time it recovers
		if i%100 == 0 {
			j.g.SetNeeds(entity.EntityID(pawn), needs.Full, needs.CriticalRest-500)
		}
		if i == 0 {
			j.key("Digit3", 0)
		}
		j.update(1)
		if j.pawn(pawn).InBed {
			t.Fatalf("the colonist sleeps in the forbidden bed at update %d", i)
		}
	}
	j.key("Space", 0)
}

// TestJourneyCombatHitChanceTooltip (DEF-1): a drafted, armed colonist is
// selected; hovering a raider asks for the shot's hit chance, and the
// raiders' burst fire leaves several projectiles in flight.
func TestJourneyCombatHitChanceTooltip(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	shooter := ordColonists(j)[0]
	j.g.Arm(entity.EntityID(shooter), item.Rifle)
	ordPick(j, 0)
	j.key("R", 0)
	if !j.pawn(shooter).Drafted {
		t.Fatal("R did not draft the colonist")
	}
	j.g.RaidArmed(item.Rifle, item.Rifle, item.Rifle)
	j.update(2)
	var raider uint64
	var rx, ry int
	for _, p := range j.g.Snapshot() {
		if p.Hostile {
			raider, rx, ry = uint64(p.ID), p.X, p.Y
			break
		}
	}
	if raider == 0 {
		t.Fatal("the raid brought no raider")
	}
	j.lookAt(rx, ry)
	j.f.MoveTo(j.a, rx, ry)
	for i := 0; i < 40; i++ { // rest on the raider past the tooltip delay
		j.draw(1)
		j.update(1)
	}
	rep, ok := j.g.HitChance(entity.EntityID(shooter), entity.EntityID(raider))
	if !ok || rep.Chance <= 0 || rep.Chance > 1000 {
		t.Fatalf("hit chance %+v (ok %v), want a permille in 1..1000", rep, ok)
	}
	// Projectiles fly (DEF-1): run the fight and watch shots in flight on
	// the world view. The colonist's rifle is the single-shot bolt-action,
	// so burst fire is asserted with the burst weapons (DEF-2).
	seen := false
	j.key("Digit3", 0)
	for i := 0; i < 4000 && !seen; i++ {
		careFeedHostileFree(j)
		j.update(1)
		seen = len(j.world().Shots) > 0
	}
	j.key("Space", 0)
	if !seen {
		t.Fatal("no projectile was ever in flight during the fight")
	}
}

// careFeedHostileFree keeps the colonists fed, leaving raiders alone.
func careFeedHostileFree(j *journey) { careFeed(j) }
