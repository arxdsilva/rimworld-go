package ebiten

import (
	"rimworld-go/internal/application/game"
)

func (r *Renderer) listenArrivals(w game.WorldView) { r.cueBus().ListenArrivals(r.cueView(), w) }

func (r *Renderer) audible(x, y float64) bool { return r.cueBus().Audible(r.cueView(), x, y) }
