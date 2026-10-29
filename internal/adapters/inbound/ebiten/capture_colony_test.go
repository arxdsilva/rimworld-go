package ebiten

import (
	"fmt"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/windows"
	"rimworld-go/internal/application/game"
)

// realBuilder wraps game.ColonyBuilder as the composition root does.
type realBuilder struct{ b *game.ColonyBuilder }

func (r *realBuilder) Step(units int) (string, float32, bool) { return r.b.Step(units) }

func (r *realBuilder) Sim() (Simulation, error) {
	g := r.b.Game()
	if g == nil {
		return nil, fmt.Errorf("colony not built")
	}
	return g, nil
}

// captureColony starts the colony of capture state ui the way a capture
// run does (a real setup colony, built by the real game), steps ticks and
// opens the UI state. It returns the app with no capture configured, so
// frames run as in play.
func captureColony(t *testing.T, ui string, ticks int) (*App, *fakeInput) {
	t.Helper()
	a := testApp(&stubSim{w: 8, h: 8})
	a.cfg.NewSimBuilder = func(ng NewGame) (SimBuilder, error) {
		return &realBuilder{game.NewColonyBuilder(game.Config{Seed: ng.Seed, Biome: ng.Biome, Setup: ng.Setup})}, nil
	}
	a.cfg.Capture = &Capture{UI: ui}
	a.finishBoot()
	a.sw, a.sh = 1280, 720
	a.captureStart(ui)
	if a.renderer == nil {
		t.Fatalf("%s: no colony started: %s", ui, a.sessionErr)
	}
	if !a.cfg.Capture.Bare {
		t.Fatalf("%s: a colony capture must skip the scripted blueprints", ui)
	}
	a.cfg.Capture = nil
	a.renderer.speed = speedPaused
	a.renderer.game.Step(ticks)
	a.renderer.syncPawns()
	a.captureColonyUI(ui)
	f := fakeOn(a)
	f.Cur.Mx, f.Cur.My = -50, -50
	return a, f
}

// captureCursorOnTool checks the capture's cursor rests on tool id's
// command button (its tooltip is what the shot shows).
func captureCursorOnTool(t *testing.T, a *App, id string) {
	t.Helper()
	cell, ok := a.arch.ToolCell(id)
	if !ok {
		t.Fatalf("no %s button in the grid", id)
	}
	if a.cursor == nil || !cell.Hit(a.cursor[0], a.cursor[1]) {
		t.Fatalf("cursor %v is not on the %s button %+v", a.cursor, id, cell)
	}
}

func TestCaptureColonyStates(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		ui    string
		ticks int
		check func(t *testing.T, a *App)
	}{
		{"factions-colony", 400, func(t *testing.T, a *App) {
			if a.uiModalTop() != windows.ColFacModalID {
				t.Fatalf("top modal = %q, want the Factions window", a.uiModalTop())
			}
			rows := colonyFactionRows(a.renderer.game.Factions())
			if len(rows) < 3 || len(rows[0].History) != 40 || len(rows[0].Events) != 2 {
				t.Fatalf("rows = %d, first history %d events %d: the capture colony lost its factions' history", len(rows), len(rows[0].History), len(rows[0].Events))
			}
		}},
		{"bio", 400, func(t *testing.T, a *App) {
			if !a.insp.Tabs().BioTabOn() {
				t.Fatalf("bio tab not showing: tab %q select %v", a.insp.Tab, a.renderer.HasSelect())
			}
			if b, ok := a.renderer.game.Bio(a.renderer.Selected()); !ok || len(b.Skills) != 11 {
				t.Fatalf("bio = %+v %v", b, ok)
			}
		}},
		{"work-incapable", 400, func(t *testing.T, a *App) {
			if !a.win.Open || a.cursor == nil {
				t.Fatalf("work open %v cursor %v: no incapable cell found to point at", a.win.Open, a.cursor)
			}
			hatched := 0
			for _, b := range a.win.Buttons() {
				if b.Disabled {
					hatched++
				}
			}
			if hatched == 0 {
				t.Fatal("the capture team has nobody incapable of anything")
			}
		}},
		{"arrival", 60, func(t *testing.T, a *App) {
			if len(a.renderer.world().Arrivals) == 0 {
				t.Fatal("no drop pods in the arrival")
			}
		}},
		{"options-story", 60, func(t *testing.T, a *App) {
			if a.nav.screen() != ports.ScreenOptions || a.opt.CurrentTab() != "story" {
				t.Fatalf("screen %s tab %s", a.nav.screen(), a.opt.CurrentTab())
			}
		}},
		{"letter-intro", 300, func(t *testing.T, a *App) {
			if a.uiModalTop() == "" {
				t.Fatal("the intro letter is not open")
			}
		}},
		{"arch-hospital", 60, func(t *testing.T, a *App) {
			if !a.arch.Open || a.arch.Cat != "furniture" || a.cursor == nil {
				t.Fatalf("architect open %v cat %q cursor %v", a.arch.Open, a.arch.Cat, a.cursor)
			}
			captureCursorOnTool(t, a, "hospital_bed")
		}},
		{"arch-cabinet", 60, func(t *testing.T, a *App) {
			if !a.arch.Open || a.arch.Cat != "production" || a.cursor == nil {
				t.Fatalf("architect open %v cat %q cursor %v", a.arch.Open, a.arch.Cat, a.cursor)
			}
			captureCursorOnTool(t, a, "tool_cabinet")
		}},
		{"arch-locked", 60, func(t *testing.T, a *App) {
			captureCursorOnTool(t, a, "campfire")
			for _, b := range a.insp.Commands().CommandGridButtons() { // the tools are command buttons
				if b.ID == "arch.tool.campfire" {
					if !b.Disabled || b.Reason != game.ReasonScenario {
						t.Fatalf("campfire = %+v, want locked by the scenario", b)
					}
					return
				}
			}
			t.Fatal("no campfire card")
		}},
		{"chips", 60, func(t *testing.T, a *App) {
			if chips := statusChips(a.renderer.world()); len(chips) != 1 || chips[0].Label != "Ashfall sky" {
				t.Fatalf("chips = %+v, want the Ashfall sky condition", chips)
			}
		}},
	} {
		c := c
		t.Run(c.ui, func(t *testing.T) {
			t.Parallel()
			a, f := captureColony(t, c.ui, c.ticks)
			f.Frame(t, a)
			if c.ui == "factions-colony" || c.ui == "letter-intro" {
				f.Frame(t, a)
			}
			c.check(t, a)
			if c.ui != "factions-colony" && c.ui != "letter-intro" && c.ui != "options-story" && a.uiSt().ModalOpen() {
				t.Fatalf("%s: a window (%q) sits over the shot", c.ui, a.uiModalTop())
			}
			drawFrame(t, a, f) // and it paints
		})
	}
}
