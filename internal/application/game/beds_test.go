package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// bedroom is a couple of lovers with a single bed each and a double bed.
func bedroom(t *testing.T) (g *Game, a, b, single1, single2, double entity.EntityID) {
	t.Helper()
	g, a, b = couple(t)
	sleepAnytime(g)
	single1 = buildNow(t, g, construction.Bed, item.Wood, construction.South, 2, 2)
	single2 = buildNow(t, g, construction.Bed, item.Wood, construction.South, 4, 2)
	double = buildNow(t, g, construction.DoubleBed, item.Wood, construction.South, 10, 2)
	now := socNow(g)
	g.setPair(a, b, social.Pair{Kind: social.PairLover, Since: now, LastChange: now})
	return
}

// tireOut sends id to bed.
func tireOut(g *Game, id entity.EntityID) {
	g.SetNeeds(id, needs.Full, needs.CriticalRest-100)
	g.interrupt(id, job.Interrupted)
	g.assignSleep(id)
}

// A partnered colonist claims a free double bed; its partner joins the
// free slot; both then keep it.
func TestBedPartnersShareDoubleBed(t *testing.T) {
	g, a, b, s1, _, double := bedroom(t)
	g.cmdAssignBedOwner(s1, a, 0) // a owns a single bed
	tireOut(g, a)
	if g.sleptAt[a] != double {
		t.Fatalf("a sleeps in %d, want the double bed %d", g.sleptAt[a], double)
	}
	if bb, _ := g.bldgs.Get(s1); bb.SlotOf(uint64(a)) >= 0 {
		t.Fatal("a kept its old single bed")
	}
	tireOut(g, b)
	if g.sleptAt[b] != double {
		t.Fatalf("b sleeps in %d, want the double bed", g.sleptAt[b])
	}
	bb, _ := g.bldgs.Get(double)
	if bb.SlotOf(uint64(a)) < 0 || bb.SlotOf(uint64(b)) < 0 || bb.SlotOf(uint64(a)) == bb.SlotOf(uint64(b)) {
		t.Fatalf("double bed slots %d / %d", bb.Claimant, bb.Claimant2)
	}
	if _, found := moodOfKind(g, a, mood.SleptAlone); found {
		t.Fatal("sleeping alone while sharing the bed")
	}
}

// No-flap: night after night the couple keeps the same bed, even with a
// second free double bed around.
func TestBedPartnerChoiceNoFlap(t *testing.T) {
	g, a, b, _, _, _ := bedroom(t)
	buildNow(t, g, construction.DoubleBed, item.Wood, construction.South, 6, 6)
	changes := 0
	last := map[entity.EntityID]entity.EntityID{}
	for range 12 { // nights
		for _, id := range []entity.EntityID{a, b, a, b} {
			tireOut(g, id)
			if was, ok := last[id]; ok && was != g.sleptAt[id] {
				changes++
			}
			last[id] = g.sleptAt[id]
		}
	}
	db, _ := g.bldgs.Get(last[a])
	if changes != 0 || last[a] != last[b] || db.Kind != construction.DoubleBed {
		t.Fatalf("bed changes %d (a in %d, b in %d)", changes, last[a], last[b])
	}
	if c := kindChanges(g, a, 400); c > 2 {
		t.Fatalf("job kind changed %d times in 400 ticks", c)
	}
}

// Sleeping alone is only for the partnered, and only when the beds
// differ.
func TestBedSleptAloneOnlyWhenPartnered(t *testing.T) {
	g, a, b, s1, s2, _ := bedroom(t)
	g.setPair(a, b, social.Pair{}) // strangers first
	g.sleptAt[a], g.sleptAt[b] = s1, s2
	if _, found := moodOfKind(g, a, mood.SleptAlone); found {
		t.Fatal("single pawn sleeps alone")
	}
	g.setPair(a, b, social.Pair{Kind: social.PairLover})
	if v, found := moodOfKind(g, a, mood.SleptAlone); !found || v != -4 {
		t.Fatalf("partnered in separate beds: %d %v", v, found)
	}
	g.sleptAt[b] = s1
	if _, found := moodOfKind(g, a, mood.SleptAlone); found {
		t.Fatal("same bed counted as alone")
	}
	g.sleptAt[b] = s2
	g.positions.Delete(b) // partner away: no thought
	if _, found := moodOfKind(g, a, mood.SleptAlone); found {
		t.Fatal("alone with the partner off the map")
	}
}

// The partner thought is the opinion over ten, capped at +-10.
func TestBedPartnerOpinionThought(t *testing.T) {
	g, a, b, _, _, _ := bedroom(t)
	want := partnerMood(g.opinion(a, b))
	if want != int(g.opinion(a, b)/1000) || want == 0 {
		t.Fatalf("partner mood %d for opinion %d", want, g.opinion(a, b))
	}
	if v, _ := moodOfKind(g, a, mood.PartnerOpinion); v != want {
		t.Fatalf("partner thought %d, want %d", v, want)
	}
	if partnerMood(10000) != 10 || partnerMood(-10000) != -10 || partnerMood(999) != 0 {
		t.Fatal("partner mood clamp")
	}
}

