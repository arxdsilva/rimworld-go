package game

import (
	"bytes"
	"rimworld-go/internal/application/game/postpone"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/internal/domain/world/terrain"
)

func TestButcheringYieldsLeather(t *testing.T) {
	t.Parallel()
	d := recipe.DefOf(recipe.ButcherAnimal) // leather by species (SIM-019)
	if d.Extra != item.Leather || !d.BySpecies || animal.DefOf(animal.Deer).Leather <= 0 {
		t.Fatalf("butchering should give leather: %+v", d)
	}
	if recipe.DefOf(recipe.MakeParka).Input.Kind != item.Leather {
		t.Fatal("parkas are sewn from leather")
	}
}

func TestColonistDressesForWinterAndSurvives(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 14, Height: 8})
	jumpTo(g, 44, 6)
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SetNeeds(id, needs.Full, needs.Full)
	g.SpawnStack(item.Parka, 1, 6, 3)
	g.SpawnStack(item.Tuque, 1, 8, 3)
	g.Step(600)
	// TEMP-4: a wiki parka (-32 C in leather) covers the day alone, so the
	// tuque is no longer needed; the outfit must leave nothing uncovered.
	mean := int(g.climate.DailyMean(g.clock.Now()))
	if wears(g, id, item.Body) != item.Parka || wears(g, id, item.SkinTorso) != item.TShirt ||
		craft.DayFit(shifted(g.apparel[id].Bands(), 0, 0), mean) != 0 {
		t.Fatalf("wearing %v: should have put on the parka over its T-shirt and cover the day", g.Wearing(id))
	}
	// a winter day and night outdoors with no shelter
	for i := 0; i < 24; i++ {
		g.Step(clock.TicksPerHour)
		g.SetNeeds(id, needs.Full, needs.Full)
	}
	if v, _ := g.vital.Get(id); v.Status != health.Alive {
		t.Fatalf("dressed colonist %v (%v): a parka should get it through", v.Status, v.Cause)
	}
}

// TEMP-4: a parka is no longer hot to wear (wiki: heat factor 0), so it
// comes off in summer for a duster, which shades (heat 85 %), not on its
// own.
func TestParkaComesOffInSummer(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 14, Height: 8})
	jumpTo(g, 15, 9)
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SetNeeds(id, needs.Full, needs.Full)
	w := g.apparel[id]
	w.On[item.Body].Kind = item.Parka
	g.apparel[id] = w
	g.SpawnStack(item.Duster, 1, 5, 3)
	g.Step(600)
	if got := wears(g, id, item.Body); got != item.Duster {
		t.Fatalf("wearing %v in midsummer: want the duster", g.Wearing(id))
	}
	if st, ok := stackOf(g, item.Parka); !ok || st.Qty != 1 {
		t.Fatal("the parka should be left on the ground")
	}
}

func TestWearOrderInMenu(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 14, Height: 8})
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SpawnStack(item.Duster, 1, 6, 3)
	found := false
	for _, o := range g.OrdersAt(id, 6, 3) {
		if o.Label == "Wear duster" && !o.Disabled {
			found = true
			g.Enqueue(o.Cmd)
		}
	}
	if !found {
		t.Fatal("order menu should offer Wear duster")
	}
	g.Step(300)
	if got := g.Wearing(id); len(got) != 3 || got[0] != "duster" { // over its T-shirt and pants
		t.Fatalf("wearing %v after the order", got)
	}
	_ = command.Prioritize
}

// PLAY-017: colonists dress at once in the best outfit they are
// comfortable in, keep the player's choice and never flap.

// dressGame is a bare w x h map with one fed colonist, Ada, at (2, 2) and
// a flat climate: every day's mean is mean (tenths of a degree C), with
// no day/night swing and the weather held.
func dressGame(t *testing.T, w, h, mean int) (*Game, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 1, Width: w, Height: h})
	g.climate = temperature.Climate{YearMean: mean}
	g.HoldWeather()
	id, err := g.SpawnNamedPawn(2, 2, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	g.SetNeeds(id, needs.Full, needs.Full)
	return g, id
}

