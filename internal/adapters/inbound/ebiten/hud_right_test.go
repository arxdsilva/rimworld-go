package ebiten

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/application/game"
)

// rightAtScale puts a at UI scale s over a physical screen that is 1280x720
// logical and returns the physical size.
func rightAtScale(t *testing.T, a *App, f *fakeInput, s float64) (int, int) {
	t.Helper()
	t.Cleanup(func() { gfx.SetScale(gfx.UnitScale) })
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
	if got, want := a.chrome.Alerts().Count(), len(a.currentAlerts()); got != want || got == 0 {
		t.Errorf("alertCount() = %d, want %d", got, want)
	}
	if got := a.chrome.Right().LetterCount(); got != 2 {
		t.Errorf("letterCount() = %d, want 2", got)
	}
	if got := a.chrome.Right().MessageCount(); got != 3 {
		t.Errorf("messageCount() = %d, want 3", got)
	}
	l := a.hud()
	if l.LetterRows != 2 || l.AlertRows != a.chrome.Alerts().Count() || l.Messages.Empty() {
		t.Errorf("layout rows: letters %d, alerts %d, messages %+v", l.LetterRows, l.AlertRows, l.Messages)
	}
	// no colony: nothing to count
	b := testApp(&stubSim{w: 4, h: 4})
	if b.chrome.Alerts().Count()+b.chrome.Right().LetterCount()+b.chrome.Right().MessageCount() != 0 {
		t.Error("counts without a renderer")
	}
	// F11 hides the HUD, so it also hides the column's buttons
	a.chrome.Hidden = true
	if len(a.chrome.Right().Buttons(a.sw)) != 0 {
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
	for name, r := range map[string]gfx.Rect{"alerts": l.Alerts, "letters": l.Letters} {
		if r.Empty() {
			t.Fatalf("no %s rect", name)
		}
		if !a.chrome.Right().RightClick(int(r.X+r.W/2), int(r.Y+r.H/2)) {
			t.Errorf("right-click on the %s rect not taken", name)
		}
	}
	if a.chrome.Right().RightClick(10, int(l.Tabs.Y)-100) {
		t.Error("right-click on the map taken")
	}
}

// pressButton left-clicks the legacy HUD button id with the mouse, the
// real path (two frames), then moves the cursor away.
func pressButton(t *testing.T, a *App, f *fakeInput, id string) {
	t.Helper()
	f.Frame(t, a)
	for _, b := range a.layoutHits(a.sw, a.sh) {
		if b.ID == id {
			f.clickAt(t, a, int(b.X+b.W/2), int(b.Y+b.H/2))
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
	cursorOver(a, rows[0].R) // "+N older"
	card, anchor, ok := a.rightHover(l)
	// the count is what the folded row hides (alerts keep a few rows, so
	// fewer letters may show)
	if want := fmt.Sprintf("%d older letters", rows[0].Older); !ok || rows[0].Older < 2 || anchor != rows[0].R || card.Title != want || card.Body == "" {
		t.Fatalf("older row card = %+v %+v %v", card, anchor, ok)
	}
	cursorOver(a, rows[len(rows)-1].R) // the newest letter
	card, _, ok = a.rightHover(l)
	last := rows[len(rows)-1].Letter
	if !ok || card.Title != last.Title || card.Clr != hud.LetterColor(last.Kind) || !strings.Contains(card.Body, last.Text) {
		t.Fatalf("letter card = %+v %v", card, ok)
	}
	// a long letter text is cut in the preview
	s := a.renderer.game.(*stubSim)
	s.world.Letters[9].Text = strings.Repeat("word ", 100)
	card, _, _ = a.rightHover(l)
	if len([]rune(card.Body)) > 300 {
		t.Errorf("preview is %d runes", len([]rune(card.Body)))
	}

	// folded alerts
	b, _ := colonyApp(t, rightWorld())
	lay := hudlayout.Compute(1280, 720, hudlayout.Inputs{Alerts: len(b.currentAlerts()), Letters: 4, GlobalH: 380, LearnH: 28})
	arows := b.alertRows(lay)
	more := arows[len(arows)-1]
	if more.More == 0 {
		t.Fatalf("fixture: no folded row in %d rows", len(arows))
	}
	cursorOver(b, more.R)
	card, _, ok = b.rightHover(lay)
	if !ok || card.Title != fmt.Sprintf("%d more alerts", more.More) || !strings.Contains(card.Body, more.Hidden[0].Label) {
		t.Fatalf("+N more card = %+v %v", card, ok)
	}

	// no card while a modal is up or the interface is hidden
	b.uiPushModal(modal{ID: "x", Title: "x", W: 200, H: 120})
	if _, _, ok := b.rightHover(lay); ok {
		t.Error("card over a modal")
	}
}

// The compatibility entry points other files and tests still call.
func TestRightColumnWrappers(t *testing.T) {
	t.Parallel()
	s := rightWorld()
	s.world.Letters = []game.LetterView{{ID: 7, Kind: "caution", Title: "Cold snap", Text: "It freezes.", X: -1, Y: -1}}
	a, _ := colonyApp(t, s)
	if len(a.chrome.Alerts().Buttons(a.sw)) == 0 || len(a.chrome.Right().LetterButtons()) != 1 || a.chrome.Right().LetterButtons()[0].ID != "letter.7" {
		t.Errorf("alert buttons %v, letter buttons %v", a.chrome.Alerts().Buttons(a.sw), a.chrome.Right().LetterButtons())
	}
	img := ebiten.NewImage(a.sw, a.sh)
	a.chrome.Alerts().Draw(img, a.sw)
	a.chrome.Banner().Draw(img, a.sw)
	a.chrome.Right().Declare(a.sw, a.sh)
	assertChromeIsTheLayout(t, a) // the column has no legacy chrome
	// a message that is a letter still opens its window through openLetter
	a.chrome.Right().OpenLetter(game.MessageView{Tick: 3, Title: "Heat wave", Letter: true, Text: "It is hot.", X: 4, Y: 5})
	if m := a.uiSt().Modals; len(m) != 1 || m[0].Title != "Heat wave" || len(m[0].Buttons) != 2 {
		t.Errorf("modals %+v, want the letter with Continue and Jump to", m)
	}
	// a letter id that is not a number, or not in the stack, does nothing
	a.chrome.Right().LetterClick("letter.older")
	a.chrome.Right().LetterClick("letter.999")
	if len(a.uiSt().Modals) != 1 {
		t.Errorf("modals %d after clicks on nothing", len(a.uiSt().Modals))
	}
}
