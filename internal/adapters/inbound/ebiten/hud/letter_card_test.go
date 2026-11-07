package hud

import (
	"fmt"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/application/game"
)

// letterCardRow is a letter row where the 1280 x 720 layout puts the
// stack: the right column, 232 wide.
var letterCardRow = gfx.Rect{X: 1040, Y: 420, W: 232, H: hudlayout.LetterRowH}

// The hover card of a letter (UIS-032) is a fixed LetterCardW x LetterCardH
// box whose right edge is LetterCardGap left of the letter's tile, not of
// the 232-wide row, level with the tile and kept on the screen.
func TestLetterCardIsFixedSizeLeftOfTile(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("A letter that goes on for a while. ", 30)
	for _, body := range []string{"Short.", long} {
		tile := LetterTile(letterCardRow)
		box, _, _ := LayoutLetterCard(tile, "Raid", body, 1280, 720)
		if box.W != hudlayout.LetterCardW || box.H != hudlayout.LetterCardH {
			t.Fatalf("card %+v, want %v x %v whatever the text", box, hudlayout.LetterCardW, hudlayout.LetterCardH)
		}
		if got := tile.X - box.Right(); got != hudlayout.LetterCardGap {
			t.Fatalf("card ends %v left of the tile %+v, want %v", got, tile, hudlayout.LetterCardGap)
		}
		if box.Y != tile.Y {
			t.Fatalf("card top %v, want the tile's %v", box.Y, tile.Y)
		}
		onScreen(t, box, 1280, 720)
	}
	// a tile at the bottom of the screen: the card moves up, keeps its size
	low := LetterTile(gfx.Rect{X: 1040, Y: 700, W: 232, H: hudlayout.LetterRowH})
	box, _, _ := LayoutLetterCard(low, "Raid", "Short.", 1280, 720)
	if box.H != hudlayout.LetterCardH || box.Right() != low.X-hudlayout.LetterCardGap {
		t.Fatalf("low card %+v", box)
	}
	onScreen(t, box, 1280, 720)
	// a 300 x 300 screen: the card stays on it, as tall as ever
	small := LetterTile(gfx.Rect{X: 60, Y: 280, W: 232, H: hudlayout.LetterRowH})
	box, title, lines := LayoutLetterCard(small, "Raid", long, 300, 300)
	if box.H != hudlayout.LetterCardH || box.W > 300-2*hudlayout.HudMargin || box.W < 200 {
		t.Fatalf("300x300 card %+v", box)
	}
	onScreen(t, box, 300, 300)
	if title == "" || len(lines) == 0 {
		t.Fatalf("300x300 card holds title %q lines %q", title, lines)
	}
}

func onScreen(t *testing.T, box gfx.Rect, sw, sh int) {
	t.Helper()
	if box.X < 0 || box.Y < 0 || box.Right() > float32(sw) || box.Bottom() > float32(sh) {
		t.Fatalf("card %+v off the %dx%d screen", box, sw, sh)
	}
}

// Every text run of a letter's card lies inside the card, with short and
// long letters, normal and larger small text; a letter too long for the
// card ends its last line with an ellipsis, a short one shows all of it.
// Not parallel: the draw trace is process-wide.
func TestLetterCardTextStaysInside(t *testing.T) {
	long := strings.Repeat("The raiders came over the ridge at dawn with torches. ", 12)
	for _, larger := range []bool{false, true} {
		for _, body := range []string{"Short.", long, "unbroken" + strings.Repeat("x", 300)} {
			t.Run(fmt.Sprintf("larger=%v/%d", larger, len(body)), func(t *testing.T) {
				gfx.SetLargerSmall(larger)
				t.Cleanup(func() { gfx.SetLargerSmall(false) })
				_, f := newFake()
				var at [2]int
				c := New(frShell{f, catalog{}, &at})
				f.w.Letters = []game.LetterView{{ID: 3, Kind: "threat", Title: "A very long letter title that does not fit in the card at all", Text: body, X: -1, Y: -1}}
				f.lay = hudlayout.Layout{Letters: gfx.Rect{X: 1040, Y: 300, W: 232, H: 400}, LetterRows: 4}
				rows := c.Right().LetterRows(f.lay)
				cx, cy := rows[0].R.Center()
				at = [2]int{int(cx), int(cy)}
				card, anchor, ok := c.Right().Hover(f.lay)
				if !ok || !card.Letter {
					t.Fatalf("hover %+v %v, want a letter card", card, ok)
				}
				box, _, lines := LayoutLetterCard(LetterTile(anchor), card.Title, card.Body, 1280, 720)
				var evs []gfx.TraceEvent
				gfx.SetDrawTrace(func(e gfx.TraceEvent) { evs = append(evs, e) })
				t.Cleanup(func() { gfx.SetDrawTrace(nil) })
				c.Right().Draw(ebiten.NewImage(1280, 720), 1280, 720)
				gfx.SetDrawTrace(nil)
				const eps = 0.6
				start := -1 // the card is drawn last, over the rows: its fill starts it
				for i, e := range evs {
					if e.Kind == "fill" && gfx.Abs32(e.X-box.X) < eps && gfx.Abs32(e.Y-box.Y) < eps && gfx.Abs32(e.W-box.W) < eps && gfx.Abs32(e.H-box.H) < eps {
						start = i
					}
				}
				if start < 0 {
					t.Fatalf("no card filled at %+v", box)
				}
				inCard := 0
				for _, e := range evs[start:] {
					if e.Kind != "text" {
						continue
					}
					inCard++
					if e.X < box.X-eps || e.X+e.W > box.Right()+eps || e.Y < box.Y-eps || e.Y+e.H > box.Bottom()+eps {
						t.Errorf("text %q at %v,%v %vx%v leaves the card %+v", e.Text, e.X, e.Y, e.W, e.H, box)
					}
				}
				if inCard < 2 {
					t.Fatalf("%d text runs in the card %+v", inCard, box)
				}
				last := lines[len(lines)-1]
				if cut := body != "Short."; cut != strings.HasSuffix(last, "…") {
					t.Errorf("cut=%v but the last line is %q", cut, last)
				}
			})
		}
	}
}

// An opened letter (UIS-073) is a frameless 620-wide window with its
// choices as rows: 480 high at least, taller when its text needs it.
func TestLetterWindowIsFramelessWithRows(t *testing.T) {
	t.Parallel()
	c, f := newFake()
	c.Right().openLetterView(game.LetterView{ID: 3, Title: "Raid", Text: "Short.", X: 4, Y: 5})
	long := strings.Repeat("A letter that goes on and on about the weather. ", 60)
	c.Right().openLetterView(game.LetterView{ID: 4, Title: "Saga", Text: long, X: -1, Y: -1})
	if len(f.ui.Modals) != 2 {
		t.Fatalf("modals %+v", f.ui.Modals)
	}
	short, tall := f.ui.Modals[0], f.ui.Modals[1]
	for _, m := range f.ui.Modals {
		if !m.Frameless || !m.RowActions || m.W != 620 || m.H < 480 {
			t.Fatalf("letter %s: frameless %v rows %v %vx%v", m.ID, m.Frameless, m.RowActions, m.W, m.H)
		}
	}
	if short.H != 480 || len(short.Buttons) != 2 || short.Buttons[0].ID != "continue" || short.Buttons[1].ID != "jump" {
		t.Fatalf("short letter %vx%v buttons %+v", short.W, short.H, short.Buttons)
	}
	if tall.H <= 480 {
		t.Fatalf("a long letter is %v high, want it to grow past 480", tall.H)
	}
}
