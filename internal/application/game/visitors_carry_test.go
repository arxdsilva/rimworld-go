package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/mathx"
)

// MAP-23 (wiki: Visitor): "When leaving, visitors will carry away wounded
// members of their faction."

// carryScene is a guest about to leave and a friend of theirs down, with the
// colonist drafted so nobody at home goes to the rescue.
func carryScene(t *testing.T) (g *Game, leaver, friend entity.EntityID, fac int) {
	t.Helper()
	g, doc, leaver, fac := guestScene(t)
	g.draft[doc] = true
	now := uint64(g.clock.Now())
	friend = g.spawnVisitor(fac, g.factionOf(fac), 16, 6, now)
	g.hurt(friend, health.DownedInjury+10, 0)
	visitsOf(g).SetStay(friend, now+clock.TicksPerDay)
	visitsOf(g).SetStay(leaver, now)
	return g, leaver, friend, fac
}

// untilCarrying steps until id carries someone.
func untilCarrying(t *testing.T, g *Game, id, friend entity.EntityID) {
	t.Helper()
	for i := 0; i < 800; i++ {
		if g.care.Carry.Patient(id) == friend {
			return
		}
		g.Step(1)
	}
	t.Fatalf("%d never lifted %d", id, friend)
}

// TestLeavingVisitorCarriesTheWoundedHome: both go, by the edge, and no shot or
// goodwill changes hands over it.
func TestLeavingVisitorCarriesTheWoundedHome(t *testing.T) {
	t.Parallel()
	g, a, b, fac := carryScene(t)
	gw := g.factionOf(fac).Goodwill
	untilCarrying(t, g, a, b)
	if !messageHas(g, "is carrying") {
		t.Fatal("no message")
	}
	for i := 0; i < 4000 && (g.onMap(a) || g.onMap(b)); i++ {
		g.Step(1)
	}
	if g.onMap(a) || g.onMap(b) {
		t.Fatalf("still here: %v %v", g.onMap(a), g.onMap(b))
	}
	if g.care.Carry.Patient(a) != 0 {
		t.Fatal("the carry outlived the walk")
	}
	if got := g.factionOf(fac).Goodwill; got != gw {
		t.Fatalf("goodwill %d, was %d", got, gw)
	}
	if p := g.personOf(b); p == nil || p.Where != whereOffMap {
		t.Fatalf("the friend's place: %+v", p)
	}
}

// TestColonyRescueGoesFirst: a guest the colony has taken to heart is not lifted
// out of a doctor's arms.
func TestColonyRescueGoesFirst(t *testing.T) {
	t.Parallel()
	g, doc, a, fac := guestScene(t)
	now := uint64(g.clock.Now())
	b := g.spawnVisitor(fac, g.factionOf(fac), 16, 6, now)
	g.hurt(b, health.DownedInjury+10, 0)
	visitsOf(g).SetStay(b, now+10*clock.TicksPerDay)
	for i := 0; i < 2000 && g.care.Carry.Patient(doc) != b && !g.inBed(b); i++ {
		g.Step(1)
	}
	if g.care.Carry.Patient(doc) != b && !g.inBed(b) {
		t.Fatal("fixture: no rescue")
	}
	visitsOf(g).SetStay(a, uint64(g.clock.Now()))
	for i := 0; i < 2000 && g.onMap(a); i++ {
		if g.care.Carry.Patient(a) != 0 {
			t.Fatal("the leaver took the doctor's patient")
		}
		g.Step(1)
	}
	if g.onMap(a) || !g.onMap(b) {
		t.Fatalf("leaver here %v, patient here %v", g.onMap(a), g.onMap(b))
	}
}

// TestCarrierDownedDropsTheFriend: both then lie where they fell, and a third
// guest can pick up either.
func TestCarrierDownedDropsTheFriend(t *testing.T) {
	t.Parallel()
	g, a, b, _ := carryScene(t)
	untilCarrying(t, g, a, b)
	g.hurt(a, health.DownedInjury+10, 0)
	g.Step(2)
	if g.care.Carry.Patient(a) != 0 {
		t.Fatal("still carrying, downed")
	}
	if !g.onMap(a) || !g.onMap(b) {
		t.Fatalf("on the map: %v %v", g.onMap(a), g.onMap(b))
	}
}

