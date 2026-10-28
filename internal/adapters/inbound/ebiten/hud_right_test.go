package ebiten

import (
	"fmt"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// rightAtScale puts a at UI scale s over a physical screen that is 1280x720
// logical and returns the physical size.
func rightAtScale(t *testing.T, a *App, f *fakeInput, s float64) (int, int) {
	t.Helper()
	t.Cleanup(func() { gfx.SetScale(unitUIScale) })
	a.devFn = func() float64 { return 1 } // not the test machine's Retina factor
	st := a.settings
	st.Accessibility.UIScaleAuto, st.Accessibility.UIScale = false, int(s*100)
	a.setSettings(st)
	pw, ph := a.LayoutF(1280*s, 720*s)
	if a.sw != 1280 || a.sh != 720 {
		t.Fatalf("scale %v: logical %dx%d, want 1280x720", s, a.sw, a.sh)
	}
	f.Frame(t, a)
	return int(pw), int(ph)
}

// The counts the layout reserves rows for are the real ones, and read
// before the layout exists (they must never ask for it).
func TestRightColumnCountsAreTheRealOnes(t *testing.T) {
	t.Parallel()
	s := rightWorld()
	s.world.Letters = []game.LetterView{{ID: 1, Kind: "info", Title: "One", X: -1, Y: -1}, {ID: 2, Kind: "good", Title: "Two", X: -1, Y: -1}}
	s.world.Messages = []game.MessageView{{Tick: 1, Text: "a"}, {Tick: 2, Text: "b"}, {Tick: 3, Text: "c"}}
	a, _ := colonyApp(t, s)
	if got, want := a.alertCount(), len(a.currentAlerts()); got != want || got == 0 {
		t.Errorf("alertCount() = %d, want %d", got, want)
	}
	if got := a.letterCount(); got != 2 {
		t.Errorf("letterCount() = %d, want 2", got)
	}
	if got := a.messageCount(); got != 3 {
		t.Errorf("messageCount() = %d, want 3", got)
	}
	l := a.hud()
	if l.LetterRows != 2 || l.AlertRows != a.alertCount() || l.Messages.Empty() {
		t.Errorf("layout rows: letters %d, alerts %d, messages %+v", l.LetterRows, l.AlertRows, l.Messages)
	}
	// no colony: nothing to count
	b := testApp(&stubSim{w: 4, h: 4})
	if b.alertCount()+b.letterCount()+b.messageCount() != 0 {
		t.Error("counts without a renderer")
	}
	// F11 hides the HUD, so it also hides the column's buttons
	a.hideUI = true
	if len(a.rightColumnButtons(a.sw)) != 0 {
		t.Error("buttons while the interface is hidden")
	}
}

// A right-click anywhere over the column is the column's (it never moves a
// drafted colonist underneath), and one beside it is not.
func TestRightClickOverTheColumnIsConsumed(t *testing.T) {
	t.Parallel()
	s := rightWorld()
	s.world.Letters = []game.LetterView{{ID: 1, Kind: "info", Title: "One", X: -1, Y: -1}}
	a, _ := colonyApp(t, s)
	l := a.hud()
	for name, r := range map[string]rect{"alerts": l.Alerts, "letters": l.Letters} {
		if r.Empty() {
			t.Fatalf("no %s rect", name)
		}
		if !a.rightClickRightColumn(int(r.X+r.W/2), int(r.Y+r.H/2)) {
			t.Errorf("right-click on the %s rect not taken", name)
		}
	}
	if a.rightClickRightColumn(10, int(l.Tabs.Y)-100) {
		t.Error("right-click on the map taken")
	}
}

// pressButton left-clicks the legacy HUD button id with the mouse, the
// real path (two frames), then moves the cursor away.
func pressButton(t *testing.T, a *App, f *fakeInput, id string) {
	t.Helper()
	f.Frame(t, a)
	for _, b := range a.layoutHits(a.sw, a.sh) {
		if b.id == id {
			f.clickAt(t, a, int(b.x+b.w/2), int(b.y+b.h/2))
			f.Cur.Mx, f.Cur.My = -50, -50
			return
		}
	}
	t.Fatalf("no button %q on %s", id, a.nav.screen())
}

// Hovering a letter previews it, the +N older row says how to reach the
// rest, and the +N more alerts row lists what it hides.
func TestRightHoverCardsForLettersAndFoldedRows(t *testing.T) {
	t.Parallel()
	a, _ := colonyApp(t, letterStub(nLetters(10)...))
	l := a.hud()
	rows := a.letterRows(l)
	cursorOver(a, rows[0].r) // "+N older"
	card, anchor, ok := a.rightHover(l)
	// the count is what the folded row hides (alerts keep a few rows, so
	// fewer letters may show)
	if want := fmt.Sprintf("%d older letters", rows[0].older); !ok || rows[0].older < 2 || anchor != rows[0].r || card.title != want || card.body == "" {
		t.Fatalf("older row card = %+v %+v %v", card, anchor, ok)
	}
	cursorOver(a, rows[len(rows)-1].r) // the newest letter
	card, _, ok = a.rightHover(l)
	last := rows[len(rows)-1].letter
	if !ok || card.title != last.Title || card.clr != letterColor(last.Kind) || !strings.Contains(card.body, last.Text) {
		t.Fatalf("letter card = %+v %v", card, ok)
	}
	// a long letter text is cut in the preview
	s := a.renderer.game.(*stubSim)
	s.world.Letters[9].Text = strings.Repeat("word ", 100)
	card, _, _ = a.rightHover(l)
	if len([]rune(card.body)) > 300 {
		t.Errorf("preview is %d runes", len([]rune(card.body)))
	}

	// folded alerts
	b, _ := colonyApp(t, rightWorld())
	lay := computeHUDLayout(1280, 720, hudState{Alerts: len(b.currentAlerts()), Letters: 4, GlobalH: 380, LearnH: 28})
	arows := b.alertRows(lay)
	more := arows[len(arows)-1]
	if more.more == 0 {
		t.Fatalf("fixture: no folded row in %d rows", len(arows))
	}
	cursorOver(b, more.r)
	card, _, ok = b.rightHover(lay)
	if !ok || card.title != fmt.Sprintf("%d more alerts", more.more) || !strings.Contains(card.body, more.hidden[0].label) {
		t.Fatalf("+N more card = %+v %v", card, ok)
	}

	// no card while a modal is up or the interface is hidden
	b.uiPushModal(modal{id: "x", title: "x", w: 200, h: 120})
	if _, _, ok := b.rightHover(lay); ok {
		t.Error("card over a modal")
	}
}

func TestStackSplit(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		total, rows, limit int
		shown, folded      int
	}{
		{0, 5, 8, 0, 0},
		{3, 0, 8, 0, 3},  // no room at all
		{3, 5, 8, 3, 0},  // everything fits
		{9, 9, 8, 8, 1},  // over the limit: the cap and a +N row
		{10, 4, 8, 3, 7}, // cramped: the count takes a row
		{5, 1, 5, 1, 0},  // one row has no room for a count
		{12, 20, 99, 12, 0},
	} {
		shown, folded := stackSplit(c.total, c.rows, c.limit)
		if shown != c.shown || folded != c.folded {
			t.Errorf("stackSplit(%d, %d, %d) = %d, %d; want %d, %d", c.total, c.rows, c.limit, shown, folded, c.shown, c.folded)
		}
	}
}

