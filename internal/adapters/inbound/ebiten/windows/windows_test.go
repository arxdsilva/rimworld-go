package windows

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/drug"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeReader is a colony with no wildlife, history or world map.
type fakeReader struct{}

func (fakeReader) Wildlife() []game.WildlifeRow                      { return nil }
func (fakeReader) History() []game.HistoryEntryView                  { return nil }
func (fakeReader) IncapableReason(entity.EntityID, work.Type) string { return "" }

// The lists keep every column when the window has room and leave the
// low-priority ones out, never the buttons, when it does not.
func TestAnimalColumnsDropToFit(t *testing.T) {
	t.Parallel()
	if c := animalColsFor(colonyFullW, false); len(c) != 0 {
		t.Fatalf("a full-width window drops %v", c)
	}
	if c := animalColsFor(wildFullW, true); len(c) != 0 {
		t.Fatalf("a full-width wild list drops %v", c)
	}
	for _, w := range []float32{1672, 1248, 1032, 900, 865, 830} {
		c := animalColsFor(w, false)
		if end := float32(c.at(colBtn) + 196 + 12); end > w {
			t.Errorf("colony list at %v px ends at %v with %d dropped", w, end, len(c))
		}
		if c.at(colBtn) < colMaster || !c.shown(colName) || !c.shown(colMaster) || !c.shown(colArea) || !c.shown(colFollow) {
			t.Errorf("colony list at %v px lost a needed column: %v", w, c)
		}
	}
	for _, w := range []float32{984, 865, 700} {
		c := animalColsFor(w, true)
		if end := float32(c.at(wildBtn) + 272 + 12); end > w {
			t.Errorf("wild list at %v px ends at %v", w, end)
		}
	}
	// Doing and Training go first, then Preg., and the columns before a
	// dropped one stay put
	c := animalColsFor(1100+dietW+6, false) // the Diet column (FOOD-5) is last to go, so the old widths shift by it
	if c.shown(colDoing) || c.shown(colTrain) || !c.shown(colPreg) || c.at(colBtn) != colBtn-108-104 || c.at(colMaster) != colMaster {
		t.Errorf("1226 px drops %v, buttons at %d, Master at %d", c, c.at(colBtn), c.at(colMaster))
	}
	c = animalColsFor(1032+dietW+6, false)
	if c.shown(colPreg) || c.at(colBtn) != colBtn-108-104-44 || c.at(colMaster) != colMaster-44 || c.at(colHealth) != colHealth-44 || c.at(colProduct) != colProduct {
		t.Errorf("1158 px drops %v, buttons at %d, Master at %d", c, c.at(colBtn), c.at(colMaster))
	}
}

func TestNextPresetCycles(t *testing.T) {
	t.Parallel()
	p := drug.NoDrugs
	seen := map[drug.Preset]bool{}
	for i := 0; i < len(AssignPresets); i++ {
		seen[p] = true
		p = NextPreset(p)
	}
	if p != drug.NoDrugs || len(seen) != 4 {
		t.Fatalf("cycle ends at %v after %v", p, seen)
	}
	if PresetLabel(drug.Unrestricted) != "Unrestricted" {
		t.Fatal(PresetLabel(drug.Unrestricted))
	}
	if PresetLabel(drug.MoodLift) != "Mood lift" {
		t.Fatal(PresetLabel(drug.MoodLift))
	}
}

