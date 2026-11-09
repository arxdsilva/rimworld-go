package light

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
)

// The lit batch (GAR-159 for the ground, GAR-161 for the objects). A lit
// sprite used to be one DrawTrianglesShader32 call: Ebitengine merged the
// commands, but each call still paid its CPU path (uniform packing, then
// the ui, mipmap, buffered and atlas layers). While a pass has the batch on
// (Lit.Batch: the live ground pass and the object pass), each lit draw
// appends its quad to one vertex buffer instead, sampling a copy of the
// image it would have sampled (the sprite, or its normal-map pair) on a
// sheet, and the buffer goes out as one call when the destination, the
// sheet page, the shader or the light changes, before any draw that does
// not join (a bake, lighting off, a bloodied figure), when a raw draw is
// about to land (Lit.Flush), and when the pass ends.
//
// The quads keep their order, their vertices (only the source rectangle
// moves to the sheet; a pair's normal-map offset stays relative) and the
// frame's uniforms, so the pixels are those of per-sprite calls from the
// sheet, byte for byte. A sheet copy is a texel copy (BlendCopy, nearest)
// of the whole source with a transparent gutter of one pixel all round,
// which is what an Ebitengine atlas page leaves around an image. Against
// per-sprite calls from the images' own atlas places, a few pixels differ
// by 1/255: the shader's source position is interpolated at another
// magnitude.
//
// The sheet is keyed by the sprite image, not by its pair: the pair cache
// may evict and rebuild a pair (and deallocates the old one mid-frame), but
// the rebuilt pair has the same pixels, so the copy already made serves it
// and no pending quad ever points at a pair.

// sheetSize is the side of a sheet page in pixels.
const sheetSize = 2048

// sheetPages bounds the pages (16 MB each, allocated as needed). The
// busiest capture scenes copy about 48 MB of distinct images, four pages
// at the shelves' fill, and a fifth was needed once the camera moved (the
// paths scene); eight hold that with room and cap the worst case at 128
// MB. A frame that needs more starts the sheet again once, and a second
// time turns the batch off for that frame (the thrash guard, StartFrame).
const sheetPages = 8

// batchQuads caps one call's quads; a longer pass flushes and goes on.
const batchQuads = 8192

// sheetSlot is an image's place on the sheet: its page and the top-left
// of its copy.
type sheetSlot struct {
	page int
	x, y float32
}

// litSheet holds copies of the images drawn so far, packed in rows
// (shelves) with a one-pixel transparent gutter.
type litSheet struct {
	art   *art.Catalog // the catalog the copies came from
	size  int          // page side (0: sheetSize)
	pages []*ebiten.Image
	used  int // pages in use
	at    map[*ebiten.Image]sheetSlot
	// x, y and rowH are the free point on page used-1 and the current
	// shelf's height.
	x, y, rowH int
	op         ebiten.DrawImageOptions
}

// side is the page side.
func (s *litSheet) side() int {
	if s.size > 0 {
		return s.size
	}
	return sheetSize
}

// reset empties the sheet (keeping its pages) for catalog c.
func (s *litSheet) reset(c *art.Catalog) {
	s.art = c
	clear(s.at)
	for _, p := range s.pages[:s.used] {
		p.Clear()
	}
	s.used, s.x, s.y, s.rowH = 0, 0, 0, 0
}

// fits reports whether an image of w x h can ever go on a page.
func (s *litSheet) fits(w, h int) bool {
	return w > 0 && h > 0 && w+2 <= s.side() && h+2 <= s.side()
}

// add copies src onto the sheet as img's; ok is false when the sheet is
// full.
func (s *litSheet) add(img, src *ebiten.Image) (sheetSlot, bool) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	side := s.side()
	if s.used > 0 && s.x+w+2 > side { // next shelf
		s.x, s.y, s.rowH = 0, s.y+s.rowH+1, 0
	}
	if s.used == 0 || s.y+h+2 > side { // next page
		if s.used == sheetPages {
			return sheetSlot{}, false
		}
		if s.used == len(s.pages) {
			s.pages = append(s.pages, ebiten.NewImage(side, side))
		}
		s.used++
		s.x, s.y, s.rowH = 0, 0, 0
	}
	sl := sheetSlot{page: s.used - 1, x: float32(s.x + 1), y: float32(s.y + 1)}
	s.op.GeoM.Reset()
	s.op.GeoM.Translate(float64(s.x+1-b.Min.X), float64(s.y+1-b.Min.Y))
	s.op.Blend = ebiten.BlendCopy
	s.pages[sl.page].DrawImage(src, &s.op)
	s.x += w + 1
	s.rowH = max(s.rowH, h)
	if s.at == nil {
		s.at = map[*ebiten.Image]sheetSlot{}
	}
	s.at[img] = sl
	return sl, true
}

