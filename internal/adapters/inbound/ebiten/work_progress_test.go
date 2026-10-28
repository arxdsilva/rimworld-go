package ebiten

import (
	"math"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// builder is a colonist building the frame at (6, 5) from (5, 5).
func builder(id int, progress float64) game.PawnView {
	return game.PawnView{ID: entity.EntityID(id), X: 5, Y: 5, ActX: 6, ActY: 5, Action: game.ActionBuild, ActionProgress: progress}
}

// A colonist at a job under way gets a small bar at the work spot: about
// 0.6 tile wide, centred on the work tile along its bottom edge (under
// the worker's feet), filled to the job's progress.
func TestWorkProgressBarDrawnOverTarget(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 16, h: 16, tick: 100})
	const tile = 64.0
	got := r.workBarRects([]game.PawnView{builder(1, 0.4)}, tile, nil)
	if len(got) != 1 {
		t.Fatalf("one bar: %+v", got)
	}
	b := got[0]
	tx, ty := r.worldToScreenF(6, 5)
	if cx := b.X + b.W/2; math.Abs(float64(cx-(tx+tile/2))) > 1e-3 {
		t.Fatalf("centred on the work tile: %v vs %v", cx, tx+tile/2)
	}
	if math.Abs(float64(b.W)-workBarW*tile) > 1e-3 || b.H < 3 || b.H > 0.1*tile {
		t.Fatalf("a small bar: %+v", b)
	}
	if feet := ty + pawnFeetY*tile; b.Y <= feet || b.Y+b.H > ty+tile {
		t.Fatalf("along the work tile's bottom, under the feet (%v): %v..%v, tile %v..%v", feet, b.Y, b.Y+b.H, ty, ty+tile)
	}
	if b.K != 0.4 {
		t.Fatalf("filled to the progress: %v", b.K)
	}
	// nothing for unknown, unstarted or finished jobs, eating, research,
	// recreation, raiders, the downed and the carried
	for _, p := range []game.PawnView{
		builder(1, -1), builder(1, 0), builder(1, 1),
		{ID: 2, ActX: 6, ActY: 5, Action: game.ActionEat, ActionProgress: 0.5},
		{ID: 2, ActX: 6, ActY: 5, Action: game.ActionResearch, ActionProgress: 0.5},
		{ID: 2, ActX: 6, ActY: 5, Action: game.ActionHorseshoes, ActionProgress: 0.5},
		{ID: 2, ActX: 6, ActY: 5, Action: game.ActionAim, ActionProgress: 0.5},
		{ID: 2, ActX: 6, ActY: 5, Action: game.ActionHaul, ActionProgress: 0.5},
		{ID: 2, ActX: 6, ActY: 5, Action: game.ActionBreach, ActionProgress: 0.5, Hostile: true},
		{ID: 2, ActX: 6, ActY: 5, Action: game.ActionBuild, ActionProgress: 0.5, Lying: true},
		{ID: 2, ActX: 6, ActY: 5, Action: game.ActionBuild, ActionProgress: 0.5, CarriedBy: 9},
	} {
		if got := r.workBarRects([]game.PawnView{p}, tile, nil); len(got) != 0 {
			t.Fatalf("no bar for %+v: %+v", p, got)
		}
	}
	// every job the sim reports progress for draws one
	for _, act := range []string{game.ActionCraft, game.ActionCook, game.ActionMine, game.ActionSow,
		game.ActionHarvest, game.ActionTend, game.ActionClean, game.ActionChop, game.ActionDeconstruct} {
		p := builder(1, 0.5)
		p.Action = act
		if got := r.workBarRects([]game.PawnView{p}, tile, nil); len(got) != 1 {
			t.Fatalf("%s: a bar", act)
		}
	}
	r.drawWorkProgress(ebiten.NewImage(512, 512), []game.PawnView{builder(1, 0.4)}, tile)
}

func TestWorkProgressBarHiddenWhenZoomedOut(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 16, h: 16, tick: 100})
	pawns := []game.PawnView{builder(1, 0.5)}
	if got := r.workBarRects(pawns, workBarMinTile-1, nil); len(got) != 0 {
		t.Fatalf("too small to read below %v px: %+v", workBarMinTile, got)
	}
	if got := r.workBarRects(pawns, workBarMinTile, nil); len(got) != 1 {
		t.Fatalf("drawn from %v px: %+v", workBarMinTile, got)
	}
}

// Several colonists on one job (a shared frame, a bench) draw one bar, the
// most advanced; separate jobs keep their own.
func TestWorkProgressBarOncePerTarget(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 16, h: 16, tick: 100})
	other := builder(3, 0.7)
	other.X, other.ActX = 8, 7
	pawns := []game.PawnView{builder(1, 0.3), builder(2, 0.6), other}
	pawns[1].X, pawns[1].Y = 6, 4 // from the north, same frame
	got := r.workBarRects(pawns, 64, nil)
	if len(got) != 2 {
		t.Fatalf("one bar per work tile: %+v", got)
	}
	if got[0].K != 0.6 || got[1].K != 0.7 {
		t.Fatalf("the shared frame shows its furthest progress: %+v", got)
	}
}