// dressRun runs up to n ticks one by one, keeping pawns fed and rested,
// until done (may be nil) reports true; it returns the ticks run.
func dressRun(g *Game, n int, done func() bool, pawns ...entity.EntityID) int {
	for i := 1; i <= n; i++ {
		for _, id := range pawns {
			g.SetNeeds(id, needs.Full, needs.Full)
		}
		g.Step(1)
		if done != nil && done() {
			return i
		}
	}
	return n
}

func wears(g *Game, id entity.EntityID, s item.Slot) item.Kind { return g.apparel[id].Kind(s) }

// A colonist busy mining on a cold day drops the work for a parka that
// turns up, within one 60-tick bucket.
func TestDressMidJobColonistFetchesParka(t *testing.T) {
	t.Parallel()
	g, id := dressGame(t, 24, 10, 20) // 2 C: everyday clothes miss 16 C of the day
	onlyWork(g, id, work.Mine)
	for x := 14; x <= 18; x++ {
		g.setTerrain(x, 5, terrain.Rock)
	}
	applyNow(g, command.Command{Kind: command.DesignateMine, X: 14, Y: 5, X2: 18, Y2: 5})
	mining := func() bool { j, _ := g.jobs.Get(id); return j.Kind == job.Mine && j.Active() }
	if dressRun(g, 600, mining, id); !mining() {
		t.Fatal("setup: the colonist never started mining")
	}
	sid, _ := g.SpawnStack(item.Parka, 1, 4, 2)
	fetching := func() bool { j, _ := g.jobs.Get(id); return j.Kind == job.Equip && j.Target == sid }
	if dressRun(g, dressEvery, fetching, id); !fetching() {
		j, _ := g.jobs.Get(id)
		t.Fatalf("still on %v %v after a bucket: should be walking to the parka", j.Kind, j.State)
	}
	if got := pawnView(g, id).ChangingInto; got != "parka" {
		t.Fatalf("view: changing into %q", got)
	}
	if dressRun(g, 600, func() bool { return wears(g, id, item.Body) == item.Parka }, id); wears(g, id, item.Body) != item.Parka {
		t.Fatal("the parka never went on")
	}
	if !hasLogLine(g, id, "Put on a parka") {
		t.Fatalf("no log line for the parka: %+v", g.messages)
	}
	// dressed, it goes back to the rock
	if dressRun(g, 600, mining, id); !mining() {
		t.Fatal("did not go back to mining")
	}
}

// The whole outfit goes on in one go: the tuque follows the parka at once
// (the cooldown holds a changed slot, not the next one).
func TestDressWholeOutfitInOneGo(t *testing.T) {
	t.Parallel()
	g, id := dressGame(t, 16, 8, -170) // -17 C: a parka over a T-shirt and pants (floor -23.5 C) still misses 3.5 C
	g.SpawnStack(item.Parka, 1, 6, 3)
	g.SpawnStack(item.Tuque, 1, 9, 3)
	both := func() bool { return wears(g, id, item.Body) == item.Parka && wears(g, id, item.Head) == item.Tuque }
	if n := dressRun(g, 400, both, id); !both() {
		t.Fatalf("wearing %v after %d ticks: want parka and tuque", g.Wearing(id), n)
	}
	if !hasLogLine(g, id, "Put on a tuque") {
		t.Fatalf("no log line for the tuque: %+v", g.messages)
	}
}

