package ebiten

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

func TestBuildAlerts(t *testing.T) {
	t.Parallel()
	pawns := []game.PawnView{
		{Name: "Ada", Status: "downed", Reason: "injured", X: 3, Y: 4},
		{Name: "Bo", Status: "downed", Reason: "injured", InBed: true},
		{Name: "Cora", Status: "alive", Mood: 20, X: 7, Y: 7},
	}
	w := game.WorldView{Food: 2, Animals: []game.AnimalView{{Species: "ridge_wolf", Label: "ridge wolf", Hostile: true, X: 9, Y: 9}}}
	got := buildAlerts(pawns, w)
	// the label ("ridge wolf"), not the species key; no target, so generic
	want := []string{"A ridge wolf is attacking!", "Ada needs rescue", "Low food (2 left)", "Cora is unhappy"}
	if len(got) != len(want) {
		t.Fatalf("alerts = %+v", got)
	}
	for i, a := range got {
		if a.label != want[i] {
			t.Fatalf("alert %d = %q, want %q", i, a.label, want[i])
		}
	}
	if !got[0].danger || !got[1].danger || got[2].danger || got[1].targets[0] != [2]int{3, 4} {
		t.Fatalf("severity/targets: %+v", got)
	}
}

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

// hostileLabel is the "hostile" alert's text for animals and pawns.
func hostileLabel(t *testing.T, pawns []game.PawnView, animals ...game.AnimalView) alert {
	t.Helper()
	for _, a := range buildAlerts(pawns, game.WorldView{Animals: animals}) {
		if a.key == "hostile" {
			return a
		}
	}
	t.Fatalf("no hostile alert for %+v", animals)
	return alert{}
}

// PLAY-017: the hostile alert says what is chasing whom.
func TestHostileAlertNamesTheChasedColonist(t *testing.T) {
	t.Parallel()
	ada := game.PawnView{ID: entity.EntityID(7), Name: "Ada", Status: "alive", Mood: 80, X: 3, Y: 4}
	wolf := game.AnimalView{ID: 20, Species: "timber_wolf", Label: "timber wolf", Hostile: true, TargetID: ada.ID, X: 9, Y: 9}

	for _, c := range []struct {
		desc    string
		pawns   []game.PawnView
		animals []game.AnimalView
		want    string
	}{
		{"label and target name", []game.PawnView{ada}, []game.AnimalView{wolf}, "A timber wolf is chasing Ada!"},
		{"a vowel takes an", []game.PawnView{ada},
			[]game.AnimalView{{ID: 21, Species: "arctic_fox", Label: "arctic fox", Hostile: true, TargetID: ada.ID, X: 1, Y: 1}},
			"An arctic fox is chasing Ada!"},
		{"no label falls back to the spaced key", []game.PawnView{ada},
			[]game.AnimalView{{ID: 22, Species: "timber_wolf", Hostile: true, TargetID: ada.ID, X: 1, Y: 1}},
			"A timber wolf is chasing Ada!"},
		{"no target stays generic", []game.PawnView{ada},
			[]game.AnimalView{{ID: 23, Species: "timber_wolf", Label: "timber wolf", Hostile: true, X: 1, Y: 1}},
			"A timber wolf is attacking!"},
		{"a target nobody knows stays generic", nil, []game.AnimalView{wolf}, "A timber wolf is attacking!"},
		{"several animals stay a count", []game.PawnView{ada},
			[]game.AnimalView{wolf, {ID: 24, Species: "boar", Label: "boar", Hostile: true, X: 5, Y: 5}},
			"2 animals are attacking!"},
	} {
		al := hostileLabel(t, c.pawns, c.animals...)
		if al.label != c.want {
			t.Errorf("%s: label %q, want %q", c.desc, al.label, c.want)
		}
		if !al.danger || len(al.targets) != len(c.animals) {
			t.Errorf("%s: danger=%v targets=%v, want a danger alert aimed at every animal", c.desc, al.danger, al.targets)
		}
	}

	// a peaceful animal raises nothing, and a chased prisoner is still named
	if got := buildAlerts(nil, game.WorldView{Animals: []game.AnimalView{{Species: "deer", Label: "deer", X: 1, Y: 1}}}); len(got) != 0 {
		t.Errorf("peaceful animal alerts: %+v", got)
	}
	pris := game.PawnView{ID: 8, Name: "Bo", Prisoner: true, Status: "alive", Mood: 80}
	boar := game.AnimalView{ID: 25, Species: "boar", Label: "boar", Hostile: true, TargetID: pris.ID, X: 1, Y: 1}
	if al := hostileLabel(t, []game.PawnView{pris}, boar); al.label != "A boar is chasing Bo!" {
		t.Errorf("prisoner target: %q", al.label)
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

func alertKeys(als []alert) []string {
	var out []string
	for _, a := range als {
		out = append(out, a.key)
	}
	return out
}

// Danger alerts come before warnings; within a severity they follow the
// key, so the list never reshuffles while its alerts persist.
func TestAlertsAreOrderedBySeverityThenKey(t *testing.T) {
	t.Parallel()
	s := rightWorld()
	got := buildAlerts(append(s.pawns[:0:0], s.pawns...), s.world)
	want := []string{"hostile", "raid", "rescue", "collapsed", "food", "unhappy"}
	if keys := alertKeys(got); strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Fatalf("alert order = %v, want %v", keys, want)
	}
	for _, al := range got {
		wantSev := sevWarning
		if al.danger {
			wantSev = sevDanger
		}
		if al.sev != wantSev {
			t.Errorf("%s: sev = %v, want %v for danger=%v", al.key, al.sev, wantSev, al.danger)
		}
	}

	// the same alerts, built from a shuffled roster, come out in the same order
	rev := append(s.pawns[:0:0], s.pawns...)
	slices.Reverse(rev)
	if keys := alertKeys(buildAlerts(rev, s.world)); strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Fatalf("reversed roster: alert order = %v, want %v", keys, want)
	}
}

