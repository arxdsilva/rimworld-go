package world

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// stackRig is a rig whose 10 x 6 visible tiles each hold a stack, drawn
// once so the camera and view are set.
func stackRig(t *testing.T) (*rig, game.WorldView) {
	w := rigWorld(32, 32)
	w.Items = w.Items[:0]
	for y := 0; y < 6; y++ {
		for x := 0; x < 10; x++ {
			w.Items = append(w.Items, game.ItemView{ID: entity.EntityID(100 + y*10 + x), Kind: "wood", Qty: 2 + (y*10+x)%75, X: x, Y: y})
		}
	}
	r := newRig(t, w, 32, 32, "item/wood_pile")
	r.frame(64)
	return r, w
}

func BenchmarkStackCounts(b *testing.B) {
	r, w := stackRig(new(testing.T))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.p.Lb.drawStackCounts(r.screen, w, 64)
	}
}
