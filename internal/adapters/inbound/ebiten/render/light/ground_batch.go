package light

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
)

// The ground batch (GAR-159). A lit ground tile used to be one
// DrawTrianglesShader32 call: Ebitengine merged the commands, but each call
// still paid its CPU path (uniform packing, then the ui, mipmap, buffered
// and atlas layers). While the pipeline's ground pass has the batch on
// (Lit.BatchGround), each lit ground draw appends its quad to one vertex
// buffer instead, sampling a copy of its image on a ground sheet, and the
// buffer goes out as one call when the destination or the sheet page
// changes, before any draw that does not join, and when the pass ends.
//
// The quads keep their order, their vertices (only the source rectangle
// moves to the sheet) and the frame's uniforms, so the pixels are those of
// per-tile calls from the sheet, byte for byte. A sheet copy is a texel
// copy (BlendCopy, nearest) with a transparent gutter of one pixel all
// round, which is what an Ebitengine atlas page leaves around an image.
// Against per-tile calls from the images' own atlas places, a few pixels
// (at most 26 of 921,600 in the live capture scenes) differ by 1/255: the
// shader's source position is interpolated at another magnitude.

// groundSheetSize is the side of a ground sheet page in pixels: a 128 px
// tile and its gutter fit 15 to a row.
const groundSheetSize = 2048

// groundSheetPages bounds the pages; a full sheet starts again empty.
const groundSheetPages = 4

// groundBatchQuads caps one call's quads; a longer pass flushes and goes on.
const groundBatchQuads = 8192

// groundSlot is an image's place on the sheet: its page and the top-left
// of its copy.
type groundSlot struct {
	page int
	x, y float32
}

// groundSheet holds copies of the ground images drawn so far, packed in
// rows (shelves) with a one-pixel transparent gutter.
type groundSheet struct {
	art   *art.Catalog // the catalog the copies came from
	size  int          // page side (0: groundSheetSize)
	pages []*ebiten.Image
	used  int // pages in use
	at    map[*ebiten.Image]groundSlot
	// x, y and rowH are the free point on page used-1 and the current
	// shelf's height.
	x, y, rowH int
	op         ebiten.DrawImageOptions
}

// side is the page side.
func (s *groundSheet) side() int {
	if s.size > 0 {
		return s.size
	}
	return groundSheetSize
}

// reset empties the sheet (keeping its pages) for catalog c.
func (s *groundSheet) reset(c *art.Catalog) {
	s.art = c
	clear(s.at)
	for _, p := range s.pages[:s.used] {
		p.Clear()
	}
	s.used, s.x, s.y, s.rowH = 0, 0, 0, 0
}

// fits reports whether an image of w x h can ever go on a page.
func (s *groundSheet) fits(w, h int) bool {
	return w > 0 && h > 0 && w+2 <= s.side() && h+2 <= s.side()
}

// add copies img onto the sheet; ok is false when the sheet is full.
func (s *groundSheet) add(img *ebiten.Image) (groundSlot, bool) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	side := s.side()
	if s.used > 0 && s.x+w+2 > side { // next shelf
		s.x, s.y, s.rowH = 0, s.y+s.rowH+1, 0
	}
	if s.used == 0 || s.y+h+2 > side { // next page
		if s.used == groundSheetPages {
			return groundSlot{}, false
		}
		if s.used == len(s.pages) {
			s.pages = append(s.pages, ebiten.NewImage(side, side))
		}
		s.used++
		s.x, s.y, s.rowH = 0, 0, 0
	}
	sl := groundSlot{page: s.used - 1, x: float32(s.x + 1), y: float32(s.y + 1)}
	s.op.GeoM.Reset()
	s.op.GeoM.Translate(float64(s.x+1-b.Min.X), float64(s.y+1-b.Min.Y))
	s.op.Blend = ebiten.BlendCopy
	s.pages[sl.page].DrawImage(img, &s.op)
	s.x += w + 1
	s.rowH = max(s.rowH, h)
	if s.at == nil {
		s.at = map[*ebiten.Image]groundSlot{}
	}
	s.at[img] = sl
	return sl, true
}

// groundBatch is the pending ground quads: their destination, sheet page,
// shader, vertices and indices (reused frame to frame).
type groundBatch struct {
	on    bool
	dst   *ebiten.Image
	page  *ebiten.Image
	sh    *ebiten.Shader
	verts []ebiten.Vertex
	idx   []uint32
	sheet groundSheet
}

// BatchGround turns the ground batch on or off and returns whether it was
// on. Either way the pending quads go out first, so a caller that draws
// past the light layer (water through render/env) turns it off around that
// draw and back on after. Only the pipeline's live ground pass turns it on.
func (l *Lit) BatchGround(on bool) bool {
	l.flushGround(FlushEnd)
	was := l.batch.on
	l.batch.on = on
	return was
}

// groundSlotOf is img's place on the ground sheet of v's catalog, copying
// it there first; ok is false when img does not go on a sheet. A sheet that
// must start again flushes the pending quads before its pages are cleared.
func (l *Lit) groundSlotOf(c *art.Catalog, img *ebiten.Image) (groundSlot, bool) {
	s := &l.batch.sheet
	if s.art == c {
		if sl, ok := s.at[img]; ok {
			return sl, true
		}
	}
	b := img.Bounds()
	if !s.fits(b.Dx(), b.Dy()) {
		return groundSlot{}, false
	}
	if s.art != c { // a new catalog (or the first use)
		l.flushGround(FlushPage)
		if s.art != nil {
			l.stats.Resets++
		}
		s.reset(c)
	}
	sl, ok := s.add(img)
	if !ok { // full: start again
		l.flushGround(FlushPage)
		s.reset(c)
		l.stats.Resets++
		sl, ok = s.add(img)
	}
	if ok {
		l.stats.Copies++
	}
	return sl, ok
}

// appendGround queues l.Verts (already placed on page) for dst with shader
// sh, flushing first when the destination, the page, the shader or the
// frame's light changed or the batch is full.
func (l *Lit) appendGround(dst *ebiten.Image, page int, sh *ebiten.Shader) {
	b := &l.batch
	pg := b.sheet.pages[page]
	if len(b.idx) > 0 {
		switch {
		case b.dst != dst:
			l.flushGround(FlushDst)
		case b.page != pg:
			l.flushGround(FlushPage)
		case b.sh != sh:
			l.flushGround(FlushShader)
		case l.uniformsStale():
			l.flushGround(FlushUniforms)
		case len(b.verts)+4 > 4*groundBatchQuads:
			l.flushGround(FlushFull)
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

// flushGround draws the pending ground quads in one call, counted under
// reason.
func (l *Lit) flushGround(why FlushReason) {
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
