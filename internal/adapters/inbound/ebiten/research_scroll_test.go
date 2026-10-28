package ebiten

import (
	"fmt"
	"strconv"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// PLAY-021: the Research window scrolls (wheel, scroll bar, page clicks)
// so every project is reachable however small the window is, and the
// project being researched shows when the window opens.

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

// pickIDs are the research.pick ids among the window's hit targets.
func pickIDs(a *App) map[string]button {
	out := map[string]button{}
	for _, b := range a.layoutHits(a.sw, a.sh) {
		if len(b.id) > 14 && b.id[:14] == "research.pick." {
			out[b.id] = b
		}
	}
	return out
}

// wheelOver turns the wheel by notches (negative: down) over the logical
// point (lx, ly) for one frame.
func wheelOver(t *testing.T, a *App, f *fakeInput, lx, ly float32, notches float64) {
	t.Helper()
	f.cur.mx, f.cur.my = int(float64(lx)*a.scale.S), int(float64(ly)*a.scale.S)
	f.cur.wheelY = notches
	f.frame(t, a)
	f.cur.wheelY = 0
}

func TestResearchWindowScrollsToTheLastProject(t *testing.T) { // sets the UI scale: not parallel
	for _, sc := range []struct {
		w, h int
		ui   float64
	}{{1280, 720, 1}, {1280, 720, 1.5}, {1280, 720, 2}, {1113, 626, 1}} {
		t.Run(fmt.Sprintf("%dx%d_ui%g", sc.w, sc.h, sc.ui), func(t *testing.T) {
			a, f := winApp(t, researchStub(16, -1))
			winScreen(t, a, sc.w, sc.h, sc.ui)
			a.applyClick("tab.research")
			f.frame(t, a)
			w := a.hud().Window
			if w.Empty() {
				t.Fatal("no Window rect")
			}
			if _, ok := pickIDs(a)["research.pick.15"]; ok {
				t.Skipf("all 16 projects fit in a %vx%v window", w.W, w.H)
			}
			if _, ok := pickIDs(a)["research.pick.0"]; !ok {
				t.Fatalf("the first project is not shown at the top: %v", pickIDs(a))
			}
			cx, cy := w.Center()
			for i := 0; i < 40; i++ {
				if _, ok := pickIDs(a)["research.pick.15"]; ok {
					break
				}
				wheelOver(t, a, f, cx, cy, -1)
			}
			last, ok := pickIDs(a)["research.pick.15"]
			if !ok {
				t.Fatalf("the wheel never reached the last project: %v", pickIDs(a))
			}
			if !w.Contains(last.x, last.y) || !w.Contains(last.x+last.w, last.y+last.h) {
				t.Fatalf("the last project's button %+v is outside the window %+v", last, w)
			}
			// a real click on it picks it
			bx, by := last.x+last.w/2, last.y+last.h/2
			f.cur.mx, f.cur.my = int(float64(bx)*a.scale.S), int(float64(by)*a.scale.S)
			f.click(t, a, mbLeft, 0)
			s := a.renderer.game.(*stubSim)
			got := cmdsOf(s, command.SetResearch)
			if len(got) != 1 || got[0].Build != 15 {
				t.Fatalf("clicking the last project sent %+v", got)
			}
			// and back up to the first
			for i := 0; i < 40; i++ {
				if _, ok := pickIDs(a)["research.pick.0"]; ok {
					break
				}
				wheelOver(t, a, f, cx, cy, 1)
			}
			if _, ok := pickIDs(a)["research.pick.0"]; !ok {
				t.Fatal("the wheel never returned to the first project")
			}
		})
	}
}

func TestResearchScrollBarAppearsOnlyWhenNeeded(t *testing.T) {
	a, f := winApp(t, researchStub(16, -1))
	winScreen(t, a, 1280, 720, 1)
	a.applyClick("tab.research")
	f.frame(t, a)
	w := a.hud().Window
	bar, ok := a.uiSt().find("research.scroll")
	if !ok {
		t.Fatal("no research.scroll widget")
	}
	if bar.r.Right() > w.Right()+0.01 || bar.r.Y < w.Y || bar.r.Bottom() > w.Bottom()+0.01 {
		t.Fatalf("scroll area %+v is outside the window %+v", bar.r, w)
	}
	if len(pickIDs(a)) >= 16 {
		t.Skip("everything fits; nothing to scroll")
	}

	// few projects: all shown, nothing to scroll
	b, g := winApp(t, researchStub(3, -1))
	winScreen(t, b, 1280, 720, 1)
	b.applyClick("tab.research")
	g.frame(t, b)
	if got := len(pickIDs(b)); got != 3 {
		t.Fatalf("3 projects show %d buttons", got)
	}
	cx, cy := b.hud().Window.Center()
	wheelOver(t, b, g, cx, cy, -3)
	if got := len(pickIDs(b)); got != 3 {
		t.Fatalf("the wheel over a short list changed the buttons: %d", got)
	}
}

func TestResearchWindowOpensOnTheCurrentProject(t *testing.T) {
	a, f := winApp(t, researchStub(16, 14))
	winScreen(t, a, 1280, 720, 1)
	a.applyClick("tab.research")
	f.frame(t, a)
	if _, ok := pickIDs(a)["research.pick.14"]; !ok {
		t.Fatalf("the project being researched is out of view: %v", pickIDs(a))
	}
	// scrolling away is not undone: the selection reveals itself on opening
	// and when it changes, not every frame
	w := a.hud().Window
	cx, cy := w.Center()
	for i := 0; i < 40; i++ {
		wheelOver(t, a, f, cx, cy, 1)
	}
	if _, ok := pickIDs(a)["research.pick.0"]; !ok {
		t.Fatal("the wheel could not leave the current project")
	}
	if _, ok := pickIDs(a)["research.pick.14"]; ok {
		t.Fatal("the window snapped back to the current project")
	}
	// close and reopen: it reveals again
	a.applyClick("tab.research")
	f.frame(t, a)
	a.applyClick("tab.research")
	f.frame(t, a)
	if _, ok := pickIDs(a)["research.pick.14"]; !ok {
		t.Fatalf("reopening does not reveal the current project: %v", pickIDs(a))
	}
	// picking another project reveals the new current one
	s := a.renderer.game.(*stubSim)
	for i := range s.world.Research {
		s.world.Research[i].Current = i == 1
	}
	f.frame(t, a)
	if _, ok := pickIDs(a)["research.pick.1"]; !ok {
		t.Fatalf("a new current project is out of view: %v", pickIDs(a))
	}
}

func TestResearchScrollBarClickPages(t *testing.T) {
	a, f := winApp(t, researchStub(16, -1))
	winScreen(t, a, 1280, 720, 1)
	a.applyClick("tab.research")
	f.frame(t, a)
	if len(pickIDs(a)) >= 16 {
		t.Skip("everything fits; nothing to scroll")
	}
	bar, ok := a.uiSt().find("research.scroll")
	if !ok {
		t.Fatal("no research.scroll widget")
	}
	before := len(pickIDs(a))
	// a click on the track near the bottom pages down
	f.cur.mx, f.cur.my = int(bar.r.Right()-3), int(bar.r.Bottom()-3)
	f.click(t, a, mbLeft, 0)
	if _, ok := pickIDs(a)["research.pick.0"]; ok {
		t.Fatalf("a click below the thumb did not page down: %v", pickIDs(a))
	}
	if got := len(pickIDs(a)); got != before {
		t.Fatalf("rows are whole: %d shown before the page, %d after", before, got)
	}
}

// Whatever the scroll offset, the window draws only inside itself, and
// every project button is a whole row inside it.
func TestResearchWindowDrawsInsideAtEveryOffset(t *testing.T) {
	for _, ui := range []float64{1, 1.5, 2} {
		t.Run("ui"+strconv.FormatFloat(ui, 'g', -1, 64), func(t *testing.T) {
			a, f := winApp(t, researchStub(16, -1))
			img := winScreen(t, a, 1280, 720, ui)
			f.frame(t, a)
			base := winTrace(a, img)
			a.applyClick("tab.research")
			f.frame(t, a)
			w := a.hud().Window
			cx, cy := w.Center()
			for step := 0; step < 20; step++ {
				wheelOver(t, a, f, cx, cy, -1)
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
				winReport(t, "scroll step "+strconv.Itoa(step), bad)
				for id, b := range pickIDs(a) {
					if b.y < w.Y || b.y+b.h > w.Bottom() {
						t.Fatalf("%s %+v is not wholly inside the window %+v", id, b, w)
					}
				}
			}
		})
	}
}
