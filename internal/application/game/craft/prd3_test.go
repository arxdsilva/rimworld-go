package craft

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/entity"
)

// TestFacilities: 6 % per tool cabinet within 6 tiles, at most two, the
// names listed in building order and cached through StructVer; spots
// link nothing.
func TestFacilities(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f).Facilities
	f.build(1, construction.Workbench, 10, 5)
	if got := c.Bonus(1); got != 0 || c.Linked(1) != nil {
		t.Fatalf("no cabinets: %d %v", got, c.Linked(1))
	}
	f.build(2, construction.ToolCabinet, 16, 2) // 6 tiles away: counts
	if got := c.Bonus(1); got != 6 || !slices.Equal(c.Linked(1), []string{"tool cabinet"}) {
		t.Fatalf("one cabinet: %d %v", got, c.Linked(1))
	}
	f.build(3, construction.ToolCabinet, 8, 8)
	f.build(4, construction.ToolCabinet, 11, 8)
	if got := c.Bonus(1); got != 12 || len(c.Linked(1)) != 2 {
		t.Fatalf("three cabinets: %d %v, want the cap of 12", got, c.Linked(1))
	}
	f.build(5, construction.ToolCabinet, 17, 5) // 7 tiles away
	f.remove(3)
	f.remove(4)
	if got := c.Bonus(1); got != 6 {
		t.Fatalf("after removing two, one far: %d", got)
	}
	if got := c.Bonus(99); got != 0 {
		t.Fatalf("no bench: %d", got)
	}
	f.build(6, construction.CraftingSpot, 15, 3)
	if got := c.Bonus(6); got != 0 {
		t.Fatalf("a spot links nothing: %d", got)
	}
}

// TestBenchSpeed: the spots' own factors, facilities and an electric
// twin's bonus, then light (0.8 in the dark, a straight ramp to 30 %),
// the room's cleanliness and outdoors 0.8; kept speedEvery ticks.
func TestBenchSpeed(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := New(f).Stations
	f.build(1, construction.Workbench, 4, 4)
	f.build(2, construction.CraftingSpot, 10, 4)
	f.build(3, construction.ButcherSpot, 14, 4)
	f.build(4, construction.ElectricSmithy, 18, 4)
	f.build(5, construction.MachiningTable, 22, 4)
	f.build(6, construction.ToolCabinet, 5, 6)
	want := map[entity.EntityID]int{1: 1060, 2: 500, 3: 700, 4: 1400, 5: 1000}
	for id, w := range want {
		if got := s.Speed(id); got != w {
			t.Errorf("bench %d: %d, want %d", id, got, w)
		}
	}
	if s.Speed(6) != 0 || s.Speed(99) != 0 {
		t.Fatal("not a bench: 0")
	}
	f.gloom = 1000 // full dark: kept until speedEvery passes
	if got := s.Speed(5); got != 1000 {
		t.Fatalf("cached: %d", got)
	}
	f.now += speedEvery
	if got := s.Speed(5); got != 800 {
		t.Fatalf("dark: %d", got)
	}
	f.gloom, f.ver = 850, f.ver+1 // 15 % light, and a building change drops the cache
	if got := s.Speed(5); got != 900 {
		t.Fatalf("half lit: %d", got)
	}
	f.gloom = 0
	for _, tc := range []struct{ clean, want int }{{-200, 850}, {-50, 900}, {-10, 950}, {0, 1000}, {60, 1000}} {
		f.clean, f.ver = tc.clean, f.ver+1
		if got := s.Speed(5); got != tc.want {
			t.Errorf("cleanliness %d: %d, want %d", tc.clean, got, tc.want)
		}
	}
	f.clean, f.outside, f.ver = 0, true, f.ver+1
	if got := s.Speed(2); got != 400 {
		t.Fatalf("crafting spot outdoors: %d", got)
	}
	if ElectricPct(construction.MachiningTable) != 0 || ElectricPct(construction.ElectricStove) != ElectricSpeedPct ||
		!ElectricBench(construction.ElectricSmelter) || ElectricBench(construction.FueledSmithy) {
		t.Fatal("electric benches")
	}
}