// litBatch is the pending lit quads: their destination, sheet page, shader,
// vertices and indices (reused frame to frame), and the thrash guard.
type litBatch struct {
	on    bool
	dst   *ebiten.Image
	page  *ebiten.Image
	sh    *ebiten.Shader
	verts []ebiten.Vertex
	idx   []uint32
	sheet litSheet
	// resets counts this frame's sheet resets; thrash is set when the
	// frame needed a second one, and keeps the batch off until StartFrame.
	resets int
	thrash bool
}

// Batch turns the lit batch on or off and returns whether it was on.
// Either way the pending quads go out first, so a caller that draws past
// the light layer (water through render/env) turns it off around that
// draw and back on after. The pipeline turns it on for the live ground
// pass and for the object pass.
func (l *Lit) Batch(on bool) bool {
	l.flush(FlushEnd)
	was := l.batch.on
	l.batch.on = on
	return was
}

// Flush draws the pending lit quads now: a raw draw (DrawImage, vector,
// gfx shapes, text, a triangle batch of its own) calls it first while the
// batch may be on, so it lands over the lit sprites queued before it.
func (l *Lit) Flush() {
	if len(l.batch.idx) > 0 {
		l.flush(FlushRaw)
	}
}

// slotOf is img's place on the sheet of catalog c, copying src (the image
// a lit draw of img samples) there first; ok is false when img does not go
// on a sheet. A sheet that must start again flushes the pending quads
// before its pages are cleared; a second start in one frame turns the
// batch off instead (thrash).
func (l *Lit) slotOf(c *art.Catalog, img, src *ebiten.Image) (sheetSlot, bool) {
	s := &l.batch.sheet
	if s.art == c {
		if sl, ok := s.at[img]; ok {
			return sl, true
		}
	}
	b := src.Bounds()
	if !s.fits(b.Dx(), b.Dy()) {
		return sheetSlot{}, false
	}
	if s.art != c { // a new catalog (or the first use)
		if s.art != nil && !l.restart() {
			return sheetSlot{}, false
		}
		l.flush(FlushPage)
		s.reset(c)
	}
	sl, ok := s.add(img, src)
	if !ok { // full: start again
		if !l.restart() {
			return sheetSlot{}, false
		}
		l.flush(FlushPage)
		s.reset(c)
		sl, ok = s.add(img, src)
	}
	if ok {
		l.stats.Copies++
	}
	return sl, ok
}

// restart counts a sheet reset, or reports false (and turns the batch off
// for the rest of the frame) when this frame already had one.
func (l *Lit) restart() bool {
	b := &l.batch
	if b.resets > 0 {
		l.flush(FlushPage)
		b.thrash = true
		return false
	}
	b.resets++
	l.stats.Resets++
	return true
}

// joins reports whether a shaded lit draw with blood may join the batch.
func (l *Lit) joins(blood float32) bool {
	return blood == 0 && l.batch.on && !l.batch.thrash
}

// appendQuad queues l.Verts (already placed on page) for dst with shader
// sh, flushing first when the destination, the page, the shader or the
// frame's light changed or the batch is full.
func (l *Lit) appendQuad(dst *ebiten.Image, page int, sh *ebiten.Shader) {
	b := &l.batch
	pg := b.sheet.pages[page]
	if len(b.idx) > 0 {
		switch {
		case b.dst != dst:
			l.flush(FlushDst)
		case b.page != pg:
			l.flush(FlushPage)
		case b.sh != sh:
			l.flush(FlushShader)
		case l.uniformsStale():
			l.flush(FlushUniforms)
		case len(b.verts)+4 > 4*batchQuads:
			l.flush(FlushFull)
		}
	}
	l.SyncUniforms()
	l.stats.Batched++
	b.dst, b.page, b.sh = dst, pg, sh
	n := uint32(len(b.verts))
	b.verts = append(b.verts, l.Verts[:]...)
	for _, i := range Indices {
		b.idx = append(b.idx, n+i)
	}
}

// flush draws the pending quads in one call, counted under why.
func (l *Lit) flush(why FlushReason) {
	b := &l.batch
	if len(b.idx) == 0 {
		return
	}
	l.count()
	l.stats.Flushes[why]++
	l.Opts.Images[0] = b.page
	b.dst.DrawTrianglesShader32(b.verts, b.idx, b.sh, &l.Opts)
	b.verts, b.idx = b.verts[:0], b.idx[:0]
	b.dst, b.page, b.sh = nil, nil, nil
}
