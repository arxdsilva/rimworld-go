package light

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
)

// groundRig is a rig whose sheet has 32 px pages, so a 24 px image needs a
// page of its own.
func groundRig() *rig {
	r := &rig{view: render.View{Art: &art.Catalog{}, W: roomWorld()}}
	r.view.Cam.Zoom = 1
	r.lit.Light = LightAt(12, summerDay, "clear", 0)
	r.lit.batch.sheet.size = 32
	return r
}

func (r *rig) drawGround(dst, img *ebiten.Image, x float64) {
	var geo ebiten.GeoM
	geo.Translate(x, 0)
	r.lit.DrawGround(&r.view, dst, img, geo, noCS, x/8, 0, false, 0)
}

// drawAt is a lit object draw (not a ground plane) at x.
func (r *rig) drawAt(dst, img *ebiten.Image, x float64, blood float32) {
	var geo ebiten.GeoM
	geo.Translate(x, 0)
	r.lit.Draw(&r.view, dst, img, geo, noCS, x/8, 0, false, blood)
}

// pending is the quads the batch holds.
func (r *rig) pending() int { return len(r.lit.batch.idx) / len(Indices) }

// TestGroundBatchFlushesOnChange (GAR-159, GAR-161): lit draws queue while
// the batch is on and go out as one call when the destination or the sheet
// page changes, before a draw that does not join (a bloodied figure), and
// when it is turned off; each quad samples its image's copy on the sheet.
func TestGroundBatchFlushesOnChange(t *testing.T) {
	r := groundRig()
	a, b, big, chair := ebiten.NewImage(8, 8), ebiten.NewImage(8, 8), ebiten.NewImage(24, 24), ebiten.NewImage(4, 4)
	dst1, dst2 := ebiten.NewImage(64, 64), ebiten.NewImage(64, 64)
	if r.lit.Batch(true) {
		t.Fatal("the batch starts off")
	}
	r.drawGround(dst1, a, 0)
	r.drawGround(dst1, b, 8)
	r.drawGround(dst1, a, 16)
	if r.lit.Calls != 0 || r.pending() != 3 {
		t.Fatalf("three ground draws: %d calls, %d pending; want 0 and 3", r.lit.Calls, r.pending())
	}
	vs := r.lit.batch.verts
	if vs[0].SrcX != 1 || vs[0].SrcY != 1 || vs[4].SrcX != 10 || vs[8].SrcX != 1 || vs[11].SrcX != 9 || vs[11].SrcY != 9 {
		t.Fatalf("sheet sources: a at %v,%v, b at %v, a again at %v (corner %v,%v)", vs[0].SrcX, vs[0].SrcY, vs[4].SrcX, vs[8].SrcX, vs[11].SrcX, vs[11].SrcY)
	}
	if vs[4].DstX != 8 || vs[8].DstX != 16 {
		t.Fatalf("destinations moved: %v, %v", vs[4].DstX, vs[8].DstX)
	}

	r.drawGround(dst2, a, 0) // another destination
	if r.lit.Calls != 1 || r.pending() != 1 {
		t.Fatalf("destination change: %d calls, %d pending; want 1 and 1", r.lit.Calls, r.pending())
	}
	r.drawGround(dst2, big, 0) // a second page
	if r.lit.Calls != 2 || r.pending() != 1 || r.lit.Opts.Images[0] != r.lit.batch.sheet.pages[0] {
		t.Fatalf("page change: %d calls, %d pending; want 2 and 1 (the first page drawn)", r.lit.Calls, r.pending())
	}
	r.drawAt(dst2, chair, 0, 0.5) // a bloodied figure: the batch goes first
	if r.lit.Calls != 4 || r.pending() != 0 || r.lit.Opts.Images[0] != chair {
		t.Fatalf("a bloodied draw: %d calls, %d pending; want the batch, then the figure (4, 0)", r.lit.Calls, r.pending())
	}
	r.drawGround(dst2, a, 0)
	if !r.lit.Batch(false) || r.lit.Calls != 5 || r.pending() != 0 {
		t.Fatalf("turning it off: %d calls, %d pending; want 5 and 0", r.lit.Calls, r.pending())
	}
	r.drawGround(dst2, b, 0) // off: drawn at once from the image itself
	if r.lit.Calls != 6 || r.pending() != 0 || r.lit.Opts.Images[0] != b || r.lit.Verts[0].SrcX != 0 {
		t.Fatalf("batch off: %d calls, %d pending", r.lit.Calls, r.pending())
	}
	if len(r.lit.batch.sheet.at) != 3 || r.lit.batch.sheet.used != 2 {
		t.Fatalf("sheet holds %d images on %d pages, want 3 on 2", len(r.lit.batch.sheet.at), r.lit.batch.sheet.used)
	}
}

