package hud

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/application/game"
)

func messageFake(texts ...string) (*Chrome, *fakeShell) {
	c, f := newFake()
	f.lay = hudlayout.Layout{Messages: gfx.Rect{X: 700, Y: 100, W: 300, H: 200}}
	for i, tx := range texts {
		f.w.Messages = append(f.w.Messages, game.MessageView{Tick: uint64(10 + i), Text: tx})
	}
	return c, f
}

func texts(c *Chrome) []string {
	var out []string
	for _, m := range c.Right().liveMessages() {
		out = append(out, m.Text)
	}
	return out
}

func TestRightClickDismissesExactlyThatMessage(t *testing.T) {
	t.Parallel()
	c, f := messageFake("one", "two", "three")
	rows := c.Right().MessageRows(f.lay)
	if len(rows) != 3 {
		t.Fatalf("%d rows", len(rows))
	}
	mid := rows[1].R
	if !c.Right().RightClick(int(mid.X+mid.W/2), int(mid.Y+mid.H/2)) {
		t.Fatal("the click was not taken")
	}
	if got := texts(c); len(got) != 2 || got[0] != "one" || got[1] != "three" {
		t.Fatalf("after dismissal %v", got)
	}
	if got := len(c.Right().MessageRows(f.lay)); got != 2 || c.Right().MessageCount() != 2 {
		t.Fatalf("rows %d, count %d", got, c.Right().MessageCount())
	}
}

func TestDismissedMessageStaysGoneThenAFreshOneShows(t *testing.T) {
	t.Parallel()
	c, f := messageFake("one", "two")
	r := c.Right().MessageRows(f.lay)[0].R
	c.Right().RightClick(int(r.X+1), int(r.Y+1))
	for i := 0; i < 5; i++ {
		f.present += 50
		if got := texts(c); len(got) != 1 || got[0] != "two" {
			t.Fatalf("frame %d: %v", i, got)
		}
	}
	// it leaves the list, and is forgotten
	f.w.Messages = f.w.Messages[1:]
	texts(c)
	if len(c.Right().session().dismissed) != 0 {
		t.Fatalf("dismissed keys kept: %v", c.Right().session().dismissed)
	}
	// a later notice with the same words, at another tick, is a new one
	f.w.Messages = append(f.w.Messages, game.MessageView{Tick: 99, Text: "one"})
	if got := texts(c); len(got) != 2 || got[1] != "one" {
		t.Fatalf("fresh notice %v", got)
	}
}
