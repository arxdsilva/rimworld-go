package mapnav

import (
	"testing"

	"rimworld-go/internal/application/game"
)

func maps3() []game.MapView {
	return []game.MapView{{ID: 0, Label: "Colony"}, {ID: 2, Label: "Site", Threat: true}, {ID: 5, Label: "Camp"}}
}

func TestGroupsFollowMapOrderAndMarkCurrent(t *testing.T) {
	ps := []game.PawnView{{ID: 1, Map: 2}, {ID: 2, Map: 0}, {ID: 3, Map: 2}, {ID: 4, Map: 9}}
	g := Groups(maps3(), ps, 2)
	if len(g) != 3 || g[0].Map != 0 || g[1].Map != 2 || g[2].Map != 5 {
		t.Fatalf("order: %+v", g)
	}
	if len(g[0].Pawns) != 1 || len(g[1].Pawns) != 2 || len(g[2].Pawns) != 0 {
		t.Fatalf("pawns: %+v", g)
	}
	if g[1].Pawns[0].ID != 1 || g[1].Pawns[1].ID != 3 {
		t.Fatalf("pawn order kept: %+v", g[1].Pawns)
	}
	if !g[1].Current || g[0].Current || !g[1].Threat || g[0].Threat {
		t.Fatalf("flags: %+v", g)
	}
}

func TestNextWraps(t *testing.T) {
	m := maps3()
	cases := []struct {
		cur  game.MapID
		dir  int
		want game.MapID
	}{{0, 1, 2}, {2, 1, 5}, {5, 1, 0}, {0, -1, 5}, {5, -1, 2}, {7, 1, 0}, {7, -1, 5}}
	for _, c := range cases {
		if got := Next(m, c.cur, c.dir); got != c.want {
			t.Errorf("Next(%d,%d) = %d, want %d", c.cur, c.dir, got, c.want)
		}
	}
	if got := Next(m[:1], 0, 1); got != 0 {
		t.Errorf("one map: %d", got)
	}
	if got := Next(nil, 3, 1); got != 3 {
		t.Errorf("no maps: %d", got)
	}
}

func TestFallbackAvoidsTheClosedMap(t *testing.T) {
	m := maps3()
	if got := Fallback(m, 2); got != 0 {
		t.Errorf("closing 2: %d", got)
	}
	if got := Fallback(m, 0); got != 2 {
		t.Errorf("closing 0: %d", got)
	}
	if got := Fallback(m[:1], 0); got != 0 {
		t.Errorf("last map: %d", got)
	}
}
