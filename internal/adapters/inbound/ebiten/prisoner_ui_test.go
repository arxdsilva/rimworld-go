package ebiten

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/prisoner"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// prisonerSim is Ada (1), the prisoner Krell (2) and a downed raider (3)
// beside a standing one (4).
func prisonerSim() *stubSim {
	s := bioSim()
	s.pawns = append(s.pawns,
		game.PawnView{ID: 2, Name: "Krell", X: 7, Y: 5, Status: "alive", Prisoner: true, Hunger: 20},
		game.PawnView{ID: 3, Name: "Vasha", X: 9, Y: 5, Status: "downed", Hostile: true, Lying: true},
		game.PawnView{ID: 4, Name: "Dorn", X: 11, Y: 5, Status: "alive", Hostile: true})
	s.prisoners = map[entity.EntityID]game.PrisonerView{2: {Mode: uint8(prisoner.Recruit), Resistance: 12, Warden: "Ada"}}
	return s
}

func TestPrisonerTabReplacesGearAndFits(t *testing.T) {
	t.Parallel()
	s := prisonerSim()
	a, f := colonyApp(t, s)
	a.renderer.Select(2)
	f.Frame(t, a)
	if _, ok := findButton(a, "inspector.gear"); ok {
		t.Fatal("a prisoner's Gear tab gives way to its Prisoner tab")
	}
	tab, ok := findButton(a, inspector.PrisonerTabID)
	if !ok || tab.Disabled {
		t.Fatalf("Prisoner tab %+v (%v)", tab, ok)
	}
	// the tab opens a panel above the pane that fits the layout's width range
	pt, _ := paneTabByID(a.insp.Pawn().PawnPane(2), inspector.PrisonerTabID)
	if r := expandedFor(1280, 720, pt); r.W < pt.WantW || r.W > hudlayout.ExpandMaxW {
		t.Fatalf("Prisoner panel %+v for want %vx%v", r, pt.WantW, pt.WantH)
	}
	a.insp.Tab = "prisoner"
	if !a.insp.Tabs().PrisonerTabOn() {
		t.Fatalf("tab = %q", a.insp.Tab)
	}
	if b, _ := findButton(a, inspector.PrisonerTabID); !b.Selected {
		t.Fatal("the Prisoner tab is the selected one")
	}
	if b, _ := findButton(a, "inspector.needs"); b.Selected {
		t.Fatal("only the Prisoner tab is selected")
	}
	// a colonist has no Prisoner tab, and its pane falls back to no open tab
	a.renderer.Select(1)
	if _, ok := findButton(a, inspector.PrisonerTabID); ok || a.insp.Tabs().PrisonerTabOn() {
		t.Fatal("no Prisoner tab for a colonist")
	}
	if b, ok := findButton(a, "inspector.gear"); !ok || b.Disabled {
		t.Fatal("a colonist keeps Gear")
	}
}

func TestPrisonerTabModesAndRelease(t *testing.T) {
	t.Parallel()
	s := prisonerSim()
	a, f := colonyApp(t, s)
	a.renderer.Select(2)
	a.insp.Tab = "prisoner"
	f.Frame(t, a)
	declarePrisonerTab(t, a)
	for _, m := range inspector.PrisonerModes {
		if !hasWidget(a, inspector.PrisonerModeID(m.Mode)) {
			t.Fatalf("mode %s missing", m.Label)
		}
	}
	if w, _ := a.uiSt().Find(inspector.PrisonerRelease); w.Disabled {
		t.Fatal("a standing prisoner can be released")
	}
	a.uiSt().Click(inspector.PrisonerModeID(prisoner.Reduce))
	declarePrisonerTab(t, a)
	if c := lastCmd(t, s); c.Kind != command.SetPrisonerMode || c.Target != 2 || c.Mode != uint8(prisoner.Reduce) {
		t.Fatalf("mode command %+v", c)
	}
	a.uiSt().Click(inspector.PrisonerRelease)
	declarePrisonerTab(t, a)
	if c := lastCmd(t, s); c.Kind != command.ReleasePrisoner || c.Target != 2 {
		t.Fatalf("release command %+v", c)
	}
	if ws := a.insp.Tabs().PrisonerWarnings(s.pawns[1], s.prisoners[2]); len(ws) != 3 {
		t.Fatalf("no warden, no prisoner bed, hungry: %q", ws)
	}
	drawFrame(t, a, f)
}

func TestPrisonerWarningsClearWhenAllSet(t *testing.T) {
	t.Parallel()
	s := prisonerSim()
	s.pawns[0].Priorities = make([]int, int(work.NumTypes))
	s.pawns[0].Priorities[work.Warden] = 2
	s.pawns[1].Hunger = 80
	a, _ := colonyApp(t, s)
	v := s.prisoners[2]
	v.ForPrisonersBed = true
	if ws := a.insp.Tabs().PrisonerWarnings(s.pawns[1], v); len(ws) != 0 {
		t.Fatalf("all set: %q", ws)
	}
}

// declarePrisonerTab declares the selected prisoner's tab the way the pane
// does each Update: into the expanded panel the layout gives it.
func declarePrisonerTab(t *testing.T, a *App) {
	t.Helper()
	tb, ok := paneTabByID(a.insp.Pawn().PawnPane(a.renderer.Selected()), inspector.PrisonerTabID)
	if !ok || tb.Declare == nil {
		t.Fatal("no Prisoner tab to declare")
	}
	tb.Declare(innerFor(1280, 720, tb))
}

func TestCaptureAndArrestActions(t *testing.T) {
	t.Parallel()
	s := prisonerSim()
	a, f := colonyApp(t, s)
	a.renderer.Select(1)
	f.Frame(t, a)
	b, ok := findButton(a, inspector.PrisonerCapture)
	if !ok || b.Disabled {
		t.Fatalf("Ada can capture the downed raider: %+v", b)
	}
	pressCmd(a, mustCmd(t, a.insp.Pawn().PawnCommands(1), inspector.PrisonerCapture))
	if c := lastCmd(t, s); c.Kind != command.Capture || c.Pawn != 1 || c.Target != 3 {
		t.Fatalf("capture command %+v", c)
	}
	pressCmd(a, mustCmd(t, a.insp.Pawn().PawnCommands(1), inspector.PrisonerArrest))
	if c := lastCmd(t, s); c.Kind != command.Arrest || c.Target != 1 || c.Pawn != 0 {
		t.Fatalf("arrest command %+v", c)
	}
	a.renderer.Select(2)
	if _, ok := findButton(a, inspector.PrisonerArrest); ok {
		t.Fatal("a prisoner is not arrested again")
	}
	if _, ok := findButton(a, inspector.PrisonerCapture); ok {
		t.Fatal("a prisoner captures no one")
	}
	drawFrame(t, a, f)
}

func TestCaptureNeedsADownedEnemy(t *testing.T) {
	t.Parallel()
	s := prisonerSim()
	s.pawns = s.pawns[:2] // no raiders
	a, f := colonyApp(t, s)
	a.renderer.Select(1)
	f.Frame(t, a)
	if b, _ := findButton(a, inspector.PrisonerCapture); !b.Disabled || b.Reason != "no downed enemy to capture" {
		t.Fatalf("no one to capture: %+v", b)
	}
}
