package ebiten

// journey_wave_c_test.go: the wave C journeys through the real UI loop:
// PRD-2 crafted item quality and stuff, BED-2 the room card's stats, DEF-2
// burst fire, grenade explosions and the gear tab's weapon and armor
// numbers. Fixtures live in journey_wave_c_helpers_test.go.

import (
	"regexp"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// TestJourneyCraftedItemShowsQuality (PRD-2): a colonist makes a spear at
// the workbench; clicking the finished item shows its quality and
// the wood it is made of in the inspector.
func TestJourneyCraftedItemShowsQuality(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	if !j.g.BuildNow("workbench", "wood", "south", 10, 5) {
		t.Fatal("no tailoring bench")
	}
	j.g.Enqueue(command.Command{Kind: command.AddBill, X: 10, Y: 5, Recipe: uint8(recipe.Spear), Mode: 0, Count: 1})
	j.g.ApplyPending()
	if _, err := j.g.SpawnStack(item.Wood, 30, 12, 5); err != nil {
		t.Fatal(err)
	}
	var made *struct {
		id   entity.EntityID
		x, y int
	}
	careRun(j, "Digit3", 12000, func() bool {
		careFeed(j)
		for _, it := range j.world().Items {
			if it.Kind == "spear" && !it.Carried {
				made = &struct {
					id   entity.EntityID
					x, y int
				}{it.ID, it.X, it.Y}
				return true
			}
		}
		return false
	})
	// let the crafter walk off, then click the spear on the floor
	j.g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: ordColonists(j)[0], X: 4, Y: 8})
	j.g.ApplyPending()
	j.tick(400)
	j.settle()
	j.lookAt(made.x, made.y)
	j.update(1)
	j.clickTile(made.x, made.y, 0, 0)
	sp := j.a.insp.Thing().ItemPane(made.id)
	// the title carries quality and stuff ("Wood spear (normal)"); the
	// lines carry hit points and storage
	text := sp.Title + "|" + strings.Join(sp.Lines, "|")
	if _, ok := wcHasQuality(text); !ok {
		t.Fatalf("the crafted spear's pane names no quality level: %q", text)
	}
	if !strings.Contains(strings.ToLower(text), "wood") {
		t.Fatalf("the crafted spear's pane does not name its stuff (wood): %q", text)
	}
}

// TestJourneyRoomCardStats (BED-2): a furnished bedroom's room card lists
// impressiveness, space, beauty and wealth; dirt tracked in lowers beauty.
func TestJourneyRoomCardStats(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	wcRoom(j, 12, 1, 18, 7, 15, 7)
	if !j.g.BuildNow("bed", "wood", "south", 14, 3) {
		t.Fatal("no bed")
	}
	j.update(2)
	rm, ok := wcRoomAt(j, 15, 4)
	if !ok || rm.Cells == 0 {
		t.Fatalf("no enclosed room at (15, 4): %+v (ok %v)", rm, ok)
	}
	view := func() game.RoomView {
		t.Helper()
		r, ok := wcRoomAt(j, 15, 4)
		if !ok {
			t.Fatal("the room vanished")
		}
		return r
	}
	before := view()
	card := strings.Join(hud.RoomCardLines(before), "\n")
	for _, want := range []string{"Impressiveness", "Space", "Beauty", "Wealth"} {
		if !strings.Contains(card, want) {
			t.Fatalf("room card lacks %q:\n%s", want, card)
		}
	}
	if wcNum(j, before, "Space") <= 0 || wcNum(j, before, "Wealth") <= 0 {
		t.Fatalf("a furnished bedroom: space %v wealth %v, want both positive", wcNum(j, before, "Space"), wcNum(j, before, "Wealth"))
	}
	// the overlay's hover card reaches the player: G, hover the room, draw
	j.key("G", 0)
	j.lookAt(15, 4)
	j.f.MoveTo(j.a, 15, 4)
	j.draw(3)
	// track dirt in: walk the colonist in and out of the room over soil
	pawn := ordColonists(j)[0]
	for i := 0; i < 8; i++ {
		x := 15
		if i%2 == 1 {
			x = 10
		}
		j.g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: pawn, X: x, Y: 4})
		j.g.ApplyPending()
		careFeed(j)
		j.tick(400)
	}
	after := view()
	if !(wcNum(j, after, "Beauty") < wcNum(j, before, "Beauty")) {
		t.Fatalf("beauty %v after dirt, was %v: filth did not lower it", wcNum(j, after, "Beauty"), wcNum(j, before, "Beauty"))
	}
}

