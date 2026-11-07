package ebiten

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/input"
)

// RSCH-2: the Research tab is a tech tree. The graph pans (wheel,
// Shift+wheel, a drag on empty space) so every project is reachable at
// every supported size, the window draws only inside itself, a click on a
// box selects it and Start research starts it, the project under way shows
// when the window opens, the "Need research project" alert opens the tab
// and developer mode adds Finish now.

// researchStub is a colony with n research projects, current (or -1)
// being researched.
func researchStub(n, current int) *stubSim {
	s := winStub(3)
	s.world.Research = nil
	for i := 0; i < n; i++ {
		s.world.Research = append(s.world.Research, game.ResearchView{ID: uint8(i), Name: fmt.Sprintf("proj%d", i), Label: fmt.Sprintf("Project %d", i),
			Desc: "A project", Cost: 100, Progress: 5 * i, Available: true, Current: i == current, Unlocks: []string{"thing"}})
	}
	return s
}

// pickIDs are the research.pick boxes among the window's hit targets.
func pickIDs(a *App) map[string]uikit.Button {
	out := map[string]uikit.Button{}
	for _, b := range a.layoutHits(a.sw, a.sh) {
		if strings.HasPrefix(b.ID, "research.pick.") {
			out[b.ID] = b
		}
	}
	return out
}

// wheelOver turns the wheel by notches (negative: down) over the logical
// point (lx, ly) for one frame, with mods held.
func wheelOver(t *testing.T, a *App, f *fakeInput, lx, ly float32, notches float64, mods input.Mod) {
	t.Helper()
	f.Cur.Mx, f.Cur.My = int(float64(lx)*a.scale.S), int(float64(ly)*a.scale.S)
	f.Cur.WheelY, f.Cur.Mods = notches, mods
	f.Frame(t, a)
	f.Cur.WheelY, f.Cur.Mods = 0, 0
}

// treeView is the graph's rect: the research.tree widget.
func treeView(t *testing.T, a *App) uikit.Widget {
	t.Helper()
	w, ok := a.uiSt().Find("research.tree")
	if !ok {
		t.Fatal("no research.tree widget")
	}
	return w
}

// researchScreens are the logical sizes the tree must work at on a
// 1280x720 display: the default, the small UI scale (75 %: more room), the
// big ones (150 %, 200 %: a small logical screen) and a small window.
var researchScreens = []struct {
	w, h int
	ui   float64
}{{1280, 720, 1}, {1707, 960, 0.75}, {854, 480, 1.5}, {640, 360, 2}, {1113, 626, 1}}

func TestResearchTreeReachesEveryProjectAndStartsOne(t *testing.T) { // sets the UI scale: not parallel
	for _, sc := range researchScreens {
		t.Run(fmt.Sprintf("%dx%d_ui%g", sc.w, sc.h, sc.ui), func(t *testing.T) {
			a, f := winApp(t, researchStub(16, -1))
			winScreen(t, a, sc.w, sc.h, sc.ui)
			a.applyClick("tab.research")
			f.Frame(t, a)
			win := a.hud().Window
			if win.Empty() {
				t.Fatal("no Window rect")
			}
			v := treeView(t, a).R
			if v.W < 100 || v.H < 60 || !win.Contains(v.X, v.Y) || v.Right() > win.Right()+0.5 || v.Bottom() > win.Bottom()+0.5 {
				t.Fatalf("graph %+v in window %+v", v, win)
			}
			seen := map[string]bool{}
			look := func() {
				for id, b := range pickIDs(a) {
					if b.X < v.X-0.5 || b.X+b.W > v.Right()+0.5 || b.Y < v.Y-0.5 || b.Y+b.H > v.Bottom()+0.5 {
						t.Fatalf("%s %+v pokes out of the graph %+v", id, b, v)
					}
					seen[id] = true
				}
			}
			cx, cy := v.Center()
			for row := 0; row < 30; row++ { // sweep: right to the end, then a notch down, then back
				look()
				for i := 0; i < 40; i++ {
					wheelOver(t, a, f, cx, cy, -1, input.ModShift)
					look()
				}
				wheelOver(t, a, f, cx, cy, -1, 0)
				for i := 0; i < 40; i++ {
					wheelOver(t, a, f, cx, cy, 1, input.ModShift)
					look()
				}
				if len(seen) == 16 {
					break
				}
			}
			if len(seen) != 16 {
				t.Fatalf("panning reached %d of 16 projects: %v", len(seen), seen)
			}
			// back to the graph's top-left, where boxes stand
			for i := 0; i < 60; i++ {
				wheelOver(t, a, f, cx, cy, 2, input.ModShift)
				wheelOver(t, a, f, cx, cy, 2, 0)
			}
			// a real click on a box selects it; Start research starts it
			var pick uikit.Button
			for _, b := range pickIDs(a) {
				if !b.Selected && b.W >= 20 && b.H >= 10 {
					pick = b
					break
				}
			}
			if pick.ID == "" {
				t.Fatal("no box to click")
			}
			f.Cur.Mx, f.Cur.My = int(float64(pick.X+pick.W/2)*a.scale.S), int(float64(pick.Y+pick.H/2)*a.scale.S)
			f.Click(t, a, inputframe.Left, 0)
			if b, ok := pickIDs(a)[pick.ID]; !ok || !b.Selected {
				t.Fatalf("clicking %s did not select it: %+v", pick.ID, b)
			}
			s := a.renderer.game.(*stubSim)
			if got := cmdsOf(s, command.SetResearch); len(got) != 0 {
				t.Fatalf("selecting a box sent %+v", got)
			}
			start, ok := a.uiSt().Find("research.start")
			if !ok || start.Disabled || !win.Contains(start.R.X, start.R.Y) || start.R.Bottom() > win.Bottom()+0.5 {
				t.Fatalf("Start research %+v (found %v) in window %+v", start.R, ok, win)
			}
			sx, sy := start.R.Center()
			f.Cur.Mx, f.Cur.My = int(float64(sx)*a.scale.S), int(float64(sy)*a.scale.S)
			f.Click(t, a, inputframe.Left, 0)
			got := cmdsOf(s, command.SetResearch)
			if len(got) != 1 || "research.pick."+strconv.Itoa(int(got[0].Build)) != pick.ID {
				t.Fatalf("Start research on %s sent %+v", pick.ID, got)
			}
		})
	}
}

