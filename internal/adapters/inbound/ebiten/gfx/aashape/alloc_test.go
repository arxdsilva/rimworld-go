// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package aashape

import (
	"image/color"
	"math"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// painterSamples is how many single-call samples each side takes. A call
// meets no pool refill with odds 9/16, so every sample of one side carrying
// one has odds (7/16)^40, under 1e-14.
const painterSamples = 40

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

// The painter adds nothing over Ebitengine's own DrawTriangles32. Outside
// a game loop that call allocates 3 (atlas.Image.DrawTriangles copies the
// vertices and indices into a queued closure) plus, under -race, a refill
// now and then: each draw takes a scratch slice from a sync.Pool twice
// (Image.invokeUsageCallbacks, for the target and the source), the race
// build drops 1 Put in 4 (sync.Pool.Put), and the next Get's New allocates
// 2. A 20-call average rounds to 3 or 4 by chance, so the fewest of five
// could be 4 on the painter's side and 3 on the other. One call at a time
// the count is exact (3, 5, 7: refills only add), so the fewest of many
// interleaved samples is each side's own cost; an allocation of the
// painter's is in every one.
func TestPainterAllocatesOnlyEbitengine(t *testing.T) { // not parallel: AllocsPerRun
	dst := ebiten.NewImage(64, 64)
	var clr color.Color = color.RGBA{200, 100, 50, 255}
	var p Painter
	p.Disc(dst, 30, 30, 12, clr) // warm: the white source, the mesh
	verts, idx := append([]ebiten.Vertex(nil), p.mesh.Verts...), append([]uint32(nil), p.mesh.Idx...)
	ebitenOnly, got := math.Inf(1), math.Inf(1)
	for range painterSamples {
		ebitenOnly = min(ebitenOnly, testing.AllocsPerRun(1, func() { dst.DrawTriangles32(verts, idx, p.white, &p.op) }))
		got = min(got, testing.AllocsPerRun(1, func() { p.Disc(dst, 30, 30, 12, clr) }))
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
