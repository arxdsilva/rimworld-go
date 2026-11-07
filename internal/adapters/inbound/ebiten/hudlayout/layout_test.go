package hudlayout

import (
	"reflect"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
)

// layoutScreens are the logical screens the invariants run on.
var layoutScreens = [][2]int{{1280, 720}, {1920, 1080}, {1113, 626}}

// Six colonists fit one full-size row; thirty go to the smallest step
// (UIS-022): two rows of 18 + 12 at 1280, one row at 1920. The bar never
// reaches the resource list or the right column.
func TestLayoutColonistBarWraps(t *testing.T) {
	t.Parallel()
	st := Inputs{GlobalH: 196, ResourcesH: 600, Alerts: 8, Letters: 4, LearnH: LearnHeaderH}
	for _, sz := range layoutScreens {
		st.Colonists = 6
		l := Compute(sz[0], sz[1], st)
		if l.BarScale != 1 || len(l.BarSlots) != 6 || l.BarMore != 0 || l.Bar.H != BarSlot+BarNameH || l.Bar.Y != BarTop {
			t.Errorf("%v: 6 colonists = scale %v, %d slots (+%d), bar %+v; want one full-size row", sz, l.BarScale, len(l.BarSlots), l.BarMore, l.Bar)
		}
		st.Colonists = 30
		l = Compute(sz[0], sz[1], st)
		rows := map[float32]int{}
		for _, s := range l.BarSlots {
			rows[s.Y]++
		}
		if l.BarScale >= 1 || len(l.BarSlots)+max(l.BarMore-1, 0) != 30 {
			t.Errorf("%v: 30 colonists = scale %v, %d slots (+%d); want them all, smaller", sz, l.BarScale, len(l.BarSlots), l.BarMore)
		}
		switch sz {
		case [2]int{1280, 720}:
			if l.BarSlots[0].W != 27 || rows[BarTop] != 18 || len(rows) != 2 || l.BarMore != 0 {
				t.Errorf("1280x720: 30 colonists = slot %v, %v by row; want 18 + 12 at 27", l.BarSlots[0].W, rows)
			}
		case [2]int{1920, 1080}:
			if l.BarSlots[0].W != 27 || len(rows) != 1 {
				t.Errorf("1920x1080: 30 colonists = slot %v, %v by row; want one row at 27", l.BarSlots[0].W, rows)
			}
		}
		for _, n := range []int{6, 30, 90} {
			st.Colonists = n
			l = Compute(sz[0], sz[1], st)
			if l.Resources.Empty() {
				t.Fatalf("%v: no resource list", sz)
			}
			if l.Bar.X < l.Resources.Right()+HudMargin || l.Bar.Right() > l.Right.X-HudMargin {
				t.Errorf("%v %d colonists: bar %+v reaches the resources (right %v) or the right column (%v)", sz, n, l.Bar, l.Resources.Right(), l.Right.X)
			}
		}
	}
}

// TestBarStepsMatchReference (UIS-020 to UIS-025): at 1920 logical, 3, 8 and
// 16 colonists are full size (48, pitch 72), 24 are 37 at pitch 56, 40 are
// 27 at pitch 41 in two rows of 34 + 6 with a row pitch of 46, 60 still fit;
// every row is n x pitch wide and centred on its own.
func TestBarStepsMatchReference(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		n           int
		slot, pitch float32
		first, rows int
	}{{3, 48, 72, 3, 1}, {8, 48, 72, 8, 1}, {16, 48, 72, 16, 1}, {24, 37, 56, 24, 1}, {40, 27, 41, 34, 2}, {60, 27, 41, 34, 2}} {
		l := Compute(1920, 1080, Inputs{Colonists: c.n})
		rows := map[float32][]gfx.Rect{}
		for _, s := range l.BarSlots {
			rows[s.Y] = append(rows[s.Y], s)
		}
		if len(l.BarSlots) != c.n || l.BarMore != 0 || l.BarSlots[0].W != c.slot || l.BarSlots[0].H != c.slot || len(rows[BarTop]) != c.first || len(rows) != c.rows {
			t.Errorf("%d colonists: %d slots (+%d), slot %v, %d in row 1, %d rows; want slot %v, %d, %d", c.n, len(l.BarSlots), l.BarMore, l.BarSlots[0].W, len(rows[BarTop]), len(rows), c.slot, c.first, c.rows)
			continue
		}
		for y, r := range rows {
			if len(r) > 1 && r[1].X-r[0].X != c.pitch {
				t.Errorf("%d colonists: pitch %v, want %v", c.n, r[1].X-r[0].X, c.pitch)
			}
			if mid := r[0].X + float32(len(r))*c.pitch/2; gfx.Abs32(mid-960) > 1 {
				t.Errorf("%d colonists: row at %v centred on %v, want 960", c.n, y, mid)
			}
		}
		if c.rows == 2 {
			if _, ok := rows[BarTop+46]; !ok {
				t.Errorf("%d colonists: second row not 46 below the first (%v)", c.n, rows)
			}
		}
	}
}

