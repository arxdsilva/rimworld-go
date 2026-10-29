package ebiten

import (
	motionpkg "rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/domain/sim/entity"
)

// framed returns key+suffix when that frame exists, else key.
func (r *Renderer) framed(key, suffix string) string { return motionpkg.Framed(r.art, key, suffix) }

// itemPop is the pop-in scale of a ground item (syncItems).
func (r *Renderer) itemPop(id entity.EntityID) float64 {
	return motionpkg.PopScale(r.itemBorn[id], r.present)
}

// sway is the wind shear for a plant at (x, y).
func (r *Renderer) sway(x, y int, amount float64) float64 {
	return motionpkg.Sway(r.present, x, y, amount)
}
