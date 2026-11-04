package selection

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// onLeft is a view holding the cells left of x = 20; like the renderer's
// test it gets a cell's corner and looks at that cell's centre.
func onLeft(wx, _ float64) bool { return wx+0.5 < 20 }

func cands(k Kind, ids ...entity.EntityID) []Cand { return GroupOf(k, ids).Members }

// groupWorld has two visible doors and one out of view, a big table whose
// corner is in view and whose centre is not, the kinds that come in rows,
// blueprints and zones.
func groupWorld() game.WorldView {
	return game.WorldView{
		Buildings: []game.BuildingView{
			{ID: 10, Kind: "door", X: 1, Y: 1, W: 1, H: 1},
			{ID: 11, Kind: "door", X: 2, Y: 1, W: 1, H: 1},
			{ID: 12, Kind: "door", X: 40, Y: 1, W: 1, H: 1},
			{ID: 13, Kind: "table", X: 3, Y: 8, W: 2, H: 2},
			{ID: 14, Kind: "table", X: 18, Y: 8, W: 6, H: 2},
			{ID: 20, Kind: "wall", X: 1, Y: 2, W: 1, H: 1},
			{ID: 21, Kind: "wall", X: 2, Y: 2, W: 1, H: 1},
			{ID: 22, Kind: "hopper", X: 3, Y: 2, W: 1, H: 1},
			{ID: 23, Kind: "hopper", X: 4, Y: 2, W: 1, H: 1},
			{ID: 24, Kind: "floor", X: 5, Y: 2, W: 1, H: 1, Floor: true},
			{ID: 25, Kind: "fence", X: 6, Y: 2, W: 1, H: 1},
			{ID: 26, Kind: "power conduit", X: 7, Y: 2, W: 1, H: 1},
		},
		Frames: []game.FrameView{
			{ID: 30, Kind: "wall", X: 1, Y: 3, W: 1, H: 1},
			{ID: 31, Kind: "wall", X: 2, Y: 3, W: 1, H: 1},
			{ID: 32, Kind: "wall", X: 40, Y: 3, W: 1, H: 1},
			{ID: 33, Kind: "door", X: 3, Y: 3, W: 1, H: 1},
			{ID: 34, Kind: "floor", X: 4, Y: 3, W: 1, H: 1, Floor: true},
			{ID: 35, Kind: "floor", X: 5, Y: 3, W: 1, H: 1, Floor: true},
		},
		Zones: []game.ZoneView{
			{ID: 1, Kind: game.ZoneGrow, Cells: [][2]int{{1, 5}, {2, 5}}, MinX: 1, MinY: 5, MaxX: 2, MaxY: 5},
			{ID: 2, Kind: game.ZoneGrow, Cells: [][2]int{{19, 6}, {21, 6}}, MinX: 19, MinY: 6, MaxX: 21, MaxY: 6}, // one cell in view
			{ID: 3, Kind: game.ZoneGrow, Cells: [][2]int{{40, 6}}, MinX: 40, MinY: 6, MaxX: 40, MaxY: 6},
			{ID: 4, Kind: game.ZoneStockpile, Cells: [][2]int{{4, 5}}, MinX: 4, MinY: 5, MaxX: 4, MaxY: 5},
			{ID: 5, Kind: game.ZoneShelf, Cells: [][2]int{{6, 5}}, MinX: 6, MinY: 5, MaxX: 6, MaxY: 5, Shelf: 13},
			{ID: 6, Kind: game.ZoneShelf, Cells: [][2]int{{7, 5}}, MinX: 7, MinY: 5, MaxX: 7, MaxY: 5, Shelf: 14},
		},
		Plants: []game.PlantView{
			{ID: 40, X: 1, Y: 5, Species: "rice", Zone: "rice"},
			{ID: 41, X: 9, Y: 9, Species: "tree"},
			{ID: 42, X: 6, Y: 5, Species: "grass"}, // on a shelf's zone
		},
	}
}