// The best garment lies outside the colonist's allowed area: it settles
// for the next best inside and never sets off for the other.
func TestDressOutOfAreaFallsBackToNextBest(t *testing.T) {
	t.Parallel()
	g, id := areaGame(t) // area: x 0..9
	g.climate = temperature.Climate{YearMean: -100}
	g.HoldWeather()
	wool, _ := g.SpawnStack(item.WoolParka, 1, 20, 3) // warmer, outside
	g.SpawnStack(item.Parka, 1, 8, 3)
	done := func() bool {
		if j, _ := g.jobs.Get(id); j.Kind == job.Equip && j.Target == wool {
			t.Fatal("set off for the wool parka outside its area")
		}
		return wears(g, id, item.Body) == item.Parka
	}
	if dressRun(g, 600, done, id); wears(g, id, item.Body) != item.Parka {
		t.Fatalf("wearing %v: want the parka inside the area", g.Wearing(id))
	}
}

// A garment whose fetch was put off (a blocked walk) is skipped for the
// next best, and Equip is postponable work.
func TestDressSkipsPostponedGarment(t *testing.T) {
	t.Parallel()
	if !postpone.Postponable(job.Equip) {
		t.Fatal("a failed Equip should be put off, so the next best is tried")
	}
	g, id := dressGame(t, 16, 8, -100)
	wool, _ := g.SpawnStack(item.WoolParka, 1, 6, 3)
	g.SpawnStack(item.Parka, 1, 9, 3)
	g.postpone(postpone.Key{Pawn: id, Kind: job.Equip, Target: wool}, job.Blocked)
	if l := g.postponedLines(id); len(l) != 1 || !strings.HasPrefix(l[0], "wear wool parka: blocked") {
		t.Fatalf("put-off lines %q", l)
	}
	if dressRun(g, 400, func() bool { return wears(g, id, item.Body) != 0 }, id); wears(g, id, item.Body) != item.Parka {
		t.Fatalf("wearing %v: the put-off wool parka should be skipped for the parka", g.Wearing(id))
	}
}

// withBio gives colonist id a build and traits (stats rebuilt).
func withBio(g *Game, id entity.EntityID, b look.Build, traits ...trait.ID) {
	p := legacyBio(id, g.name(id))
	p.Look.Build = b
	p.Traits = traits
	g.giveBio(id, p)
}

// The comfort shift (build, traits) counts the same for what a colonist
// wears and for what it might wear: a cold-hardy colonist still takes a
// tuque worth 1 C, and a thin one keeps a parka that taking off would not
// help.
func TestDressComfortShiftCountsOnBothSides(t *testing.T) {
	t.Parallel()
	// Cold-hardy: floor 2.5 C in a T-shirt and pants (TEMP-4: base 16 C
	// less 3.9 and 3.6, less 6). Day 1..21 C: they miss 1.5 C, a tuque
	// (floor -5.5 C) misses nothing: worth 1.5 C, so it goes on.
	g, id := dressGame(t, 16, 8, 110)
	withBio(g, id, look.Average, trait.ColdHardy)
	if c, _ := g.comfortShift(id); c != -60 {
		t.Fatalf("setup: cold-hardy shift %d", c)
	}
	g.SpawnStack(item.Tuque, 1, 6, 3)
	if dressRun(g, 400, func() bool { return wears(g, id, item.Head) != 0 }, id); wears(g, id, item.Head) != item.Tuque {
		t.Fatalf("cold-hardy colonist wearing %v: the tuque is worth 1 C to it", g.Wearing(id))
	}
	// TEMP-4: no garment is hot to wear any more (wiki), so the shift is
	// proved on the cold side twice over. Thin: floor 10.5 C in a T-shirt
	// and pants. Day 7.8..27.8 C: they miss 2.7 C, a tuque nothing: worth
	// 2.7 C, so it goes on; an average colonist (floor 8.5 C) misses only
	// 0.7 C, under the 1 C gain, and leaves it.
	g, id = dressGame(t, 16, 8, 178)
	withBio(g, id, look.Thin)
	if c, _ := g.comfortShift(id); c != 20 {
		t.Fatalf("setup: thin shift %d", c)
	}
	g.SpawnStack(item.Tuque, 1, 6, 3)
	if dressRun(g, 400, func() bool { return wears(g, id, item.Head) != 0 }, id); wears(g, id, item.Head) != item.Tuque {
		t.Fatalf("thin colonist wearing %v: the tuque is worth 2.7 C to it", g.Wearing(id))
	}
	g, id = dressGame(t, 16, 8, 178)
	g.SpawnStack(item.Tuque, 1, 6, 3)
	if dressRun(g, 600, nil, id); wears(g, id, item.Head) != 0 {
		t.Fatal("an average colonist took a tuque worth 0.7 C")
	}
}

