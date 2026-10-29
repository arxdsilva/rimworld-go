package ebiten

// journey_care_test.go: the SIM-063..066 order journeys through the real
// UI loop: select a colonist, right-click the pawn, choose the row, watch the
// outcome. Fixtures live in journey_care_more_helpers_test.go.

import (
	"fmt"
	"strings"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// TestJourneyOrdersFeedPatient (SIM-064): "Feed <name>" on a downed, hungry
// colonist: the doer delivers a meal and the patient's food need rises.
func TestJourneyOrdersFeedPatient(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{12, 6}))
	careBed(j, "hospital bed", 16, 2)
	doer, patient, px, py := careDoerAndPatient(j)
	if _, err := j.g.SpawnStack(item.Meal, 4, 9, 4); err != nil {
		t.Fatal(err)
	}
	const hungry = needs.CriticalHunger - 800
	j.g.SetNeeds(entity.EntityID(patient), hungry, needs.Full)
	name := j.pawn(patient).Name
	ordPick(j, 0)
	labels := careRightClick(j, px, py)
	j.pick("Feed " + name)
	c, ok := j.lastCmd(command.Prioritize)
	if !ok || c.Order != command.OrderFeedPatient || c.Pawn != doer || c.Target != patient {
		t.Fatalf("Feed sent %+v (%v), want Prioritize/OrderFeedPatient doer %d target %d; menu %q", c, ok, doer, patient, labels)
	}
	careRun(j, "Digit2", 6000, func() bool {
		careFeedExcept(j, patient)
		return j.pawn(patient).Hunger >= needs.Percent(hungry)+20 // the read model is in percent
	})
}

// TestJourneyOrdersStripPrisoner (SIM-065): "Strip <name>" on a prisoner:
// the weapon leaves the body and lies on the ground as an item.
func TestJourneyOrdersStripPrisoner(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{9, 4}))
	careCapturedRaider(j, 14, 2)
	prisoner := careFirstPrisoner(j)
	if j.pawn(prisoner).Weapon == "fists" {
		j.g.Arm(entity.EntityID(prisoner), item.Club) // capture already took the raid's club
		j.update(1)
	}
	p := j.pawn(prisoner)
	if p.Weapon == "fists" {
		t.Fatalf("the prisoner holds no weapon to strip (%q)", p.Weapon)
	}
	ordPick(j, 0)
	doer := uint64(j.a.renderer.Selected())
	labels := careRightClick(j, p.X, p.Y)
	j.pick("Strip " + p.Name)
	c, ok := j.lastCmd(command.Prioritize)
	if !ok || c.Order != command.OrderStrip || c.Pawn != doer || c.Target != prisoner {
		t.Fatalf("Strip sent %+v (%v), want Prioritize/OrderStrip doer %d target %d; menu %q", c, ok, doer, prisoner, labels)
	}
	before := len(j.world().Items)
	careRun(j, "Digit2", 6000, func() bool {
		careFeed(j)
		return j.pawn(prisoner).Weapon == "fists"
	})
	if len(j.world().Items) <= before {
		t.Fatalf("stripped, but no gear item appeared (%d items before, %d after)", before, len(j.world().Items))
	}
}

// TestJourneyOrdersTakePrisonerToBed (SIM-063): a downed prisoner lying
// outside a bed gets "Take <name> to bed" and ends up in a prison bed.
func TestJourneyOrdersTakePrisonerToBed(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{9, 4}, [2]int{6, 8}))
	bed := careBed(j, "sleeping spot", 20, 2)
	carePrisonBed(j, bed)
	doer := ordColonists(j)[2]
	ordPick(j, 2)
	if got := uint64(j.a.renderer.Selected()); got != doer {
		t.Fatalf("selected %d, want the first colonist %d", got, doer)
	}
	prisoner := careArrestedOutOfBed(j)
	careDown(j, prisoner)
	if j.pawn(prisoner).InBed {
		t.Fatal("fixture: the prisoner is still in a bed")
	}
	p := j.pawn(prisoner)
	labels := careRightClick(j, p.X, p.Y)
	j.pick("Take " + p.Name + " to bed")
	c, ok := j.lastCmd(command.Prioritize)
	if !ok || c.Order != command.OrderCarryToPrisonBed || c.Pawn != doer || c.Target != prisoner {
		t.Fatalf("Take to bed sent %+v (%v), want Prioritize/OrderCarryToPrisonBed doer %d target %d; menu %q", c, ok, doer, prisoner, labels)
	}
	careRun(j, "Digit3", 8000, func() bool {
		careFeed(j)
		return j.pawn(prisoner).InBed
	})
	for _, b := range j.world().Buildings {
		if b.ID == bed && b.Claimant != entity.EntityID(prisoner) {
			t.Fatalf("the prisoner is in a bed, but not the prison bed (claimant %d)", b.Claimant)
		}
	}
}

