package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/application/game/haul"
	"rimworld-go/internal/application/game/mind"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/weather"
)

// PRD-2: made things roll a quality from the maker's skill, keep their
// material, lose hit points under the open sky, and worn apparel wears
// out and taints; a colonist in a good mood may be inspired.

func setSkill(g *Game, id entity.EntityID, sk skill.ID, lvl uint8) {
	(*g.sk.skills[id])[sk].Level = lvl
}

func TestCraftedClubHasQualityStuffAndHP(t *testing.T) {
	t.Parallel()
	g, ada, bench := clubGame(t)
	g.SpawnStack(item.Wood, 10, 6, 5)
	done := func() bool { return g.craft.Bills.Of(bench)[0].Done == 1 }
	runBillTicks(g, ada, 6000, done)
	if !done() {
		t.Fatal("no club made")
	}
	var club ItemView
	for _, iv := range g.World().Items {
		if iv.Kind == "club" {
			club = iv
		}
	}
	if club.Quality == "" || club.Stuff != "wood" || club.MaxHP != 65 || club.HP != 65 || club.Tainted {
		t.Fatalf("club view %+v", club)
	}
}

// TestQualityDistributionAtSkill10: on the game's craft stream a
// skill-10 crafter turns out mostly normal and good work, never
// legendary, and the stream is the game's own (same seed, same rolls).
func TestQualityDistributionAtSkill10(t *testing.T) {
	t.Parallel()
	roll := func() [7]int {
		g := New(Config{Seed: 3, Width: 10, Height: 10})
		ada, _ := g.SpawnNamedPawn(1, 1, "Ada")
		setSkill(g, ada, skill.Crafting, 10)
		var n [7]int
		for range 3000 {
			n[g.craft.Quality.Roll(ada, skill.Crafting)]++
		}
		return n
	}
	n := roll()
	if n[quality.Legendary] != 0 || n[quality.Normal] < 900 || n[quality.Good] < 1000 || n[quality.Excellent] < 200 || n[quality.Awful] > 10 {
		t.Fatalf("skill 10: %v", n)
	}
	if roll() != n {
		t.Fatal("quality rolls are not deterministic")
	}
}

func TestInspiredRollInGame(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 10, Height: 10})
	ada, _ := g.SpawnNamedPawn(1, 1, "Ada")
	setSkill(g, ada, skill.Crafting, 10)
	g.mind.Inspire.Start(ada, mind.Creativity)
	pv := pawnView(g, ada)
	if pv.Inspiration != "inspired creativity" || pv.InspirationTicks != mind.InspireDuration {
		t.Fatalf("pawn view %q %d", pv.Inspiration, pv.InspirationTicks)
	}
	if l := g.craft.Quality.Roll(ada, skill.Crafting); l < quality.Normal {
		t.Fatalf("inspired roll %v", l)
	}
	if _, ok := g.mind.Inspire.Of(ada); ok || pawnView(g, ada).Inspiration != "" {
		t.Fatal("the inspiration was not spent")
	}
}

func TestBuiltBedRollsQuality(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 12, Height: 12})
	ada, _ := g.SpawnNamedPawn(1, 1, "Ada")
	g.placeFrameStuff(construction.Bed, item.Wood, construction.South, 5, 5)
	fid := g.layerFrameAt(5, 5, construction.LayerObject)
	fr, _ := g.frames.Get(fid)
	fr.Wood, fr.WorkLeft = fr.Need, 1
	g.frames.Set(fid, fr)
	for i := 0; i < 8 && g.layerBldgAt(5, 5, construction.LayerObject) == 0; i++ {
		g.clock.Advance(1)
		g.doConstruct(ada, job.Record{Kind: job.Construct, Target: fid})
	}
	b, ok := g.bldgs.Get(g.layerBldgAt(5, 5, construction.LayerObject))
	if _, has := quality.FromStored(b.Quality); !ok || !has {
		t.Fatalf("bed %+v", b)
	}
	wall := buildNow(t, g, construction.Wall, item.Wood, construction.South, 9, 9)
	if w, _ := g.bldgs.Get(wall); w.Quality != 0 {
		t.Fatal("a wall has a quality")
	}
	for _, bv := range g.World().Buildings {
		if bv.Kind == "bed" && bv.Quality == "" {
			t.Fatal("bed view shows no quality")
		}
	}
}

// TestDeteriorationOutdoorsVsRoofed: a club lying outside loses hit
// points by the hour, five times as fast in rain; one under a roof keeps.
func TestDeteriorationOutdoorsVsRoofed(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 12, Height: 12})
	g.HoldWeather()
	out, _ := g.SpawnStack(item.Club, 1, 2, 2)
	in, _ := g.SpawnStack(item.Club, 1, 8, 8)
	if len(g.roofs) == 0 {
		g.roofs = make([]roof.Cell, 12*12)
	}
	g.roofs[8*12+8] = roof.Natural | roof.NaturalThick // overhead rock: needs no support
	day := func() {
		for range clock.HoursPerDay {
			g.clock.Advance(int(clock.TicksPerHour - uint64(g.clock.Now())%clock.TicksPerHour))
			g.phaseClimate()
		}
	}
	day()
	day()
	o, _ := g.stacks.Get(out)
	i, _ := g.stacks.Get(in)
	if o.Wear != 2*item.DeteriorationPerDay/1000 || i.Wear != 0 {
		t.Fatalf("outside lost %d, roofed %d", o.Wear, i.Wear)
	}
	g.weather.Kind = weather.Rain
	day()
	o, _ = g.stacks.Get(out)
	if o.Wear != 4+item.DeteriorationPerDay*item.RainFactor/1000 {
		t.Fatalf("after a rainy day: %d", o.Wear)
	}
}