// A kit window opens in the Window rect replacing the open window (the
// Architect and the order menu give way), a second open of the same id
// does nothing, its tab toggles it shut, and a quiet close skips OnClose.
func TestKitWindowOpensTogglesAndCloses(t *testing.T) {
	t.Parallel()
	sh, w := newFake()
	sh.env.Game = fakeReader{}
	w.Show("work")
	w.Assign().Launch()
	if !w.KitOpen() || w.KitID() != AssignModalID || w.Mode != AssignModalID {
		t.Fatalf("Assign not open: mode %q kit %q", w.Mode, w.KitID())
	}
	if sw, shh := w.Want(); sw != assignW || shh != assignHeight(0) || w.Anchor() != sh.WindowRect() {
		t.Fatalf("want %vx%v anchor %v", sw, shh, w.Anchor())
	}
	if len(sh.intents) != 2 || sh.intents[1] != (ports.CloseArchitect{}) || sh.floats != 2 {
		t.Fatalf("intents %v floats %d", sh.intents, sh.floats)
	}
	w.Assign().Launch() // already open: nothing
	if len(sh.intents) != 2 {
		t.Fatalf("a second open showed the window again: %v", sh.intents)
	}
	w.Assign().Toggle()
	if w.Open || w.KitID() != "" {
		t.Fatal("the tab did not close Assign")
	}
	closed := 0
	w.OpenKit(uikit.Modal{ID: "x", W: 10, H: 10, OnClose: func() { closed++ }})
	w.CloseModal("x")
	if w.Open || closed != 0 {
		t.Fatalf("quiet close: open %v closed %d", w.Open, closed)
	}
	w.OpenKit(uikit.Modal{ID: "x", OnClose: func() { closed++ }})
	w.Click("win.close")
	if w.Open || closed != 1 {
		t.Fatalf("the X: open %v closed %d", w.Open, closed)
	}
}

// The hand-drawn windows lay their buttons out in the Window rect, and
// their clicks become commands.
func TestHandDrawnWindowsButtonsAndClicks(t *testing.T) {
	t.Parallel()
	sh, w := newFake()
	sh.env.Game = fakeReader{}
	sh.pawns = []game.PawnView{{ID: 1, Name: "Ada", Status: "alive"}, {ID: 2, Name: "Bo", Status: "alive"}}
	sh.world.Research = []game.ResearchView{{ID: 3, Label: "Stonecutting", Available: true}, {ID: 4, Label: "Done", Done: true}}
	sh.world.Animals = []game.AnimalView{{ID: 50, Species: "dustbuck", Tame: true}, {ID: 51, Species: "dustbuck"}}
	if len(w.Buttons()) != 0 {
		t.Fatal("buttons with no window open")
	}
	w.Show("work")
	if got := len(w.Buttons()); got != 3*len(WorkLive) { // cells, then a header per column (CTL-014)
		t.Fatalf("work buttons %d, want %d", got, 3*len(WorkLive))
	}
	if ww, wh := w.Want(); ww <= 0 || wh <= 0 {
		t.Fatalf("work wants %vx%v", ww, wh)
	}
	w.Show("schedule")
	if got := len(w.Buttons()); got != len(schedBrushes)+3*24 {
		t.Fatalf("schedule buttons %d", got)
	}
	w.Schedule().Paint(ids.PrefixSchedCell + "2.5")
	w.Show("research")
	bs := w.Buttons()
	if len(bs) != 2 || bs[0].ID != ids.PrefixResearchPick+"3" || bs[0].Disabled || bs[1].Disabled { // every box selects, even a done one (RSCH-2)
		t.Fatalf("research buttons %+v", bs)
	}
	w.Research().Pick(bs[1].ID)
	w.Research().start() // a done project does not start
	w.Research().Pick(bs[0].ID)
	if id, ok := w.Research().Selected(); !ok || id != 3 {
		t.Fatalf("picking %s selected %v (%v)", bs[0].ID, id, ok)
	}
	w.Research().start()
	w.Show("animals")
	if got := len(w.Buttons()); got != 2+1+3 { // Colony / Wild, Auto-slaughter (FOOD-5), then Master, Slaughter, Release (the Diet column is dropped in this narrow window)
		t.Fatalf("animals buttons %d", got)
	}
	w.Animals().Click("animals.release.50")
	want := []command.Kind{command.SetSchedule, command.SetResearch, command.ReleaseAnimal}
	if len(sh.cmds) != len(want) {
		t.Fatalf("commands %+v", sh.cmds)
	}
	for i, k := range want {
		if sh.cmds[i].Kind != k {
			t.Fatalf("command %d is %v, want %v", i, sh.cmds[i].Kind, k)
		}
	}
	if sh.cmds[0].Pawn != 2 || sh.cmds[0].Index != 5 || sh.cmds[1].Build != 3 || sh.cmds[2].Pawn != 50 {
		t.Fatalf("command fields %+v", sh.cmds)
	}
}

