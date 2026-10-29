package ebiten

import (
	"fmt"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/domain/sim/entity"
)

// The inspect pane (PLAY-018, CORE-PANE). The per-kind panes belong to
// other owners, so these tests inject a paneSpec through the test seam.

// setTestPane makes every selection show spec (a spec with no Title is
// "not migrated").
func (a *App) setTestPane(spec paneSpec) {
	a.insp.TestPane = func(selKind, entity.EntityID) paneSpec { return spec }
}

// setTestCommands replaces the per-kind command producers (nil: the real
// ones) and forgets any merged group commands.
func (a *App) setTestCommands(fn func(*App, selKind, entity.EntityID) []cmdButton) {
	a.insp.TestCommands = nil
	if fn != nil {
		a.insp.TestCommands = func(k selKind, id entity.EntityID) []cmdButton { return fn(a, k, id) }
	}
	a.updates++ // a new Update forgets any merged group commands
}

func noCommands(*App, selKind, entity.EntityID) []cmdButton { return nil }

func tabSpec(id, label string, w, h float32, draw func(*ebiten.Image, rect)) paneTab {
	return paneTab{ID: "inspector." + id, Label: label, WantW: w, WantH: h, Draw: draw}
}

func paneColony(t *testing.T, spec paneSpec) (*App, *stubSim, *fakeInput) {
	t.Helper()
	a, s, f := inputColony(t, [2]int{3, 3})
	a.renderer.Select(1)
	a.setTestPane(spec)
	f.Frame(t, a)
	return a, s, f
}

func fullPane(tabs ...paneTab) paneSpec {
	return paneSpec{
		Title:    "Ada",
		Subtitle: "Colonist, 31",
		Bars:     []paneBar{{Label: "Health", Pct: 82, Color: uiGood}, {Label: "Mood", Pct: 55, Color: uiWarning}, {Label: "Food", Pct: 20, Color: uiDanger}},
		Chips:    []paneChip{{ID: "pane.area", Label: "Area: Home zone", Tip: "Where Ada may go"}},
		Lines:    []string{"Doing: hauling wood", "Resting soon"},
		Tabs:     tabs,
		Close:    "inspector.close",
	}
}

func TestPaneShownNeedsATitle(t *testing.T) {
	t.Parallel()
	a, _, _ := inputColony(t, [2]int{3, 3})
	a.renderer.Select(1)
	// every selection kind has its pane (PLAY-018): the colonist's is up
	if !a.paneShown() || a.hud().Pane.Empty() {
		t.Fatal("the colonist's pane is shown in the layout")
	}
	// a spec without a Title shows nothing, and no legacy top-left panel
	// (it sat at 8,40) is left blocking the map in its place
	a.setTestPane(paneSpec{})
	if a.paneShown() || !a.hud().Pane.Empty() {
		t.Fatal("a pane without a Title is not shown")
	}
	for _, r := range a.chromeRects(a.sw, a.sh) {
		if r.X == 8 && r.Y == 40 {
			t.Fatalf("the legacy inspector rect %+v still blocks the map", r)
		}
	}
	a.setTestPane(fullPane())
	if !a.paneShown() || a.hud().Pane.Empty() {
		t.Fatal("a pane with a Title is shown in the layout")
	}
	a.arch.Open = true
	if a.paneShown() {
		t.Fatal("the Architect takes the lower-left from the pane")
	}
	a.arch.Open = false
	a.win.Open = true
	if a.paneShown() {
		t.Fatal("a main-tab window hides the pane")
	}
}

