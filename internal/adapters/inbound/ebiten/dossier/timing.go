package dossier

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
)

// Developer timing of the card (RHUD-042). The debug inspector asks for the
// lines every frame it is up; the card only measures while it is asked, so a
// game without developer mode never reads the clock here. Wall-clock time is
// for this overlay alone and reaches nothing else.

const (
	timingShown = 6    // the slowest widgets listed
	timingKeep  = 0.95 // the moving average's weight on what it knew
)

// cardTimer is the card's moving averages, in milliseconds.
type cardTimer struct {
	asked       bool // the overlay read the lines since the last draw
	seen        bool // a draw was measured
	drawMS      float64
	declareMS   float64
	prepareMS   float64
	widgetMS    []float64 // by item index of the layout last drawn
	widgetNames []string
	sorted      []int
}

// settle moves an average towards a new reading.
func settle(avg, ms float64) float64 {
	if avg == 0 {
		return ms
	}
	return avg*timingKeep + ms*(1-timingKeep)
}

func sinceMS(t time.Time) float64 { return float64(time.Since(t)) / float64(time.Millisecond) }

// reset forgets the numbers (another selection).
func (t *cardTimer) reset() {
	t.seen = false
	t.drawMS, t.declareMS, t.prepareMS = 0, 0, 0
	t.widgetMS, t.widgetNames = t.widgetMS[:0], t.widgetNames[:0]
}

// sizeFor keeps one average per item of the layout being drawn.
func (t *cardTimer) sizeFor(n int) {
	if len(t.widgetMS) != n {
		t.widgetMS = make([]float64, n)
		t.widgetNames = make([]string, n)
	}
}

// itemName is how the overlay calls an item.
func itemName(it *item) string {
	switch {
	case it.kind == kRule:
		return "divider"
	case it.text != "":
		return it.text
	case it.id != "":
		return it.id
	}
	return "item"
}

// TimingLines is the card's numbers for the debug inspector, and asks the card
// to go on measuring. Empty until a draw was measured.
func (p *Panel) TimingLines() []string {
	t := &p.timer
	t.asked = true
	if !t.seen {
		return nil
	}
	lines := []string{fmt.Sprintf("card: draw %.3f ms, declare %.3f ms, data and layout %.3f ms", t.drawMS, t.declareMS, t.prepareMS)}
	t.sorted = t.sorted[:0]
	for i := range t.widgetMS {
		if t.widgetNames[i] != "" {
			t.sorted = append(t.sorted, i)
		}
	}
	slices.SortStableFunc(t.sorted, func(a, b int) int { return cmp.Compare(t.widgetMS[b], t.widgetMS[a]) })
	for _, i := range t.sorted[:min(timingShown, len(t.sorted))] {
		lines = append(lines, fmt.Sprintf("  %.3f ms  %s", t.widgetMS[i], t.widgetNames[i]))
	}
	return lines
}

// drawTimed paints the items that fit, each under the clock.
func (p *Panel) drawTimed(dst *ebiten.Image, l *layout, bx gfx.Rect) {
	t := &p.timer
	t.sizeFor(len(l.items))
	for i := range l.items {
		it := &l.items[i]
		if it.y+it.h > bx.H+0.5 {
			t.widgetNames[i] = ""
			continue
		}
		at := time.Now()
		p.drawItem(dst, it, bx.X+it.x, bx.Y+it.y)
		t.widgetMS[i] = settle(t.widgetMS[i], sinceMS(at))
		t.widgetNames[i] = itemName(it)
	}
}
