package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
)

// animalBaseKey is the animal's standing sprite for its current facing:
// "animal/<species>_<facing>" (ART-011); a female of a dimorphic species
// uses "animal/<species>_f_<facing>" when that art exists (else the male
// set, so a species without female art still draws).
func (r *Renderer) animalBaseKey(an game.AnimalView) string {
	facing := r.last[an.ID].facing.String()
	if an.Female {
		if k := "animal/" + an.Species + "_f_" + facing; r.art.has(k) {
			return k
		}
	}
	return "animal/" + an.Species + "_" + facing
}

// animalFrameKey is the sprite drawn for the animal this frame: its base
// key with the trot, attack, graze or flinch frame (framed falls back to
// the standing sprite when a frame is missing).
func (r *Renderer) animalFrameKey(an game.AnimalView) string {
	base := r.animalBaseKey(an)
	if r.animalResting(an) {
		if k := base + "_rest"; r.art.has(k) {
			return k
		}
		return r.framed(base, "_graze1") // no rest pose yet: head down
	}
	return r.framed(base, r.animalFrame(an))
}

// animalResting reports a sleeping animal that is standing still: it
// draws the rest pose.
func (r *Renderer) animalResting(an game.AnimalView) bool {
	return an.Sleeping && r.animalMotion(an.ID).Blend < minBlend
}

// animalRestFallback is true when a resting animal has no rest sprite and
// shows its grazing frame dimmed instead.
func (r *Renderer) animalRestFallback(an game.AnimalView) bool {
	return r.animalResting(an) && !r.art.has(r.animalBaseKey(an)+"_rest")
}

// restDim is the colour scale of the stand-in rest frame (10% darker).
var restDim = colorScale{0.9, 0.9, 0.9, 1}

// youngPose scales a juvenile's figure about its feet (Scale 0.55..1; 0
// or 1 is full size).
func youngPose(ps pose, scale float64) pose {
	if scale <= 0 || scale >= 1 {
		return ps
	}
	if ps.sx == 0 {
		ps.sx = 1
	}
	if ps.sy == 0 {
		ps.sy = 1
	}
	ps.sx *= scale
	ps.sy *= scale
	return ps
}

// drawAnimalFigure draws one animal's body, posed, at (wx, wy): the same
// call in the object pass and in the shadow pass (r.cast set). It has no
// side effects beyond drawing.
func (r *Renderer) drawAnimalFigure(dst *ebiten.Image, an game.AnimalView, wx, wy, tile float64) {
	if !r.art.has(r.animalBaseKey(an)) {
		if r.cast != nil {
			return // no art, no silhouette
		}
		sx, sy := r.worldToScreenF(wx, wy)
		vector.FillCircle(dst, sx+float32(tile)/2, sy+float32(tile)/2, float32(tile)*0.3, color.RGBA{150, 120, 80, 255}, false)
		return
	}
	r.figureBlood = r.animalBlood(an) // the whole figure (blood_draw.go)
	ps := youngPose(r.animalPose(an), an.Scale)
	if r.animalRestFallback(an) {
		ps.cs = restDim
	}
	r.drawSpritePose(dst, r.animalFrameKey(an), color.NRGBA{}, wx, wy, tile, ps)
	r.figureBlood = 0
}