// TestCommandsNeverDropped (UIS-052): more commands than cells keep every
// one reachable: the last cell stands for the rest.
func TestCommandsNeverDropped(t *testing.T) {
	t.Parallel()
	for _, n := range []int{8, 30, 400} {
		l := Compute(1280, 720, Inputs{Colonists: 3, PaneShown: true, Commands: n, GlobalH: 60})
		shown := len(l.GridCells)
		if l.GridMore > 0 {
			shown += l.GridMore - 1
		}
		if shown != n {
			t.Errorf("%d commands: %d cells, more %d", n, len(l.GridCells), l.GridMore)
		}
		for _, g := range l.GridCells {
			if g.Y < l.Bar.Bottom() || g.Right() > l.Right.X || g.Bottom() > l.Tabs.Y-CmdBottom {
				t.Errorf("%d commands: cell %+v leaves its band", n, g)
			}
		}
	}
}

// TestMetricsMatchReference pins each reference size to its gap row
// (`.agents/gaps/UI_SCALE_GAP_ANALYSIS.md`): a change here changes the row.
func TestMetricsMatchReference(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		row       string
		got, want float32
	}{
		{"UIS-040 tab bar", TabsH, 34},
		{"UIS-044 pane width", PaneW, 432},
		{"UIS-044 pane height", PaneH, 165},
		{"UIS-045 pane tab row", PaneTabsH, 30},
		{"UIS-045 non-pawn tab", PaneTabW, 70},
		{"UIS-046 pane inset", PaneInset, 12},
		{"UIS-046 name baseline", PaneNameBaseline, 28},
		{"UIS-047 status bar width", PaneBarW, 93},
		{"UIS-047 status bar height", PaneBarH, 16},
		{"UIS-047 status bar gap", PaneBarGap, 6},
		{"UIS-047 status bar top", PaneBarTop, 36},
		{"UIS-048 pane line", PaneLineH, 16},
		{"UIS-050 command cell", CmdCell, 75},
		{"UIS-050 command stride", CmdStride, 80},
		{"UIS-050 grid gap", CmdGridGap, 14},
		{"UIS-050 grid bottom", CmdBottom, 16},
		{"UIS-051 hotkey inset", CmdInset, 6},
		{"UIS-051 icon", CmdIconMax, 40},
		{"UIS-053 category button width", ArchCatW, 100},
		{"UIS-053 category button height", ArchCatH, 32},
		{"UIS-053 category panel", ArchCatsW, 200},
		{"UIS-053 search row", ArchSearchH, 28},
		{"UIS-054 description width", ArchDescW, 200},
		{"UIS-054 description height", ArchDescH, 270},
		{"UIS-055 designator gap", ArchGridGap, 10},
		{"UIS-056 icon", GlobalIcon, 24},
		{"UIS-056 icon pitch", GlobalIconPitch, 28},
		{"UIS-056 per row", GlobalPerRow, 5},
		{"UIS-056 right inset", GlobalRight, 3},
		{"UIS-056 bottom", GlobalBottom, 6},
		{"UIS-057 speed width", SpeedW, 32},
		{"UIS-057 speed height", SpeedH, 24},
		{"UIS-057 speed right", SpeedRight, 6},
		{"UIS-058 text pitch", GlobalRowPitch, 26},
		{"UIS-058 text right", GlobalTextRight, 8},
		{"UIS-059 cell readout x", CellX, 15},
		{"UIS-059 cell readout pitch", CellPitch, 19},
		{"UIS-059 cell readout bottom", CellBottom, 17},
		{"UIS-020 slot", BarSlot, 48},
		{"UIS-021 pitch", BarPitch, 72},
		{"UIS-024 bar top", BarTop, 21},
		{"UIS-029 message x", MsgX, 140},
		{"UIS-029 message y", MsgY, 17},
		{"UIS-030 alert row", AlertRowH, 22},
		{"UIS-030 alert right", AlertRight, 7},
		{"UIS-031 letter width", LetterW, 38},
		{"UIS-031 letter row", LetterRowH, 30},
		{"UIS-031 letter gap", LetterGap, 12},
		{"UIS-031 letter right", LetterRight, 12},
		{"UIS-060 window pad", WindowPad, 18},
		{"UIS-060 window edge", WindowEdge, 1},
		{"UIS-017 menu row", MenuRowH, 29},
		{"UIS-017 table row", TableRowH, 30},
		{"UIS-016 option row", OptionRowH, 35},
		{"UIS-016 option control", OptionCtrlH, 30},
		{"UIS-070 float row", FloatRowH, 29},
		{"UIS-070 float icon", FloatIcon, 19},
		{"UIS-070 float pad", FloatPadW, 48},
		{"UIS-069 menu width", PauseW, 450},
		{"UIS-069 menu height", PauseH, 391},
		{"UIS-069 menu button width", PauseBtnW, 168},
		{"UIS-069 menu button height", PauseBtnH, 44},
		{"UIS-069 menu button pitch", PauseBtnPitch, 52},
		{"UIS-069 menu inset", PauseInset, 19},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.row, c.got, c.want)
		}
	}
	for tab, want := range map[string][2]float32{"log": {630, 510}, "gear": {460, 450}, "social": {540, 510}, "bio": {514, 489}, "needs": {580, 520}, "health": {630, 430}} {
		if w, h, ok := PaneTabSize(tab); !ok || w != want[0] || h != want[1] || w > ExpandMaxW {
			t.Errorf("UIS-049 %s panel = %v x %v (%v), want %v", tab, w, h, ok, want)
		}
	}
	if ArchCatsH(0) != 6*32+28 || ArchCatsH(13) != 7*32+28 {
		t.Errorf("UIS-053 category panel heights %v, %v", ArchCatsH(0), ArchCatsH(13))
	}
}