// "Wear parka" in summer: the player's choice stays on, and the body slot
// is the player's until another garment is ordered for it.
func TestDressForcedSummerParkaStays(t *testing.T) {
	t.Parallel()
	g, id := dressGame(t, 16, 8, 250) // midsummer: a parka does not help (wiki heat 0), a duster does
	g.SpawnStack(item.Parka, 1, 6, 3)
	duster, _ := g.SpawnStack(item.Duster, 1, 4, 5)
	wear := func(x, y int, label string) {
		t.Helper()
		for _, o := range g.OrdersAt(id, x, y) {
			if o.Label == label && !o.Disabled {
				g.Enqueue(o.Cmd)
				return
			}
		}
		t.Fatalf("no %q order", label)
	}
	wear(6, 3, "Wear parka")
	if dressRun(g, 300, func() bool { return wears(g, id, item.Body) == item.Parka }, id); wears(g, id, item.Body) != item.Parka {
		t.Fatal("the ordered parka never went on")
	}
	if !g.apparel[id].IsForced(item.Body) {
		t.Fatal("an ordered garment marks its slot as the player's choice")
	}
	if pv := pawnView(g, id); len(pv.ApparelForced) != 3 || !pv.ApparelForced[0] || pv.ApparelForced[1] { // the parka, not the T-shirt
		t.Fatalf("view: forced %v", pv.ApparelForced)
	}
	dressRun(g, 3*clock.TicksPerHour, func() bool {
		if j, _ := g.jobs.Get(id); j.Kind == job.Equip && j.Target == duster {
			t.Fatal("went for the duster over the player's parka")
		}
		return false
	}, id)
	if wears(g, id, item.Body) != item.Parka {
		t.Fatalf("the player's parka came off: wearing %v", g.Wearing(id))
	}
	wear(4, 5, "Wear duster")
	if dressRun(g, 300, func() bool { return wears(g, id, item.Body) == item.Duster }, id); wears(g, id, item.Body) != item.Duster {
		t.Fatal("a new order for the slot should replace the parka")
	}
	if !g.apparel[id].IsForced(item.Body) {
		t.Fatal("the new choice is the player's too")
	}
}

// No flapping: the daily mean swings between a parka day and a duster day
// every half hour, with two equal parkas and a duster at hand. Clothes
// change at most once per cooldown, and never from one parka into the
// other. TEMP-4: a parka no longer costs heat (wiki), so the warm side is
// the duster's: over a T-shirt and pants (8.5..29.2 C), on a 2 C day
// (-8..12 C) the duster misses 6.9 C and the parka nothing; on a 25 C day
// (15..35 C) the parka misses 5.8 C and the duster nothing.
func TestDressNoFlap(t *testing.T) {
	t.Parallel()
	const cold, warm = 20, 250
	g, id := dressGame(t, 16, 8, cold)
	g.SpawnStack(item.Parka, 1, 6, 3)
	g.SpawnStack(item.Parka, 1, 7, 3)
	g.SpawnStack(item.Duster, 1, 8, 3)
	prev, last, changes := g.apparel[id], uint64(0), 0
	for i := 0; i < 12*clock.TicksPerHour; i++ {
		if i%(clock.TicksPerHour/2) == 0 {
			g.climate.YearMean = cold + warm - g.climate.YearMean
		}
		dressRun(g, 1, nil, id)
		if j, _ := g.jobs.Get(id); j.Kind == job.Equip && j.Active() && wears(g, id, item.Body) == item.Parka &&
			g.stacks.Ref(j.Target) != nil && g.stacks.Ref(j.Target).Kind == item.Parka {
			t.Fatalf("tick %d: changing from one parka into another", i)
		}
		if w := g.apparel[id]; w != prev {
			now := uint64(g.clock.Now())
			if changes > 0 && now-last < dressCool {
				t.Fatalf("tick %d: clothes changed again %d ticks after the last change", i, now-last)
			}
			prev, last = w, now
			changes++
		}
	}
	if changes < 3 {
		t.Fatalf("%d changes in 12 hours: the swings should still be followed once each cooldown", changes)
	}
}

