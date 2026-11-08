package ebiten

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
)

// GAR E9: the boot screen compiles and warms the world shaders, and a
// renderer's first lit or water draw uses those, so the first colony
// frame compiles nothing.
func TestShadersCompiledBeforeFirstFrame(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 8, h: 8})
	screen := ebiten.NewImage(64, 64)
	defer screen.Deallocate()
	a.Draw(screen) // the boot screen
	lit, err := light.SharedLit()
	water, werr := env.SharedWater()
	if err != nil || werr != nil || lit == nil || water == nil {
		t.Fatalf("world shaders: lit %v, water %v", err, werr)
	}
	r := bareRenderer()
	if r.lit.LitShader() != lit {
		t.Fatal("the renderer compiled its own lit shader")
	}
	if r.pipe().Env.Water.WaterShader() != water {
		t.Fatal("the renderer compiled its own water shader")
	}
}