// asleepTogether puts the couple asleep in the double bed.
func asleepTogether(t *testing.T, g *Game, a, b, double entity.EntityID) {
	t.Helper()
	for range 200 {
		for _, id := range []entity.EntityID{a, b} {
			if j, _ := g.jobs.Get(id); j.Kind != job.Sleep {
				tireOut(g, id)
			}
		}
		g.Step(1)
		ja, _ := g.jobs.Get(a)
		jb, _ := g.jobs.Get(b)
		if ja.Kind == job.Sleep && ja.State == job.Do && jb.Kind == job.Sleep && jb.State == job.Do {
			if _, ok := g.lovinBed(a, b); ok {
				return
			}
		}
	}
	t.Fatal("couple never lay down together")
}

// Lovin': a young couple asleep in the same double bed makes love, then
// both feel it; never while one of them is downed.
func TestLovinInDoubleBedNotWhileDowned(t *testing.T) {
	g, a, b, _, _, double := bedroom(t)
	asleepTogether(t, g, a, b, double)
	v, _ := g.vital.Get(b)
	v.Status, v.Cause = health.Downed, health.CauseNoBed
	g.vital.Set(b, v)
	lo := min(a, b)
	for range 50 {
		g.lovinCheck(lo, socNow(g))
		if ja, _ := g.jobs.Get(a); ja.Kind == job.Lovin {
			t.Fatal("lovin' while downed")
		}
	}
	v.Status, v.Cause = health.Alive, health.CauseNone
	g.vital.Set(b, v)
	for range 50 {
		g.lovinCheck(lo, socNow(g))
		if ja, _ := g.jobs.Get(a); ja.Kind == job.Lovin {
			break
		}
	}
	ja, _ := g.jobs.Get(a)
	jb, _ := g.jobs.Get(b)
	if ja.Kind != job.Lovin || jb.Kind != job.Lovin || ja.Target != b || jb.Target != a {
		t.Fatalf("no lovin': %v / %v", ja.Kind, jb.Kind)
	}
	for range lovinTicks + 5 {
		g.SetNeeds(a, needs.Full, needs.CriticalRest-100)
		g.SetNeeds(b, needs.Full, needs.CriticalRest-100)
		g.Step(1)
	}
	for _, id := range []entity.EntityID{a, b} {
		if v, found := moodOfKind(g, id, mood.Lovin); !found || v != 8 {
			t.Fatalf("lovin' mood %d %v", v, found)
		}
	}
	if s := g.gatherSt(); s.lovinAt[a] <= socNow(g) {
		t.Fatal("no lovin' cooldown")
	}
	g.lovinCheck(lo, socNow(g)) // cooldown: no second time at once
	if ja, _ := g.jobs.Get(a); ja.Kind == job.Lovin {
		t.Fatal("lovin' again inside the cooldown")
	}
}

// AssignBedOwner: the owner gets the slot and gives up its old bed; 0
// clears the slot; a bad slot or a non-colonist is ignored.
func TestBedAssignOwnerCommand(t *testing.T) {
	g, a, b, s1, _, double := bedroom(t)
	g.cmdAssignBedOwner(s1, a, 0)
	g.Enqueue(command.Command{Kind: command.AssignBedOwner, Target: uint64(double), Pawn: uint64(a), Index: 1})
	g.Step(1)
	db, _ := g.bldgs.Get(double)
	if db.Claimant2 != uint64(a) {
		t.Fatalf("slot 1 = %d, want %d", db.Claimant2, a)
	}
	if sb, _ := g.bldgs.Get(s1); sb.SlotOf(uint64(a)) >= 0 {
		t.Fatal("old bed kept")
	}
	g.cmdAssignBedOwner(double, b, 1) // b takes a's slot
	db, _ = g.bldgs.Get(double)
	if db.Claimant2 != uint64(b) || db.SlotOf(uint64(a)) >= 0 {
		t.Fatalf("slot 1 = %d after reassigning", db.Claimant2)
	}
	g.cmdAssignBedOwner(double, 0, 1)
	g.cmdAssignBedOwner(double, a, 5)
	g.cmdAssignBedOwner(s1, a, 1) // a single bed has one slot
	db, _ = g.bldgs.Get(double)
	sb, _ := g.bldgs.Get(s1)
	if db.Claimant2 != 0 || db.Claimant != 0 || sb.Claimant != 0 {
		t.Fatalf("clear/ignore failed: %+v %+v", db, sb)
	}
}
