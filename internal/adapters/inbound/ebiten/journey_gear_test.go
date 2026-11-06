package ebiten

// journey_gear_test.go (GEAR-2): the colonist's Gear tab through the real
// loop. Drop (and Shift-Drop, which waits behind the queued orders), Eat
// from the pack and the garment lock each send their command, and the
// colony shows the result; Info opens the Info window over the map.

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/infocard"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector/gear"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
	"rimworld-go/pkg/mathx"
)

// gearOpen selects the first colonist and opens its Gear tab.
func gearOpen(j *journey) {
	j.t.Helper()
	ordPick(j, 0)
	j.clickID("inspector.gear")
	j.update(2)
	j.draw(2)
	j.reset()
}

// gearReveal scrolls the Gear tab until widget id is declared (the list
// can be taller than the panel).
func gearReveal(j *journey, id string) {
	j.t.Helper()
	k := j.a.uiSt()
	for off := float32(0); off < 3000; off += 20 {
		if k.Scroll == nil {
			k.Scroll = map[string]float32{}
		}
		k.Scroll[gear.ScrollID] = off
		j.update(1)
		if _, ok := k.Find(id); ok {
			return
		}
	}
	j.t.Fatalf("the Gear tab never shows %s", id)
}

// gearShiftClick clicks kit widget id with Shift held.
func gearShiftClick(j *journey, id string) {
	j.t.Helper()
	j.update(1)
	w, ok := j.a.uiSt().Find(id)
	if !ok || w.Disabled {
		j.t.Fatalf("widget %s: %+v (%v)", id, w, ok)
	}
	x, y := w.R.Center()
	s := float32(j.a.scale.S)
	j.f.Cur.Mx, j.f.Cur.My = int(x*s), int(y*s)
	j.f.Click(j.t, j.a, inputframe.Left, input.ModShift)
	j.f.Cur.Mx, j.f.Cur.My = -50, -50
}

// gearLooseAt counts kind lying loose at the feet of a colonist at
// (x, y): its tile or one beside it, as it may have stepped on since.
func gearLooseAt(j *journey, kind string, x, y int) int {
	n := 0
	for _, it := range j.g.World().Items {
		if it.Kind == kind && !it.Carried && mathx.Abs(it.X-x) <= 1 && mathx.Abs(it.Y-y) <= 1 {
			n += it.Qty
		}
	}
	return n
}

// TestJourneyGearDropWeapon: Drop beside the wielded knife sends DropGear
// for the weapon, at once, and the knife lies at the colonist's feet.
func TestJourneyGearDropWeapon(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	pawn := ordColonists(j)[0]
	j.g.Arm(entity.EntityID(pawn), item.Knife)
	gearOpen(j)
	if !j.clickID(gear.DropID("weapon", 0)) {
		t.Fatal("Drop is disabled for a colonist's weapon")
	}
	c, ok := j.lastCmd(command.DropGear)
	if !ok || c.Pawn != pawn || c.Mode != 2 || c.Queue {
		t.Fatalf("command %+v (sent %v), want DropGear mode 2 (weapon), not queued", c, ok)
	}
	j.tick(2)
	p := j.pawn(pawn)
	if p.Weapon != "fists" || gearLooseAt(j, item.Knife.String(), p.X, p.Y) != 1 {
		t.Fatalf("holds %q, %d knives at (%d, %d)", p.Weapon, gearLooseAt(j, item.Knife.String(), p.X, p.Y), p.X, p.Y)
	}
}

// TestJourneyGearShiftDropGarment: Shift-Drop on a garment queues the
// drop behind the current orders; the colonist takes it off and it lies
// at its feet.
func TestJourneyGearShiftDropGarment(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	pawn := ordColonists(j)[0]
	worn := j.pawn(pawn).Apparel
	if len(worn) == 0 {
		t.Fatal("fixture: a new colonist is dressed")
	}
	gearOpen(j)
	id := gear.DropID("worn", 0)
	gearReveal(j, id)
	gearShiftClick(j, id)
	c, ok := j.lastCmd(command.DropGear)
	if !ok || c.Pawn != pawn || c.Mode != 1 || c.Index != 0 || !c.Queue {
		t.Fatalf("command %+v (sent %v), want a queued DropGear of garment 0", c, ok)
	}
	kind := worn[0]
	careRun(j, "Digit3", 600, func() bool {
		careFeed(j)
		return len(j.pawn(pawn).Apparel) == len(worn)-1
	})
	p := j.pawn(pawn)
	if len(p.Apparel) != len(worn)-1 || gearLooseAt(j, kind, p.X, p.Y) != 1 {
		t.Fatalf("wears %v; %d %s at (%d, %d)", p.Apparel, gearLooseAt(j, kind, p.X, p.Y), kind, p.X, p.Y)
	}
}