// TestWorkSlotsCoverEveryType: each work.Type has exactly one slot in
// work.Order, and the columns sit where a colonist breaks ties.
func TestWorkSlotsCoverEveryType(t *testing.T) {
	t.Parallel()
	seen := map[int]work.Type{}
	for ty := work.Type(0); ty < work.NumTypes; ty++ {
		s := workSlot(int(ty))
		if s < 0 || s >= int(work.NumTypes) || work.Order[s] != ty {
			t.Fatalf("type %d has slot %d, Order %v", ty, s, work.Order)
		}
		if prev, dup := seen[s]; dup {
			t.Fatalf("types %d and %d share slot %d", prev, ty, s)
		}
		seen[s] = ty
	}
	if len(work.Order) != int(work.NumTypes) || len(WorkColumns) != int(work.NumTypes) {
		t.Fatalf("Order %d, columns %d, types %d", len(work.Order), len(WorkColumns), work.NumTypes)
	}
	r := uikit.Button{X: 10, Y: 10, W: 900, H: 400}
	x := func(ty work.Type) float32 { return WorkCellRect(r, 0, int(ty)).X }
	if !(x(work.Grow) < x(work.Haul) && x(work.Clean) < x(work.Research)) {
		t.Fatalf("Grow %v Haul %v Clean %v Research %v", x(work.Grow), x(work.Haul), x(work.Clean), x(work.Research))
	}
}

// cursorShell is a fakeShell whose pointer a test places.
type cursorShell struct {
	*fakeShell
	mx, my int
}

func (c *cursorShell) Cursor() (int, int) { return c.mx, c.my }

// workFixture opens Work over three colonists; Cy cannot do Cook (col 3).
func workFixture() (*cursorShell, *Set) {
	f := &fakeShell{}
	f.kit = uikit.New(fakeHost{f})
	sh := &cursorShell{fakeShell: f}
	sh.env.Game = fakeReader{}
	var inc work.Mask
	inc = inc.Set(work.Cook, true)
	sh.pawns = []game.PawnView{
		{ID: 1, Name: "Ada", Priorities: []int{0, 0, 0, 2}},
		{ID: 2, Name: "Bo", Priorities: []int{0, 0, 0, 4}},
		{ID: 3, Name: "Cy", Incapable: inc, Priorities: []int{0, 0, 0, 0}},
	}
	w := New(sh)
	w.Show("work")
	return sh, w
}

func cellID(p uint64, col int) string {
	return ids.PrefixWorkCell + uitoa(p) + "." + uitoa(uint64(col))
}

func uitoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for ; n > 0; n /= 10 {
		s = string(rune('0'+n%10)) + s
	}
	return s
}

func cellCenter(t *testing.T, w *Set, id string) (int, int) {
	t.Helper()
	for _, b := range w.Work().cellButtons() {
		if b.ID == id {
			return int(b.X + b.W/2), int(b.Y + b.H/2)
		}
	}
	t.Fatalf("no cell %s", id)
	return 0, 0
}

func TestPrevPriorityInvertsNext(t *testing.T) {
	t.Parallel()
	for v := 0; v <= 4; v++ {
		if PrevPriority(NextPriority(v)) != v || NextPriority(PrevPriority(v)) != v {
			t.Fatalf("not inverse at %d", v)
		}
	}
	if PrevPriority(1) != 0 || PrevPriority(0) != 4 {
		t.Fatal("cycle")
	}
}

func TestWorkRightClickLowersAndCycles(t *testing.T) {
	t.Parallel()
	sh, w := workFixture()
	sh.mx, sh.my = cellCenter(t, w, cellID(1, 3))
	if !w.Work().RightClick(sh.mx, sh.my) {
		t.Fatal("not consumed")
	}
	if len(sh.cmds) != 1 || sh.cmds[0].Kind != command.SetWorkPriority || sh.cmds[0].Pawn != 1 || sh.cmds[0].Count != 1 {
		t.Fatalf("%+v", sh.cmds)
	}
	sh.pawns[0].Priorities[3] = 0 // off cycles to 4
	w.Work().RightClick(sh.mx, sh.my)
	if sh.cmds[1].Count != 4 {
		t.Fatalf("off lowered to %d", sh.cmds[1].Count)
	}
	if w.Work().RightClick(-5, -5) {
		t.Fatal("click off the cells was consumed")
	}
}