func TestMultiSaysWhichBuildingsGroup(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		b    game.BuildingView
		want bool
	}{
		{"door", game.BuildingView{Kind: "door"}, true},
		{"bed", game.BuildingView{Kind: "bed"}, true},
		{"fence gate", game.BuildingView{Kind: "fence gate"}, true},
		{"wall", game.BuildingView{Kind: "wall"}, false},
		{"hopper", game.BuildingView{Kind: "hopper"}, false},
		{"floor", game.BuildingView{Kind: "floor", Floor: true}, false},
		{"paved tile", game.BuildingView{Kind: "paved tile", Floor: true}, false},
		{"carpet", game.BuildingView{Kind: "carpet"}, false},
		{"fence", game.BuildingView{Kind: "fence"}, false},
		{"power conduit", game.BuildingView{Kind: "power conduit"}, false},
		{"hidden conduit", game.BuildingView{Kind: "hidden conduit"}, false},
		{"waterproof conduit", game.BuildingView{Kind: "waterproof conduit"}, false},
	} {
		if got := Multi(c.b); got != c.want {
			t.Errorf("Multi(%s) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestAlikeIsTheDoubleClickRule(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		click Cand
		want  []Cand // nil: not a group, a plain click
	}{
		{"doors in view only", Cand{Kind: Building, ID: 11}, cands(Building, 11, 10)},
		{"a table counts by its centre", Cand{Kind: Building, ID: 13}, cands(Building, 13)},
		{"wall", Cand{Kind: Building, ID: 20}, nil},
		{"hopper", Cand{Kind: Building, ID: 22}, nil},
		{"floor", Cand{Kind: Building, ID: 24}, nil},
		{"fence", Cand{Kind: Building, ID: 25}, nil},
		{"conduit", Cand{Kind: Building, ID: 26}, nil},
		{"a building that is gone", Cand{Kind: Building, ID: 99}, nil},
		{"wall blueprints group, in view only", Cand{Kind: Frame, ID: 31}, cands(Frame, 31, 30)},
		{"a blueprint takes its own kind", Cand{Kind: Frame, ID: 33}, cands(Frame, 33)},
		{"floor blueprints do not", Cand{Kind: Frame, ID: 34}, nil},
		{"a blueprint that is gone", Cand{Kind: Frame, ID: 99}, nil},
		{"zones with a cell in view", Cand{Kind: Zone, ID: 1}, cands(Zone, 1, 2)},
		{"a zone's own kind", Cand{Kind: Zone, ID: 4}, cands(Zone, 4)},
		{"a shelf's zone is the shelf's", Cand{Kind: Zone, ID: 5}, nil},
		{"a zone's plant takes the zones", Cand{Kind: Plant, ID: 40}, cands(Zone, 1, 2)},
		{"a wild tree", Cand{Kind: Plant, ID: 41}, nil},
		{"a plant on a shelf's zone", Cand{Kind: Plant, ID: 42}, nil},
		{"a plant that is gone", Cand{Kind: Plant, ID: 99}, nil},
		{"not this rule's kinds", Cand{Kind: Item, ID: 10}, nil},
	} {
		got, ok := Alike(groupWorld(), c.click, onLeft)
		if ok != (c.want != nil) || !slices.Equal(got, c.want) {
			t.Errorf("%s: Alike = %v, %v, want %v", c.name, got, ok, c.want)
		}
	}
}

func TestAlikeLeavesFogOut(t *testing.T) {
	t.Parallel()
	w := groupWorld()
	w.TerrainW, w.Fog = 50, make([]uint8, 50*50)
	w.Fog[1*50+2] = 1 // the second door
	w.Fog[3*50+2] = 1 // the second wall blueprint
	w.Fog[6*50+19] = 1
	for _, c := range []struct {
		click Cand
		want  []Cand
	}{
		{Cand{Kind: Building, ID: 10}, cands(Building, 10)},
		{Cand{Kind: Frame, ID: 30}, cands(Frame, 30)},
		{Cand{Kind: Zone, ID: 1}, cands(Zone, 1)},
	} {
		if got, _ := Alike(w, c.click, onLeft); !slices.Equal(got, c.want) {
			t.Errorf("%v: %v, want %v", c.click, got, c.want)
		}
	}
}

func TestBoxedTakesOneKindInOrder(t *testing.T) {
	t.Parallel()
	row := func(y int) InView { return func(wx, wy float64) bool { return wx < 20 && int(wy) == y } }
	for _, c := range []struct {
		name string
		in   InView
		kind Kind
		ids  []entity.EntityID
	}{
		{"buildings before the rest, walls left out", row(1), Building, []entity.EntityID{10, 11}},
		{"only what comes in rows: no group", row(2), None, nil},
		{"blueprints, floors left out", row(3), Frame, []entity.EntityID{30, 31, 33}},
		{"zones, a shelf's left out", row(5), Zone, []entity.EntityID{1, 4}},
		{"buildings beat blueprints and zones", func(wx, _ float64) bool { return wx < 20 }, Building, []entity.EntityID{10, 11, 13}},
		{"nothing", func(float64, float64) bool { return false }, None, nil},
	} {
		kind, ids := Boxed(groupWorld(), c.in)
		if kind != c.kind || !slices.Equal(ids, c.ids) {
			t.Errorf("%s: Boxed = %v %v, want %v %v", c.name, kind, ids, c.kind, c.ids)
		}
	}
}

// shiftModel has every kind on its own tile for Shift clicks.
func shiftModel() (*Model, *fakeShell) {
	w := groupWorld()
	w.Items = []game.ItemView{{ID: 50, X: 12, Y: 12}, {ID: 51, X: 13, Y: 12}}
	w.Animals = []game.AnimalView{{ID: 60, X: 14, Y: 12}}
	sh := &fakeShell{w: w, pawns: []game.PawnView{{ID: 70, X: 15, Y: 12}}}
	return New(sh, &fakeHost{}), sh
}

func shiftAt(m *Model, sh *fakeShell, x, y int) {
	sh.cursor = [2]int{x*10 + 5, y*10 + 5}
	m.ShiftClick()
}

func TestShiftMixesKindsAndTakesThemOut(t *testing.T) {
	t.Parallel()
	m, sh := shiftModel()
	m.Select(Cand{Kind: Building, ID: 10})
	shiftAt(m, sh, 12, 12) // an item
	shiftAt(m, sh, 3, 3)   // a door blueprint
	shiftAt(m, sh, 4, 5)   // a stockpile
	shiftAt(m, sh, 14, 12) // an animal
	shiftAt(m, sh, 15, 12) // a colonist
	want := []Cand{{Building, 10}, {Item, 50}, {Frame, 33}, {Zone, 4}, {Animal, 60}, {Pawn, 70}}
	if !slices.Equal(m.Group.Members, want) || m.Group.Kind() != Mixed {
		t.Fatalf("members %v, kind %v, want %v", m.Group.Members, m.Group.Kind(), want)
	}
	if !m.InGroup(Zone, 4) || m.InGroup(Building, 4) {
		t.Fatal("membership goes by kind and id")
	}
	if b := m.SelectedBuildings(); len(b) != 1 || b[0].ID != 10 {
		t.Fatalf("buildings %v", b)
	}
	if f := m.SelectedFrames(); len(f) != 1 || f[0].ID != 33 {
		t.Fatalf("frames %v", f)
	}
	if z := m.SelectedZones(); len(z) != 1 || z[0].ID != 4 {
		t.Fatalf("zones %v", z)
	}

	shiftAt(m, sh, 12, 12)
	shiftAt(m, sh, 14, 12)
	shiftAt(m, sh, 15, 12)
	shiftAt(m, sh, 4, 5)
	want = []Cand{{Building, 10}, {Frame, 33}}
	if !slices.Equal(m.Group.Members, want) {
		t.Fatalf("after taking four out: %v, want %v", m.Group.Members, want)
	}
	shiftAt(m, sh, 1, 1)
	if m.Grouped() || !m.HasFrame || m.SelFrame != 33 {
		t.Fatalf("a group of one is the single selection: %+v frame %v", m.Group, m.SelFrame)
	}
	shiftAt(m, sh, 3, 3)
	if _, ok := m.Selected(); ok || m.Grouped() {
		t.Fatal("taking the last one out leaves nothing")
	}
}

func TestShiftOnWhatCannotGroupIsAPlainClick(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		x, y int
		want Cand
	}{
		{"wall", 1, 2, Cand{Building, 20}},
		{"hopper", 3, 2, Cand{Building, 22}},
		{"floor", 5, 2, Cand{Building, 24}},
		{"fence", 6, 2, Cand{Building, 25}},
		{"conduit", 7, 2, Cand{Building, 26}},
		{"floor blueprint", 4, 3, Cand{Frame, 34}},
		{"tree", 9, 9, Cand{Plant, 41}},
	} {
		m, sh := shiftModel()
		m.SetGroup(Building, []entity.EntityID{10, 11})
		if c.want.Kind == Building && c.want.ID == 26 {
			m.host = powerShown{&fakeHost{}}
		}
		shiftAt(m, sh, c.x, c.y)
		if got, ok := m.Selected(); !ok || got != c.want || m.Grouped() {
			t.Errorf("%s: selected %v %v grouped %v, want %v alone", c.name, got, ok, m.Grouped(), c.want)
		}
	}
}

