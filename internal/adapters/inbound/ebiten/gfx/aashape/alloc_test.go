package aashape

import (
	"image/color"
	"math"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// mixed adds n shapes of every kind, sizes and angles varying.
func mixed(m *Mesh, n int) {
	tri := [][2]float32{{0, -6}, {6, 4}, {-6, 4}}
	c := Color{0.8, 0.6, 0.2, 0.9}
	for i := range n {
		x, y := float32(20+i%20*30), float32(20+i/20*30)
		f := float32(i % 7)
		switch i % 6 {
		case 0:
			m.Disc(x, y, 1+f*2, c)
		case 1:
			m.Ring(x, y, 4+f*2, 1.5, c)
		case 2:
			a := float64(i) * 0.37
			m.Line(x, y, x+20*float32(math.Cos(a)), y+20*float32(math.Sin(a)), 0.5+f/2, c)
		case 3:
			m.Rect(x, y, 10+f, 6, c)
		case 4:
			m.Frame(x, y, 12, 8+f, 1, c)
		case 5:
			m.ConvexPoly(tri, 1+f/4, c)
		}
	}
}

// A warm shape call allocates nothing: the mesh keeps its storage.
func TestMeshAllocatesNothing(t *testing.T) { // not parallel: AllocsPerRun
	var m Mesh
	mixed(&m, 60)
	if got := testing.AllocsPerRun(50, func() { m.Reset(); mixed(&m, 60) }); got != 0 {
		t.Fatalf("a warm mesh allocates %v per 60 shapes", got)
	}
}

// The painter adds nothing over Ebitengine's own DrawTriangles32 (which
// queues the draw outside a game loop). That queue is never drained here,
// so its buffers grow now and then and one sample of either side can carry
// a growth; the fewest of several interleaved samples is each side's cost
// per call. An allocation of the painter's own is in every sample.
func TestPainterAllocatesOnlyEbitengine(t *testing.T) { // not parallel: AllocsPerRun
	dst := ebiten.NewImage(64, 64)
	var clr color.Color = color.RGBA{200, 100, 50, 255}
	var p Painter
	p.Disc(dst, 30, 30, 12, clr) // warm: the white source, the mesh
	verts, idx := append([]ebiten.Vertex(nil), p.mesh.Verts...), append([]uint32(nil), p.mesh.Idx...)
	ebitenOnly, got := math.Inf(1), math.Inf(1)
	for range 5 {
		ebitenOnly = min(ebitenOnly, testing.AllocsPerRun(20, func() { dst.DrawTriangles32(verts, idx, p.white, &p.op) }))
		got = min(got, testing.AllocsPerRun(20, func() { p.Disc(dst, 30, 30, 12, clr) }))
	}
	if got > ebitenOnly {
		t.Fatalf("Painter.Disc allocates %v, DrawTriangles32 alone %v", got, ebitenOnly)
	}
}

// BenchmarkAAShapes tessellates 200 mixed shapes into a reused mesh: the
// CPU work that replaces vector's stencil path per frame.
func BenchmarkAAShapes(b *testing.B) {
	var m Mesh
	mixed(&m, 200)
	b.ReportAllocs()
	for b.Loop() {
		m.Reset()
		mixed(&m, 200)
	}
	b.ReportMetric(float64(len(m.Verts)), "verts")
}