// Every alert the game can raise explains itself in a sentence or two.
func TestEveryAlertCarriesAnExplanation(t *testing.T) {
	t.Parallel()
	s := rightWorld()
	w := s.world
	w.Pens = []game.PenView{{ID: 3, Name: "Pen 3", Enclosed: false, X: 2, Y: 2}}
	w.Animals = append(w.Animals,
		game.AnimalView{ID: 21, Species: "timbergnaw", Label: "timbergnaw", X: 1, Y: 1},
		game.AnimalView{ID: 22, Species: "cow", Label: "cow", Tame: true, Penned: true, Pen: "Roaming", X: 4, Y: 4})
	w.Frames = []game.FrameView{{ID: 5, Kind: "wall", X: 8, Y: 8, Status: "No wood"}}
	w.Buildings = []game.BuildingView{{ID: 6, Kind: "campfire", Fuel: true, FuelPct: 0, X: 12, Y: 12}}
	w.Fires = []game.FireView{{X: 2, Y: 2}}
	w.TerrainW = 80
	w.Home = make([]uint8, 80*80)
	w.Home[2*80+2] = 1
	pawns := append(s.pawns[:0:0], s.pawns...)
	pawns = append(pawns,
		game.PawnView{ID: 4, Name: "Dee", Status: "downed", Reason: "no food", X: 1, Y: 1},
		game.PawnView{ID: 5, Name: "Eli", Status: "alive", Mood: 80, MentalBreak: "wander", X: 2, Y: 2},
		game.PawnView{ID: 6, Name: "Fay", Status: "alive", Mood: 80, Hypothermia: 500, Heatstroke: 500, X: 3, Y: 3},
		game.PawnView{ID: 7, Name: "Gil", Status: "alive", Mood: 80, Bleeding: true, BleedDeathHours: 5, X: 4, Y: 4},
		game.PawnView{ID: 8, Name: "Hal", Status: "alive", Mood: 80, Infection: 40, InfectionLosing: true, Wounded: true, UntendedHours: 6, X: 5, Y: 5})
	got := buildAlerts(pawns, w)
	seen := map[string]bool{}
	for _, al := range got {
		seen[strings.SplitN(al.key, ".", 2)[0]] = true
		if len(al.explain) < 24 {
			t.Errorf("alert %q has explanation %q", al.key, al.explain)
		}
		if al.explain == alertExplain("no-such-alert") {
			t.Errorf("alert %q fell back to the generic explanation", al.key)
		}
	}
	for _, k := range []string{"bleeding", "infection", "untended", "hostile", "raid", "rescue", "starving", "collapsed", "food", "break", "hypothermia",
		"heatstroke", "nofuel", "waitingframes", "unhappy", "fire", "penopen", "unpenned", "timbergnaw"} {
		if !seen[k] {
			t.Errorf("fixture does not raise %q (got %v)", k, alertKeys(got))
		}
	}
	if alertExplain("no-such-alert") == "" {
		t.Error("an unknown alert still gets a generic explanation")
	}
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
		ids[b.id] = b
	}
	for _, r := range rows {
		b, ok := ids["alert."+r.al.key]
		if !ok || b.x != r.r.X || b.y != r.r.Y || b.w != r.r.W || b.h != r.r.H {
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
		if strings.Contains(b.label, "more") {
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
				if box.right() > row.r.X {
					t.Errorf("row %d: card %+v overlaps the alert %+v", i, box, row.r)
				}
				if box.X < 0 || box.Y < 0 || box.right() > float32(a.sw) || box.bottom() > float32(a.sh) {
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

// A card for a row at the bottom of a short screen is pulled back up.
func TestRightCardIsClampedOnScreen(t *testing.T) {
	t.Parallel()
	c := rightCard{title: "Title", body: strings.Repeat("A long explanation that wraps over several lines. ", 6)}
	for _, anchor := range []rect{{1040, 690, 232, 24}, {1040, 0, 232, 24}, {1040, 360, 232, 24}} {
		box, title, body := layoutRightCard(c, anchor, 1280, 720)
		if box.right() > anchor.X || box.X < 0 || box.Y < 0 || box.bottom() > 720 {
			t.Errorf("anchor %+v: card %+v", anchor, box)
		}
		if len(title) == 0 || len(body) < 2 {
			t.Errorf("anchor %+v: title %v body %v", anchor, title, body)
		}
	}
	// a screen too narrow for the card still keeps it on screen
	box, _, _ := layoutRightCard(c, rect{150, 100, 100, 24}, 300, 300)
	if box.X < 0 || box.right() > 300 {
		t.Errorf("narrow screen: card %+v", box)
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
		x, y := a.renderer.TileAt(int((v.x0+v.x1)/2), int((v.y0+v.y1)/2))
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
	f.cur.mx, f.cur.my = int(row.X+row.W/2), int(row.Y+row.H/2)
	f.click(t, a, mbRight, 0) // right: back to the first
	if c := centre(); !near(c, at(0)) {
		t.Fatalf("right-click centres %v, want back at %v", c, at(0))
	}
	f.click(t, a, mbRight, 0) // right again: wraps to the last
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

// The danger backing pulses; reduced motion holds it still.
func TestAlertPulse(t *testing.T) {
	t.Parallel()
	lo, hi := 2.0, -1.0
	for p := uint64(0); p < 120; p++ {
		v := alertPulse(p, false)
		lo, hi = min(lo, v), max(hi, v)
		if v < 0.5 || v > 1.0001 {
			t.Fatalf("pulse(%d) = %v out of range", p, v)
		}
		if alertPulse(p, true) != 1 {
			t.Fatalf("reduced motion pulse(%d) = %v, want a steady 1", p, alertPulse(p, true))
		}
	}
	if hi-lo < 0.2 {
		t.Fatalf("pulse barely moves: %v..%v", lo, hi)
	}
}

// TestBleedingAlert (MED-01): a colonist bleeding out within 12 hours
// raises a danger alert on it; a slower bleed, a dead colonist, a
// prisoner and a raider do not.
func TestBleedingAlert(t *testing.T) {
	t.Parallel()
	pawns := []game.PawnView{
		{ID: 1, Name: "Ada", Status: "downed", Reason: "injured", InBed: true, Bleeding: true, BleedDeathHours: 9, X: 3, Y: 4},
		{ID: 2, Name: "Bo", Status: "alive", Mood: 80, Bleeding: true, BleedDeathHours: 13},
		{ID: 3, Name: "Cy", Status: "dead", Bleeding: true, BleedDeathHours: 2},
		{ID: 4, Name: "Dorn", Status: "downed", Prisoner: true, Bleeding: true, BleedDeathHours: 2},
		{ID: 5, Name: "Krell", Status: "downed", Hostile: true, Lying: true, Bleeding: true, BleedDeathHours: 2},
		{ID: 6, Name: "Eve", Status: "alive", Mood: 80, BleedDeathHours: 0}, // not bleeding: the zero hours mean nothing
	}
	got := buildAlerts(pawns, game.WorldView{Food: 100})
	var bleed []alert
	for _, a := range got {
		if a.key == "bleeding" {
			bleed = append(bleed, a)
		}
	}
	if len(bleed) != 1 || bleed[0].label != "Ada is bleeding out" || !bleed[0].danger ||
		len(bleed[0].targets) != 1 || bleed[0].targets[0] != [2]int{3, 4} {
		t.Fatalf("bleeding alerts %+v (all %v)", bleed, alertKeys(got))
	}
	if got[0].key != "bleeding" {
		t.Fatalf("a danger sorts first: %v", alertKeys(got))
	}
	pawns[1].BleedDeathHours = 12
	got = buildAlerts(pawns, game.WorldView{Food: 100})
	for _, a := range got {
		if a.key == "bleeding" && a.label != "2 colonists are bleeding out" {
			t.Fatalf("two bleeding: %q", a.label)
		}
	}
}

// TestInfectionAlert (MED-02): a colonist losing to an infection and one
// with a wound untended 4 hours or more raise warnings; a winning
// infection, a fresh wound, the dead and prisoners do not.
func TestInfectionAlert(t *testing.T) {
	t.Parallel()
	pawns := []game.PawnView{
		{ID: 1, Name: "Ada", Status: "alive", Mood: 80, Infection: 40, InfectionLosing: true, X: 3, Y: 4},
		{ID: 2, Name: "Bo", Status: "alive", Mood: 80, Infection: 40},
		{ID: 3, Name: "Cy", Status: "dead", Infection: 90, InfectionLosing: true, Wounded: true, UntendedHours: 9},
		{ID: 4, Name: "Dorn", Status: "downed", Prisoner: true, Infection: 60, InfectionLosing: true, Wounded: true, UntendedHours: 9},
		{ID: 5, Name: "Eve", Status: "alive", Mood: 80, Wounded: true, UntendedHours: 4, X: 5, Y: 6},
		{ID: 6, Name: "Fin", Status: "alive", Mood: 80, Wounded: true, UntendedHours: 3},
	}
	got := buildAlerts(pawns, game.WorldView{Food: 100})
	find := func(key string) []alert {
		var out []alert
		for _, a := range got {
			if a.key == key {
				out = append(out, a)
			}
		}
		return out
	}
	inf := find("infection")
	if len(inf) != 1 || inf[0].label != "Ada is losing to an infection" || inf[0].danger || inf[0].sev != sevWarning ||
		len(inf[0].targets) != 1 || inf[0].targets[0] != [2]int{3, 4} || inf[0].explain == alertExplain("no-such-alert") {
		t.Fatalf("infection alerts %+v (all %v)", inf, alertKeys(got))
	}
	un := find("untended")
	if len(un) != 1 || un[0].label != "Eve has an untended wound" || un[0].danger || un[0].targets[0] != [2]int{5, 6} ||
		un[0].explain == alertExplain("no-such-alert") {
		t.Fatalf("untended alerts %+v (all %v)", un, alertKeys(got))
	}
	pawns[1].InfectionLosing = true
	got = buildAlerts(pawns, game.WorldView{Food: 100})
	if inf := find("infection"); len(inf) != 1 || inf[0].label != "2 colonists are losing to infections" {
		t.Fatalf("two losing: %+v", inf)
	}
}
