package game

import (
	"bytes"
	"slices"
	"strings"
	"testing"

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
	d := recipe.DefOf(recipe.ButcherAnimal) // leather by species (SIM-019)
	if d.Extra != item.Leather || !d.BySpecies || animal.DefOf(animal.Deer).Leather <= 0 {
		t.Fatalf("butchering should give leather: %+v", d)
	}
	if recipe.DefOf(recipe.MakeParka).Input.Kind != item.Leather {
		t.Fatal("parkas are sewn from leather")
	}
}

func TestColonistDressesForWinterAndSurvives(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 8})
	jumpTo(g, 44, 6)
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SetNeeds(id, needs.Full, needs.Full)
	g.SpawnStack(item.Parka, 1, 6, 3)
	g.SpawnStack(item.Tuque, 1, 8, 3)
	g.Step(600)
	if got := g.Wearing(id); len(got) != 2 {
		t.Fatalf("wearing %v: should have put on the parka and tuque", got)
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

func TestParkaComesOffInSummer(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 8})
	jumpTo(g, 15, 9)
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SetNeeds(id, needs.Full, needs.Full)
	w := g.apparel[id]
	w.On[item.Body].Kind = item.Parka
	g.apparel[id] = w
	g.Step(200)
	if got := g.Wearing(id); len(got) != 0 {
		t.Fatalf("still wearing %v in midsummer", got)
	}
	if st, ok := stackOf(g, item.Parka); !ok || st.Qty != 1 {
		t.Fatal("the parka should be left on the ground")
	}
}

func TestWearOrderInMenu(t *testing.T) {
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
	if got := g.Wearing(id); len(got) != 1 || got[0] != "duster" {
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
	g, id := dressGame(t, 16, 8, -100) // -10 C: a parka alone still misses 8 C
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
	if !postponable(job.Equip) {
		t.Fatal("a failed Equip should be put off, so the next best is tried")
	}
	g, id := dressGame(t, 16, 8, -100)
	wool, _ := g.SpawnStack(item.WoolParka, 1, 6, 3)
	g.SpawnStack(item.Parka, 1, 9, 3)
	g.postpone(postKey{Pawn: id, Kind: job.Equip, Target: wool}, job.Blocked)
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
	// Cold-hardy: floor 2 C. Day 1..21 C: everyday clothes miss 1 C, a
	// tuque (floor -4 C) misses nothing: worth 1 C, so it goes on.
	g, id := dressGame(t, 16, 8, 110)
	withBio(g, id, look.Average, trait.ColdHardy)
	if c, _ := g.comfortShift(id); c != -60 {
		t.Fatalf("setup: cold-hardy shift %d", c)
	}
	g.SpawnStack(item.Tuque, 1, 6, 3)
	if dressRun(g, 400, func() bool { return wears(g, id, item.Head) != 0 }, id); wears(g, id, item.Head) != item.Tuque {
		t.Fatalf("cold-hardy colonist wearing %v: the tuque is worth 1 C to it", g.Wearing(id))
	}
	// Thin: floor 10 C. Day 7.5..27.5 C in a parka (ceiling 25 C) misses
	// 2.5 C; in everyday clothes (floor 10 C) also 2.5 C: no gain, kept.
	g, id = dressGame(t, 16, 8, 175)
	withBio(g, id, look.Thin)
	if c, _ := g.comfortShift(id); c != 20 {
		t.Fatalf("setup: thin shift %d", c)
	}
	w := g.apparel[id]
	w.On[item.Body].Kind = item.Parka
	g.apparel[id] = w
	dressRun(g, 600, nil, id)
	if wears(g, id, item.Body) != item.Parka {
		t.Fatal("thin colonist took off a parka that taking off does not help")
	}
}

// "Wear parka" in summer: the player's choice stays on, and the body slot
// is the player's until another garment is ordered for it.
func TestDressForcedSummerParkaStays(t *testing.T) {
	t.Parallel()
	g, id := dressGame(t, 16, 8, 250) // midsummer: a parka is too hot, a duster helps
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
	if pv := pawnView(g, id); len(pv.ApparelForced) != 1 || !pv.ApparelForced[0] {
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

// No flapping: the daily mean swings across the parka's threshold every
// half hour, with two equal parkas at hand. Clothes change at most once
// per cooldown, and never from one parka into the other.
func TestDressNoFlap(t *testing.T) {
	t.Parallel()
	const cold, warm = 20, 200 // 2 C: a parka saves 16 C; 20 C: it costs 3 C
	g, id := dressGame(t, 16, 8, cold)
	g.SpawnStack(item.Parka, 1, 6, 3)
	g.SpawnStack(item.Parka, 1, 7, 3)
	prev, last, changes := g.apparel[id], uint64(0), 0
	for i := 0; i < 12*clock.TicksPerHour; i++ {
		if i%(clock.TicksPerHour/2) == 0 {
			g.climate.YearMean = cold + warm - g.climate.YearMean
		}
		dressRun(g, 1, nil, id)
		if j, _ := g.jobs.Get(id); j.Kind == job.Equip && j.Active() && wears(g, id, item.Body) == item.Parka {
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
	if w := g.Wearing(id); len(w) != 0 {
		t.Fatalf("drafted colonist dressed in %v", w)
	}
}

// A piece to take off comes off without stopping the work.
func TestDressTakesOffWithoutStoppingWork(t *testing.T) {
	t.Parallel()
	g, id := dressGame(t, 16, 8, 250) // midsummer: the parka is too hot
	w := g.apparel[id]
	w.On[item.Body].Kind = item.Parka
	g.apparel[id] = w
	atBucket(g, id)
	rec := job.Record{Kind: job.Mine, State: job.Goto, TX: 12, TY: 5, DestX: 13, DestY: 5}
	g.jobs.Set(id, rec)
	g.dressInterrupt(id, rec)
	if j, _ := g.jobs.Get(id); j.Kind != job.Mine {
		t.Fatalf("now %v: taking a parka off should not stop the work", j.Kind)
	}
	if wears(g, id, item.Body) != 0 || !hasLogLine(g, id, "Took off the parka") {
		t.Fatalf("wearing %v, messages %+v", g.Wearing(id), g.messages)
	}
	if st, ok := stackOf(g, item.Parka); !ok || st.X != 2 || st.Y != 2 {
		t.Fatal("the parka should be dropped at its feet")
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
	g.stacks.Set(split, item.Stack{Kind: item.Parka, Qty: 1, X: 6, Y: 6})
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
	if got := h.Wearing(id); len(got) != 1 || got[0] != "parka" {
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
