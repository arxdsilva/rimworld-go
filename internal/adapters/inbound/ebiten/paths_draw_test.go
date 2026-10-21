package ebiten

import (
	"testing"

	"rimworld-go/internal/application/game"
)

func idScreen(x, y float64) (float32, float32)  { return float32(x * 64), float32(y * 64) }
func atTile(p game.PawnView) (float64, float64) { return float64(p.X), float64(p.Y) }

func TestPlanPathsEveryColonistWithAPath(t *testing.T) {
	t.Parallel()
	pawns := []game.PawnView{
		{ID: 1, X: 1, Y: 1, Path: [][2]int{{2, 1}, {3, 1}}, HasGoal: true, GoalX: 3, GoalY: 1},
		{ID: 2, X: 1, Y: 3, Drafted: true, Path: [][2]int{{1, 4}}, HasGoal: true, GoalX: 1, GoalY: 4},
		{ID: 3, X: 5, Y: 5}, // idle: no path
		{ID: 4, X: 2, Y: 2, Hostile: true, Path: [][2]int{{3, 2}}}, // raider: skipped
		{ID: 5, X: 40, Y: 40, Path: [][2]int{{41, 40}}},            // off-screen
	}
	plans := planPaths(pawns, 2, true, 1, 640, 480, idScreen, atTile)
	if len(plans) != 2 {
		t.Fatalf("plans = %d, want 2 (colonists 1 and 2): %+v", len(plans), plans)
	}
	if len(plans[0].segs) != 2 || !plans[0].hasGoal {
		t.Fatalf("first plan = %+v", plans[0])
	}
	if !plans[1].drafted || !plans[1].selected {
		t.Fatalf("second plan = %+v", plans[1])
	}
}

func TestPlanPathsDropAndZoomedOut(t *testing.T) {
	t.Parallel()
	p := game.PawnView{ID: 1, X: 1, Y: 1, Path: [][2]int{{2, 1}}, HasGoal: true, GoalX: 2, GoalY: 1, HasDrop: true, DropX: 4, DropY: 1}
	plans := planPaths([]game.PawnView{p}, 0, false, 1, 640, 480, idScreen, atTile)
	if len(plans) != 1 || !plans[0].hasDrop {
		t.Fatalf("plans = %+v", plans)
	}
	far := planPaths([]game.PawnView{p}, 0, false, 0.2, 640, 480, idScreen, atTile)
	if len(far) != 1 || len(far[0].segs) != 0 || !far[0].hasGoal {
		t.Fatalf("zoomed out draws only the goal marker: %+v", far)
	}
}

func TestPlanPathsKeepsSegmentCrossingScreen(t *testing.T) {
	t.Parallel()
	p := game.PawnView{ID: 1, X: -3, Y: 2, Path: [][2]int{{12, 2}}} // from off the left edge to off the right
	plans := planPaths([]game.PawnView{p}, 0, false, 1, 640, 480, idScreen, atTile)
	if len(plans) != 1 || len(plans[0].segs) != 1 {
		t.Fatalf("a segment crossing the screen is drawn: %+v", plans)
	}
}
