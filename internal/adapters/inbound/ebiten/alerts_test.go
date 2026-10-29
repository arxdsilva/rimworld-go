package ebiten

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

func TestAlertClickCentresCamera(t *testing.T) {
	t.Parallel()
	a, g, id := liveColony(t, 0)
	g.SetNeeds(id, 1, 1)
	for i := 0; i < 50 && len(a.currentAlerts()) == 0; i++ {
		g.Step(100)
	}
	als := a.currentAlerts()
	if len(als) == 0 {
		t.Fatal("expected an alert for a starving/collapsed colonist")
	}
	before := a.renderer.cam
	a.focusAlert("alert." + als[0].key)
	if a.renderer.cam == before && len(als[0].targets) > 0 {
		t.Fatal("clicking an alert moves the camera")
	}
}

// ---- PLAY-018: severity order, explanations, cards, cycling --------------

// rightWorld is a stub colony whose alerts are, in the order buildAlerts
// raises them: hostile (danger), rescue (danger), collapsed (warning), food
// (warning), raid (danger) and unhappy (warning).
func rightWorld() *stubSim {
	s := &stubSim{w: 80, h: 80}
	s.pawns = []game.PawnView{
		{ID: 1, Name: "Ada", Status: "downed", Reason: "injured", X: 3, Y: 4},
		{ID: 2, Name: "Bo", Status: "downed", Reason: "no bed", X: 5, Y: 6},
		{ID: 3, Name: "Cora", Status: "alive", Mood: 10, X: 7, Y: 7},
		{ID: 90, Name: "Reaver", Hostile: true, Status: "alive", X: 30, Y: 30},
		{ID: 91, Name: "Reaver", Hostile: true, Status: "alive", X: 50, Y: 40},
	}
	s.world.Food = 1
	s.world.Animals = []game.AnimalView{{ID: 20, Species: "timber_wolf", Label: "timber wolf", Hostile: true, X: 9, Y: 9}}
	return s
}

// cursorOver puts the logical cursor on r's centre.
func cursorOver(a *App, r rect) { a.cursor = &[2]int{int(r.X + r.W/2), int(r.Y + r.H/2)} }

func TestAlertRowsFillTheLayoutsAlertsRect(t *testing.T) {
	t.Parallel()
	a, _ := colonyApp(t, rightWorld())
	l := a.hud()
	rows := a.alertRows(l)
	if len(rows) != len(a.currentAlerts()) || l.AlertRows != len(rows) {
		t.Fatalf("rows %d, alerts %d, layout rows %d", len(rows), len(a.currentAlerts()), l.AlertRows)
	}
	if a.alertCount() != len(rows) {
		t.Fatalf("alertCount() = %d, want %d", a.alertCount(), len(rows))
	}
	for i, r := range rows {
		if r.r.X != l.Alerts.X || r.r.W != l.Alerts.W || r.r.H != alertRowH || r.r.Y != l.Alerts.Y+float32(i)*(alertRowH+hudGap) {
			t.Errorf("row %d at %+v, Alerts %+v", i, r.r, l.Alerts)
		}
		if r.al.key != a.currentAlerts()[i].key {
			t.Errorf("row %d is %q, want %q", i, r.al.key, a.currentAlerts()[i].key)
		}
	}
	// each row is a button with the legacy id
	ids := map[string]button{}
	for _, b := range a.rightColumnButtons(a.sw) {
		ids[b.ID] = b
	}
	for _, r := range rows {
		b, ok := ids["alert."+r.al.key]
		if !ok || b.X != r.r.X || b.Y != r.r.Y || b.W != r.r.W || b.H != r.r.H {
			t.Errorf("alert.%s button = %+v (found %v), want the row %+v", r.al.key, b, ok, r.r)
		}
	}
}

// When the rows run out the last one counts the rest.
func TestAlertsPlusNMore(t *testing.T) {
	t.Parallel()
	a, _ := colonyApp(t, rightWorld())
	als := a.currentAlerts()
	if len(als) < 5 {
		t.Fatalf("need five alerts, have %v", alertKeys(als))
	}
	l := computeHUDLayout(1280, 720, hudState{Alerts: len(als), Letters: 4, GlobalH: 380, LearnH: 28})
	if l.AlertRows == 0 || l.AlertRows >= len(als) {
		t.Fatalf("layout rows = %d for %d alerts: the fixture must not fit", l.AlertRows, len(als))
	}
	rows := a.alertRows(l)
	if len(rows) != l.AlertRows {
		t.Fatalf("rows = %d, want %d", len(rows), l.AlertRows)
	}
	last := rows[len(rows)-1]
	shown := len(rows) - 1
	if want := len(als) - shown; last.more != want {
		t.Fatalf("last row counts %d more, want %d", last.more, want)
	}
	if got := alertRowLabel(last); got != fmt.Sprintf("+%d more", len(als)-shown) {
		t.Fatalf("last row reads %q", got)
	}
	for i, r := range rows[:shown] {
		if r.more != 0 || r.al.key != als[i].key {
			t.Errorf("row %d = %+v, want alert %q", i, r, als[i].key)
		}
	}
	if len(last.hidden) != last.more {
		t.Fatalf("hidden alerts %d, more %d", len(last.hidden), last.more)
	}
	// the "+N more" row is not a button: only real alerts jump
	for _, b := range a.alertButtonsIn(l) {
		if strings.Contains(b.Label, "more") {
			t.Errorf("button %+v for the +N row", b)
		}
	}
}