// TestLeaverGivesUpOnAFriendOutOfReach is the no-flap test: walled in, the
// friend costs one try. The leaver then walks out and doesn't turn back.
func TestLeaverGivesUpOnAFriendOutOfReach(t *testing.T) {
	t.Parallel()
	g, a, b, _ := carryScene(t)
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			if max(mathx.Abs(dx), mathx.Abs(dy)) == 2 {
				buildNow(t, g, construction.Wall, item.Steel, construction.South, 16+dx, 6+dy)
			}
		}
	}
	tries := 0
	fetching := false
	for i := 0; i < 4000 && g.onMap(a); i++ {
		g.Step(1)
		rec, ok := visitorRecOf(g, a)
		if !ok {
			break
		}
		if now := rec.Fetch != 0; now != fetching {
			fetching = now
			if now {
				tries++
			}
		}
	}
	if g.onMap(a) {
		t.Fatal("the leaver stayed")
	}
	if tries > 1 {
		t.Fatalf("%d tries: it flaps", tries)
	}
	if !g.onMap(b) {
		t.Fatal("the friend was reached through the wall")
	}
}

// TestTwoLeaversShareOneFriend: only one lifts them, and the other goes alone.
func TestTwoLeaversShareOneFriend(t *testing.T) {
	t.Parallel()
	g, a, b, fac := carryScene(t)
	now := uint64(g.clock.Now())
	c := g.spawnVisitor(fac, g.factionOf(fac), 18, 6, now)
	visitsOf(g).SetStay(c, now)
	for i := 0; i < 4000 && (g.onMap(a) || g.onMap(b) || g.onMap(c)); i++ {
		g.Step(1)
		if g.care.Carry.Patient(a) != 0 && g.care.Carry.Patient(c) != 0 {
			t.Fatal("two carriers")
		}
		ra, oka := visitorRecOf(g, a)
		rc, okc := visitorRecOf(g, c)
		if oka && okc && ra.Fetch != 0 && ra.Fetch == rc.Fetch {
			t.Fatal("both walk over for the same friend")
		}
	}
	if g.onMap(a) || g.onMap(b) || g.onMap(c) {
		t.Fatalf("still here: %v %v %v", g.onMap(a), g.onMap(b), g.onMap(c))
	}
}

// carryRun is a carry played through: the hash, with a save mid-carry if asked.
func carryRun(t *testing.T, reload bool) uint64 {
	t.Helper()
	g, a, b, _ := carryScene(t)
	untilCarrying(t, g, a, b)
	if reload {
		h := saveLoad(t, g)
		if h.StateHash() != g.StateHash() {
			t.Fatal("a save mid-carry moved the hash")
		}
		if h.care.Carry.Patient(a) != b {
			t.Fatal("the save put the friend down")
		}
		g = h
	}
	g.Step(600)
	return g.StateHash()
}

// TestCarrySameSeedAndSaveHash: same seed same hash, a save in the middle too.
func TestCarrySameSeedAndSaveHash(t *testing.T) {
	t.Parallel()
	x := carryRun(t, false)
	if y := carryRun(t, false); x != y {
		t.Fatalf("same seed: %x, %x", x, y)
	}
	if z := carryRun(t, true); x != z {
		t.Fatalf("a save moved it: %x, %x", x, z)
	}
}

// TestFactionTurnsMidCarry: the carrier puts the friend down and leaves as an
// enemy, alone (they run on the raid AI from there).
func TestFactionTurnsMidCarry(t *testing.T) {
	t.Parallel()
	g, a, b, fac := carryScene(t)
	untilCarrying(t, g, a, b)
	turnHostile(t, g, fac)
	g.Step(2)
	if !g.isRaider(a) || g.visitor(a) {
		t.Fatal("the carrier did not turn")
	}
	if g.care.Carry.Patient(a) != 0 {
		t.Fatal("an enemy goes on carrying")
	}
	if !g.onMap(b) {
		t.Fatal("the friend went with them")
	}
}