// powerShown keeps a hidden or plain conduit clickable, as the overlay does.
type powerShown struct{ *fakeHost }

func (powerShown) PowerShown() bool { return true }

func TestShiftStartsFromAWallWithoutKeepingIt(t *testing.T) {
	t.Parallel()
	m, sh := shiftModel()
	m.Select(Cand{Kind: Building, ID: 20})
	shiftAt(m, sh, 1, 1)
	if m.Grouped() || m.SelBuilding != 10 {
		t.Fatalf("a wall never joins: group %v, building %v", m.Group.Members, m.SelBuilding)
	}
}

func TestSelectedNamesTheZone(t *testing.T) {
	t.Parallel()
	m, _ := shiftModel()
	m.Select(Cand{Kind: Zone, ID: 4})
	if c, ok := m.Selected(); !ok || c != (Cand{Zone, 4}) {
		t.Fatalf("selected %v %v", c, ok)
	}
	if k, ids := m.SelectionSet(); k != Zone || !slices.Equal(ids, []entity.EntityID{4}) {
		t.Fatalf("set %v %v", k, ids)
	}
}

func TestApplyBoxKeepsOrAddsToTheSelection(t *testing.T) {
	t.Parallel()
	doors := []entity.EntityID{10, 11}
	for _, c := range []struct {
		name  string
		start func(*Model)
		kind  Kind
		ids   []entity.EntityID
		add   bool
		want  []Cand
	}{
		{"furniture replaces", func(m *Model) { m.Select(Cand{Item, 50}) }, Building, doors, false, cands(Building, 10, 11)},
		{"Shift joins the same kind", func(m *Model) { m.Select(Cand{Building, 13}) }, Building, doors, true, cands(Building, 13, 10, 11)},
		{"Shift mixes another kind", func(m *Model) { m.Select(Cand{Item, 50}) }, Building, doors, true, []Cand{{Item, 50}, {Building, 10}, {Building, 11}}},
		{"Shift on a group of two kinds", func(m *Model) { m.SetMembers([]Cand{{Item, 50}, {Pawn, 70}}) }, Zone, []entity.EntityID{1}, true, []Cand{{Item, 50}, {Pawn, 70}, {Zone, 1}}},
		{"Shift does not keep a wall", func(m *Model) { m.Select(Cand{Building, 20}) }, Building, doors, true, cands(Building, 10, 11)},
		{"an empty Shift box keeps the group", func(m *Model) { m.SetGroup(Item, []entity.EntityID{50, 51}) }, None, nil, true, cands(Item, 50, 51)},
		{"an empty box clears", func(m *Model) { m.SetGroup(Item, []entity.EntityID{50, 51}) }, None, nil, false, nil},
	} {
		m, _ := shiftModel()
		c.start(m)
		m.ApplyBox(c.kind, c.ids, c.add)
		if !slices.Equal(m.Group.Members, c.want) {
			t.Errorf("%s: %v, want %v", c.name, m.Group.Members, c.want)
		}
	}
}