// Hovering an alert shows its explanation in a card to the left of the
// row, clamped on screen, at every UI scale.
func TestAlertHoverCardSitsLeftOfTheAlert(t *testing.T) {
	for _, s := range []float64{1, 1.5, 2} {
		t.Run(fmt.Sprintf("x%v", s), func(t *testing.T) {
			a, f := colonyApp(t, rightWorld())
			pw, ph := rightAtScale(t, a, f, s)
			l := a.hud()
			rows := a.alertRows(l)
			if len(rows) == 0 {
				t.Fatal("no alert rows")
			}
			for i, row := range rows {
				cursorOver(a, row.r)
				card, anchor, ok := a.rightHover(l)
				if !ok || anchor != row.r {
					t.Fatalf("row %d: hover = %+v %+v %v, want its row %+v", i, card, anchor, ok, row.r)
				}
				if card.title != row.al.label || !strings.Contains(card.body, row.al.explain) {
					t.Errorf("row %d card = %q / %q, want label %q and explanation %q", i, card.title, card.body, row.al.label, row.al.explain)
				}
				box, _, _ := layoutRightCard(card, anchor, a.sw, a.sh)
				if box.Right() > row.r.X {
					t.Errorf("row %d: card %+v overlaps the alert %+v", i, box, row.r)
				}
				if box.X < 0 || box.Y < 0 || box.Right() > float32(a.sw) || box.Bottom() > float32(a.sh) {
					t.Errorf("row %d: card %+v off the %dx%d screen", i, box, a.sw, a.sh)
				}
			}
			// drawing the column with the card up does not panic
			a.drawRightColumn(ebiten.NewImage(pw, ph), a.sw, a.sh)
			a.cursor = &[2]int{-50, -50}
			if _, _, ok := a.rightHover(l); ok {
				t.Error("a card with the cursor off the column")
			}
		})
	}
}

// Left-click walks an alert's targets forward, right-click walks back.
func TestAlertClickAndRightClickCycleTargets(t *testing.T) {
	t.Parallel()
	a, f := colonyApp(t, rightWorld())
	var al alert
	for _, c := range a.currentAlerts() {
		if c.key == "raid" {
			al = c
		}
	}
	if len(al.targets) != 2 {
		t.Fatalf("raid alert targets = %v, want two raiders", al.targets)
	}
	l := a.hud()
	var row rect
	for _, r := range a.alertRows(l) {
		if r.al.key == "raid" {
			row = r.r
		}
	}
	centre := func() [2]int { // the tile in the middle of the map viewport
		v := a.mapViewport()
		x, y := a.renderer.TileAt(int((v.X0+v.X1)/2), int((v.Y0+v.Y1)/2))
		return [2]int{x, y}
	}
	at := func(i int) [2]int { return al.targets[i] }
	near := func(got, want [2]int) bool { return abs(got[0]-want[0]) <= 1 && abs(got[1]-want[1]) <= 1 }

	pressButton(t, a, f, "alert.raid") // left: first target
	if c := centre(); !near(c, at(0)) {
		t.Fatalf("first click centres %v, want %v", c, at(0))
	}
	pressButton(t, a, f, "alert.raid") // left: next
	if c := centre(); !near(c, at(1)) {
		t.Fatalf("second click centres %v, want %v", c, at(1))
	}
	f.Cur.Mx, f.Cur.My = int(row.X+row.W/2), int(row.Y+row.H/2)
	f.Click(t, a, mbRight, 0) // right: back to the first
	if c := centre(); !near(c, at(0)) {
		t.Fatalf("right-click centres %v, want back at %v", c, at(0))
	}
	f.Click(t, a, mbRight, 0) // right again: wraps to the last
	if c := centre(); !near(c, at(1)) {
		t.Fatalf("second right-click centres %v, want %v (wrapped)", c, at(1))
	}
	pressButton(t, a, f, "alert.raid") // left continues forward from there: wraps to the first
	if c := centre(); !near(c, at(0)) {
		t.Fatalf("click after going back centres %v, want %v", c, at(0))
	}

	// a right-click on an alert never reaches the map (no move order, no deselect)
	if got := cmdsOf(a.renderer.game.(*stubSim), command.MovePawn); len(got) != 0 {
		t.Fatalf("right-click on an alert enqueued %+v", got)
	}
}

func alertKeys(als []alert) []string {
	var out []string
	for _, a := range als {
		out = append(out, a.key)
	}
	return out
}