func TestPaneTabOpensAndCollapses(t *testing.T) {
	t.Parallel()
	a, _, _ := paneColony(t, fullPane(
		tabSpec("needs", "Needs", 480, 220, nil),
		tabSpec("log", "Log", 300, 300, nil),
	))
	if !a.hud().Expanded.Empty() || a.paneOpenTab() != "" {
		t.Fatal("tabs start collapsed")
	}
	click(t, a, "inspector.needs")
	l := a.hud()
	if a.paneOpenTab() != "inspector.needs" || l.Expanded.W != 480 || l.Expanded.H != 220 {
		t.Fatalf("clicking a closed tab opens it at its wanted size: open=%q expanded=%+v", a.paneOpenTab(), l.Expanded)
	}
	if l.Expanded.Bottom() != l.PaneTabs.Y {
		t.Fatalf("the panel opens upward from the tabs: %+v over %+v", l.Expanded, l.PaneTabs)
	}
	click(t, a, "inspector.log")
	if l := a.hud(); a.paneOpenTab() != "inspector.log" || l.Expanded.W != expandMinW || l.Expanded.H != 300 {
		t.Fatalf("another tab replaces it (width clamped to the minimum): %q %+v", a.paneOpenTab(), l.Expanded)
	}
	click(t, a, "inspector.log")
	if a.paneOpenTab() != "" || !a.hud().Expanded.Empty() {
		t.Fatal("clicking the open tab collapses it")
	}
}

func TestPaneTabsAreMeasuredAndReportTheirState(t *testing.T) {
	t.Parallel()
	spec := fullPane(
		tabSpec("a", "Go", 400, 100, nil),
		tabSpec("b", "A rather long tab label", 400, 100, nil),
		paneTab{ID: "inspector.c", Label: "Gear", Disabled: true, Reason: "nobody to dress"},
	)
	a, _, _ := paneColony(t, spec)
	l := a.hud()
	var tabs []button
	for _, b := range a.paneButtons() {
		if strings.HasPrefix(b.ID, "inspector.") && b.ID != "inspector.close" {
			tabs = append(tabs, b)
		}
	}
	if len(tabs) != 3 {
		t.Fatalf("tab buttons = %+v", tabs)
	}
	for i, b := range tabs {
		r := rect{X: b.X, Y: b.Y, W: b.W, H: b.H}
		if r.X < l.PaneTabs.X || r.Right() > l.PaneTabs.Right()+0.01 || r.Y < l.PaneTabs.Y || r.Bottom() > l.PaneTabs.Bottom()+0.01 {
			t.Errorf("tab %d %+v leaves the tab strip %+v", i, r, l.PaneTabs)
		}
		if i > 0 && b.X < tabs[i-1].X+tabs[i-1].W {
			t.Errorf("tab %d overlaps tab %d", i, i-1)
		}
	}
	if !(tabs[1].W > tabs[0].W) {
		t.Errorf("widths follow the label: %.0f vs %.0f", tabs[0].W, tabs[1].W)
	}
	if w, _ := measure("A rather long tab label", tsBody); float64(tabs[1].W) < w {
		t.Errorf("tab width %.0f is narrower than its label (%.0f)", tabs[1].W, w)
	}
	if !tabs[2].Disabled || tabs[2].Reason != "nobody to dress" {
		t.Errorf("a disabled tab reports its reason: %+v", tabs[2])
	}
	click(t, a, "inspector.c")
	if a.paneOpenTab() != "" || a.hint != "nobody to dress" {
		t.Fatalf("a disabled tab does not open: open=%q hint=%q", a.paneOpenTab(), a.hint)
	}
	click(t, a, "inspector.a")
	a2 := a.paneButtons()
	sel := 0
	for _, b := range a2 {
		if b.Selected {
			sel++
		}
	}
	if sel != 1 {
		t.Fatalf("the open tab is drawn selected, got %d selected buttons", sel)
	}
}

func TestEscapeClosesTheOpenTabFirst(t *testing.T) {
	t.Parallel()
	a, _, _ := paneColony(t, fullPane(tabSpec("needs", "Needs", 480, 220, nil)))
	if a.closePaneTab() {
		t.Fatal("nothing is open")
	}
	click(t, a, "inspector.needs")
	if !a.closePaneTab() || a.paneOpenTab() != "" {
		t.Fatal("Escape closes the open tab")
	}
	if a.closePaneTab() {
		t.Fatal("and only the tab")
	}
}