func TestPickFindsTheMembersOwnKind(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		members []Cand
		click   string
		want    Cand
	}{
		{"same kind", cands(Building, 10, 11), "group.pick.11", Cand{Building, 11}},
		{"a mixed group's item", []Cand{{Building, 10}, {Item, 50}}, "group.pick.50", Cand{Item, 50}},
		{"a mixed group's blueprint", []Cand{{Frame, 33}, {Item, 50}}, "group.pick.33", Cand{Frame, 33}},
		{"a zone whose id is also a building's", []Cand{{Building, 4}, {Zone, 4}}, PickID(Cand{Zone, 4}), Cand{Zone, 4}},
		{"a building whose id is also a zone's", []Cand{{Zone, 10}, {Building, 10}}, PickID(Cand{Building, 10}), Cand{Building, 10}},
	} {
		m, _ := shiftModel()
		m.SetMembers(c.members)
		m.ApplyGroupClick(c.click)
		if got, ok := m.Selected(); !ok || got != c.want || m.Grouped() {
			t.Errorf("%s: selected %v %v grouped %v, want %v", c.name, got, ok, m.Grouped(), c.want)
		}
	}
	m, _ := shiftModel()
	m.SetGroup(Item, []entity.EntityID{50, 51})
	m.ApplyGroupClick("group.pick.99")
	if !m.Grouped() {
		t.Fatal("a pick of no member changes nothing")
	}
}

