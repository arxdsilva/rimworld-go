package light

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
)

// groundRig is a rig whose ground sheet has 32 px pages, so a 24 px image
// needs a page of its own.
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

// pending is the quads the ground batch holds.
func (r *rig) pending() int { return len(r.lit.batch.idx) / len(Indices) }

// TestGroundBatchFlushesOnChange (GAR-159): lit ground draws queue while the
// batch is on and go out as one call when the destination or the sheet
// page changes, before a draw that does not join, and when it is turned
// off; each quad samples its image's copy on the sheet.
func TestGroundBatchFlushesOnChange(t *testing.T) {
	r := groundRig()
	a, b, big, chair := ebiten.NewImage(8, 8), ebiten.NewImage(8, 8), ebiten.NewImage(24, 24), ebiten.NewImage(4, 4)
	dst1, dst2 := ebiten.NewImage(64, 64), ebiten.NewImage(64, 64)
	if r.lit.BatchGround(true) {
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
	r.lit.Draw(&r.view, dst2, chair, ebiten.GeoM{}, noCS, 0, 0, false, 0) // not ground: the batch goes first
	if r.lit.Calls != 4 || r.pending() != 0 || r.lit.Opts.Images[0] != chair {
		t.Fatalf("a non-ground draw: %d calls, %d pending; want the batch, then the chair (4, 0)", r.lit.Calls, r.pending())
	}
	r.drawGround(dst2, a, 0)
	if !r.lit.BatchGround(false) || r.lit.Calls != 5 || r.pending() != 0 {
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
	r.lit.BatchGround(true)
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
	r.lit.BatchGround(false)
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
		r.lit.BatchGround(true)
		for i := range 64 {
			img := a
			if i%3 == 0 {
				img = b
			}
			r.drawGround(dst, img, float64(i%8)*8)
		}
		r.lit.BatchGround(false)
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