// TestJourneyGearLockGarment: the lock toggle sends SetApparelForced and
// the garment shows locked; a second click unlocks it.
func TestJourneyGearLockGarment(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	pawn := ordColonists(j)[0]
	if len(j.pawn(pawn).Apparel) == 0 {
		t.Fatal("fixture: a new colonist is dressed")
	}
	gearOpen(j)
	id := gear.LockID(0)
	for _, on := range []bool{true, false} {
		gearReveal(j, id)
		j.reset()
		if !j.clickID(id) {
			t.Fatal("the lock is disabled")
		}
		c, ok := j.lastCmd(command.SetApparelForced)
		if !ok || c.Pawn != pawn || c.Index != 0 || c.Enabled != on {
			t.Fatalf("command %+v (sent %v), want SetApparelForced garment 0 to %v", c, ok, on)
		}
		j.tick(1)
		if f := j.pawn(pawn).ApparelForced; len(f) == 0 || f[0] != on {
			t.Fatalf("forced %v, want garment 0 %v", f, on)
		}
		j.update(1)
		if w, _ := j.a.uiSt().Find(id); strings.HasPrefix(w.Tip, "Unlock") != on {
			t.Fatalf("the lock's tip %q, want locked %v", w.Tip, on)
		}
	}
}

// TestJourneyGearEatFromPack: a colonist that packed a spare meal eats it
// from the Gear tab; the meal is gone and its food bar rises.
func TestJourneyGearEatFromPack(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	pawn := ordColonists(j)[0]
	id := entity.EntityID(pawn)
	j.g.Enqueue(command.Command{Kind: command.SetGameplayToggle, Index: int(gameplay.SpareMeal), Enabled: true})
	for h := 0; h < 24; h++ {
		j.g.Enqueue(command.Command{Kind: command.SetSchedule, Pawn: 0, Index: h, Mode: uint8(schedule.Recreation)})
	}
	if _, err := j.g.SpawnStack(item.Meal, 1, 10, 4); err != nil {
		t.Fatal(err)
	}
	packed := func() bool {
		p := j.pawn(pawn)
		return len(p.Pack) == 1 && p.Pack[0].Name == item.Meal.String()
	}
	careRun(j, "Digit3", 4000, func() bool {
		j.g.SetNeeds(id, needs.Full*8/10, needs.Full)
		return packed()
	})
	if !packed() {
		t.Fatalf("fixture: the colonist never packed the meal: %+v", j.pawn(pawn).Pack)
	}
	j.g.SetNeeds(id, needs.Full/2, needs.Full) // not hungry (no meal of its own), not full (the tab may feed it)
	hunger := j.pawn(pawn).Hunger
	entry := int(j.pawn(pawn).Pack[0].ID)
	gearOpen(j)
	eat := gear.EatID(entry)
	gearReveal(j, eat)
	if !j.clickID(eat) {
		t.Fatalf("Eat is disabled: %+v", j.pawn(pawn).Pack)
	}
	c, ok := j.lastCmd(command.ConsumeFromPack)
	if !ok || c.Pawn != pawn || c.Index != entry {
		t.Fatalf("command %+v (sent %v), want ConsumeFromPack of entry %d", c, ok, entry)
	}
	j.tick(needs.EatWorkTicks + 20)
	p := j.pawn(pawn)
	if len(p.Pack) != 0 || p.Hunger <= hunger {
		t.Fatalf("pack %+v, hunger %d (was %d)", p.Pack, p.Hunger, hunger)
	}
}

// TestJourneyGearInfoOpensTheCard: Info on a garment row opens the Info
// window over the map and keeps the colonist selected.
func TestJourneyGearInfoOpensTheCard(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	pawn := ordColonists(j)[0]
	gearOpen(j)
	id := gear.InfoID("worn", 0)
	gearReveal(j, id)
	if !j.clickID(id) {
		t.Fatal("Info is disabled")
	}
	j.update(2)
	j.draw(1)
	if j.a.uiModalTop() != infocard.ID {
		t.Fatalf("top modal %q, want the Info window", j.a.uiModalTop())
	}
	if sel, ok := j.a.shell.Selected(); !ok || uint64(sel) != pawn {
		t.Fatalf("the colonist lost the selection: %v %v", sel, ok)
	}
}