func TestItemsAndAnimalsStillToggle(t *testing.T) {
	t.Parallel()
	m, sh := shiftModel()
	shiftAt(m, sh, 12, 12)
	shiftAt(m, sh, 13, 12)
	if !slices.Equal(m.Group.Members, cands(Item, 50, 51)) || m.Group.Kind() != Item {
		t.Fatalf("items %v", m.Group.Members)
	}
	m.Clear(None)
	m.ToggleInSet(Cand{Animal, 60})
	if !m.HasAnimal || m.SelAnimal != 60 {
		t.Fatal("one animal is a single selection")
	}
}

func TestMembersReadOnlyWhatTheGroupHolds(t *testing.T) {
	t.Parallel()
	calls := 0
	id := func(b game.BuildingView) entity.EntityID { calls++; return b.ID }
	views := groupWorld().Buildings
	if got := members(nil, views, id); got != nil || calls != 0 {
		t.Fatalf("no ids read %d views, got %v", calls, got)
	}
	got := members([]entity.EntityID{11, 99, 10}, views, id)
	if len(got) != 2 || got[0].ID != 11 || got[1].ID != 10 {
		t.Fatalf("members %v, want 11 then 10, the gone one dropped", got)
	}
}

func TestSelectableKindsBeyondTheGroup(t *testing.T) {
	t.Parallel()
	m, _ := shiftModel()
	for _, c := range []struct {
		c    Cand
		want bool
	}{
		{Cand{Pawn, 70}, true},
		{Cand{Animal, 60}, true},
		{Cand{Item, 50}, true},
		{Cand{Building, 10}, true},
		{Cand{Building, 20}, false},
		{Cand{Frame, 30}, true},
		{Cand{Frame, 34}, false},
		{Cand{Zone, 1}, true},
		{Cand{Zone, 5}, false},
		{Cand{Plant, 40}, false},
		{Cand{None, 1}, false},
	} {
		if got := m.CanGroup(c.c); got != c.want {
			t.Errorf("CanGroup(%v) = %v, want %v", c.c, got, c.want)
		}
	}
}