// atBucket advances the clock to colonist id's next dress bucket.
func atBucket(g *Game, id entity.EntityID) {
	for (uint64(g.clock.Now())+uint64(id))%dressEvery != 0 {
		g.clock.Advance(1)
	}
}

// Needs, carried loads, the player's orders and drafted colonists are
// never interrupted for clothes; plain work is.
func TestDressNeverInterruptsNeedsOrDrafts(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		rec  job.Record
		stop bool // the job gives way to the parka
	}{
		{"mining", job.Record{Kind: job.Mine, State: job.Goto, TX: 12, TY: 5, DestX: 13, DestY: 5}, true},
		{"eating", job.Record{Kind: job.Eat, State: job.Do, TX: 2, TY: 2}, false},
		{"sleeping", job.Record{Kind: job.Sleep, State: job.Do, TX: 2, TY: 2}, false},
		{"warming", job.Record{Kind: job.Warm, State: job.Goto, TX: 12, TY: 5}, false},
		{"ordered", job.Record{Kind: job.Mine, State: job.Goto, TX: 12, TY: 5, DestX: 13, DestY: 5, Forced: true}, false},
		{"carrying", job.Record{Kind: job.Haul, State: job.Goto, TX: 12, TY: 5, Carry: 999}, false},
	} {
		g, id := dressGame(t, 16, 8, 20)
		g.SpawnStack(item.Parka, 1, 4, 2)
		atBucket(g, id)
		g.jobs.Set(id, c.rec)
		g.dressInterrupt(id, c.rec)
		j, _ := g.jobs.Get(id)
		if stopped := j.Kind == job.Equip; stopped != c.stop {
			t.Errorf("%s: now %v (gave way: %v, want %v)", c.name, j.Kind, stopped, c.stop)
		}
	}
	// a drafted colonist in the cold stays as it is
	g, id := dressGame(t, 16, 8, 20)
	g.SpawnStack(item.Parka, 1, 4, 2)
	g.setDraft(id, true)
	dressRun(g, 600, nil, id)
	if w := g.Wearing(id); len(w) != 2 || wears(g, id, item.Body) != 0 { // its T-shirt and pants only
		t.Fatalf("drafted colonist dressed in %v", w)
	}
}