func TestOpenTabIsDroppedWhenItVanishes(t *testing.T) {
	t.Parallel()
	a, _, f := paneColony(t, fullPane(tabSpec("needs", "Needs", 480, 220, nil)))
	click(t, a, "inspector.needs")
	a.setTestPane(fullPane(tabSpec("log", "Log", 400, 100, nil))) // another kind: no Needs tab
	f.Frame(t, a)
	if a.paneOpenTab() != "" || !a.hud().Expanded.Empty() {
		t.Fatal("a tab the new selection does not have must not stay open")
	}
	a.setTestPane(paneSpec{})
	f.Frame(t, a)
	if a.hud().PaneTabs.W != 0 {
		t.Fatal("no pane, no tabs")
	}
}

func TestExpandedTabDrawsInsideItsPanel(t *testing.T) {
	t.Parallel()
	var got rect
	var clip rect
	draw := func(screen *ebiten.Image, r rect) {
		got = r
		b := screen.Bounds()
		clip = rect{X: float32(b.Min.X), Y: float32(b.Min.Y), W: float32(b.Dx()), H: float32(b.Dy())}
	}
	a, _, _ := paneColony(t, fullPane(tabSpec("needs", "Needs", 520, 240, draw)))
	click(t, a, "inspector.needs")
	l := a.hud()
	a.drawPane(ebiten.NewImage(1280, 720))
	inner := l.Expanded.Inset(paneInset)
	if got != inner {
		t.Fatalf("the tab body gets the panel inset by %d: got %+v, want %+v", paneInset, got, inner)
	}
	if clip.X < l.Expanded.X || clip.Y < l.Expanded.Y || clip.Right() > l.Expanded.Right()+1 || clip.Bottom() > l.Expanded.Bottom()+1 {
		t.Fatalf("the body draws on a surface clipped to %+v, outside the panel %+v", clip, l.Expanded)
	}
}

func TestExpandedTabDeclaresItsKitWidgets(t *testing.T) {
	t.Parallel()
	var got rect
	spec := fullPane(paneTab{ID: "inspector.kit", Label: "Kit", WantW: 500, WantH: 200, Declare: func(r rect) { got = r }})
	a, _, f := paneColony(t, spec)
	click(t, a, "inspector.kit")
	f.Frame(t, a)
	if want := a.hud().Expanded.Inset(paneInset); got != want {
		t.Fatalf("Declare got %+v, want %+v", got, want)
	}
}

func TestExpandedFollowsTheLayoutCaps(t *testing.T) {
	t.Parallel()
	a, _, _ := paneColony(t, fullPane(tabSpec("huge", "Huge", 900, 5000, nil)))
	click(t, a, "inspector.huge")
	l := a.hud()
	if l.Expanded.W != expandMaxW {
		t.Fatalf("width is clamped to %d: %+v", expandMaxW, l.Expanded)
	}
	if l.Expanded.Y < l.Bar.Bottom() {
		t.Fatalf("the panel stops below the colonist bar: %+v vs bar %+v", l.Expanded, l.Bar)
	}
	if a.expandedRect() != l.Expanded || a.paneRect() != l.Pane {
		t.Fatal("paneRect and expandedRect are the layout's")
	}
}

func TestPaneButtonsStayInsideThePane(t *testing.T) {
	t.Parallel()
	spec := fullPane(tabSpec("a", "A", 400, 100, nil))
	spec.Chips = append(spec.Chips,
		paneChip{ID: "pane.c2", Label: "Second chip"},
		paneChip{ID: "pane.c3", Label: "A third chip with a rather long label"},
		paneChip{ID: "pane.c4", Label: "Fourth"},
	)
	a, _, _ := paneColony(t, spec)
	l := a.hud()
	ids := map[string]bool{}
	for _, b := range a.paneButtons() {
		ids[b.ID] = true
		if strings.HasPrefix(b.ID, "inspector.") && b.ID != "inspector.close" {
			continue // tabs live in the strip above
		}
		r := rect{X: b.X, Y: b.Y, W: b.W, H: b.H}
		if r.X < l.Pane.X || r.Y < l.Pane.Y || r.Right() > l.Pane.Right()+0.01 || r.Bottom() > l.Pane.Bottom()+0.01 {
			t.Errorf("%s %+v leaves the pane %+v", b.ID, r, l.Pane)
		}
	}
	for _, id := range []string{"inspector.close", "pane.area", "pane.c2", "pane.c3", "pane.c4"} {
		if !ids[id] {
			t.Errorf("no %s button", id)
		}
	}
	for _, b := range a.paneButtons() {
		if b.ID == "pane.area" && b.Tip != "Where Ada may go" {
			t.Errorf("chip tip = %q", b.Tip)
		}
	}
}