// TestGroundBatchBakeOffAndCatalog: a bake, lighting off and a failed
// shader draw as before and flush the batch first; a new catalog starts
// the sheet again.
func TestGroundBatchBakeOffAndCatalog(t *testing.T) {
	r := groundRig()
	a := ebiten.NewImage(8, 8)
	dst := ebiten.NewImage(64, 64)
	r.lit.Batch(true)
	r.drawGround(dst, a, 0)
	r.lit.DrawGround(&r.view, dst, a, ebiten.GeoM{}, noCS, 0, 0, true, 0) // a bake
	if r.lit.Calls != 1 || r.pending() != 0 {
		t.Fatalf("bake: %d calls, %d pending; want the batch flushed, the bake uncounted", r.lit.Calls, r.pending())
	}
	r.drawGround(dst, a, 0)
	r.lit.Off = true
	r.drawGround(dst, a, 0)
	if r.lit.Calls != 3 || r.pending() != 0 {
		t.Fatalf("lighting off: %d calls, %d pending; want 3 and 0", r.lit.Calls, r.pending())
	}
	r.lit.Off = false
	r.drawGround(dst, a, 0)
	sh, err := r.lit.Shader, r.lit.Err
	r.lit.Shader, r.lit.Err = nil, errTestShader{}
	r.drawGround(dst, a, 0)
	if r.lit.Calls != 5 || r.pending() != 0 {
		t.Fatalf("failed shader: %d calls, %d pending; want 5 and 0", r.lit.Calls, r.pending())
	}
	r.lit.Shader, r.lit.Err = sh, err
	r.drawGround(dst, a, 0)
	r.view.Art = &art.Catalog{} // a new catalog: the sheet starts again
	r.drawGround(dst, a, 0)
	if r.lit.Calls != 6 || r.pending() != 1 || len(r.lit.batch.sheet.at) != 1 || r.lit.batch.sheet.art != r.view.Art {
		t.Fatalf("new catalog: %d calls, %d pending, %d on the sheet", r.lit.Calls, r.pending(), len(r.lit.batch.sheet.at))
	}
	r.lit.Batch(false)
}

type errTestShader struct{}

func (errTestShader) Error() string { return "test: no shader" }

// TestGroundBatchReusesItsBuffer: a warm ground pass allocates nothing
// beyond Ebitengine's own draw call.
func TestGroundBatchReusesItsBuffer(t *testing.T) {
	r := groundRig()
	a, b := ebiten.NewImage(8, 8), ebiten.NewImage(8, 8)
	dst := ebiten.NewImage(64, 64)
	pass := func() {
		r.lit.Batch(true)
		for i := range 64 {
			img := a
			if i%3 == 0 {
				img = b
			}
			r.drawGround(dst, img, float64(i%8)*8)
		}
		r.lit.Batch(false)
	}
	pass()
	verts, idx := r.lit.batch.verts[:64*4], r.lit.batch.idx[:64*6]
	sh := r.litShader()
	r.lit.Opts.Images[0] = r.lit.batch.sheet.pages[0]
	ebitenOnly := testing.AllocsPerRun(200, func() {
		dst.DrawTrianglesShader32(verts, idx, sh, &r.lit.Opts)
	})
	allocs := testing.AllocsPerRun(200, pass)
	// +1: Ebiten's own sync.Pool (invokeUsageCallbacks) jitters under GC and -race
	if allocs > ebitenOnly+1 {
		t.Fatalf("a ground pass allocates %v, one Ebiten call %v", allocs, ebitenOnly)
	}
	if &r.lit.batch.verts[:1][0] != &verts[0] {
		t.Fatal("the vertex buffer was not reused")
	}
}