// A drag on empty space pans the graph.
func TestResearchTreeDragPans(t *testing.T) {
	a, f := winApp(t, researchStub(16, -1))
	winScreen(t, a, 1280, 768, 2) // 2x on 2560x1536, the smallest screen a step above 1 allows (UIS-003)
	a.applyClick("tab.research")
	f.Frame(t, a)
	v := treeView(t, a).R
	// start from the far corner so a drag toward the origin has room
	cx, cy := v.Center()
	for i := 0; i < 40; i++ {
		wheelOver(t, a, f, cx, cy, -1, input.ModShift)
		wheelOver(t, a, f, cx, cy, -1, 0)
	}
	before := pickIDs(a)
	// an empty point in the graph
	x, y, found := float32(0), float32(0), false
	for py := v.Y + 3; py < v.Bottom()-3 && !found; py += 7 {
		for px := v.X + 3; px < v.Right()-3 && !found; px += 7 {
			free := true
			for _, b := range before {
				if b.Contains(int(px), int(py)) {
					free = false
				}
			}
			x, y, found = px, py, free
		}
	}
	if !found {
		t.Fatal("no empty point in the graph")
	}
	s := float32(a.scale.S)
	f.Cur.Mx, f.Cur.My = int(x*s), int(y*s)
	f.Cur.Press[inputframe.Left], f.Cur.Down[inputframe.Left] = true, true
	f.Frame(t, a)
	f.Cur.Mx, f.Cur.My = int((x+300)*s), int((y+200)*s)
	f.Frame(t, a)
	f.Cur.Down[inputframe.Left], f.Cur.Release[inputframe.Left] = false, true
	f.Frame(t, a)
	after := pickIDs(a)
	moved := false
	for id, b := range after {
		if o, ok := before[id]; ok && (o.X != b.X || o.Y != b.Y) {
			moved = true
		}
	}
	if !moved && len(after) == len(before) {
		t.Fatalf("a 300 px drag moved nothing (content may fit: %d boxes)", len(after))
	}
	if len(cmdsOf(a.renderer.game.(*stubSim), command.SetResearch)) != 0 {
		t.Fatal("the drag selected or started something")
	}
}

