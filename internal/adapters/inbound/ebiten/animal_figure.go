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
	return r.framed(r.animalBaseKey(an), r.animalFrame(an))
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
	r.drawSpritePose(dst, r.animalFrameKey(an), color.NRGBA{}, wx, wy, tile, r.animalPose(an))
}
