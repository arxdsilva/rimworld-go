package hud

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/application/game"
)

// The older row is a button that opens the newest letter folded into it
// (RHUD-474), and the layout's gap spaces the rows (RHUD-475).
func TestLetterOlderRowOpensNewestFolded(t *testing.T) {
	t.Parallel()
	c, f := newFake()
	for i := uint64(1); i <= 6; i++ {
		f.w.Letters = append(f.w.Letters, game.LetterView{ID: i, Title: "L"})
	}
	f.lay = hudlayout.Layout{Letters: gfx.Rect{X: 900, Y: 300, W: 200, H: 400}, LetterRows: 4, LetterGap: 10}
	rows := c.Right().LetterRows(f.lay)
	if len(rows) == 0 || rows[0].Older == 0 {
		t.Fatalf("rows %+v, want a +N older row first", rows)
	}
	if want := uint64(rows[0].Older); rows[0].Opens != want {
		t.Fatalf("older row opens %d, want %d (the newest folded)", rows[0].Opens, want)
	}
	if d := rows[1].R.Y - rows[0].R.Bottom(); d != 10 {
		t.Fatalf("gap %v, want 10", d)
	}
	var found bool
	for _, b := range c.Right().LetterButtons() {
		found = found || b.ID == "letter."+string(rune('0'+rows[0].Opens))
	}
	if !found {
		t.Fatal("no button for the older row")
	}
}
