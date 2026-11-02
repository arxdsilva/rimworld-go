package light

import (
	"math"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
)

func near(a, b, eps float64) bool { return math.Abs(a-b) <= eps }

// TestColonistShadowPass runs the pass on hand-built feet: every blob
// goes out in one batched draw, the batch and texture are reused across
// frames, and nothing is drawn with shadows off, no feet or no light.
// Not parallel: it counts allocations.
func TestColonistShadowPass(t *testing.T) {
	v := &render.View{W: game.WorldView{TerrainW: 20, DayOfYear: 15}, Cam: camera.Camera{Zoom: 1, Placed: true}}
	l := LightAt(12, 15, "clear", 0)
	feet := []Foot{{X: 2.5, Y: 2.5, Scale: 1}, {X: 6.5, Y: 4.5, Scale: 1}, {X: 9.5, Y: 3.5, Scale: 1}}
	var st Shadows
	screen := ebiten.NewImage(640, 400)
	st.Draw(v, l, screen, feet, 15)
	if st.Drawn != 3 || st.Flushes != 1 {
		t.Fatalf("drew %d blobs in %d draws, want 3 in 1", st.Drawn, st.Flushes)
	}
	tex := st.blobImg
	many := make([]Foot, 40)
	for i := range many {
		many[i] = Foot{X: float64(i%10) + 0.5, Y: float64(i/10) + 0.5, Scale: 1}
	}
	st.Draw(v, l, screen, many, 15) // grow the batch once
	few := testing.AllocsPerRun(20, func() { st.Draw(v, l, screen, feet, 15) })
	lots := testing.AllocsPerRun(20, func() { st.Draw(v, l, screen, many, 15) })
	if st.blobImg != tex || st.Drawn != 40 {
		t.Fatal("the blob texture was remade")
	}
	if lots > few+2 { // ebiten's per-draw bookkeeping varies by one or two; a per-colonist cost would add ~37
		t.Fatalf("%.0f allocations a frame for 40 colonists, %.0f for 3", lots, few)
	}
	flushes := st.Flushes
	st.Draw(v, l, screen, nil, 15)
	st.Draw(v, State{}, screen, feet, 15)
	st.SetOff(true)
	st.Draw(v, l, screen, feet, 15)
	if st.Flushes != flushes || st.Drawn != 0 {
		t.Fatalf("drew with no feet, no light or shadows off: %d draws, %d blobs", st.Flushes-flushes, st.Drawn)
	}
	st.SetOff(false)
	st.Draw(v, LightAt(23, 15, "clear", 0), screen, feet, 15) // a faint blob under the moon
	if st.Drawn != 3 || st.Flushes != flushes+1 {
		t.Fatalf("moonlight: %d blobs", st.Drawn)
	}
}

func TestSunElevationFromLight(t *testing.T) {
	t.Parallel()
	for _, el := range []float64{0.1, 0.5, 1.1} {
		l := State{SunL: F32v(LightDir(el, 0.7))}
		if got := SunElevation(l); !near(got, el, 1e-5) {
			t.Fatalf("elevation %v, want %v", got, el)
		}
	}
	if SunElevation(LightAt(2, 15, "clear", 0)) != 0 {
		t.Fatal("the sun is up at 02:00")
	}
}

func TestShadowTintBluerAtLowSun(t *testing.T) {
	t.Parallel()
	lo, hi := ShadowTint(0), ShadowTint(1)
	if !(lo[2] > hi[2] && lo[0] == hi[0]) {
		t.Fatalf("tint %v %v", lo, hi)
	}
}
