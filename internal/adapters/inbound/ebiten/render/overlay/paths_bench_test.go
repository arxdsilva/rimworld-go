package overlay

import (
	"sync"
	"testing"
	_ "unsafe" // go:linkname, below

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Outside a game loop Ebitengine's atlas keeps every draw queued until a
// frame that never comes; a benchmark empties the queue as it goes, like
// the root's tests (test_main_test.go, whose TestAtlasQueueDrains fails if
// an Ebitengine upgrade renames these).

//go:linkname benchAtlasDeferred github.com/hajimehoshi/ebiten/v2/internal/atlas.deferred
var benchAtlasDeferred []func()

//go:linkname benchAtlasDeferredM github.com/hajimehoshi/ebiten/v2/internal/atlas.deferredM
var benchAtlasDeferredM sync.Mutex

// BenchmarkDrawPaths is the path overlay's real draw for 30 routed
// colonists (a third drafted, one selected), each with a goal mark, with
// the screen used after every frame so vector's deferred anti-aliased
// fills are paid inside the loop.
func BenchmarkDrawPaths(b *testing.B) {
	o := &Set{}
	v := testView(game.WorldView{})
	for i := range 30 {
		x, y := 1+i%6*2, 1+i/6*2
		var path [][2]int
		for k := 1; k <= 8; k++ {
			path = append(path, [2]int{x + k%3, y + k/3})
		}
		v.Pawns = append(v.Pawns, game.PawnView{ID: entity.EntityID(i + 1), X: x, Y: y, Path: path, Drafted: i%3 == 0,
			HasGoal: true, GoalX: path[7][0], GoalY: path[7][1]})
	}
	v.Selected, v.HasSelect = v.Pawns[4].ID, true
	screen := ebiten.NewImage(640, 640)
	pos := func(p game.PawnView) (float64, float64) { return float64(p.X), float64(p.Y) }
	b.ReportAllocs()
	for b.Loop() {
		o.DrawPaths(v, screen, 640, 640, 64, pos)
		screen.Clear() // a use of the screen: vector's deferred fills run here
		benchAtlasDeferredM.Lock()
		benchAtlasDeferred = nil
		benchAtlasDeferredM.Unlock()
	}
}
