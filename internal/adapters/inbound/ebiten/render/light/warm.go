package light

import (
	"sync"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Shader warm-up (GAR E9). A Kage shader costs a compile and a Metal
// pipeline the first time it draws; done inside a frame, that is a hitch
// on the first lit or water frame. The world shaders therefore compile
// once per process, drawing one pixel off-screen inside the Once (which
// also sizes their uniforms under it: parallel tests share them, as with
// art.TintShader), and WarmScreen adds the screen's pipeline from the
// boot and menu screens, before any colony frame.

// SharedLit is lit.kage, compiled and warmed once per process.
var SharedLit = sync.OnceValues(func() (*ebiten.Shader, error) { return Compile(LitKage) })

// Compile compiles a Kage source and draws one pixel with it off-screen.
func Compile(src []byte) (*ebiten.Shader, error) {
	sh, err := ebiten.NewShader(src)
	if err != nil {
		return nil, err
	}
	img, dst := ebiten.NewImage(1, 1), ebiten.NewImage(1, 1)
	defer img.Deallocate()
	defer dst.Deallocate()
	var v [4]ebiten.Vertex
	for i := range v {
		x, y := float32(i&1), float32(i>>1)
		v[i] = ebiten.Vertex{DstX: x, DstY: y, SrcX: x, SrcY: y, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}
	}
	op := &ebiten.DrawTrianglesShaderOptions{}
	op.Images[0] = img
	dst.DrawTrianglesShader32(v[:], Indices[:], sh, op)
	return sh, nil
}

// WarmScreen draws one zero-area triangle through each shader onto dst
// (the screen): no fragment runs, so no pixel changes, but the pipeline
// for dst's format and the default blend exists before the first world
// frame. Nil shaders (a failed compile) are skipped.
func WarmScreen(dst *ebiten.Image, shaders ...*ebiten.Shader) {
	var v [3]ebiten.Vertex
	for _, sh := range shaders {
		if sh != nil {
			dst.DrawTrianglesShader32(v[:], Indices[:3], sh, &ebiten.DrawTrianglesShaderOptions{})
		}
	}
}
