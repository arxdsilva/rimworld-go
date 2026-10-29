package ebiten

import (
	"rimworld-go/internal/application/game"
)

func (r *Renderer) listenArrivals(w game.WorldView) { r.cueBus().ListenArrivals(r.liveView(), w) }

func (r *Renderer) audible(x, y float64) bool { return r.cueBus().Audible(r.liveView(), x, y) }
