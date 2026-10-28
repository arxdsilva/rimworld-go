package ebiten

import (
	"fmt"
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"sort"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-018 layout invariants: computeHUDLayout places every region so they
// never overlap, the colonist bar and the command grid wrap inside their
// bands, a window never covers the HUD it must leave visible, and the HUD
// blocks the map exactly where it draws.

// assertChromeIsTheLayout checks the HUD blocks the map exactly over the
// layout's regions (and the learning helper's lesson card, which opens
// beside its region): probes across the screen hit the HUD where a region
// covers them and reach the map everywhere else. No region keeps a legacy
// area of its own.
func assertChromeIsTheLayout(t *testing.T, a *App) {
	t.Helper()
	sw, sh := a.size()
	regions := append(a.hud().Chrome(), a.learnChrome()...)
	const step = 20
	for y := step / 2; y < sh; y += step {
		for x := step / 2; x < sw; x += step {
			in := false
			for _, r := range regions {
				if r.Hit(x, y) {
					in = true
					break
				}
			}
			if got := a.colonyChromeHit(x, y, sw, sh); got != in {
				t.Fatalf("(%d,%d): the HUD hit-test says %v, the layout's regions %v (regions %+v)", x, y, got, in, regions)
			}
		}
	}
}

// assertButtonsInRegions checks every HUD hit target lies inside a layout
// region (or the learning helper's card): no button floats outside the
// regions the HUD draws, where a click would fall through to the map.
func assertButtonsInRegions(t *testing.T, a *App) {
	t.Helper()
	sw, sh := a.size()
	regions := append(a.hud().Chrome(), a.learnChrome()...)
	for _, b := range a.colonyButtons(sw, sh) {
		br := rect{X: b.X, Y: b.Y, W: b.W, H: b.H}
		if br.Empty() {
			continue
		}
		in := false
		for _, r := range regions {
			if br.X >= r.X-0.5 && br.Y >= r.Y-0.5 && br.Right() <= r.Right()+0.5 && br.Bottom() <= r.Bottom()+0.5 {
				in = true
				break
			}
		}
		if !in {
			t.Errorf("button %s %+v lies outside every layout region", b.ID, br)
		}
	}
}

// layoutRegions are a layout's shown regions by name (each command cell
// and each colonist-bar slot its own, the minimap included).
func layoutRegions(l hudLayout) map[string]rect {
	out := map[string]rect{}
	for name, r := range map[string]rect{"Tabs": l.Tabs, "Pane": l.Pane, "PaneTabs": l.PaneTabs, "Expanded": l.Expanded,
		"ArchCats": l.ArchCats, "ArchDesc": l.ArchDesc, "Window": l.Window, "Global": l.Global, "Minimap": l.Minimap,
		"Letters": l.Letters, "Learn": l.Learn, "Alerts": l.Alerts, "Resources": l.Resources, "Messages": l.Messages} {
		if !r.Empty() {
			out[name] = r
		}
	}
	for i, c := range l.GridCells {
		out[fmt.Sprintf("Cmd%02d", i)] = c
	}
	for i, s := range l.BarSlots {
		out[fmt.Sprintf("Slot%02d", i)] = s
	}
	return out
}

// overlaps are the pairs of regions of l that share area (touching edges
// is fine), and the regions that leave the w x h screen.
func layoutOverlaps(l hudLayout, w, h int) []string {
	rs := layoutRegions(l)
	names := make([]string, 0, len(rs))
	for n := range rs {
		names = append(names, n)
	}
	sort.Strings(names)
	var bad []string
	screen := rect{X: 0, Y: 0, W: float32(w), H: float32(h)}
	for i, n := range names {
		r := rs[n]
		if r.X < -0.01 || r.Y < -0.01 || r.Right() > screen.Right()+0.01 || r.Bottom() > screen.Bottom()+0.01 {
			bad = append(bad, fmt.Sprintf("%s %+v leaves the %dx%d screen", n, r, w, h))
		}
		for _, m := range names[i+1:] {
			if o := r.Intersect(rs[m]); o.W > 0.01 && o.H > 0.01 {
				bad = append(bad, fmt.Sprintf("%s %+v overlaps %s %+v", n, r, m, rs[m]))
			}
		}
	}
	return bad
}

// layoutScreens are the logical screens the invariants run on: the
// default window, a big one and a small one.
var layoutScreens = [][2]int{{1280, 720}, {1920, 1080}, {1113, 626}}

// maxArchTools is the size of the Architect's biggest category: the most
// command buttons the Architect lays out.
func maxArchTools() int {
	n := 0
	for _, c := range archCategories {
		n = max(n, len(toolsIn(c.id)))
	}
	return n
}

// layoutStates is a spread of HUD states: every lower-left mode (nothing,
// the pane with and without a tall tab, the Architect with a description,
// a window), with the colonist bar, letters, alerts, messages and the
// resource list full, the minimap and the learning helper on and off.
func layoutStates() []hudState {
	var out []hudState
	for _, minimap := range []bool{false, true} {
		for _, learn := range []float32{0, learnHeaderH, 200} {
			for _, colonists := range []int{1, 6, 30, 90} {
				for _, cmds := range []int{0, 1, 13, 27, 60} {
					base := hudState{
						Colonists: colonists, Commands: cmds, GlobalH: 196, MinimapOn: minimap, LearnH: learn,
						Alerts: 12, Letters: 11, ResourcesH: 600, Messages: 5,
					}
					nothing := base
					nothing.Commands = 0
					pane := base
					pane.PaneShown = true
					tall := pane
					tall.ExpandedW, tall.ExpandedH = expandMaxW+100, 2000
					arch := base // the biggest category, the tallest description
					arch.ArchOpen, arch.Commands, arch.ArchDescH = true, maxArchTools(), 2000
					win := base
					win.WindowW, win.WindowH = 4000, 4000
					out = append(out, nothing, pane, tall, arch, win)
				}
			}
		}
	}
	return out
}

// No two regions of the layout overlap, and none leaves the screen, at
// every screen and every state.
func TestLayoutRegionsNeverOverlap(t *testing.T) {
	t.Parallel()
	for _, sz := range layoutScreens {
		for _, st := range layoutStates() {
			l := computeHUDLayout(sz[0], sz[1], st)
			if bad := layoutOverlaps(l, sz[0], sz[1]); len(bad) > 0 {
				t.Fatalf("%dx%d %+v:\n\t%s", sz[0], sz[1], st, strings.Join(bad, "\n\t"))
			}
		}
	}
}

// busyStub is a colony with every HUD region up: n colonists (the first
// is the one the tests select), alerts (downed colonists, a low mood,
// raiders, a hostile animal, no food), eleven letters, five messages,
// stored goods, and a map big enough for the minimap.
func busyStub(n int) *stubSim {
	s := rightWorld()
	s.w, s.h = 200, 200
	for i := len(s.pawns); i < n; i++ {
		s.pawns = append(s.pawns, game.PawnView{ID: entity.EntityID(100 + i), Name: fmt.Sprintf("Colonist%d", i), X: 10 + i%20, Y: 12 + i/20, Status: "alive", Mood: 60})
	}
	s.world.Letters = nLetters(11)
	for i := range 5 {
		s.world.Messages = append(s.world.Messages, game.MessageView{Tick: uint64(i + 1), Text: fmt.Sprintf("Something happened, number %d.", i)})
	}
	for i := range 12 {
		s.world.Stored = append(s.world.Stored, game.StoredResourceView{Kind: fmt.Sprintf("kind%02d", i), Label: fmt.Sprintf("Kind %02d", i), Category: "Materials", Qty: 10 + i})
	}
	s.world.TerrainW, s.world.Terrain = s.w, make([]uint8, s.w*s.h) // ground for the cell info
	return s
}

// layoutScreen sets the UI scale and a physical screen of about w x h
// logical pixels at it (a scale that does not divide the size exactly
// rounds the logical screen by a pixel), and returns the logical size.
// It changes package globals: never in parallel.
func layoutScreen(t *testing.T, a *App, w, h int, ui float64) (int, int) {
	t.Helper()
	old := gfx.CurrentScale()
	t.Cleanup(func() { gfx.SetScale(old) })
	a.scale = uiScaleState{Dev: 1, UI: ui, S: ui}
	a.syncUIScale()
	a.setPhysical(int(math.Round(float64(w)*ui)), int(math.Round(float64(h)*ui)))
	if abs(a.sw-w) > 1 || abs(a.sh-h) > 1 {
		t.Fatalf("logical screen %dx%d, want about %dx%d", a.sw, a.sh, w, h)
	}
	return a.sw, a.sh
}

// layoutCase is one HUD state of the live invariants.
type layoutCase struct {
	name string
	open func(t *testing.T, a *App)
}

func layoutCases() []layoutCase {
	return []layoutCase{
		{"nothing", func(t *testing.T, a *App) {}},
		{"colonist", func(t *testing.T, a *App) { a.renderer.Select(3) }},
		{"colonist_social", func(t *testing.T, a *App) {
			a.renderer.Select(3)
			if !a.openPaneTab("social") {
				t.Fatal("the Social tab did not open")
			}
		}},
		{"colonist_log", func(t *testing.T, a *App) {
			a.renderer.Select(3)
			if !a.openPaneTab("log") {
				t.Fatal("the Log tab did not open")
			}
		}},
		{"architect", func(t *testing.T, a *App) {
			a.openArchitect()
			a.selectArchCategory("furniture")
			a.selectArchTool("hospital_bed")
		}},
		{"window_work", func(t *testing.T, a *App) { a.applyClick("tab.work") }},
		{"window_history", func(t *testing.T, a *App) { a.applyClick("tab.history") }},
	}
}

// The live layout never overlaps either: at logical 1280x720, 1920x1080
// and 1113x626, UI scales 1, 1.5 and 2, the minimap and the learning
// helper on and off, 6 and 30 colonists, every lower-left mode, with every
// region filled by the real producers.
func TestLiveLayoutRegionsNeverOverlap(t *testing.T) { // not parallel: it sets the UI scale
	for _, n := range []int{6, 30} {
		for _, sz := range layoutScreens {
			for _, ui := range []float64{1, 1.5, 2} {
				for _, minimap := range []bool{true, false} {
					for _, learn := range []bool{true, false} {
						a, f := colonyApp(t, busyStub(n))
						sw, sh := layoutScreen(t, a, sz[0], sz[1], ui)
						a.renderer.mini.Off = !minimap
						a.settings.Gameplay.ShowLearningHelper = learn
						for _, c := range layoutCases() {
							name := fmt.Sprintf("%dcol %dx%d ui%g minimap=%v learn=%v %s", n, sw, sh, ui, minimap, learn, c.name)
							c.open(t, a)
							f.Frame(t, a)
							l := a.hud()
							if !minimap != l.Minimap.Empty() || learn == l.Learn.Empty() {
								t.Fatalf("%s: minimap %+v, learn %+v", name, l.Minimap, l.Learn)
							}
							if bad := layoutOverlaps(l, sw, sh); len(bad) > 0 {
								t.Fatalf("%s:\n\t%s", name, strings.Join(bad, "\n\t"))
							}
							(&visualScene{a: a}).closeAll(t)
						}
					}
				}
			}
		}
	}
}

// Thirteen and twenty-seven commands fill the bottom row from the pane
// rightward, then wrap upward; the grid stops short of the lower-right
// controls (its right edge at most Global.X - 8).
func TestLayoutCommandGridWrapsUpward(t *testing.T) {
	t.Parallel()
	for _, sz := range layoutScreens {
		for _, n := range []int{13, 27} {
			st := hudState{Colonists: 6, PaneShown: true, Commands: n, GlobalH: 196}
			l := computeHUDLayout(sz[0], sz[1], st)
			if len(l.GridCells) == 0 {
				t.Fatalf("%v %d: no cells", sz, n)
			}
			first := l.GridCells[0]
			if first.X != l.Pane.Right()+hudMargin || first.Bottom() != l.Tabs.Y-hudMargin {
				t.Errorf("%v %d: the first cell %+v is not bottom-left of the grid, right of the pane %+v", sz, n, first, l.Pane)
			}
			if want := min(n, l.GridCols*cmdMaxRows); len(l.GridCells) != want {
				t.Errorf("%v %d: %d cells, want %d (%d columns x %d rows)", sz, n, len(l.GridCells), want, l.GridCols, cmdMaxRows)
			}
			if l.GridCols*2 < n && len(l.GridCells) > l.GridCols && l.GridCells[l.GridCols].Y >= first.Y {
				t.Errorf("%v %d: the row after the first does not sit above it", sz, n)
			}
			for i, c := range l.GridCells {
				if c.Right() > l.Global.X-hudMargin {
					t.Errorf("%v %d: cell %d %+v passes Global.X - 8 = %v", sz, n, i, c, l.Global.X-hudMargin)
				}
				if i > 0 && c.Y > l.GridCells[i-1].Y {
					t.Errorf("%v %d: cell %d %+v sits below the one before it %+v (the grid wraps upward)", sz, n, i, c, l.GridCells[i-1])
				}
			}
		}
	}
}

// A main-tab window never covers the colonist bar, the alerts, the
// letters or the lower-right controls, however big it asks to be.
func TestLayoutWindowLeavesTheHUD(t *testing.T) {
	t.Parallel()
	for _, sz := range layoutScreens {
		for _, n := range []int{1, 6, 30, 90} {
			st := hudState{Colonists: n, GlobalH: 196, Alerts: 12, Letters: 3, LearnH: learnHeaderH, WindowW: 5000, WindowH: 5000}
			l := computeHUDLayout(sz[0], sz[1], st)
			if l.Window.Empty() {
				t.Fatalf("%v: no window", sz)
			}
			for name, r := range map[string]rect{"Bar": l.Bar, "Alerts": l.Alerts, "Letters": l.Letters, "Global": l.Global, "Learn": l.Learn, "Tabs": l.Tabs} {
				if r.Empty() {
					t.Fatalf("%v %d: no %s", sz, n, name)
				}
				if o := l.Window.Intersect(r); !o.Empty() {
					t.Errorf("%v %d colonists: Window %+v covers %s %+v", sz, n, l.Window, name, r)
				}
			}
		}
	}
}

// The learning helper folds to its header when a short screen with the
// minimap up leaves no room for its lessons, so it never runs into the
// minimap.
func TestLayoutLearnFoldsWhenCramped(t *testing.T) {
	t.Parallel()
	st := hudState{MinimapOn: true, GlobalH: 240, LearnH: 196}
	l := computeHUDLayout(1113, 626, st)
	if l.Learn.H != learnHeaderH || !l.Learn.Intersect(l.Minimap).Empty() {
		t.Fatalf("Learn %+v beside the minimap %+v: want the header alone, clear of it", l.Learn, l.Minimap)
	}
	if l = computeHUDLayout(1280, 720, st); l.Learn.H != 196 {
		t.Fatalf("Learn %+v: a tall enough screen keeps the lessons", l.Learn)
	}
	// the live helper then lists no lessons and blocks nothing below its header
	a, _ := colonyApp(t, busyStub(6))
	a.settings.Gameplay.ShowLearningHelper = true
	p := rect{X: 900, Y: 8, W: rightW, H: learnHeaderH}
	if !a.learnFolded(p) {
		t.Fatal("a Learn rect shorter than the lessons shows the header alone")
	}
}

// Chrome blocks the map exactly where the HUD draws: the centre of every
// region the layout placed consumes a click; the map beside the last
// command button, and the hovered cell's info (floating text, not
// chrome), let it through.
func TestChromeBlocksDrawnRegionsOnly(t *testing.T) {
	t.Parallel()
	a, f := colonyApp(t, busyStub(6))
	sw, sh := a.size()
	center := func(r rect) (int, int) { x, y := r.Center(); return int(x), int(y) }
	for _, c := range layoutCases() {
		c.open(t, a)
		f.Frame(t, a)
		l := a.hud()
		for name, r := range layoutRegions(l) {
			if name == "Minimap" { // its own click path: it recentres the camera
				continue
			}
			if x, y := center(r); !a.colonyChromeHit(x, y, sw, sh) {
				t.Errorf("%s: the centre of %s %+v does not consume a click", c.name, name, r)
			}
		}
		if n := len(l.GridCells); n > 0 {
			last := l.GridCells[n-1]
			next := last
			next.X += cmdStride
			if next.Right() <= l.Global.X-hudMargin {
				if x, y := center(next); a.colonyChromeHit(x, y, sw, sh) {
					t.Errorf("%s: empty grid space right of the last command blocks the map", c.name)
				}
			}
			above := l.GridCells[0]
			above.Y = l.Grid.Y - cmdStride
			if x, y := center(above); a.colonyChromeHit(x, y, sw, sh) && l.Expanded.Empty() && l.ArchDesc.Empty() {
				t.Errorf("%s: empty grid space above the grid blocks the map", c.name)
			}
		}
		assertChromeIsTheLayout(t, a)
		assertButtonsInRegions(t, a)
		(&visualScene{a: a}).closeAll(t)
	}
	// cell info: with nothing selected the hovered cell's lines float at the
	// lower-left, and a click on them still reaches the map
	f.Cur.Mx, f.Cur.My = sw/2, sh/2
	f.Frame(t, a)
	r := a.cellInfoRect()
	if r.Empty() {
		t.Fatal("no cell info over the map")
	}
	if x, y := center(r); a.colonyChromeHit(x, y, sw, sh) {
		t.Errorf("the cell info %+v blocks the map", r)
	}
}

// A HUD right-click never steals a drafted colonist's move: a right-click
// on the map moves them (even right beside a region), and only a click on
// a region the HUD drew is the HUD's.
func TestHUDRightClickNeverStealsADraftedMove(t *testing.T) {
	t.Parallel()
	s := busyStub(6)
	s.pawns[2].Drafted, s.pawns[2].Status = true, "alive" // Cora
	s.world.Letters = s.world.Letters[:3]                 // room for the alerts too
	a, f := colonyApp(t, s)
	a.renderer.Select(3)
	f.Frame(t, a)
	sw, sh := a.size()
	l := a.hud()
	if len(l.GridCells) == 0 || l.Letters.Empty() || l.Alerts.Empty() {
		t.Fatalf("the HUD is not busy: cells %d letters %+v alerts %+v", len(l.GridCells), l.Letters, l.Alerts)
	}
	// nowhere off the regions does the HUD take a right-click
	regions := append(l.Chrome(), a.learnChrome()...)
	for y := 4; y < sh; y += 12 {
		for x := 4; x < sw; x += 12 {
			in := false
			for _, r := range regions {
				in = in || r.Hit(x, y)
			}
			if !in && a.hudRightClick(x, y) {
				t.Fatalf("(%d,%d): a right-click off the HUD was taken by it", x, y)
			}
		}
	}
	// on the map beside the command grid, the drafted colonist moves
	beside := l.GridCells[len(l.GridCells)-1]
	beside.X += cmdStride
	bx, by := beside.Center()
	s.cmds = nil
	f.Cur.Mx, f.Cur.My = int(bx), int(by)
	f.Click(t, a, mbRight, 0)
	if m := cmdsOf(s, command.MovePawn); len(m) != 1 || m[0].Pawn != 3 {
		t.Fatalf("a right-click on the map beside the grid: moves %+v", m)
	}
	// on a letter row or a command button, nobody moves
	for name, r := range map[string]rect{"letters": l.Letters, "command": l.GridCells[0], "alerts": l.Alerts} {
		s.cmds = nil
		a.closeFloat()
		cx, cy := r.Center()
		f.Cur.Mx, f.Cur.My = int(cx), int(cy)
		f.Click(t, a, mbRight, 0)
		if m := cmdsOf(s, command.MovePawn); len(m) != 0 {
			t.Errorf("a right-click on the %s moved the colonist: %+v", name, m)
		}
	}
}

// A kit window (History, Assign, World, Factions) has no hidden Work cells
// under it: only the hand-drawn windows have legacy buttons, so a click in
// the History window never changes a work priority.
func TestKitWindowHasNoHiddenWorkCells(t *testing.T) {
	t.Parallel()
	s := busyStub(6)
	a, f := colonyApp(t, s)
	a.applyClick("tab.history")
	f.Frame(t, a)
	if !a.windowKitOpen() {
		t.Fatal("the History window is not open")
	}
	for _, b := range a.colonyButtons(a.sw, a.sh) {
		if strings.HasPrefix(b.ID, "work.") {
			t.Fatalf("hidden Work button %s under the History window", b.ID)
		}
	}
	w := a.hud().Window
	for _, p := range [][2]float32{{0.3, 0.4}, {0.5, 0.5}, {0.7, 0.6}} {
		s.cmds = nil
		f.Cur.Mx, f.Cur.My = int(w.X+w.W*p[0]), int(w.Y+w.H*p[1])
		f.Click(t, a, mbLeft, 0)
		if c := cmdsOf(s, command.SetWorkPriority); len(c) != 0 {
			t.Fatalf("a click in the History window set a work priority: %+v", c)
		}
	}
}

// Alerts are ongoing problems the player must act on: a crowded right
// column at 1280x720 (the minimap, a long letter stack, the learning
// helper folded to its header as the app folds it when cramped) still
// leaves them a few rows; the letters give way first (PLAY-018).
func TestAlertsKeepRowsWhenLettersPileUp(t *testing.T) {
	st := hudState{Colonists: 6, Alerts: 5, Letters: 12, LearnH: 28, MinimapOn: true, GlobalH: 196}
	l := computeHUDLayout(1280, 720, st)
	if l.AlertRows < minAlertRows {
		t.Fatalf("alerts got %d rows under 12 letters, want at least %d", l.AlertRows, minAlertRows)
	}
	if l.LetterRows < 1 {
		t.Fatal("the letters vanished entirely")
	}
	if !l.Alerts.Empty() && !l.Letters.Empty() && l.Alerts.Bottom() > l.Letters.Y {
		t.Fatalf("alerts %+v overlap letters %+v", l.Alerts, l.Letters)
	}
	// with few alerts, the letters keep everything else
	st.Alerts = 1
	if l1 := computeHUDLayout(1280, 720, st); l1.LetterRows < l.LetterRows {
		t.Fatalf("one alert took more room than five: letters %d < %d", l1.LetterRows, l.LetterRows)
	}
}

// The learning helper is the least urgent thing on the right: when it is
// open on a short screen (1280x720 with the minimap up), it folds to its
// header before alerts or letters lose their rows.
func TestLearnFoldsBeforeAlertsAndLettersShrink(t *testing.T) {
	st := hudState{Colonists: 6, Alerts: 5, Letters: 12, LearnH: 200, MinimapOn: true, GlobalH: 196}
	l := computeHUDLayout(1280, 720, st)
	if l.Learn.H != learnHeaderH {
		t.Fatalf("learning helper %v tall, want folded to its %v header", l.Learn.H, float32(learnHeaderH))
	}
	if l.AlertRows < minAlertRows || l.LetterRows < 2 {
		t.Fatalf("alerts %d rows, letters %d rows: the folded helper left them too little", l.AlertRows, l.LetterRows)
	}
	// on a tall screen it stays open
	if l2 := computeHUDLayout(1920, 1080, st); l2.Learn.H != 200 {
		t.Fatalf("1080p: helper %v tall, want 200 (open)", l2.Learn.H)
	}
}