// A piece the plan displaces comes off at once, before the walk, and is
// dropped at the colonist's feet. TEMP-4: no garment is hot to wear any
// more (wiki heat factors are never negative), so nothing comes off for
// the weather alone (the take-off-only branch of dressInterrupt is kept
// for safety); what comes off is what a chosen garment displaces. On a
// 16 C day (6..26 C) a T-shirt and pants (floor 8.5 C) miss 2.5 C and
// tribalwear (floor 6.1 C) 0.1 C: the pants it covers come off first.
func TestDressTakesOffWithoutStoppingWork(t *testing.T) {
	t.Parallel()
	g, id := dressGame(t, 16, 8, 160)
	tribal, _ := g.SpawnStack(item.Tribalwear, 1, 9, 3)
	if o, ok := g.dressPlan(id, true); !ok || o.Pick[item.SkinTorso].Stack != tribal || o.Pick[item.SkinLegs].Kind != 0 {
		t.Fatalf("plan %+v (worth %v): tribalwear, pants off", o.Pick, ok)
	}
	atBucket(g, id)
	g.dress(id)
	if j, _ := g.jobs.Get(id); j.Kind != job.Equip || j.Target != tribal {
		t.Fatalf("now %v: should be walking to the tribalwear", j.Kind)
	}
	if wears(g, id, item.SkinLegs) != 0 || !hasLogLine(g, id, "Took off the pants") {
		t.Fatalf("wearing %v, log %+v", g.Wearing(id), g.logLines(id))
	}
	if st, ok := stackOf(g, item.Pants); !ok || st.X != 2 || st.Y != 2 {
		t.Fatal("the pants should be dropped at its feet")
	}
	if dressRun(g, 400, func() bool { return wears(g, id, item.SkinTorso) == item.Tribalwear }, id); wears(g, id, item.SkinTorso) != item.Tribalwear {
		t.Fatalf("wearing %v: the tribalwear never went on", g.Wearing(id))
	}
	if wears(g, id, item.SkinLegs) != 0 || g.apparel[id].Naked() {
		t.Fatalf("tribalwear covers torso and legs alone: %v", g.Wearing(id))
	}
}

// The apparel index follows spawned stacks, stacks split off without
// SpawnStack (a hauler's load) and stacks that are gone.
func TestApparelIndex(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 8})
	a, _ := g.SpawnStack(item.Parka, 2, 5, 5)
	g.SpawnStack(item.Wood, 5, 6, 5)
	split := g.entities.Next() // haul-style split
	g.itemIDs = append(g.itemIDs, split)
	g.putStack(split, item.Stack{Kind: item.Parka, Qty: 1, X: 6, Y: 6})
	if got := g.apparelStock(true); !slices.Equal(got, []entity.EntityID{a, split}) {
		t.Fatalf("index %v, want [%d %d]", got, a, split)
	}
	g.deleteItem(a)
	if got := g.apparelStock(true); !slices.Equal(got, []entity.EntityID{split}) {
		t.Fatalf("index %v after delete, want [%d]", got, split)
	}
	if k := g.apparelKinds(); !slices.Equal(k, []item.Kind{item.Parka}) {
		t.Fatalf("kinds %v", k)
	}
}

// The player's choice and the cooldown are saved; the cooldown is not
// shown as put-off work.
func TestDressStateSurvivesSave(t *testing.T) {
	t.Parallel()
	g, id := dressGame(t, 16, 8, 250)
	g.SpawnStack(item.Parka, 1, 6, 3)
	for _, o := range g.OrdersAt(id, 6, 3) {
		if o.Label == "Wear parka" {
			g.Enqueue(o.Cmd)
		}
	}
	dressRun(g, 300, func() bool { return wears(g, id, item.Body) == item.Parka }, id)
	if !g.dressCooling(id, item.Body) {
		t.Fatal("a change starts the slot's cooldown")
	}
	if l := g.postponedLines(id); len(l) != 0 {
		t.Fatalf("the cooldown shows as put-off work: %v", l)
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "dress"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !h.apparel[id].IsForced(item.Body) || h.apparel[id].Kind(item.Body) != item.Parka || !h.dressCooling(id, item.Body) {
		t.Fatalf("after load: worn %v, cooling %v", h.apparel[id], h.dressCooling(id, item.Body))
	}
	if got := h.Wearing(id); len(got) != 3 || got[0] != "parka" { // and its T-shirt and pants
		t.Fatalf("Wearing %v: the forced bits are not a garment", got)
	}
}

// hasLogLine: colonist id's own log has a line containing sub.
func hasLogLine(g *Game, id entity.EntityID, sub string) bool {
	for _, l := range g.logLines(id) {
		if strings.Contains(l.Text, sub) {
			return true
		}
	}
	return false
}
