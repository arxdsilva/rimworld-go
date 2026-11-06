package hud

import (
	"fmt"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/application/game"
	"rimworld-go/pkg/msgfmt"
)

// catalog is a hand-built translation; a missing key is the source.
type catalog map[string]string

func (c catalog) T(s string) string {
	if v, ok := c[s]; ok {
		return v
	}
	return s
}

// frShell is the fake shell speaking c's language.
type frShell struct {
	*fakeShell
	c  catalog
	at *[2]int // the cursor
}

func (f frShell) Cursor() (int, int) { return f.at[0], f.at[1] }

func (f frShell) T(src string) string { return f.c.T(src) }

func (f frShell) Tf(src string, args ...any) string { return fmt.Sprintf(f.c.T(src), args...) }

var fr = catalog{
	"%s tamed a %s.": "%s a apprivoisé un %s.", "boar": "sanglier",
	"Raid": "Raid ennemi", "Letter: %s": "Lettre : %s", "Letter": "Lettre",
	"%s will send a trade caravan.": "%s enverra une caravane.",
}

// English draws a message or letter's Text byte for byte, with or without
// Msg; another catalog translates the format, each argument by exact key,
// and the title. A view without Msg (an older save) keeps its Text.
func TestMessagesAndLettersGoThroughTheCatalog(t *testing.T) {
	t.Parallel()
	tamed := msgfmt.Of("%s tamed a %s.", "Ann", "boar")
	with := game.MessageView{Text: tamed.String(), Msg: tamed}
	without := game.MessageView{Text: "Ann tamed a boar."}
	for _, m := range []game.MessageView{with, without} {
		if got := MessageLineIn(lang.English{}, m); got != "Ann tamed a boar." {
			t.Fatalf("English line %q", got)
		}
	}
	if got := MessageLineIn(fr, with); got != "Ann a apprivoisé un sanglier." {
		t.Fatalf("translated line %q", got)
	}
	if got := MessageLineIn(fr, without); got != "Ann tamed a boar." {
		t.Fatalf("a view without Msg must show its Text, got %q", got)
	}
	notice := game.MessageView{Letter: true, Title: "Raid", Text: "x"}
	if MessageLineIn(lang.English{}, notice) != "Letter: Raid" || MessageLineIn(fr, notice) != "Lettre : Raid ennemi" {
		t.Fatalf("letter notice %q / %q", MessageLineIn(lang.English{}, notice), MessageLineIn(fr, notice))
	}
	l := game.LetterView{ID: 7, Title: "Raid", Text: tamed.String(), Msg: tamed}
	if LetterRowLabelIn(lang.English{}, LetterRow{Letter: l}) != "Raid" || LetterRowLabelIn(fr, LetterRow{Letter: l}) != "Raid ennemi" {
		t.Fatal("letter row title")
	}
	if LetterTextIn(lang.English{}, l) != l.Text || LetterTextIn(fr, l) != "Ann a apprivoisé un sanglier." {
		t.Fatalf("letter body %q", LetterTextIn(fr, l))
	}
}

// The letter window and the row's hover card read the letter in the
// shell's language; the window id keeps the English title.
func TestLetterWindowAndCardTranslate(t *testing.T) {
	t.Parallel()
	_, f := newFake()
	var at [2]int
	c := New(frShell{f, fr, &at})
	trade := msgfmt.Of("%s will send a trade caravan.", "The Ridge")
	l := game.LetterView{ID: 3, Tick: 9, Title: "Raid", Text: trade.String(), Msg: trade}
	f.w.Letters = []game.LetterView{l}
	f.lay = hudlayout.Layout{Letters: gfx.Rect{X: 900, Y: 300, W: 200, H: 400}, LetterRows: 4}
	rows := c.Right().LetterRows(f.lay)
	cx, cy := rows[0].R.Center()
	at = [2]int{int(cx), int(cy)}
	card, _, ok := c.Right().Hover(f.lay)
	if !ok || card.Title != "Raid ennemi" || !strings.HasPrefix(card.Body, "The Ridge enverra une caravane.") {
		t.Fatalf("card %+v", card)
	}
	c.Right().openLetterView(l)
	if n := len(f.ui.Modals); n != 1 || f.ui.Modals[0].Title != "Raid ennemi" || f.ui.Modals[0].ID != "letter.3" {
		t.Fatalf("window %+v", f.ui.Modals)
	}
	// A message opened as a letter carries its Msg into the window.
	f.ui.Modals = nil
	c.Right().OpenLetter(game.MessageView{Tick: 4, Letter: true, Title: "", Text: trade.String(), Msg: trade})
	if len(f.ui.Modals) != 1 || f.ui.Modals[0].Title != "Lettre" || f.ui.Modals[0].ID != "letter.4.Letter" {
		t.Fatalf("opened message %+v", f.ui.Modals)
	}
}