func TestTatteredApparelThought(t *testing.T) {
	t.Parallel()
	g, id := dressGame(t, 12, 8, 100)
	w := g.apparel[id]
	w.On[item.Body] = craft.Piece{Kind: item.Duster, Stuff: item.Cloth, Wear: 120} // 80 of 200
	g.apparel[id] = w
	if !hasThought(g, id, mood.RattyApparel, -3) {
		t.Fatalf("ratty: %v", g.thoughts(id))
	}
	w.On[item.Body].Wear = 190 // 10 of 200
	g.apparel[id] = w
	if !hasThought(g, id, mood.TatteredApparel, -5) || hasThought(g, id, mood.RattyApparel, -3) {
		t.Fatalf("tattered: %v", g.thoughts(id))
	}
	if v := pawnView(g, id).Worn; len(v) != 1 || !v[0].Tattered || v[0].HP != 10 {
		t.Fatalf("worn view %+v", v)
	}
}

// TestTaintedApparelThought: a colonist's clothes fall tainted beside
// its body when it dies; nobody puts them on by choice, and one ordered
// to wear them is unhappy about it.
func TestTaintedApparelThought(t *testing.T) {
	t.Parallel()
	g, ada := dressGame(t, 14, 8, 100)
	bo, _ := g.SpawnNamedPawn(6, 2, "Bo")
	w := g.apparel[ada]
	w.On[item.Body] = craft.Piece{Kind: item.Parka, Quality: quality.Stored(quality.Good)}
	g.apparel[ada] = w
	g.die(ada, health.CauseInjury)
	if g.apparel[ada].Kind(item.Body) != 0 {
		t.Fatal("the dead still wear the parka")
	}
	var sid entity.EntityID
	for _, id := range g.itemIDs {
		if st, _ := g.stacks.Get(id); st.Kind == item.Parka {
			sid = id
		}
	}
	st, _ := g.stacks.Get(sid)
	if sid == 0 || !st.Tainted() || st.Of != ada || st.Quality != quality.Stored(quality.Good) {
		t.Fatalf("dropped parka %+v", st)
	}
	if o, ok := g.dressPlan(bo, true); ok && o.pick[item.Body].sid == sid {
		t.Fatal("a colonist chose a dead man's parka")
	}
	g.wear(bo, sid, st, true)
	if !hasThought(g, bo, mood.TaintedApparel, -5) {
		t.Fatalf("tainted: %v", g.thoughts(bo))
	}
	if v := pawnView(g, bo).Worn; len(v) != 1 || !v[0].Tainted || v[0].Quality != "good" {
		t.Fatalf("worn view %+v", v)
	}
}

// TestInspirationNoFlapInGame: an inspiration started in the game stays
// through the hourly checks whatever the mood, until it fades, and the
// calm keeps a new one off; the save keeps it.
func TestInspirationNoFlapInGame(t *testing.T) {
	t.Parallel()
	g, ada := dressGame(t, 10, 8, 150)
	g.mind.Inspire.Start(ada, mind.Creativity)
	for h := 0; h < 7*clock.HoursPerDay; h++ {
		g.clock.Advance(clock.TicksPerHour)
		g.mind.Inspire.Check(ada)
		if _, ok := g.mind.Inspire.Of(ada); !ok {
			t.Fatalf("the inspiration ended after %d hours", h)
		}
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "t"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := h.mind.Inspire.Of(ada); !ok {
		t.Fatal("the save lost the inspiration")
	}
	h.clock.Advance(clock.TicksPerDay)
	h.mind.Inspire.Check(ada)
	if _, ok := h.mind.Inspire.Of(ada); ok {
		t.Fatal("not faded after 8 days")
	}
	h.clock.Advance(mind.InspireCalm / 2)
	h.mind.Inspire.Check(ada)
	if _, ok := h.mind.Inspire.Of(ada); ok {
		t.Fatal("inspired again inside the calm")
	}
}

// TestWornPiecesSurviveSaveAndHaul: a worn piece's material, quality and
// wear survive the save, and a hauled made thing keeps them.
func TestWornPiecesSurviveSaveAndHaul(t *testing.T) {
	t.Parallel()
	g, ada := dressGame(t, 10, 8, 100)
	w := g.apparel[ada]
	w.On[item.Head] = craft.Piece{Kind: item.Tuque, Stuff: item.Cloth, Quality: 4, Wear: 9}
	w.SetForced(item.Head, true)
	g.apparel[ada] = w
	var buf bytes.Buffer
	if err := g.Save(&buf, "t"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if h.apparel[ada] != w {
		t.Fatalf("loaded %+v, want %+v", h.apparel[ada], w)
	}
	st := item.Stack{Kind: item.Club, Qty: 2, Quality: 3, Wear: 5, Stuff: item.Steel}
	if got := haul.SplitLoad(st, 1); got.Quality != 3 || got.Wear != 5 || got.Stuff != item.Steel {
		t.Fatalf("split %+v", got)
	}
}

// hasThought reports a thought of kind k worth v among id's.
func hasThought(g *Game, id entity.EntityID, k mood.Kind, v int) bool {
	for _, th := range g.thoughts(id) {
		if th.Kind == k && th.Value == v {
			return true
		}
	}
	return false
}
