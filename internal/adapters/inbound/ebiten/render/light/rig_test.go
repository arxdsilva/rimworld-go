package light

import (
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render"
)

// rig is a Lit with the frame view it reads: the tests' stand-in for the
// renderer, its methods the renderer's old entry points.
type rig struct {
	lit  Lit
	view render.View
}

func (r *rig) lightTile(tx, ty int) (sky, lamp float32) { return r.lit.LightTile(&r.view, tx, ty) }

func (r *rig) lightFoot(wx, wy float64) (sky, lamp float32) { return r.lit.LightFoot(&r.view, wx, wy) }

func (r *rig) litShader() *ebiten.Shader { return r.lit.LitShader() }

func (r *rig) litSource(img *ebiten.Image) (*ebiten.Image, float32, bool) {
	return r.lit.SourceOf(&r.view, img)
}

// drawLit is a lit draw outside a bake and the shadow pass, bloodless.
func (r *rig) drawLit(dst, img *ebiten.Image, geo ebiten.GeoM, cs [4]float32, wx, wy float64) {
	r.lit.Draw(&r.view, dst, img, geo, cs, wx, wy, nil, false, 0)
}

// unit is (x, y) normalised.
func unit(x, y float64) (float64, float64) {
	n := math.Hypot(x, y)
	return x / n, y / n
}
