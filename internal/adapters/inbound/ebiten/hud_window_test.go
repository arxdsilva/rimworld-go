package ebiten

import (
	"fmt"
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// PLAY-018 WINDOWS: the 11-tab strip and the main-tab windows (Work,
// Schedule, Research, Animals, Wildlife, Assign, History, Factions, World)
// that open bottom-left in the layout's Window rect.

// winStub is a colony with something in every window: n colonists, tame
// animals, wild species, research projects (one under way), history and
// factions.
func winStub(n int) *stubSim {
	s := &stubSim{}
	for i := 0; i < n; i++ {
		s.pawns = append(s.pawns, game.PawnView{ID: entity.EntityID(i + 1), Name: fmt.Sprintf("Colonist%d", i+1), X: 4 + i%8, Y: 4 + i/8, Status: "alive"})
	}
	for i := 0; i < 6; i++ {
		s.world.Animals = append(s.world.Animals, game.AnimalView{ID: entity.EntityID(50 + i), Species: "dustbuck", Label: "Dustbuck",
			Name: fmt.Sprintf("Pet%d", i), Tame: true, Adult: true, Health: 100, Food: 60, AgeDays: 40, Mode: "wandering", AreaRef: -1})
	}
	for i := 0; i < 8; i++ {
		s.world.Research = append(s.world.Research, game.ResearchView{ID: uint8(i), Name: fmt.Sprintf("proj%d", i), Label: fmt.Sprintf("Project %d", i),
			Desc: "A project", Cost: 100, Progress: 10 * i, Available: true, Current: i == 3, Unlocks: []string{"thing"}})
	}
	s.wild = []game.WildlifeRow{
		{Species: "deer", Label: "Deer", Count: 4, Diet: "plants", TameChance: 40, Meat: 30},
		{Species: "wolf", Label: "Wolf", Count: 2, Diet: "meat", Predator: true, TameChance: 5, Meat: 40},
		{Species: "boar", Label: "Boar", Count: 3, Diet: "plants", TameChance: 20, Meat: 35},
	}
	for i := 0; i < 40; i++ {
		s.history = append(s.history, game.HistoryEntryView{Tick: uint64(100 * i), Text: fmt.Sprintf("Something happened %d.", i), Kind: "colony"})
	}
	s.factions = factionViews()
	s.worldMap = game.WorldMapView{Tile: 7, Caravans: []game.CaravanView{{ID: 40, Members: []string{"Cora"}, Tile: 8, Dest: 9, Progress: 25, State: "Travelling"}}}
	return s
}

// winCase is one main-tab window: how it opens, what is open when it does,
// and the widget/button id prefixes its contents use.
type winCase struct {
	name     string
	tab      string // the tab strip button id
	kit      string // the kit modal id (assign, history, factions, world), "" for the legacy windows
	prefixes []string
	open     func(a *App)
}

func winCases() []winCase {
	click := func(id string) func(a *App) { return func(a *App) { a.applyClick(id) } }
	return []winCase{
		{"work", "tab.work", "", []string{"work."}, click("tab.work")},
		{"schedule", "tab.schedule", "", []string{"sched."}, click("tab.schedule")},
		{"research", "tab.research", "", []string{"research."}, click("tab.research")},
		{"animals", "tab.animals", "", []string{"animals."}, click("tab.animals")},
		{"wildlife", "tab.wildlife", "", []string{"animals."}, click("tab.wildlife")},
		{"assign", "tab.assign", assignModalID, []string{"assign.", "modal." + assignModalID}, func(a *App) { a.openAssign() }},
		{"history", "tab.history", historyModalID, []string{"history.", "modal." + historyModalID}, func(a *App) { a.openHistory() }},
		{"factions", "tab.factions", colFacModalID, []string{"fac.", "colfac.", "modal." + colFacModalID}, func(a *App) { a.openColonyFactions() }},
		{"world", "tab.world", worldModalID, []string{"world.", "modal." + worldModalID}, func(a *App) { a.openWorld() }},
	}
}

func winApp(t *testing.T, s *stubSim) (*App, *fakeInput) {
	t.Helper()
	a, f := colonyApp(t, s)
	f.Cur.Mx, f.Cur.My = -50, -50
	return a, f
}

// ---- the tab strip ------------------------------------------------------

func TestTabStripHasElevenEqualTabs(t *testing.T) {
	t.Parallel()
	a, _ := winApp(t, winStub(3))
	for _, size := range [][2]int{{1280, 720}, {1920, 1080}, {1113, 626}} {
		a.sw, a.sh = size[0], size[1]
		bs := a.hudTabButtons(a.sw, a.sh)
		want := []string{"tab.architect", "tab.work", "tab.schedule", "tab.assign", "tab.animals", "tab.wildlife", "tab.research", "tab.world", "tab.history", "tab.factions", "tab.menu"}
		if len(bs) != len(want) {
			t.Fatalf("%v: %d tabs, want %d", size, len(bs), len(want))
		}
		w := float32(a.sw) / float32(len(want))
		for i, b := range bs {
			if b.ID != want[i] {
				t.Errorf("%v: tab %d is %q, want %q", size, i, b.ID, want[i])
			}
			if math.Abs(float64(b.W-w)) > 0.01 || math.Abs(float64(b.X-float32(i)*w)) > 0.01 {
				t.Errorf("%v: %s is x %v w %v, want x %v w %v", size, b.ID, b.X, b.W, float32(i)*w, w)
			}
			if b.Y != float32(a.sh-tabsH) || b.H != tabsH {
				t.Errorf("%v: %s is y %v h %v, want y %d h %d", size, b.ID, b.Y, b.H, a.sh-tabsH, tabsH)
			}
		}
		if last := bs[len(bs)-1]; math.Abs(float64(last.X+last.W-float32(a.sw))) > 0.01 {
			t.Errorf("%v: the strip ends at %v, want the screen's edge", size, last.X+last.W)
		}
	}
}

func TestTabStripIsTheLayoutsTabsRect(t *testing.T) {
	t.Parallel()
	a, _ := winApp(t, winStub(3))
	tabs := a.hud().Tabs
	if tabs != (rect{X: 0, Y: float32(a.sh - 28), W: float32(a.sw), H: 28}) {
		t.Fatalf("Tabs = %+v", tabs)
	}
	for _, b := range a.hudTabButtons(a.sw, a.sh) {
		if !(rect{X: b.X, Y: b.Y, W: b.W, H: b.H}).Intersect(tabs).Empty() && (b.Y != tabs.Y || b.H != tabs.H) {
			t.Errorf("%s leaves the Tabs rect: %+v", b.ID, b)
		}
	}
	assertChromeIsTheLayout(t, a) // the layout covers the strip
	a.applyClick("tab.work")
	assertChromeIsTheLayout(t, a) // and the window
}

func TestTabStripShowsKeyHints(t *testing.T) {
	t.Parallel()
	a, _ := winApp(t, winStub(1))
	tips := map[string]string{}
	for _, b := range a.hudTabButtons(a.sw, a.sh) {
		tips[b.ID] = b.Tip
	}
	for id, key := range map[string]string{"tab.work": "[F1]", "tab.animals": "[F4]", "tab.wildlife": "[F5]", "tab.research": "[F6]", "tab.history": "[F9]"} {
		if !strings.Contains(tips[id], key) {
			t.Errorf("%s tip %q lacks %s", id, tips[id], key)
		}
	}
	if !strings.HasPrefix(tips["tab.wildlife"], "Wildlife") {
		t.Errorf("Wildlife tip %q", tips["tab.wildlife"])
	}
}

// Labels shrink with an ellipsis when a tab is narrow; they never run
// into the next tab.
func TestTabLabelsEllipsizeWhenNarrow(t *testing.T) {
	a, _ := winApp(t, winStub(1))
	sw, sh := 640, 360 // 58 px a tab
	a.sw, a.sh = sw, sh
	img := ebiten.NewImage(sw, sh)
	var ev []traceEvent
	gfx.SetDrawTrace(func(e traceEvent) { ev = append(ev, e) })
	defer func() { gfx.SetDrawTrace(nil) }()
	a.drawTabs(img, sw, sh)
	gfx.SetDrawTrace(nil)
	bs := a.hudTabButtons(sw, sh)
	shortened := 0
	for _, e := range ev {
		if e.Kind != "text" {
			continue
		}
		in := false
		for _, b := range bs {
			if e.X >= b.X-0.5 && e.X+e.W <= b.X+b.W+0.5 && e.Y >= b.Y-0.5 && e.Y+e.H <= b.Y+b.H+0.5 {
				in = true
			}
		}
		if strings.HasSuffix(e.Text, "…") {
			shortened++
		}
		if !in {
			t.Errorf("label %q at %.1f,%.1f %.1fx%.1f is outside every tab", e.Text, e.X, e.Y, e.W, e.H)
		}
	}
	if shortened == 0 {
		t.Error("no label was ellipsized at 58 px a tab")
	}
}

// Research shows a thin fill for the project under way.
func TestResearchTabShowsProgressFill(t *testing.T) {
	s := winStub(1)
	s.world.Research[3].Progress, s.world.Research[3].Cost = 50, 100
	a, _ := winApp(t, s)
	img := ebiten.NewImage(a.sw, a.sh)
	fills := func() []traceEvent {
		var ev []traceEvent
		gfx.SetDrawTrace(func(e traceEvent) { ev = append(ev, e) })
		defer func() { gfx.SetDrawTrace(nil) }()
		a.drawTabs(img, a.sw, a.sh)
		var out []traceEvent
		for _, b := range a.hudTabButtons(a.sw, a.sh) {
			if b.ID != "tab.research" {
				continue
			}
			for _, e := range ev {
				if e.Kind == "fill" && e.H <= 4 && e.X >= b.X && e.X+e.W <= b.X+b.W+0.5 && e.Y >= b.Y+b.H-8 {
					out = append(out, e)
				}
			}
		}
		return out
	}
	got := fills()
	if len(got) < 2 { // the track and the fill
		t.Fatalf("research tab draws %d thin fills, want a track and a fill: %v", len(got), got)
	}
	track, fill := got[0], got[len(got)-1]
	if fill.W <= 0 || math.Abs(float64(fill.W/track.W)-0.5) > 0.05 {
		t.Errorf("fill %.1f of a %.1f track, want half", fill.W, track.W)
	}
	s.world.Research[3].Current = false
	if n := len(fills()); n != 0 {
		t.Errorf("no project under way, yet %d thin fills", n)
	}
}

// ---- toggling -----------------------------------------------------------

func TestWildlifeTabOpensTheWildlifeList(t *testing.T) {
	t.Parallel()
	a, f := winApp(t, winStub(2))
	click(t, a, "tab.wildlife")
	f.Frame(t, a)
	if !a.win.Open || a.win.Mode != "animals" || !a.win.Wild {
		t.Fatalf("Wildlife tab: open %v mode %q wild %v", a.win.Open, a.win.Mode, a.win.Wild)
	}
	sel := map[string]bool{}
	for _, b := range a.hudTabButtons(a.sw, a.sh) {
		sel[b.ID] = b.Selected
	}
	if !sel["tab.wildlife"] || sel["tab.animals"] {
		t.Fatalf("selected tabs %v: only Wildlife", sel)
	}
	if w := a.hud().Window; w.Empty() {
		t.Fatal("the Wildlife list opens in the Window rect")
	}
	click(t, a, "animals.focus.deer") // the list is the wild species table
	// clicking the open tab again closes it
	click(t, a, "tab.wildlife")
	if a.win.Open {
		t.Fatal("the open Wildlife tab closes it")
	}
}

func TestAnimalsTabWhileWildlifeIsOpenShowsTheColony(t *testing.T) {
	t.Parallel()
	a, f := winApp(t, winStub(2))
	click(t, a, "tab.wildlife")
	f.Frame(t, a)
	click(t, a, "tab.animals") // switching tabs replaces the window; it never just closes it
	if !a.win.Open || a.win.Mode != "animals" || a.win.Wild {
		t.Fatalf("Animals over Wildlife: open %v mode %q wild %v", a.win.Open, a.win.Mode, a.win.Wild)
	}
	sel := map[string]bool{}
	for _, b := range a.hudTabButtons(a.sw, a.sh) {
		sel[b.ID] = b.Selected
	}
	if !sel["tab.animals"] || sel["tab.wildlife"] {
		t.Fatalf("selected tabs %v: only Animals", sel)
	}
	click(t, a, "tab.animals")
	if a.win.Open {
		t.Fatal("the open Animals tab closes it")
	}
	click(t, a, "tab.animals")
	click(t, a, "tab.wildlife") // and Wildlife replaces Animals
	if !a.win.Open || !a.win.Wild {
		t.Fatalf("Wildlife over Animals: open %v wild %v", a.win.Open, a.win.Wild)
	}
}

// Switching tabs replaces the window and clicking the open tab closes it,
// for every pair of windows.
func TestTabsReplaceAndToggleWindows(t *testing.T) {
	t.Parallel()
	cases := winCases()
	for _, from := range cases {
		for _, to := range cases {
			t.Run(from.name+"_to_"+to.name, func(t *testing.T) {
				a, f := winApp(t, winStub(3))
				click(t, a, from.tab)
				f.Frame(t, a)
				if !a.windowOpenFor(from.tab) {
					t.Fatalf("%s tab did not open its window", from.name)
				}
				if from.tab == to.tab {
					click(t, a, to.tab)
					f.Frame(t, a)
					if a.win.Open || a.uiModalTop() != "" {
						t.Fatalf("the open %s tab should close its window (open %v, mode %q, top %q)", from.name, a.win.Open, a.win.Mode, a.uiModalTop())
					}
					return
				}
				click(t, a, to.tab)
				f.Frame(t, a)
				if !a.windowOpenFor(to.tab) {
					t.Fatalf("%s tab did not replace %s (mode %q top %q)", to.name, from.name, a.win.Mode, a.uiModalTop())
				}
				if a.windowOpenFor(from.tab) {
					t.Fatalf("%s is still open next to %s", from.name, to.name)
				}
			})
		}
	}
}

// windowOpenFor reports the window of tab is the one open.
func (a *App) windowOpenFor(tab string) bool {
	for _, c := range winCases() {
		if c.tab != tab {
			continue
		}
		if c.kit != "" {
			return a.uiModalTop() == c.kit && a.win.Open
		}
		if !a.win.Open || a.uiModalTop() != "" {
			return false
		}
		switch tab {
		case "tab.work":
			return a.win.Mode == "work"
		case "tab.schedule":
			return a.win.Mode == "schedule"
		case "tab.research":
			return a.win.Mode == "research"
		case "tab.animals":
			return a.win.Mode == "animals" && !a.win.Wild
		case "tab.wildlife":
			return a.win.Mode == "animals" && a.win.Wild
		}
	}
	return false
}

func TestEscapeClosesEveryWindow(t *testing.T) {
	t.Parallel()
	for _, c := range winCases() {
		t.Run(c.name, func(t *testing.T) {
			a, f := winApp(t, winStub(3))
			c.open(a)
			f.Frame(t, a)
			if !a.win.Open {
				t.Fatalf("%s did not open", c.name)
			}
			f.Key(t, a, "Escape", 0)
			if a.win.Open || a.uiModalTop() != "" {
				t.Fatalf("Escape left %s open (open %v, top %q)", c.name, a.win.Open, a.uiModalTop())
			}
			if a.nav.overlay() != overlayNone {
				t.Fatal("the Escape that closes a window must not pause")
			}
		})
	}
}

// The documented order holds with a window open: window, then selection,
// then pause.
func TestEscapeOrderWindowSelectionPause(t *testing.T) {
	t.Parallel()
	a, f := winApp(t, winStub(3))
	a.renderer.Select(1)
	a.openAssign()
	f.Frame(t, a)
	f.Key(t, a, "Escape", 0)
	if a.win.Open || !a.renderer.HasSelect() {
		t.Fatalf("first Escape: window open %v selection %v", a.win.Open, a.renderer.HasSelect())
	}
	f.Key(t, a, "Escape", 0)
	if a.renderer.HasSelect() || a.nav.overlay() != overlayNone {
		t.Fatalf("second Escape: selection %v overlay %v", a.renderer.HasSelect(), a.nav.overlay())
	}
	f.Key(t, a, "Escape", 0)
	if a.nav.overlay() != overlayPause {
		t.Fatalf("third Escape: overlay %v, want pause", a.nav.overlay())
	}
}

// The window keys toggle too, and they replace the open window.
func TestWindowKeysToggleAndReplace(t *testing.T) {
	t.Parallel()
	a, f := winApp(t, winStub(3))
	f.Key(t, a, "F3", 0)
	if a.uiModalTop() != assignModalID {
		t.Fatalf("F3 top %q", a.uiModalTop())
	}
	f.Key(t, a, "F9", 0)
	if a.uiModalTop() != historyModalID {
		t.Fatalf("F9 over Assign: top %q", a.uiModalTop())
	}
	f.Key(t, a, "F9", 0)
	if a.uiModalTop() != "" || a.win.Open {
		t.Fatalf("F9 again: top %q open %v", a.uiModalTop(), a.win.Open)
	}
	f.Key(t, a, "F1", 0)
	f.Key(t, a, "F3", 0)
	if a.uiModalTop() != assignModalID {
		t.Fatalf("F3 over Work: top %q", a.uiModalTop())
	}
	f.Key(t, a, "F5", 0)
	if !a.win.Open || a.win.Mode != "animals" || !a.win.Wild || a.uiModalTop() != "" {
		t.Fatalf("F5 over Assign: open %v mode %q wild %v top %q", a.win.Open, a.win.Mode, a.win.Wild, a.uiModalTop())
	}
}

func TestWindowCloseClick(t *testing.T) {
	t.Parallel()
	a, f := winApp(t, winStub(3))
	a.openHistory()
	f.Frame(t, a)
	press(t, a, f, "modal."+historyModalID+".close")
	if a.win.Open || a.uiModalTop() != "" {
		t.Fatal("the X closes the window")
	}
	a.applyClick("tab.work")
	a.applyWindowClick("win.close")
	if a.win.Open {
		t.Fatal("win.close closes the open window")
	}
}

// Opening a window closes the Architect, like the Work tab always did.
func TestWindowClosesTheArchitect(t *testing.T) {
	t.Parallel()
	a, _ := winApp(t, winStub(3))
	a.openArchitect()
	a.openAssign()
	if a.arch.Open {
		t.Fatal("the Architect should give way to a window")
	}
}

// ---- geometry -----------------------------------------------------------

// winScreen sets the logical screen size and UI scale. It changes package
// globals: never in parallel.
func winScreen(t *testing.T, a *App, w, h int, ui float64) *ebiten.Image {
	t.Helper()
	old := gfx.CurrentScale()
	t.Cleanup(func() { gfx.SetScale(old); gfx.SetDrawTrace(nil); gfx.ForcePass("hud") })
	a.scale = uiScaleState{Dev: 1, UI: ui, S: ui}
	a.syncUIScale()
	pw, ph := int(math.Round(float64(w)*ui)), int(math.Round(float64(h)*ui))
	a.setPhysical(pw, ph)
	if a.sw != w || a.sh != h {
		t.Fatalf("logical screen %dx%d, want %dx%d", a.sw, a.sh, w, h)
	}
	return ebiten.NewImage(pw, ph)
}

func winTrace(a *App, img *ebiten.Image) []traceEvent {
	var ev []traceEvent
	gfx.SetDrawTrace(func(e traceEvent) { ev = append(ev, e) })
	defer func() { gfx.SetDrawTrace(nil) }()
	img.Clear()
	a.Draw(img)
	return winPass(ev)
}

var winScreens = []struct {
	w, h int
	ui   float64
}{{1280, 720, 1}, {1280, 720, 1.5}, {1280, 720, 2}, {1920, 1080, 1}, {1920, 1080, 1.5}, {1920, 1080, 2}}

// Every window sits in the Window rect, bottom-left above the tabs, and
// everything it draws stays inside it (the resource list, which re-flows
// above a tall window, stays inside its own rect).
func TestWindowsStayInsideTheWindowRect(t *testing.T) {
	for _, sc := range winScreens {
		for _, c := range winCases() {
			t.Run(fmt.Sprintf("%dx%d_ui%g_%s", sc.w, sc.h, sc.ui, c.name), func(t *testing.T) {
				a, f := winApp(t, winStub(8))
				img := winScreen(t, a, sc.w, sc.h, sc.ui)
				f.Frame(t, a)
				base := winTrace(a, img)
				c.open(a)
				f.Frame(t, a)
				w := a.hud().Window
				if w.Empty() {
					t.Fatalf("%s: no Window rect", c.name)
				}
				if w.X != 0 || w.Bottom() != a.hud().Tabs.Y {
					t.Errorf("%s: Window %+v, want bottom-left above the tabs (Tabs.Y %v)", c.name, w, a.hud().Tabs.Y)
				}
				if w.Right() > a.hud().Right.X-hudMargin+0.01 {
					t.Errorf("%s: Window right %v reaches the right column at %v", c.name, w.Right(), a.hud().Right.X)
				}
				got := winTrace(a, img)
				added := winAdded(base, got)
				if len(added) == 0 {
					t.Fatalf("%s: opening it drew nothing", c.name)
				}
				s := sc.ui
				// the resource list gives way to a tall window: it shrinks and
				// scrolls (a scroll bar appears), inside its own rect only
				res := a.hud().Resources
				var bad []string
				for _, e := range added {
					if !res.Empty() && winInside(e, float64(res.X)*s, float64(res.Y)*s, float64(res.Right())*s, float64(res.Bottom())*s, 1) {
						continue
					}
					if !winInside(e, float64(w.X)*s, float64(w.Y)*s, float64(w.Right())*s, float64(w.Bottom())*s, 1) {
						bad = append(bad, fmt.Sprintf("%s is outside the window (%.0f,%.0f)-(%.0f,%.0f)", winString(e), float64(w.X)*s, float64(w.Y)*s, float64(w.Right())*s, float64(w.Bottom())*s))
					}
				}
				winReport(t, c.name, bad)
				// and the window is what covers the area: the HUD hit-test sees it
				cx, cy := w.Center()
				if !a.colonyChromeHit(int(cx), int(cy), a.sw, a.sh) {
					t.Errorf("%s: the window's centre does not consume clicks", c.name)
				}
			})
		}
	}
}

// A window never covers the colonist bar, the alerts, the letters, the
// lower-right controls, the minimap, the resource list or the tab strip.
func TestWindowsNeverCoverTheHUD(t *testing.T) {
	for _, n := range []int{6, 30} {
		for _, sc := range winScreens {
			for _, c := range winCases() {
				t.Run(fmt.Sprintf("%dcol_%dx%d_ui%g_%s", n, sc.w, sc.h, sc.ui, c.name), func(t *testing.T) {
					a, f := winApp(t, winStub(n))
					winScreen(t, a, sc.w, sc.h, sc.ui)
					c.open(a)
					f.Frame(t, a)
					l := a.hud()
					if l.Window.Empty() {
						t.Fatalf("%s: no Window rect", c.name)
					}
					for name, r := range map[string]rect{"Bar": l.Bar, "Alerts": l.Alerts, "Letters": l.Letters, "Learn": l.Learn,
						"Global": l.Global, "Minimap": l.Minimap, "Resources": l.Resources, "Messages": l.Messages, "Tabs": l.Tabs, "Right": l.Right} {
						if r.Empty() {
							continue
						}
						if !l.Window.Intersect(r).Empty() {
							t.Errorf("%s: Window %+v covers %s %+v", c.name, l.Window, name, r)
						}
					}
					if l.Window.Y < l.Bar.Bottom() && !l.Bar.Empty() {
						t.Errorf("%s: Window top %v is inside the bar (bottom %v)", c.name, l.Window.Y, l.Bar.Bottom())
					}
					// the inspect pane and the command grid give way
					if !l.Pane.Empty() || !l.Expanded.Empty() || !l.Grid.Empty() || !l.ArchCats.Empty() {
						t.Errorf("%s: the pane or the grid is still up beside the window", c.name)
					}
				})
			}
		}
	}
}

// What a window declares (buttons and kit widgets) lies inside it.
func TestWindowContentsLieInsideTheWindow(t *testing.T) {
	t.Parallel()
	for _, c := range winCases() {
		t.Run(c.name, func(t *testing.T) {
			a, f := winApp(t, winStub(8))
			c.open(a)
			f.Frame(t, a)
			f.Frame(t, a)
			w := a.hud().Window
			tol := float32(0.5)
			inside := func(id string, r rect) {
				if r.X < w.X-tol || r.Y < w.Y-tol || r.Right() > w.Right()+tol || r.Bottom() > w.Bottom()+tol {
					t.Errorf("%s: %s %+v leaves the window %+v", c.name, id, r, w)
				}
			}
			n := 0
			for _, b := range a.colonyButtons(a.sw, a.sh) {
				for _, p := range c.prefixes {
					if strings.HasPrefix(b.ID, p) {
						inside(b.ID, rect{X: b.X, Y: b.Y, W: b.W, H: b.H})
						n++
					}
				}
			}
			for _, wd := range a.uiSt().Cur {
				for _, p := range c.prefixes {
					if strings.HasPrefix(wd.ID, p) {
						inside(wd.ID, wd.R)
						n++
					}
				}
			}
			if n == 0 {
				t.Fatalf("%s: found no content to check", c.name)
			}
		})
	}
}

// The wheel over a window never zooms the map.
func TestWheelOverAWindowIsNotTheMapsWheel(t *testing.T) {
	t.Parallel()
	for _, c := range winCases() {
		t.Run(c.name, func(t *testing.T) {
			a, f := winApp(t, winStub(4))
			if a.uiWheelOver(5, 5) {
				t.Fatal("nothing open: the wheel is the map's")
			}
			c.open(a)
			f.Frame(t, a)
			w := a.hud().Window
			cx, cy := w.Center()
			if !a.uiWheelOver(int(cx), int(cy)) {
				t.Errorf("%s: the wheel over the window's middle is still the map's", c.name)
			}
			if a.uiWheelOver(int(w.Right())+20, int(w.Y)-20) {
				t.Errorf("%s: the wheel outside the window is not the map's", c.name)
			}
		})
	}
}

// The windows' wanted sizes are real sizes, not zero, and the layout hands
// each the room it asks for when the screen has it.
func TestWindowWantsAreSizes(t *testing.T) {
	t.Parallel()
	for _, c := range winCases() {
		t.Run(c.name, func(t *testing.T) {
			a, f := winApp(t, winStub(5))
			if w, h := a.windowWant(); w != 0 || h != 0 {
				t.Fatalf("nothing open, yet the want is %vx%v", w, h)
			}
			c.open(a)
			f.Frame(t, a)
			w, h := a.windowWant()
			if w < 300 || h < 150 {
				t.Fatalf("%s wants %vx%v", c.name, w, h)
			}
			a.sw, a.sh = 1920, 1080
			f.Frame(t, a)
			win := a.hud().Window
			if win.W != w || win.H > h+0.01 {
				t.Errorf("%s: wants %vx%v, got %vx%v on a 1920x1080 screen", c.name, w, h, win.W, win.H)
			}
		})
	}
}

// ---- the Work/Schedule/Research/Animals windows keep working ----------

func TestWorkWindowCellsStillToggle(t *testing.T) {
	t.Parallel()
	a, f := winApp(t, winStub(3))
	click(t, a, "tab.work")
	f.Frame(t, a)
	n := 0
	for _, b := range a.layoutHits(a.sw, a.sh) {
		if strings.HasPrefix(b.ID, "work.cell.") {
			n++
		}
	}
	if n == 0 {
		t.Fatal("the Work window has no cells")
	}
	click(t, a, "work.cell.1.2")
}

// The Schedule window holds the 24 hour cells of every row and the brushes.
func TestScheduleWindowKeepsItsCells(t *testing.T) {
	t.Parallel()
	a, f := winApp(t, winStub(3))
	click(t, a, "tab.schedule")
	f.Frame(t, a)
	have := map[string]bool{}
	for _, b := range a.layoutHits(a.sw, a.sh) {
		have[b.ID] = true
	}
	for _, id := range []string{"sched.brush.0", "sched.cell.0.0", "sched.cell.0.23", "sched.cell.3.12"} {
		if !have[id] {
			t.Errorf("Schedule window lacks %s", id)
		}
	}
}

func TestResearchWindowKeepsItsProjects(t *testing.T) {
	t.Parallel()
	a, f := winApp(t, winStub(3))
	click(t, a, "tab.research")
	f.Frame(t, a)
	have := map[string]bool{}
	for _, b := range a.layoutHits(a.sw, a.sh) {
		have[b.ID] = true
	}
	if !have["research.pick.0"] || !have["research.pick.3"] {
		t.Fatal("the Research window lists its projects")
	}
}

// At 1280 the Colony list drops its low-priority columns instead of running
// off the window, and the buttons stay inside.
func TestAnimalsWindowFitsNarrowScreens(t *testing.T) {
	t.Parallel()
	for _, size := range [][2]int{{1280, 720}, {1113, 626}, {1920, 1080}} {
		for _, wild := range []bool{false, true} {
			t.Run(fmt.Sprintf("%dx%d_wild%v", size[0], size[1], wild), func(t *testing.T) {
				a, f := winApp(t, winStub(3))
				a.sw, a.sh = size[0], size[1]
				a.win.Open, a.win.Mode, a.win.Wild = true, "animals", wild
				f.Frame(t, a)
				w := a.hud().Window
				n := 0
				for _, b := range a.animalsTabButtons() {
					n++
					if b.X < w.X || b.X+b.W > w.Right() || b.Y < w.Y || b.Y+b.H > w.Bottom() {
						t.Errorf("%s %+v leaves the window %+v", b.ID, b, w)
					}
				}
				if n < 3 {
					t.Fatalf("only %d buttons", n)
				}
			})
		}
	}
}

// TestAnimalColumnsDropToFit moved to windows/windows_test.go.

// ---- the modal stack's new options -------------------------------------

func TestModalsStayCentredWhenNotAnchored(t *testing.T) {
	t.Parallel()
	m := modal{ID: "x", Title: "X", W: 400, H: 300}
	frame, title, body, footer := modalGeometry(1280, 720, m)
	if frame != (rect{X: 440, Y: 210, W: 400, H: 300}) {
		t.Fatalf("frame %+v, want centred", frame)
	}
	if title.Y != frame.Y || title.H != modalTitleH || body.X != frame.X+modalPad || !footer.Empty() {
		t.Fatalf("title %+v body %+v footer %+v", title, body, footer)
	}
	// a modal that is too big is shrunk to the screen edge clearance
	frame, _, _, _ = modalGeometry(1280, 720, modal{ID: "y", W: 5000, H: 5000})
	if frame != (rect{X: modalEdge, Y: modalEdge, W: 1280 - 2*modalEdge, H: 720 - 2*modalEdge}) {
		t.Fatalf("big frame %+v", frame)
	}
}

func TestAnchoredModalSitsOnItsAnchor(t *testing.T) {
	t.Parallel()
	a, f := winApp(t, winStub(1))
	anchor := rect{X: 0, Y: 300, W: 500, H: 380}
	var body rect
	a.uiPushModal(modal{ID: "anch", Title: "Anchored", W: 900, H: 900,
		Anchor:  func(*App) rect { return anchor },
		Draw:    func(_ *App, b rect) { body = b },
		Buttons: []modalButton{{ID: "ok", Label: "OK", Closes: true}},
	})
	f.Frame(t, a)
	if !a.uiModalOpen() || a.uiModalTop() != "anch" {
		t.Fatal("an anchored modal is still a stacked, blocking modal")
	}
	if body.X != anchor.X+modalPad || body.Y != anchor.Y+modalTitleH+modalPad || body.W != anchor.W-2*modalPad {
		t.Fatalf("body %+v in anchor %+v", body, anchor)
	}
	x, ok := a.uiSt().Find("modal.anch.close")
	if !ok || x.R.Right() > anchor.Right() || x.R.Y < anchor.Y || x.R.Bottom() > anchor.Y+modalTitleH {
		t.Fatalf("the X %+v is not in the anchored title bar", x.R)
	}
	// Esc closes it like any modal
	f.Key(t, a, "Escape", 0)
	if a.uiModalOpen() {
		t.Fatal("Escape closes an anchored modal")
	}
}

// A modeless modal is a window: no scrim, the HUD stays live (it is not on
// the stack, so no modal input context), and the window reaches Escape,
// uiCloseTop and uiModalTop like a modal does.
func TestModelessModalIsAWindow(t *testing.T) {
	t.Parallel()
	a, f := winApp(t, winStub(2))
	closed := 0
	a.uiPushModal(modal{ID: "win1", Title: "One", W: 500, H: 300, Modeless: true, Anchor: (*App).windowAnchor,
		Draw: func(a *App, b rect) { a.uiLabel(b.X, b.Y, "hello", tsBody) }, OnClose: func() { closed++ }})
	f.Frame(t, a)
	if a.uiModalOpen() {
		t.Fatal("a modeless modal is not on the stack")
	}
	if a.uiModalTop() != "win1" || !a.win.Open {
		t.Fatalf("top %q open %v", a.uiModalTop(), a.win.Open)
	}
	for _, c := range a.in.Stack {
		if c == input.CtxModal {
			t.Fatal("a modeless window must not take the modal input context")
		}
	}
	if w := a.hud().Window; w.Empty() || w.W != 500 {
		t.Fatalf("Window %+v", w)
	}
	// a second one replaces it
	a.uiPushModal(modal{ID: "win2", Title: "Two", W: 400, H: 300, Modeless: true, Anchor: (*App).windowAnchor})
	if a.uiModalTop() != "win2" {
		t.Fatalf("top %q, want the replacement", a.uiModalTop())
	}
	// a real modal opens over it
	a.uiPushModal(modal{ID: "dlg", Title: "Dialog", W: 300, H: 200})
	if a.uiModalTop() != "dlg" || !a.uiModalOpen() {
		t.Fatalf("top %q", a.uiModalTop())
	}
	a.uiCloseTop()
	if a.uiModalTop() != "win2" {
		t.Fatalf("top %q after the dialog closes", a.uiModalTop())
	}
	a.uiCloseTop()
	if a.uiModalTop() != "" || a.win.Open {
		t.Fatalf("top %q open %v after closing the window", a.uiModalTop(), a.win.Open)
	}
	// onClose runs when the window is closed by uiCloseTop
	a.uiPushModal(modal{ID: "win3", Title: "Three", W: 400, H: 300, Modeless: true, Anchor: (*App).windowAnchor, OnClose: func() { closed++ }})
	a.uiCloseTop()
	if closed != 1 {
		t.Fatalf("onClose ran %d times, want once (for win3)", closed)
	}
}

func TestEscapeInPanelAndModelessKeepsTheHUDLive(t *testing.T) {
	t.Parallel()
	a, f := winApp(t, winStub(2))
	a.openHistory()
	f.Frame(t, a)
	// a tab click works with the window open (a stacked modal would swallow it)
	click(t, a, "tab.work")
	if !a.win.Open || a.win.Mode != "work" || a.uiModalTop() != "" {
		t.Fatalf("Work over History: open %v mode %q top %q", a.win.Open, a.win.Mode, a.uiModalTop())
	}
}

// winPass is the HUD draws of a trace.
func winPass(ev []traceEvent) []traceEvent {
	var out []traceEvent
	for _, e := range ev {
		if e.Pass == "hud" {
			out = append(out, e)
		}
	}
	return out
}

// winAdded is the events of got that base does not hold (a multiset
// difference): what opening a window drew.
func winAdded(base, got []traceEvent) []traceEvent {
	have := map[string]int{}
	for _, e := range base {
		have[winKey(e)]++
	}
	var out []traceEvent
	for _, e := range got {
		if k := winKey(e); have[k] > 0 {
			have[k]--
		} else {
			out = append(out, e)
		}
	}
	return out
}

// winKey names a draw: kind, text and rectangle.
func winKey(e traceEvent) string {
	return fmt.Sprintf("%s|%s|%.2f|%.2f|%.2f|%.2f", e.Kind, e.Text, e.X, e.Y, e.W, e.H)
}

// winString is a draw for failure messages.
func winString(e traceEvent) string {
	return fmt.Sprintf("%s %q %.1f,%.1f %.1fx%.1f", e.Kind, e.Text, e.X, e.Y, e.W, e.H)
}

// winInside reports the event's rectangle lies in [x0,x1] x [y0,y1].
func winInside(e traceEvent, x0, y0, x1, y1, tol float64) bool {
	return float64(e.X) >= x0-tol && float64(e.Y) >= y0-tol && float64(e.X+e.W) <= x1+tol && float64(e.Y+e.H) <= y1+tol
}

// winReport fails t with the problems, capped so a systematic bug stays readable.
func winReport(t *testing.T, what string, bad []string) {
	t.Helper()
	if len(bad) == 0 {
		return
	}
	if len(bad) > 8 {
		bad = append(bad[:8], fmt.Sprintf("... and %d more", len(bad)-8))
	}
	t.Errorf("%s: %d problem(s):\n\t%s", what, len(bad), strings.Join(bad, "\n\t"))
}