func TestWorkRightClickIncapableConsumesWithNoEffect(t *testing.T) {
	t.Parallel()
	sh, w := workFixture()
	mx, my := cellCenter(t, w, cellID(3, 3))
	if !w.Work().RightClick(mx, my) || len(sh.cmds) != 0 {
		t.Fatalf("consumed or sent: %+v", sh.cmds)
	}
}

func TestWorkHeaderShiftSetsAllCapable(t *testing.T) {
	t.Parallel()
	sh, w := workFixture()
	head := ids.PrefixWorkHead + "3"
	w.Work().HeadClick(head, false, false)
	if len(sh.cmds) != 0 {
		t.Fatalf("no shift changed %+v", sh.cmds)
	}
	w.Work().HeadClick(head, true, false) // Ada has 2: next is 3 for everyone capable
	if len(sh.cmds) != 2 {
		t.Fatalf("%+v", sh.cmds)
	}
	for _, c := range sh.cmds {
		if c.Count != 3 || c.Pawn == 3 {
			t.Fatalf("%+v", c)
		}
	}
	sh.cmds = nil
	sh.pawns[1].Priorities[3] = 3 // already equal: only Ada is sent
	sh.pawns[0].Priorities[3] = 3
	sh.pawns[0].Priorities[3] = 2
	w.Work().HeadClick(head, true, false)
	if len(sh.cmds) != 1 || sh.cmds[0].Pawn != 1 {
		t.Fatalf("sent where equal: %+v", sh.cmds)
	}
}

func TestWorkHeaderShiftRightIsInverse(t *testing.T) {
	t.Parallel()
	sh, w := workFixture()
	w.Work().HeadClick(ids.PrefixWorkHead+"3", true, true) // Ada 2 -> 1 for all capable
	if len(sh.cmds) != 2 || sh.cmds[0].Count != 1 || sh.cmds[1].Count != 1 {
		t.Fatalf("%+v", sh.cmds)
	}
}

func TestWorkDragPaintsOncePerNewCellAndSkipsIncapable(t *testing.T) {
	t.Parallel()
	sh, w := workFixture()
	sh.pawns[1].Priorities[3] = 0
	sh.f = inputframe.NewFrame(inputframe.Raw{Down: [inputframe.NumMouse]bool{inputframe.Left: true}})
	w.Work().Click(cellID(1, 3)) // 2 -> 3, armed with 3
	if len(sh.cmds) != 1 || sh.cmds[0].Count != 3 {
		t.Fatalf("%+v", sh.cmds)
	}
	drag := func(id string) {
		sh.mx, sh.my = cellCenter(t, w, id)
		w.Work().Drag()
	}
	drag(cellID(1, 3)) // the pressed cell again: nothing
	drag(cellID(2, 3))
	drag(cellID(2, 3)) // same cell: not re-sent
	drag(cellID(3, 3)) // incapable: skipped
	if len(sh.cmds) != 2 || sh.cmds[1].Pawn != 2 || sh.cmds[1].Count != 3 {
		t.Fatalf("%+v", sh.cmds)
	}
	sh.f = inputframe.NewFrame(inputframe.Raw{}) // released
	drag(cellID(1, 4))
	if len(sh.cmds) != 2 || w.paint.Armed {
		t.Fatalf("painted after release: %+v", sh.cmds)
	}
}

func TestWorkColumnsFollowOrderForHeadersAndCells(t *testing.T) {
	t.Parallel()
	_, w := workFixture()
	r := w.Rect()
	for _, b := range w.Work().buttons() {
		if len(b.ID) > len(ids.PrefixWorkHead) && b.ID[:len(ids.PrefixWorkHead)] == ids.PrefixWorkHead {
			col := int(b.ID[len(b.ID)-1] - '0')
			if len(b.ID) > len(ids.PrefixWorkHead)+1 {
				col = col + 10*int(b.ID[len(b.ID)-2]-'0')
			}
			if b.X != WorkCellRect(uikit.Button{X: r.X, Y: r.Y, W: r.W, H: r.H}, 0, col).X {
				t.Fatalf("header %s at %v", b.ID, b.X)
			}
		}
	}
}