// TestLitBatchKeepsOrder (GAR-161): ground planes and objects join one
// batch in the order they are drawn, and a raw draw's Flush sends them out
// before it.
func TestLitBatchKeepsOrder(t *testing.T) {
	r := groundRig()
	soil, chair, lamp := ebiten.NewImage(8, 8), ebiten.NewImage(4, 4), ebiten.NewImage(6, 6)
	dst := ebiten.NewImage(64, 64)
	r.lit.StartFrame()
	r.lit.Batch(true)
	r.drawGround(dst, soil, 0)
	r.drawAt(dst, chair, 8, 0)
	r.drawAt(dst, lamp, 16, 0)
	r.drawAt(dst, chair, 24, 0)
	if r.pending() != 4 || r.lit.Calls != 0 {
		t.Fatalf("%d pending, %d calls; want 4 and 0", r.pending(), r.lit.Calls)
	}
	for i, want := range []float32{0, 8, 16, 24} {
		if got := r.lit.batch.verts[4*i].DstX; got != want {
			t.Fatalf("quad %d at %v, want %v (the draw order)", i, got, want)
		}
	}
	if r.lit.batch.verts[4].SrcX != r.lit.batch.verts[12].SrcX {
		t.Fatal("the chair's two draws sample one copy")
	}
	r.lit.Flush()
	if r.pending() != 0 || r.lit.Calls != 1 || r.lit.Stats().Flushes[FlushRaw] != 1 {
		t.Fatalf("Flush: %d pending, %d calls, stats %v", r.pending(), r.lit.Calls, r.lit.Stats())
	}
	r.lit.Flush() // nothing pending: no call
	if r.lit.Calls != 1 || r.lit.Stats().Flushes[FlushRaw] != 1 {
		t.Fatal("an empty Flush draws nothing")
	}
	r.lit.Batch(false)
}

// TestLitBatchSkipsBloodBakeOff: a bloodied figure (its noise reads
// absolute source positions), a bake and a plain draw (lighting off) never
// join; each sends the batch out first under its reason.
func TestLitBatchSkipsBloodBakeOff(t *testing.T) {
	r := groundRig()
	a := ebiten.NewImage(8, 8)
	dst := ebiten.NewImage(64, 64)
	r.lit.StartFrame()
	r.lit.Batch(true)
	r.drawAt(dst, a, 0, 0)
	r.drawAt(dst, a, 8, 0.4)
	if r.pending() != 0 || r.lit.Opts.Images[0] != a || r.lit.Verts[0].Custom3 != 0.4 || r.lit.Verts[0].SrcX != 0 {
		t.Fatalf("blood: %d pending, drawn from its own image %v", r.pending(), r.lit.Opts.Images[0] == a)
	}
	r.drawAt(dst, a, 0, 0)
	r.lit.Draw(&r.view, dst, a, ebiten.GeoM{}, noCS, 0, 0, true, 0)
	r.drawAt(dst, a, 0, 0)
	r.lit.Off = true
	r.drawAt(dst, a, 0, 0)
	r.lit.Off = false
	st := r.lit.Stats()
	if r.pending() != 0 || st.Flushes[FlushBlood] != 1 || st.Flushes[FlushBake] != 1 || st.Flushes[FlushOff] != 1 || st.Batched != 3 {
		t.Fatalf("%d pending, stats %v; want blood, bake and off flushes, 3 batched", r.pending(), st)
	}
	r.lit.Batch(false)
}

// TestLitBatchPairOffsetRelative: a sprite with a normal map joins through
// its pair's copy; the normal map's offset stays relative (the pair's
// width), negated when the draw mirrors it.
func TestLitBatchPairOffsetRelative(t *testing.T) {
	r := groundRig()
	r.lit.batch.sheet.size = 0 // full-size pages
	spr, n := ebiten.NewImage(6, 4), ebiten.NewImage(6, 4)
	r.view.Art = &art.Catalog{Normals: map[*ebiten.Image]*ebiten.Image{spr: n}}
	dst := ebiten.NewImage(64, 64)
	r.lit.StartFrame()
	r.lit.Batch(true)
	r.drawAt(dst, spr, 0, 0)
	var mirror ebiten.GeoM
	mirror.Scale(-1, 1)
	mirror.Translate(30, 0)
	r.lit.Draw(&r.view, dst, spr, mirror, noCS, 1, 0, false, 0)
	sl, ok := r.lit.batch.sheet.at[spr]
	vs := r.lit.batch.verts
	if !ok || r.pending() != 2 || vs[0].SrcX != sl.x || vs[0].SrcY != sl.y || vs[3].SrcX != sl.x+6 {
		t.Fatalf("pair source: slot %v (%v), quad from %v,%v to %v", sl, ok, vs[0].SrcX, vs[0].SrcY, vs[3].SrcX)
	}
	if vs[0].Custom2 != 7 || vs[4].Custom2 != -7 {
		t.Fatalf("normal offsets %v and %v, want 7 and -7 (6 + the gap)", vs[0].Custom2, vs[4].Custom2)
	}
	r.lit.Batch(false)
}

