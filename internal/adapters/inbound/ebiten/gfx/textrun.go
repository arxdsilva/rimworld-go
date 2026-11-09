package gfx

import (
	"math"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// RunCap bounds how many rendered runs a Runs keeps.
const RunCap = 256

// Runs caches the rendered image of short integer labels (stack counts) so a
// repeated number is one DrawImage instead of a shaping pass and one draw per
// glyph (GAR-160). The zero value is ready; a Runs belongs to one draw
// layer and, like every canvas helper, to the main goroutine.
//
// A run is rendered white at the origin of a layout and tinted when drawn, so
// it lands on the same pixels as Text: Ebitengine lays a line out at (0, 0)
// and only then translates the glyph images by the draw offset. It is keyed by
// the number, the face it is rasterised with (a UI scale or font change makes
// another face), the HUD scale its metrics were measured at and the style
// size; the colour is not part of the key. At most RunCap runs are kept, the
// least recently drawn evicted first.
type Runs struct {
	m    map[runKey]*Run
	tick uint64
}

type runKey struct {
	n       int
	f       *text.GoTextFace
	sc, hud float64
	size    float64
	bold    bool
}

// Run is one rendered number and its logical metrics.
type Run struct {
	s    string
	img  *ebiten.Image
	pad  int
	used uint64
	// W and Step are TextW and LineStep of the number in the style it was
	// made for.
	W, Step float32
}

// Len is how many runs are cached.
func (c *Runs) Len() int { return len(c.m) }

// Int returns the cached run of n in st, rendering it on first use.
func (c *Runs) Int(n int, st TextStyle) *Run {
	sc := UIScale()
	fontMu.Lock()
	f, _ := face(st, sc)
	fontMu.Unlock()
	key := runKey{n: n, f: f, sc: sc, hud: HUDScale(), size: sizeOf(st), bold: st.Bold}
	c.tick++
	if r := c.m[key]; r != nil {
		r.used = c.tick
		return r
	}
	if c.m == nil {
		c.m = make(map[runKey]*Run, RunCap)
	}
	if len(c.m) >= RunCap {
		c.evict()
	}
	r := &Run{s: strconv.Itoa(n), used: c.tick, W: TextW(strconv.Itoa(n), st), Step: LineStep(st)}
	r.render(f, st, sc)
	c.m[key] = r
	return r
}

// evict drops the least recently drawn run.
func (c *Runs) evict() {
	var oldK runKey
	var old *Run
	for k, r := range c.m {
		if old == nil || r.used < old.used {
			oldK, old = k, r
		}
	}
	if old != nil {
		if old.img != nil {
			old.img.Deallocate()
		}
		delete(c.m, oldK)
	}
}

// render draws the run's white glyphs into its own image, inset by pad so
// glyph overhang is kept.
func (r *Run) render(f *text.GoTextFace, st TextStyle, sc float64) {
	fontMu.Lock()
	defer fontMu.Unlock()
	_, k := face(st, sc)
	ls := logicalLineH(f, k, sizeOf(st)) * sc
	w, h := text.Measure(r.s, f, ls)
	r.pad = int(math.Ceil(f.Size/2)) + 1
	iw, ih := int(math.Ceil(w))+2*r.pad, int(math.Ceil(h))+2*r.pad
	if iw <= 2*r.pad || ih <= 2*r.pad {
		return
	}
	r.img = ebiten.NewImage(iw, ih)
	op := &text.DrawOptions{}
	op.GeoM.Translate(float64(r.pad), float64(r.pad))
	op.LineSpacing = ls
	text.Draw(r.img, r.s, f, op)
}

// Draw draws the run with its top-left at logical (x, y), as Text(dst, the
// number, x, y, st) does. A draw that Text would queue (a world pass) or
// trace goes through Text itself.
func (r *Run) Draw(dst *ebiten.Image, x, y float64, st TextStyle) {
	if r.img == nil {
		return
	}
	if deferring(dst) || cur.drawTrace != nil {
		Text(dst, r.s, x, y, st)
		return
	}
	sc := UIScale()
	var op ebiten.DrawImageOptions
	op.GeoM.Translate(x*sc-float64(r.pad), y*sc-float64(r.pad))
	c := st.Color
	if c == nil {
		c = TextColor
	}
	op.ColorScale.ScaleWithColor(c)
	dst.DrawImage(r.img, &op)
}