func TestBoundsOf(t *testing.T) {
	t.Parallel()
	if got := BoundsOf(nil); got.W != 0 || got.H != 0 {
		t.Fatalf("BoundsOf(nil) = %+v, want zero", got)
	}
	rs := []gfx.Rect{{X: 10, Y: 20, W: 5, H: 5}, {X: 0, Y: 30, W: 4, H: 10}}
	if got := BoundsOf(rs); got != (gfx.Rect{X: 0, Y: 20, W: 15, H: 20}) {
		t.Fatalf("BoundsOf = %+v", got)
	}
}

func TestPerfRectEmptyWhenOff(t *testing.T) {
	t.Parallel()
	l := Compute(1280, 720, Inputs{GlobalH: 196, ResourcesH: 100})
	if !PerfRect(l, 0, 10).Empty() {
		t.Fatal("zero-width perf box must be empty")
	}
	if r := PerfRect(l, 100, 60); r.Empty() {
		t.Fatal("a small box fits on a 1280x720 screen")
	}
}

// Two maps' colonists sit in two separate frames, side by side, centred,
// with each slot inside its frame, below its chip, and never over the right
// column; a lone plain bar has no frames at all.
func TestLayoutBarGroupsFrameEachMap(t *testing.T) {
	t.Parallel()
	for _, sz := range layoutScreens {
		l := Compute(sz[0], sz[1], Inputs{Colonists: 6, GlobalH: 196})
		if len(l.BarGroups) != 0 {
			t.Fatalf("%v: a single map has %d frames", sz, len(l.BarGroups))
		}
		st := Inputs{Colonists: 9, GlobalH: 196, Alerts: 3}
		st.BarGroups[0], st.BarGroups[1] = 6, 3
		l = Compute(sz[0], sz[1], st)
		if len(l.BarGroups) != 2 || len(l.BarSlots) != 9 || l.BarMore != 0 {
			t.Fatalf("%v: %d frames, %d slots (+%d)", sz, len(l.BarGroups), len(l.BarSlots), l.BarMore)
		}
		a, b := l.BarGroups[0], l.BarGroups[1]
		if a.Frame.Right() >= b.Frame.X || a.N != 6 || b.First != 6 || b.N != 3 {
			t.Errorf("%v: frames %+v %+v", sz, a, b)
		}
		for i, s := range l.BarSlots {
			g := a
			if i >= 6 {
				g = b
			}
			if s.X < g.Frame.X || s.Right() > g.Frame.Right() || s.Y < g.Chip.Bottom() || s.Bottom() > g.Frame.Bottom() {
				t.Errorf("%v: slot %d %+v outside its frame %+v / chip %+v", sz, i, s, g.Frame, g.Chip)
			}
		}
		if l.Bar.Right() > l.Right.X {
			t.Errorf("%v: bar %+v reaches the right column %+v", sz, l.Bar, l.Right)
		}
	}
}

// Too many colonists for any step keep the slots that fit and end in "+N".
func TestLayoutBarGroupsOverflow(t *testing.T) {
	t.Parallel()
	st := Inputs{Colonists: 120, GlobalH: 196}
	st.BarGroups[0], st.BarGroups[1] = 60, 60
	l := Compute(1280, 720, st)
	if l.BarMore == 0 || len(l.BarSlots)+l.BarMore-1 != 120 {
		t.Fatalf("%d slots, more %d", len(l.BarSlots), l.BarMore)
	}
	if l.Bar.Right() > l.Right.X {
		t.Errorf("bar %+v reaches the right column", l.Bar)
	}
}

