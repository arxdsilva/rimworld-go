package ebiten

import (
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/dossier/sheet"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	inspectorpkg "rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/domain/sim/command"
)

// TestJourneyDossierCompact: the compact docked card (RHUD-003, DOS-10) keeps
// every selector, the tabs behind its rows and its gear, and on every window
// size the pane and its tabs stay within 30 % of the screen with the commands
// clear of them.
func TestJourneyDossierCompact(t *testing.T) {
	j := newJourney(t)
	j.g.Enqueue(command.Command{Kind: command.CreateArea})
	j.tick(1)
	areas := j.g.Areas()
	if len(areas) == 0 {
		t.Fatal("no area to pick")
	}
	dossierOn(j, true)
	p := j.a.renderer.snapshot()[0]
	dossierClick(j, p.X, p.Y)
	compactFits(t, j, "1280x720")

	// the area selector: the game's list, a row assigns the area
	if !clickCardItem(j, "dossier.d.", architect.AreaPickID) {
		t.Fatalf("no row opens the area list; items %v", cardIDs(j, "dossier.d."))
	}
	j.draw(1)
	j.clickID("areapick.row." + strconv.Itoa(int(areas[0].ID)))
	j.update(2)
	if c, ok := j.lastCmd(command.AssignArea); !ok || c.Pawn != uint64(p.ID) || c.Index != int(areas[0].ID) {
		t.Fatalf("area command %+v, sent %v", c, ok)
	}
	closeAll(j)

	// the hour selector
	if !clickCardItem(j, "dossier.d.", "pick.dossier.hour") {
		t.Fatalf("no row opens the hour menu; items %v", cardIDs(j, "dossier.d."))
	}
	j.draw(1)
	pickRow(j, "dossier.hour", 2) // Recreation
	if c, ok := j.lastCmd(command.SetSchedule); !ok || c.Pawn != uint64(p.ID) || c.Mode != 2 {
		t.Fatalf("schedule command %+v, sent %v", c, ok)
	}
	closeAll(j)

	// the food selector
	if !clickCardItem(j, "dossier.d.", "pick.dossier.diet") {
		t.Fatalf("no row opens the food menu; items %v", cardIDs(j, "dossier.d."))
	}
	j.draw(1)
	pickRow(j, "dossier.diet", 0)
	if c, ok := j.lastCmd(command.SetFoodPolicy); !ok || c.Pawn != uint64(p.ID) {
		t.Fatalf("food command %+v, sent %v", c, ok)
	}
	closeAll(j)

	// the first bar (vigour) opens the health tab: it is the first row of the
	// strip's left column, under the two lines and the rule
	l := j.a.hud()
	g := inspectorpkg.PaneLayout(l.Pane, j.a.insp.SelectionPane())
	m := sheet.MetricsFor(sheet.DockedFor(j.a.settings.HUD, j.a.sh).Text)
	bar := ""
	for _, w := range j.a.uiSt().Cur {
		if strings.HasPrefix(w.ID, "dossier.d.") && w.R.X == g.Card.X && w.R.Y == g.Card.Y+2*m.BodyH+m.GapH {
			bar = w.ID
		}
	}
	if bar == "" {
		t.Fatalf("no bar at the strip's top left (%v, %v); items %v", g.Card.X, g.Card.Y+2*m.BodyH+m.GapH, cardIDs(j, "dossier.d."))
	}
	j.clickID(bar)
	j.update(1)
	if got := j.a.insp.PaneOpenTab(); got != "inspector.health" {
		t.Fatalf("the vigour bar opened %q", got)
	}
	j.a.insp.SelectPaneTab("")
	j.update(1)

	// the gear: upper right of the card, shown while the pointer is over it
	cx, cy := g.Card.Center()
	sc := float32(j.a.scale.S)
	j.f.Cur.Mx, j.f.Cur.My = int(cx*sc), int(cy*sc)
	j.update(2)
	gw, ok := j.a.uiSt().Find(ids.DossierGear)
	if !ok {
		t.Fatal("no gear with the pointer over the card")
	}
	if gw.R.Right() > g.Card.Right() || gw.R.X < g.Card.Right()-30 || gw.R.Y < g.Card.Y || gw.R.Y > g.Card.Y+10 {
		t.Fatalf("the gear %+v is not in the card's upper right %+v", gw.R, g.Card)
	}
	j.clickID(ids.DossierGear)
	j.update(2)
	if j.a.uiModalTop() != "dossier.setup" {
		t.Fatalf("the gear opened %q, want the setup dialog", j.a.uiModalTop())
	}
	j.draw(1)
	closeAll(j)
	j.f.Cur.Mx, j.f.Cur.My = -50, -50

	// the window grows and shrinks: the pane follows, within 30 %
	for _, sz := range [][2]int{{1920, 1080}, {1024, 768}, {1280, 720}} {
		winScreen(t, j.a, sz[0], sz[1], 1)
		j.update(3)
		j.img = nil
		compactFits(t, j, strconv.Itoa(sz[0])+"x"+strconv.Itoa(sz[1]))
		j.draw(1)
	}
}

// compactFits checks the docked pane on the current screen: pane and tabs at
// most 30 % of the height, the card's rows inside its body, no command over the
// pane or its tabs.
func compactFits(t *testing.T, j *journey, name string) {
	t.Helper()
	j.update(1)
	l := j.a.hud()
	if l.Pane.Empty() {
		t.Fatalf("%s: no pane", name)
	}
	if share := (l.Pane.H + l.PaneTabs.H) / float32(j.a.sh); share > 0.30 {
		t.Errorf("%s: pane %+v and tabs take %.1f %% of %d", name, l.Pane, share*100, j.a.sh)
	}
	if l.PaneTabs.H != hudlayout.PaneTabsH {
		t.Errorf("%s: tabs %+v", name, l.PaneTabs)
	}
	g := inspectorpkg.PaneLayout(l.Pane, j.a.insp.SelectionPane())
	if g.Card.Empty() {
		t.Fatalf("%s: the pane holds no card", name)
	}
	if h := j.a.dos.panel.Height(true, g.Card.W); h <= 0 || h > g.Card.H {
		t.Errorf("%s: the card is %v tall in a %v body", name, h, g.Card.H)
	}
	if len(l.GridCells) == 0 {
		t.Errorf("%s: no commands", name)
	}
	for _, c := range l.GridCells {
		if rectsMeet(c, l.Pane) || rectsMeet(c, l.PaneTabs) {
			t.Errorf("%s: command %+v meets the pane %+v or its tabs %+v", name, c, l.Pane, l.PaneTabs)
		}
	}
	for _, w := range j.a.uiSt().Cur {
		if strings.HasPrefix(w.ID, "dossier.d.") && !insideRect(w.R, g.Card) {
			t.Errorf("%s: card row %s %+v outside the body %+v", name, w.ID, w.R, g.Card)
		}
	}
}

func insideRect(r, in gfx.Rect) bool {
	return r.X >= in.X-0.5 && r.Y >= in.Y-0.5 && r.Right() <= in.Right()+0.5 && r.Bottom() <= in.Bottom()+0.5
}

// closeAll closes the open menus and tabs.
func closeAll(j *journey) {
	for j.a.uiSt().CloseTop() {
	}
	j.a.insp.SelectPaneTab("")
	j.update(2)
}