// TestLitBatchKeysByImage: the sheet is keyed by the sprite, not its pair:
// a pair the cache evicted and rebuilt (a new image, the old one
// deallocated mid-frame) is served by the copy already on the sheet.
func TestLitBatchKeysByImage(t *testing.T) {
	r := groundRig()
	r.lit.batch.sheet.size = 0
	s1, s2, n := ebiten.NewImage(6, 4), ebiten.NewImage(6, 4), ebiten.NewImage(6, 4)
	r.view.Art = &art.Catalog{Normals: map[*ebiten.Image]*ebiten.Image{s1: n, s2: n}}
	r.lit.Pairs.Limit = 1
	dst := ebiten.NewImage(64, 64)
	r.lit.StartFrame()
	r.lit.Batch(true)
	r.drawAt(dst, s1, 0, 0)
	p1, _ := r.lit.Pairs.Get(s1, r.view.Present)
	r.drawAt(dst, s2, 8, 0) // evicts s1's pair
	r.drawAt(dst, s1, 16, 0)
	p2, _ := r.lit.Pairs.Get(s1, r.view.Present)
	if r.lit.Pairs.Evicted < 2 || p1 == p2 {
		t.Fatalf("the pair was not rebuilt (evicted %d)", r.lit.Pairs.Evicted)
	}
	vs := r.lit.batch.verts
	if st := r.lit.Stats(); st.Copies != 2 || vs[0].SrcX != vs[8].SrcX || r.pending() != 3 {
		t.Fatalf("copies %d, s1 sampled at %v then %v; want 2 copies, one place", st.Copies, vs[0].SrcX, vs[8].SrcX)
	}
	r.lit.Batch(false)
}

// TestLitBatchSheetResetOncePerFrame: a full sheet starts again once in a
// frame; a frame that would need a second start draws unbatched to its end
// (the thrash guard), and the next frame batches again.
func TestLitBatchSheetResetOncePerFrame(t *testing.T) {
	r := groundRig() // 32 px pages: one 24 px image a page
	imgs := make([]*ebiten.Image, 2*sheetPages+1)
	for i := range imgs {
		imgs[i] = ebiten.NewImage(24, 24)
	}
	dst := ebiten.NewImage(64, 64)
	r.lit.StartFrame()
	r.lit.Batch(true)
	for _, img := range imgs[:sheetPages+1] { // the last one starts the sheet again
		r.drawAt(dst, img, 0, 0)
	}
	if st := r.lit.Stats(); st.Resets != 1 || st.Copies != sheetPages+1 || r.lit.batch.thrash {
		t.Fatalf("one start: stats %v, thrash %v", st, r.lit.batch.thrash)
	}
	for _, img := range imgs[sheetPages+1:] { // fills it again: a second start would thrash
		r.drawAt(dst, img, 0, 0)
	}
	st := r.lit.Stats()
	if st.Resets != 1 || !r.lit.batch.thrash || r.pending() != 0 || r.lit.Opts.Images[0] != imgs[len(imgs)-1] {
		t.Fatalf("thrash: stats %v, thrash %v, %d pending", st, r.lit.batch.thrash, r.pending())
	}
	r.lit.Batch(false)
	r.lit.StartFrame() // the next frame batches again
	r.lit.Batch(true)
	r.drawAt(dst, imgs[0], 0, 0)
	if r.lit.batch.thrash || r.pending() != 1 {
		t.Fatalf("next frame: thrash %v, %d pending", r.lit.batch.thrash, r.pending())
	}
	r.lit.Batch(false)
}