// The compatibility entry points other files and tests still call.
func TestRightColumnWrappers(t *testing.T) {
	t.Parallel()
	s := rightWorld()
	s.world.Letters = []game.LetterView{{ID: 7, Kind: "caution", Title: "Cold snap", Text: "It freezes.", X: -1, Y: -1}}
	a, _ := colonyApp(t, s)
	if len(a.alertButtons(a.sw)) == 0 || len(a.letterButtons()) != 1 || a.letterButtons()[0].id != "letter.7" {
		t.Errorf("alert buttons %v, letter buttons %v", a.alertButtons(a.sw), a.letterButtons())
	}
	img := ebiten.NewImage(a.sw, a.sh)
	a.drawAlerts(img, a.sw)
	a.drawGatheringBanner(img, a.sw)
	a.rightColumnUI(a.sw, a.sh)
	assertChromeIsTheLayout(t, a) // the column has no legacy chrome
	// a message that is a letter still opens its window through openLetter
	a.openLetter(game.MessageView{Tick: 3, Title: "Heat wave", Letter: true, Text: "It is hot.", X: 4, Y: 5})
	if m := a.uiSt().modals; len(m) != 1 || m[0].title != "Heat wave" || len(m[0].buttons) != 2 {
		t.Errorf("modals %+v, want the letter with Continue and Jump to", m)
	}
	// a letter id that is not a number, or not in the stack, does nothing
	a.applyLetterClick("letter.older")
	a.applyLetterClick("letter.999")
	if len(a.uiSt().modals) != 1 {
		t.Errorf("modals %d after clicks on nothing", len(a.uiSt().modals))
	}
}