// TestJourneyOrdersRescueVisitor (SIM-063): a downed visitor offered
// "Rescue <name>" is carried to a free bed.
func TestJourneyOrdersRescueVisitor(t *testing.T) {
	t.Parallel()
	j, visitor := careVisitorJourney(t)
	careBed(j, "hospital bed", 16, 2)
	careDownGently(j, visitor)
	v := j.pawn(visitor)
	if !v.Visitor {
		t.Fatal("fixture: the downed pawn is not a visitor")
	}
	doer, _ := careFirstColonist(j)
	careSelect(j, doer)
	labels := careRightClickSettled(j, v.X, v.Y)
	j.pick("Rescue " + v.Name)
	c, ok := j.lastCmd(command.Prioritize)
	if !ok || c.Order != command.OrderRescue || c.Pawn != doer || c.Target != visitor {
		t.Fatalf("Rescue sent %+v (%v), want Prioritize/OrderRescue doer %d target %d; menu %q", c, ok, doer, visitor, labels)
	}
	careRun(j, "Digit3", 8000, func() bool {
		careFeed(j)
		return j.pawn(visitor).InBed
	})
}

// TestJourneyOrdersArrestBerserk (SIM-066): a colonist in a berserk break
// can be arrested from the menu by another colonist, and attacked by a
// drafted one.
func TestJourneyOrdersArrestBerserk(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{12, 6}))
	carePrisonBed(j, careBed(j, "sleeping spot", 16, 2))
	ids := ordColonists(j)
	doer, target := ids[0], ids[1]
	j.g.ForceBreak(entity.EntityID(target), mentalbreak.Berserk)
	j.update(1)
	tp := j.pawn(target)
	if tp.MentalBreak == "" {
		t.Fatal("fixture: the forced break did not start")
	}
	ordPick(j, 0)
	if got := uint64(j.a.renderer.Selected()); got != doer {
		t.Fatalf("selected %d, want %d", got, doer)
	}
	labels := careRightClick(j, tp.X, tp.Y)
	j.pick("Arrest " + tp.Name)
	c, ok := j.lastCmd(command.Prioritize)
	if !ok || c.Order != command.OrderArrest || c.Pawn != doer || c.Target != target {
		t.Fatalf("Arrest sent %+v (%v), want Prioritize/OrderArrest doer %d target %d; menu %q", c, ok, doer, target, labels)
	}
	// attack orders are for a drafted colonist, as against raiders
	j.key("R", 0)
	j.update(1)
	if !j.pawn(doer).Drafted {
		t.Fatal("R did not draft the doer")
	}
	tp = j.pawn(target)
	labels = careRightClick(j, tp.X, tp.Y)
	if !strings.Contains(strings.Join(labels, "|"), "Melee attack "+tp.Name) && !strings.Contains(strings.Join(labels, "|"), "Shoot "+tp.Name) {
		t.Fatalf("a berserk colonist offers a drafted colonist no attack row; menu %q", labels)
	}
}

// TestJourneyHudJoinOfferAccept (SIM-063): a rescued stranger's letter
// offers Accept in place of Continue; pressing it sends AcceptLetter and
// the stranger joins the colony.
func TestJourneyHudJoinOfferAccept(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	s := j.g.DevJoinOffer(10, 10)
	j.update(2)
	var id uint64
	for _, l := range j.world().Letters {
		if l.JoinOffer && l.Focus == s {
			id = l.ID
		}
	}
	if id == 0 {
		t.Fatalf("no join offer letter: %+v", j.world().Letters)
	}
	if !j.clickID(fmt.Sprintf("letter.%d", id)) {
		t.Fatal("letter row disabled")
	}
	if !j.clickID(fmt.Sprintf("modal.letter.%d.accept", id)) {
		t.Fatal("the join offer's Accept button is missing or disabled")
	}
	c, ok := j.lastCmd(command.AcceptLetter)
	if !ok || c.Target != id {
		t.Fatalf("Accept sent %+v (%v), want AcceptLetter %d", c, ok, id)
	}
	j.update(2)
	joined := false
	for _, p := range j.g.Snapshot() {
		if p.ID == s {
			joined = true
		}
	}
	if !joined {
		t.Fatal("accepted, the stranger is not a colonist")
	}
}