func TestPaneBodyScrollsAndOwnsTheWheel(t *testing.T) {
	t.Parallel()
	spec := fullPane()
	for i := range 40 {
		spec.Lines = append(spec.Lines, fmt.Sprintf("Line %d of a long history", i))
	}
	a, _, f := paneColony(t, spec)
	l := a.hud()
	g := paneLayout(l.Pane, spec)
	if g.ContentH <= g.Body.H {
		t.Fatalf("40 lines must overflow the %v px body, content %v", g.Body.H, g.ContentH)
	}
	z := a.renderer.cam.Zoom
	a.renderer.smooth.Instant = true
	f.Cur.Mx, f.Cur.My = int(g.Body.X+20), int(g.Body.Y+20)
	f.Cur.WheelY = -1
	f.Frame(t, a)
	if off := a.uiSt().Scroll[paneBodyID]; off <= 0 {
		t.Fatalf("the wheel over the body scrolls it, offset %v", off)
	}
	if a.renderer.cam.Zoom != z {
		t.Fatal("the camera must not zoom while the pane has the wheel")
	}
	// anywhere on the pane keeps the wheel from the camera
	f.Cur.Mx, f.Cur.My = int(l.Pane.X+20), int(l.Pane.Y+10)
	f.Cur.WheelY = 1
	f.Frame(t, a)
	if a.renderer.cam.Zoom != z {
		t.Fatal("the wheel over the pane header must not zoom the map")
	}
}

func TestPaneRowsAreClickableAndScroll(t *testing.T) {
	t.Parallel()
	spec := paneSpec{Title: "6 colonists", Close: "group.close"}
	for i := range 30 {
		spec.Rows = append(spec.Rows, paneRow{ID: fmt.Sprintf("pane.row.%d", i), Label: fmt.Sprintf("Row %d", i), Tip: "pick"})
	}
	a, _, _ := paneColony(t, spec)
	l := a.hud()
	g := paneLayout(l.Pane, spec)
	vis := map[string]button{}
	for _, b := range a.paneButtons() {
		if strings.HasPrefix(b.ID, "pane.row.") {
			vis[b.ID] = b
		}
	}
	if len(vis) == 0 || len(vis) >= 30 {
		t.Fatalf("only the rows in view are targets: %d of 30", len(vis))
	}
	for id, b := range vis {
		if b.Y < g.Body.Y-0.01 || b.Y+b.H > g.Body.Bottom()+0.01 {
			t.Errorf("%s %+v leaves the body %+v", id, b, g.Body)
		}
	}
	if _, ok := vis["pane.row.0"]; !ok {
		t.Fatal("the first row is in view")
	}
	a.uiSt().Scroll[paneBodyID] = 10 * paneRowH
	again := map[string]button{}
	for _, b := range a.paneButtons() {
		if strings.HasPrefix(b.ID, "pane.row.") {
			again[b.ID] = b
		}
	}
	if _, ok := again["pane.row.0"]; ok {
		t.Fatal("scrolled past, row 0 is not a target")
	}
	if _, ok := again["pane.row.12"]; !ok {
		t.Fatalf("scrolled to row 10, row 12 is in view: %v", again)
	}
}