// TestStationView: a bench reports its links and speed, a sculpture its
// title, anything else keeps its epitaph.
func TestStationView(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	f.build(1, construction.Workbench, 4, 4)
	f.build(2, construction.ToolCabinet, 5, 6)
	f.build(3, construction.SmallSculpture, 9, 9)
	f.build(4, construction.Grave, 12, 12)
	c.Art.Import(map[entity.EntityID]Artwork{3: {Seed: 42, Maker: "Ada"}})
	if l, sp, e := c.Stations.View(1, f.bldgs[1], ""); len(l) != 1 || sp != 1060 || e != "" {
		t.Fatalf("bench view %v %d %q", l, sp, e)
	}
	if l, sp, e := c.Stations.View(3, f.bldgs[3], ""); l != nil || sp != 0 || e != ArtTitle(42) {
		t.Fatalf("sculpture view %v %d %q", l, sp, e)
	}
	if _, _, e := c.Stations.View(4, f.bldgs[4], "Rest well"); e != "Rest well" {
		t.Fatalf("grave keeps its epitaph: %q", e)
	}
}

// TestForgeFuel: a wood-fired forge works only with fuel, burns it while
// worked and puts the job off when it burns out; the bill reads "no fuel".
func TestForgeFuel(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	f.pawns = []entity.EntityID{pawn}
	f.build(1, construction.FueledSmithy, 4, 4)
	f.put(50, item.Stack{Kind: item.Steel, Qty: 20, X: 1, Y: 1})
	c.Bills.Set(1, []recipe.Bill{{Recipe: recipe.MakeMace, Mode: recipe.DoCount, Target: 2}})
	if c.Shop.Assign(pawn, crafting) {
		t.Fatal("forged with no fuel")
	}
	if _, st := c.Bills.Line(1, 0); st != BillNoFuel {
		t.Fatalf("status %q", st)
	}
	f.fuel[1] = 700
	j := runBill(t, f, c, crafting, 100)
	if j.State != job.Complete || f.Stock(item.Mace) != 1 || f.burnt != recipe.DefOf(recipe.MakeMace).Work {
		t.Fatalf("job %v maces %d burnt %d", j.State, f.Stock(item.Mace), f.burnt)
	}
	f.jobs = map[entity.EntityID]job.Record{}
	f.claims = map[entity.EntityID]entity.EntityID{}
	j = runBill(t, f, c, crafting, 1) // fetched; 100 fuel left, 600 work to do
	j = runBill2(f, c, 100)
	if j.State != job.Fail || j.Fail != job.Blocked || f.fuel[1] != 0 {
		t.Fatalf("burnt out: %+v fuel %d", j, f.fuel[1])
	}
}

// runBill2 keeps working pawn's current job for at most n ticks.
func runBill2(f *fakeHost, c *Craft, n int) job.Record {
	for range n {
		j := f.jobs[pawn]
		if j.State == job.Complete || j.State == job.Fail {
			return j
		}
		c.Shop.Do(pawn, j)
	}
	return f.jobs[pawn]
}

// TestSculpture: an art bench bill sets a sculpture down beside the
// bench in the material carried, with a rolled quality and an original
// title and description, and says so; with no room it waits.
func TestSculpture(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	f.pawns = []entity.EntityID{pawn}
	f.names[pawn] = "Ada"
	f.build(1, construction.ArtBench, 10, 10)
	f.put(50, item.Stack{Kind: item.MarbleBlocks, Qty: 20, X: 1, Y: 1})
	c.Bills.Set(1, []recipe.Bill{{Recipe: recipe.SculptSmall, Mode: recipe.UntilHave, Target: 1}})
	j := runBill(t, f, c, crafting, 100)
	if j.State != job.Complete {
		t.Fatalf("job %+v", j)
	}
	var sid entity.EntityID
	for _, id := range f.ids {
		if f.bldgs[id].Kind == construction.SmallSculpture {
			sid = id
		}
	}
	b := f.bldgs[sid]
	if sid == 0 || b.Stuff != item.MarbleBlocks || b.Quality == 0 || b.X == 10 && b.Y == 11 {
		t.Fatalf("sculpture %d %+v", sid, b)
	}
	if f.Stock(item.MarbleBlocks) != 20-construction.DefOf(construction.SmallSculpture).Wood {
		t.Fatalf("marble left %d", f.Stock(item.MarbleBlocks))
	}
	p, ok := c.Art.Artwork(sid)
	desc := c.Art.Describe(sid, b)
	if !ok || p.Maker != "Ada" || !strings.Contains(desc, "marble small sculpture") || !strings.HasSuffix(desc, "Carved by Ada.") {
		t.Fatalf("art %+v %q", p, desc)
	}
	if len(f.msgs) == 0 || !strings.Contains(f.msgs[len(f.msgs)-1], "Ada finished \""+c.Art.Title(sid, b)+"\"") {
		t.Fatalf("messages %v", f.msgs)
	}
	if c.Shop.Assign(pawn, crafting) {
		t.Fatal("until you have one: the standing piece counts")
	}
	if c.Art.Title(1, f.bldgs[1]) != "" || c.Art.Describe(1, f.bldgs[1]) != "" {
		t.Fatal("a bench has no art")
	}
	saved := c.Art.Export()
	f.remove(sid)
	if len(saved) != 1 || len(c.Art.Export()) != 0 {
		t.Fatalf("export keeps standing pieces only: %v", saved)
	}
	// no room: every cell around the spot blocked
	f.blocked = map[[2]int]bool{}
	for y := 5; y <= 17; y++ {
		for x := 4; x <= 16; x++ {
			f.blocked[[2]int{x, y}] = true
		}
	}
	c.Bills.Set(1, []recipe.Bill{{Recipe: recipe.SculptLarge, Mode: recipe.DoCount, Target: 1}})
	f.jobs = map[entity.EntityID]job.Record{}
	f.claims = map[entity.EntityID]entity.EntityID{}
	if j := runBill(t, f, c, crafting, 100); j.State != job.Fail || j.Fail != job.Blocked || c.Bills.Of(1)[0].Done != 0 {
		t.Fatalf("no room: %+v", j)
	}
}