// A floating card over the right column pushes the letters off it, and the
// gap spaces them; without the card nothing changes (RHUD-471, 472, 475, 478).
func TestLettersAvoidTheCard(t *testing.T) {
	t.Parallel()
	base := Compute(1920, 1080, Inputs{Letters: 3, PaneShown: true})
	same := Compute(1920, 1080, Inputs{Letters: 3, PaneShown: true, LetterGap: 0})
	if !reflect.DeepEqual(base, same) {
		t.Fatal("a zero gap changed the layout")
	}
	card := gfx.Rect{X: base.Right.X - 100, Y: base.Letters.Y - 20, W: 300, H: 200}
	l := Compute(1920, 1080, Inputs{Letters: 3, PaneShown: true, LetterAvoid: card, LetterGap: 8})
	if l.LetterRows == 0 || !l.Letters.Intersect(card).Empty() {
		t.Fatalf("letters %+v overlap the card %+v (rows %d)", l.Letters, card, l.LetterRows)
	}
	if l.LetterGap != 8 {
		t.Fatalf("gap %v, want 8", l.LetterGap)
	}
	// a card off the column leaves the letters where they were
	far := Compute(1920, 1080, Inputs{Letters: 3, PaneShown: true, LetterAvoid: gfx.Rect{X: 10, Y: 10, W: 300, H: 300}})
	if !reflect.DeepEqual(far, base) {
		t.Fatal("a card away from the column moved the letters")
	}
}

// The compact card's pane (RHUD-003, 004): as wide as its tabs while five
// command columns stay free, never narrower than the plain pane, never under
// the right column; as tall as asked, never under CardPaneMinH. The eight
// commands of a colonist wrap to two rows beside it and never meet it.
func TestCardPaneLeavesCommandRoom(t *testing.T) {
	cases := []struct {
		sw, sh       int
		paneW, paneH float32
		wantW, wantH float32
		cols         int
	}{
		{1288, 724, 576, 168, 576, 168, 5},
		{1280, 720, 576, 168, 576, 168, 5},
		{1024, 768, 624, 179, 432, 179, 4},
		{1920, 1080, 864, 252, 864, 252, 10},
		{853, 480, 432, 141, 432, 141, 0}, // a small window: the plain pane's width is the least
	}
	for _, c := range cases {
		l := Compute(c.sw, c.sh, Inputs{Colonists: 3, PaneShown: true, Commands: 8, PaneW: c.paneW, PaneH: c.paneH, GlobalH: 60})
		if l.Pane.W != c.wantW || l.Pane.H != c.wantH {
			t.Errorf("%dx%d: pane %+v, want %v x %v", c.sw, c.sh, l.Pane, c.wantW, c.wantH)
		}
		if l.Pane.Right() > l.Right.X-HudMargin {
			t.Errorf("%dx%d: pane %+v reaches the right column at %v", c.sw, c.sh, l.Pane, l.Right.X)
		}
		if (l.Pane.H+PaneTabsH)*100 > float32(c.sh)*30 && c.sh >= 550 {
			t.Errorf("%dx%d: pane and tabs %v tall, more than 30 %%", c.sw, c.sh, l.Pane.H+PaneTabsH)
		}
		if c.cols == 0 {
			continue
		}
		if l.GridCols != c.cols || len(l.GridCells) != 8 {
			t.Errorf("%dx%d: %d columns, %d cells, want %d and 8", c.sw, c.sh, l.GridCols, len(l.GridCells), c.cols)
		}
		rows := map[float32]bool{}
		for _, g := range l.GridCells {
			rows[g.Y] = true
			if g.X < l.Pane.Right() && g.Y < l.Pane.Bottom() && g.Bottom() > l.PaneTabs.Y {
				t.Errorf("%dx%d: command %+v meets the pane %+v", c.sw, c.sh, g, l.Pane)
			}
		}
		if want := (8 + c.cols - 1) / c.cols; len(rows) != want {
			t.Errorf("%dx%d: %d command rows, want %d", c.sw, c.sh, len(rows), want)
		}
	}
	// a short card pane is never under CardPaneMinH; the plain pane is unchanged
	if l := Compute(1280, 720, Inputs{PaneShown: true, PaneW: 576, PaneH: 90}); l.Pane.H != CardPaneMinH {
		t.Errorf("a 90 card pane: %+v", l.Pane)
	}
	if l := Compute(1280, 720, Inputs{PaneShown: true}); l.Pane.W != PaneW || l.Pane.H != PaneH {
		t.Errorf("the plain pane: %+v", l.Pane)
	}
}