func TestDrawPaneStaysInItsRegions(t *testing.T) {
	// serial: it installs the canvas trace and changes the UI scale
	for _, scale := range []float64{1, 1.5} {
		draw := func(screen *ebiten.Image, r rect) {
			uiFillRect(screen, r.X, r.Y, r.W, r.H, uiBtn, false)
		}
		spec := fullPane(tabSpec("needs", "Needs", 520, 240, draw), tabSpec("log", "Log", 400, 100, nil))
		for i := range 30 {
			spec.Lines = append(spec.Lines, fmt.Sprintf("A long line of text number %d that has to wrap inside the narrow body", i))
		}
		a, _, _ := paneColony(t, spec) // built at scale 1: an Update resets the scale
		if scale != 1 {
			withUIScale(t, scale)
		}
		clickDirect(t, a, "inspector.needs") // a mouse click's Update would reset the forced scale; this test is about the drawing
		l := a.hud()
		regions := []rect{l.Pane, l.PaneTabs, l.Expanded}
		var rects []rect
		gfx.SetCanvasTrace(func(_ string, x, y, w, h float32) { rects = append(rects, rect{X: x, Y: y, W: w, H: h}) })
		a.drawPane(ebiten.NewImage(int(1280*scale), int(720*scale)))
		gfx.SetCanvasTrace(nil)
		if len(rects) == 0 {
			t.Fatalf("scale %v: the pane drew nothing", scale)
		}
		s := float32(scale)
		for _, r := range rects {
			ok := false
			for _, g := range regions {
				if r.X >= g.X*s-0.5 && r.Y >= g.Y*s-0.5 && r.Right() <= g.Right()*s+0.5 && r.Bottom() <= g.Bottom()*s+0.5 {
					ok = true
				}
			}
			if !ok {
				t.Errorf("scale %v: draw %+v is outside Pane, PaneTabs and Expanded %+v", scale, r, regions)
			}
		}
	}
}

func TestPaneLayoutDropsWhatDoesNotFit(t *testing.T) {
	t.Parallel()
	spec := fullPane()
	for i := range 30 {
		spec.Chips = append(spec.Chips, paneChip{ID: fmt.Sprintf("c%d", i), Label: fmt.Sprintf("Chip number %d", i)})
	}
	p := rect{X: 0, Y: 500, W: paneW, H: paneH}
	g := paneLayout(p, spec)
	for i, c := range g.Chips {
		if c.Bottom() > p.Bottom()-4+0.01 {
			t.Errorf("chip %d %+v runs out of the pane", i, c)
		}
	}
	if !g.Body.Empty() && g.Body.Bottom() > p.Bottom() {
		t.Errorf("body %+v runs out of the pane", g.Body)
	}
}

// selectPaneTab opens the selection's own tab (the colonist's pane is
// real, not a test spec); a pane without that tab ignores the click.
func TestSelectPaneTabOpensTheSelectionsTab(t *testing.T) {
	t.Parallel()
	a, _, _ := inputColony(t, [2]int{3, 3})
	a.renderer.Select(1)
	a.selectPaneTab("inspector.needs")
	if a.insp.Tab != "needs" || a.paneOpenTab() != "inspector.needs" {
		t.Fatalf("the colonist's Needs tab: inspectTab=%q open=%q", a.insp.Tab, a.paneOpenTab())
	}
	a.closePaneTab()
	a.setTestPane(paneSpec{})
	a.selectPaneTab("inspector.needs")
	if a.insp.Tab != "character" || a.paneOpenTab() != "" {
		t.Fatalf("a pane without the tab took the click: inspectTab=%q open=%q", a.insp.Tab, a.paneOpenTab())
	}
}

func TestDrawPaneBodyWithRowsAndAMenu(t *testing.T) {
	t.Parallel()
	spec := paneSpec{Title: "3 colonists", Lines: []string{"first", "second"}, Close: "group.close"}
	for i := range 12 {
		spec.Rows = append(spec.Rows, paneRow{ID: fmt.Sprintf("r%d", i), Label: fmt.Sprintf("Row %d", i)})
	}
	a, _, _ := paneColony(t, spec)
	img := ebiten.NewImage(1280, 720)
	g := paneLayout(a.hud().Pane, spec)
	a.cursor = &[2]int{int(g.Body.X) + 10, int(g.Body.Y+g.RowsY0) + 5} // hover the first row
	a.uiSt().Scroll[paneBodyID] = paneRowH
	a.drawPane(img)
	a.openPick(300, 300, false, pickEntries(), func(int) {})
	a.drawFloat(img) // the pick menu draws without the queue footer
}