// arrestScene is a colonist about to arrest one of two guests.
func arrestScene(t *testing.T, chance int) (g *Game, doc, a, b entity.EntityID, fac int) {
	t.Helper()
	g, doc, a, fac = guestScene(t)
	now := uint64(g.clock.Now())
	b = g.spawnVisitor(fac, g.factionOf(fac), 12, 8, now)
	visitsOf(g).SetStay(a, now+10*clock.TicksPerDay)
	visitsOf(g).SetStay(b, now+10*clock.TicksPerDay)
	g.sk.stats[doc].sheet[stat.ArrestSuccess] = int32(chance)
	bed := buildNow(t, g, construction.Bed, item.Wood, construction.South, 20, 3)
	bb, _ := g.bldgs.Get(bed)
	bb.ForPrisoners = true
	g.bldgs.Set(bed, bb)
	return g, doc, a, b, fac
}

// TestArrestingAVisitorTurnsTheFaction (wiki: Events, Factions): reaching the
// target, whatever the roll, sets relations to -75 and makes enemies of the
// others on the map.
func TestArrestingAVisitorTurnsTheFaction(t *testing.T) {
	t.Parallel()
	if arrestGoodwill != -75 {
		t.Fatalf("arrest goodwill %d, the wiki says -75", arrestGoodwill)
	}
	for _, c := range []struct {
		name   string
		chance int
	}{{"held", 1000}, {"resisted", 0}} {
		g, doc, a, b, fac := arrestScene(t, c.chance)
		gw := g.factionOf(fac).Goodwill
		g.Enqueue(command.Command{Kind: command.Arrest, Pawn: uint64(doc), Target: uint64(a)})
		for i := 0; i < 800 && g.factionOf(fac).Goodwill == gw; i++ {
			g.Step(1)
		}
		f := g.factionOf(fac)
		if f.Goodwill != arrestGoodwill || f.Relation != faction.Hostile {
			t.Fatalf("%s: goodwill %d, relation %v", c.name, f.Goodwill, f.Relation)
		}
		g.Step(3)
		if g.visitor(b) || !g.isRaider(b) {
			t.Fatalf("%s: the other guest stayed a guest", c.name)
		}
		if len(turnedLetters(g)) != 1 {
			t.Fatalf("%s: letters %+v", c.name, turnedLetters(g))
		}
	}
}

// TestArrestAttemptStartsNoSecondDrop: taking them costs nothing beyond the attempt.
func TestArrestAttemptStartsNoSecondDrop(t *testing.T) {
	t.Parallel()
	g, doc, a, _, fac := arrestScene(t, 1000)
	g.Enqueue(command.Command{Kind: command.Arrest, Pawn: uint64(doc), Target: uint64(a)})
	for i := 0; i < 800 && !g.prison.Is(a); i++ {
		g.Step(1)
	}
	if !g.prison.Is(a) {
		t.Fatal("a sure arrest took nobody")
	}
	if f := g.factionOf(fac); f.Goodwill != arrestGoodwill {
		t.Fatalf("goodwill %d: the capture took more than the attempt", f.Goodwill)
	}
}

// TestTurnedGuestsRaiseNoRaidAlert: they run for the edge, so the view has them
// fleeing, which keeps them off the raid alert (hud/alerts.go), the letter
// is "caution" and nothing pauses for it.
func TestTurnedGuestsRaiseNoRaidAlert(t *testing.T) {
	t.Parallel()
	g, _, a, fac := guestScene(t)
	g.Step(2)
	turnHostile(t, g, fac)
	g.Step(2)
	v, ok := raiderViewOf(g, a)
	if !ok || !v.Hostile || !v.Fleeing {
		t.Fatalf("view %+v", v)
	}
	ls := turnedLetters(g)
	if len(ls) != 1 || ls[0].Kind != letterCaution {
		t.Fatalf("letter %+v", ls)
	}
	for _, m := range g.messageViews() {
		if m.Danger && strings.Contains(m.Text, g.personName(a)) {
			t.Fatalf("a danger message would pause: %q", m.Text)
		}
	}
}