func TestResearchTreeOpensOnTheCurrentProject(t *testing.T) {
	a, f := winApp(t, researchStub(16, 14))
	winScreen(t, a, 640, 360, 2) // 200 % on 1280x720: not everything fits
	a.applyClick("tab.research")
	f.Frame(t, a)
	if b, ok := pickIDs(a)["research.pick.14"]; !ok || !b.Selected {
		t.Fatalf("the project being researched is not shown selected: %v", pickIDs(a))
	}
	// panning away is not undone: the selection reveals itself on opening
	v := treeView(t, a).R
	cx, cy := v.Center()
	for i := 0; i < 60; i++ {
		wheelOver(t, a, f, cx, cy, 1, input.ModShift)
		wheelOver(t, a, f, cx, cy, 1, 0)
	}
	if _, ok := pickIDs(a)["research.pick.14"]; ok {
		t.Skip("everything fits; nothing to pan away from")
	}
	f.Frame(t, a)
	if _, ok := pickIDs(a)["research.pick.14"]; ok {
		t.Fatal("the window snapped back to the current project")
	}
	// close and reopen: it reveals again
	a.applyClick("tab.research")
	f.Frame(t, a)
	a.applyClick("tab.research")
	f.Frame(t, a)
	if _, ok := pickIDs(a)["research.pick.14"]; !ok {
		t.Fatalf("reopening does not reveal the current project: %v", pickIDs(a))
	}
}

// Whatever the pan, the window draws only inside itself.
func TestResearchTreeDrawsInsideAtEveryOffset(t *testing.T) {
	for _, sc := range researchScreens {
		t.Run(fmt.Sprintf("%dx%d_ui%g", sc.w, sc.h, sc.ui), func(t *testing.T) {
			a, f := winApp(t, researchStub(16, 3))
			img := winScreen(t, a, sc.w, sc.h, sc.ui)
			f.Frame(t, a)
			base := winTrace(a, img)
			a.applyClick("tab.research")
			f.Frame(t, a)
			w := a.hud().Window
			v := treeView(t, a).R
			cx, cy := v.Center()
			ui := sc.ui
			for step := 0; step < 16; step++ {
				mods := input.Mod(0)
				if step%2 == 0 {
					mods = input.ModShift
				}
				wheelOver(t, a, f, cx, cy, -2, mods)
				added := winAdded(base, winTrace(a, img))
				if len(added) == 0 {
					t.Fatal("the window drew nothing")
				}
				res := a.hud().Resources // the resource list re-flows above a tall window
				var bad []string
				for _, e := range added {
					if !res.Empty() && winInside(e, float64(res.X)*ui, float64(res.Y)*ui, float64(res.Right())*ui, float64(res.Bottom())*ui, 1) {
						continue
					}
					if float64(e.X) >= float64(w.Right())*ui-1 {
						continue // the right column (letters, helper): not the window's
					}
					if !winInside(e, float64(w.X)*ui, float64(w.Y)*ui, float64(w.Right())*ui, float64(w.Bottom())*ui, 1) {
						bad = append(bad, winString(e)+" is outside the window")
					}
				}
				winReport(t, "pan step "+strconv.Itoa(step), bad)
			}
		})
	}
}

// The "Need research project" alert (WorldView.ResearchAlert) is listed,
// and a click on it opens the Research tab.
func TestResearchAlertOpensTheTab(t *testing.T) {
	s := researchStub(4, -1)
	s.world.ResearchAlert = true
	a, f := winApp(t, s)
	winScreen(t, a, 1280, 720, 1)
	f.Frame(t, a)
	if a.win.Open {
		t.Fatal("a window is open already")
	}
	click(t, a, "alert.research")
	if !a.win.Open || a.win.Mode != "research" {
		t.Fatalf("the alert opened %q (open %v)", a.win.Mode, a.win.Open)
	}
}

// Developer mode adds Finish now under Start research: it makes the
// selected project current and finishes it.
func TestResearchDevFinishNow(t *testing.T) {
	a, f := winApp(t, researchStub(4, -1))
	winScreen(t, a, 1280, 720, 1)
	a.applyClick("tab.research")
	f.Frame(t, a)
	if _, ok := a.uiSt().Find("research.devfinish"); ok {
		t.Fatal("Finish now shows outside developer mode")
	}
	set := a.settings
	set.Gameplay.DevMode = true
	a.setSettings(set)
	f.Frame(t, a)
	if _, ok := a.uiSt().Find("research.devfinish"); !ok {
		t.Fatal("no Finish now in developer mode")
	}
	click(t, a, "research.devfinish")
	s := a.renderer.game.(*stubSim)
	fin := cmdsOf(s, command.DevFinishResearch)
	set0 := cmdsOf(s, command.SetResearch)
	if len(fin) != 1 || len(set0) != 1 || set0[0].Build != fin[0].Build {
		t.Fatalf("Finish now sent %+v then %+v", set0, fin)
	}
}