// wcShotsPeak runs the fight and returns the most projectiles in flight at
// one moment.
func wcShotsPeak(j *journey, updates int) int {
	peak := 0
	j.key("Digit3", 0)
	for i := 0; i < updates; i++ {
		careFeed(j)
		j.update(1)
		peak = max(peak, len(j.world().Shots))
	}
	j.key("Space", 0)
	return peak
}

// TestJourneyBurstFire (DEF-2): a drafted colonist with an assault rifle
// and raiders with the same fire bursts; several projectiles are in flight
// at once.
func TestJourneyBurstFire(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	ar := wcKind(j, "assault rifle")
	shooter := ordColonists(j)[0]
	j.g.Arm(entity.EntityID(shooter), ar)
	ordPick(j, 0)
	j.key("R", 0)
	if !j.pawn(shooter).Drafted {
		t.Fatal("R did not draft the colonist")
	}
	if j.pawn(shooter).Weapon != "assault rifle" {
		t.Fatalf("wielded %q, want assault rifle", j.pawn(shooter).Weapon)
	}
	j.g.RaidArmed(ar, ar, ar)
	j.update(1) // the raid alert pauses the game (pause on threat); unpause after it
	if peak := wcShotsPeak(j, 4000); peak < 2 {
		t.Fatalf("at most %d projectile(s) in flight at once, want a burst of several", peak)
	}
}

// TestJourneyGrenadeExplosion (DEF-2): raiders throw frag grenades at a
// huddle of colonists; the blast hurts several of them while a colonist
// far outside the radius is not touched.
func TestJourneyGrenadeExplosion(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{9, 4}, [2]int{8, 5}, [2]int{9, 5}, [2]int{1, 22}))
	frag := wcKind(j, "frag grenades")
	cols := ordColonists(j)
	far := cols[4]
	j.g.RaidArmed(frag, frag, frag)
	j.update(1) // the raid alert pauses the game (pause on threat); unpause after it
	j.key("Digit3", 0)
	hurt := 0
	for i := 0; i < 5000 && hurt < 2; i++ {
		careFeed(j)
		j.update(1)
		hurt = 0
		for _, id := range cols[:4] {
			if j.pawn(id).Health < 100 {
				hurt++
			}
		}
	}
	j.key("Space", 0)
	blasted := false
	for _, b := range j.world().Bursts {
		if blast := wcStr(j, b, "Blast"); blast == "bomb" && wcNum(j, b, "Radius") > 0 {
			blasted = true
		}
	}
	if !blasted {
		t.Fatalf("no bomb burst with a radius on WorldView.Bursts: %+v", j.world().Bursts)
	}
	if hurt < 2 {
		t.Fatalf("%d colonists of the huddle hurt, want the blast to hit at least 2", hurt)
	}
	if h := j.pawn(far).Health; h < 100 {
		t.Fatalf("the colonist far from the blast lost health (%d%%)", h)
	}
}

var wcDigit = regexp.MustCompile(`\d`)

// TestJourneyGearTabWeaponStats (DEF-2): the Gear tab shows the wielded
// weapon's damage and range and the worn armor's ratings.
func TestJourneyGearTabWeaponStats(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	ar := wcKind(j, "assault rifle")
	vest := wcKind(j, "flak vest")
	pawn := ordColonists(j)[0]
	j.g.Arm(entity.EntityID(pawn), ar)
	if _, err := j.g.SpawnStack(vest, 1, 9, 4); err != nil {
		t.Fatal(err)
	}
	wcPrioritize(j, pawn, 9, 4)
	careRun(j, "Digit3", 3000, func() bool {
		careFeed(j)
		for _, a := range j.pawn(pawn).Apparel {
			if a == "flak vest" {
				return true
			}
		}
		return false
	})
	ordPick(j, 0)
	j.clickID("inspector.gear")
	j.update(2)
	j.draw(2)
	p := j.pawn(pawn)
	// the Gear tab draws its rows, then the weapon and armor stat lines
	text := strings.ToLower(p.Weapon + "\n" + strings.Join(inspector.GearRows(p), "\n") +
		"\n" + strings.Join(inspector.GearStatLines(p), "\n"))
	for _, want := range []string{"damage", "range", "armor"} {
		if !strings.Contains(text, want) {
			t.Fatalf("gear tab lacks %q:\n%s", want, text)
		}
	}
	if st := wcField(j, p, "WeaponStats"); st.IsZero() {
		t.Fatal("PawnView.WeaponStats is zero for an assault rifle")
	}
	if ar := wcField(j, p, "ApparelArmor"); ar.Len() == 0 {
		t.Fatal("PawnView.ApparelArmor is empty for a worn flak vest")
	}
	if !wcDigit.MatchString(text) {
		t.Fatalf("gear tab shows no numbers:\n%s", text)
	}
}
