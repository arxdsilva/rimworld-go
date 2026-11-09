package light

import (
	"image"

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

// Copies happen only at the start of a frame (StartFrame), never between
// two draws of the frame. A draw to a sheet page between two draws to the
// screen ends the screen's render pass, and on a 5K screen each such break
// cost about a millisecond of GPU time: the frame after a mid-frame sheet
// reset copied about 170 images and took 30 ms (PERF-015). So a lit draw
// whose image is not on the sheet yet draws on its own, as before the
// batch, and asks for a copy; the next StartFrame makes the copies in the
// order the frame asked for them, which is draw order.

// sheetSize is the side of a sheet page in pixels. One frame's sprites at
// the 5K display's art scale cover about 12 to 15 million pixels with
// their pairs (PERF-015), so at 4096 they fit on one page and the batch
// seldom changes page; at 2048 they spread over four or five and the
// batch changed page every few quads.
const sheetSize = 4096

// sheetPages bounds the pages (64 MB each, unmanaged, allocated as
// needed): 128 MB at most, room for about two frames' distinct sprites, so
// animation frames and sprites coming into view join without the sheet
// starting again.
const sheetPages = 2

// resetCooldown is the fewest frames between two sheet resets not forced
// by a new catalog: a sheet that cannot hold a frame's sprites draws the
// rest on their own rather than starting again every frame.
const resetCooldown = 120

// scatterRun is the fewest quads a frame's batches should average between
// page changes. Below it (in a frame of at least scatterMin batched quads)
// the frame's sprites lie spread over the pages among stale copies, and the
// next StartFrame (after the cooldown) starts the sheet again, so the frame
// after copies them together, in draw order.
const (
	scatterRun = 8
	scatterMin = 256
)

// batchQuads caps one call's quads; a longer pass flushes and goes on.
const batchQuads = 8192

// sheetSlot is an image's place on the sheet: its page and the top-left
// of its copy.
type sheetSlot struct {
	page int
	x, y float32
}

// sheetCopy is an image a frame asked to copy and the image its lit draw
// samples (itself or its normal-map pair).
type sheetCopy struct{ img, src *ebiten.Image }

// litSheet holds copies of the images drawn so far, packed in rows
// (shelves) with a one-pixel transparent gutter, and the copies the frame
// asked for.
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
	// want is the copies asked for since the last StartFrame, in order,
	// all from catalog wantArt; wanted is each image's index in want.
	want    []sheetCopy
	wanted  map[*ebiten.Image]int
	wantArt *art.Catalog
	// frame counts StartFrames; a reset not forced by a new catalog waits
	// for frame resetAt (resetCooldown after the last one).
	frame, resetAt int
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
	s.resetAt = s.frame + resetCooldown
}

// fits reports whether an image of w x h can ever go on a page.
func (s *litSheet) fits(w, h int) bool {
	return w > 0 && h > 0 && w+2 <= s.side() && h+2 <= s.side()
}

// add copies src onto the sheet as img's; ok is false when the sheet is
// full. Only StartFrame calls it (through copyAsked).
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
			// unmanaged: a page of its own, never moved within
			// Ebitengine's shared atlas when it is drawn to
			s.pages = append(s.pages, ebiten.NewImageWithOptions(image.Rect(0, 0, side, side), &ebiten.NewImageOptions{Unmanaged: true}))
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

// ask records that a lit draw of img, sampling src, found no copy on the
// sheet of catalog c; the next StartFrame copies it. A second ask for img
// keeps the newer src (the pair cache may have rebuilt its pair); asks
// from an earlier catalog in the same frame are dropped.
func (s *litSheet) ask(c *art.Catalog, img, src *ebiten.Image) {
	b := src.Bounds()
	if !s.fits(b.Dx(), b.Dy()) {
		return
	}
	if s.wantArt != c {
		s.dropAsks()
		s.wantArt = c
	}
	if i, ok := s.wanted[img]; ok {
		s.want[i].src = src
		return
	}
	if s.wanted == nil {
		s.wanted = map[*ebiten.Image]int{}
	}
	s.wanted[img] = len(s.want)
	s.want = append(s.want, sheetCopy{img, src})
}

// dropAsks forgets the asked copies (and their image references).
func (s *litSheet) dropAsks() {
	clear(s.want)
	s.want = s.want[:0]
	clear(s.wanted)
}

// litBatch is the pending lit quads: their destination, sheet page, shader,
// vertices and indices (reused frame to frame), and the sheet.
type litBatch struct {
	on    bool
	dst   *ebiten.Image
	page  *ebiten.Image
	sh    *ebiten.Shader
	verts []ebiten.Vertex
	idx   []uint32
	sheet litSheet
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

// slotOf is img's place on the sheet of catalog c; ok is false when img
// has no copy there yet, and then it asks for one (src is the image a lit
// draw of img samples), made at the next StartFrame.
func (l *Lit) slotOf(c *art.Catalog, img, src *ebiten.Image) (sheetSlot, bool) {
	s := &l.batch.sheet
	if s.art == c {
		if sl, ok := s.at[img]; ok {
			return sl, true
		}
	}
	s.ask(c, img, src)
	return sheetSlot{}, false
}

// copyAsked runs in StartFrame, before the frame draws: it starts the
// sheet again when it must, then makes the copies the last frame asked
// for, in order. A new catalog always starts it again. At most once per
// resetCooldown frames it also starts again when the asked copies do not
// fit, or when the last frame's batches (prev, its stats) averaged fewer
// than scatterRun quads per page change (scatterMin quads or more); it
// then drops the asks, so the next frame draws on its own and asks for
// all its images in draw order. A copy whose pair the cache has since
// dropped (and deallocated) is skipped: its image asks again when it is
// drawn again.
func (l *Lit) copyAsked(prev Stats) {
	s := &l.batch.sheet
	l.flush(FlushEnd) // the pages may be cleared: nothing may sample them
	s.frame++
	cooled := s.frame >= s.resetAt
	scattered := prev.Batched >= scatterMin && prev.Flushes[FlushPage]*scatterRun > prev.Batched
	switch {
	case len(s.want) > 0 && s.art != nil && s.art != s.wantArt:
		l.resetSheet(s.wantArt)
	case cooled && scattered:
		l.resetSheet(s.art)
		s.dropAsks()
		return
	}
	if len(s.want) == 0 {
		return
	}
	if s.art == nil {
		s.art = s.wantArt
	}
	for _, w := range s.want {
		if !l.livePair(w.img, w.src) {
			continue
		}
		if _, ok := s.add(w.img, w.src); ok {
			l.stats.Copies++
			continue
		}
		if s.frame >= s.resetAt { // full: start again, packed in the next frame's order
			l.resetSheet(s.art)
		}
		break
	}
	s.dropAsks()
}

// resetSheet starts the sheet again for catalog c, counted in Stats.
func (l *Lit) resetSheet(c *art.Catalog) {
	l.batch.sheet.reset(c)
	l.stats.Resets++
}

// livePair reports whether src is still what a lit draw of img samples:
// img itself, or the pair the cache holds for it.
func (l *Lit) livePair(img, src *ebiten.Image) bool {
	if src == img {
		return true
	}
	e, ok := l.Pairs.m[img]
	return ok && e.Value.(*pairEntry).pair == src
}

// joins reports whether a shaded lit draw with blood may join the batch.
func (l *Lit) joins(blood float32) bool {
	return blood == 0 && l.batch.on
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