// TestSmeltAndButcherSpot: the smelter gives a quarter of a gun's steel
// back; a butcher spot gets the butchering bill by default; actions
// name bench work.
func TestSmeltAndButcherSpot(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	f.pawns = []entity.EntityID{pawn}
	f.build(1, construction.ElectricSmelter, 4, 4)
	f.put(50, item.Stack{Kind: item.Minigun, Qty: 1, X: 1, Y: 1})
	c.Bills.Set(1, []recipe.Bill{{Recipe: recipe.SmeltWeapon, Mode: recipe.DoCount, Target: 1}})
	if j := runBill(t, f, c, crafting, 100); j.State != job.Complete || f.Stock(item.Steel) != 4 || f.Stock(item.Minigun) != 0 {
		t.Fatalf("smelt %v steel %d", j.State, f.Stock(item.Steel))
	}
	c.Bills.Defaults(2, construction.ButcherSpot)
	if b := c.Bills.Of(2); len(b) != 1 || b[0].Recipe != recipe.ButcherAnimal {
		t.Fatalf("butcher spot bills %v", b)
	}
	for k, want := range map[construction.Kind]string{construction.ButcherSpot: ActionButcher, construction.ElectricStove: ActionCook,
		construction.ElectricTailoringBench: ActionTailor, construction.Pyre: ActionCremate, construction.FueledSmithy: ActionCraft} {
		if got := Action(k); got != want {
			t.Errorf("%s: %q, want %q", k, got, want)
		}
	}
}

// TestRecipeViewsBenches: every bench that runs a recipe is listed, the
// electric twins included, and a sculpture's output is its kind.
func TestRecipeViewsBenches(t *testing.T) {
	t.Parallel()
	for _, r := range Recipes() {
		switch r.Name {
		case "make_mace":
			if !r.RunsAt("wood-fired forge") || !r.RunsAt("electric forge") || r.RunsAt("workbench") {
				t.Fatalf("mace benches %v", r.Benches)
			}
		case "club":
			if !r.RunsAt("workbench") || !r.RunsAt("crafting spot") || !r.RunsAt("electric forge") {
				t.Fatalf("club benches %v", r.Benches)
			}
		case "simple_meal":
			if !r.RunsAt("stove") || !r.RunsAt("electric stove") {
				t.Fatalf("meal benches %v", r.Benches)
			}
		case "sculpt_grand":
			if r.Output != "grand sculpture" || !slices.Equal(r.Benches, []string{"sculpting bench"}) {
				t.Fatalf("grand %+v", r)
			}
		}
	}
}

// TestArtGrammar: titles and descriptions are deterministic per seed,
// vary across seeds and read their material and quality.
func TestArtGrammar(t *testing.T) {
	t.Parallel()
	titles := map[string]bool{}
	for seed := uint32(0); seed < 200; seed++ {
		if ArtTitle(seed) != ArtTitle(seed) || ArtTitle(seed) == "" {
			t.Fatal("not deterministic")
		}
		titles[ArtTitle(seed)] = true
	}
	if len(titles) < 100 {
		t.Fatalf("only %d titles in 200 seeds", len(titles))
	}
	d := ArtDescription(9, construction.GrandSculpture, item.Wood, quality.Excellent, "")
	if !strings.HasPrefix(d, "An excellent wooden grand sculpture of ") || strings.Contains(d, "Carved by") ||
		!strings.Contains(d, artCodas[quality.Excellent]) {
		t.Fatalf("description %q", d)
	}
	if d := ArtDescription(9, construction.SmallSculpture, item.Steel, quality.Normal, "Bo"); !strings.HasPrefix(d, "A normal steel small") {
		t.Fatalf("description %q", d)
	}
}
