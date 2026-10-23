package game_test

import (
	"reflect"
	"testing"

	"rimworld-go/internal/application/game"
)

// TestViewCacheMatchesFresh: the renderer's read models (view cache on,
// with its memos and reused buffers) equal freshly built ones tick after
// tick on a colony under a raid (logs, stains, postponed work, motion),
// and a view handed out earlier is never rewritten.
func TestViewCacheMatchesFresh(t *testing.T) {
	if testing.Short() {
		t.Skip("long run")
	}
	colony := func() *game.Game {
		g := game.NewColony(game.Config{Seed: 1, Width: 120, Height: 120})
		g.Step(1500)
		g.Raid(8)
		g.Step(300)
		return g
	}
	cached, fresh := colony(), colony()
	cached.SetViewCache(true)
	prevW, prevP := cached.World(), cached.Snapshot()
	wantW, wantP := fresh.World(), fresh.Snapshot()
	for i := 0; i < 1500; i++ {
		cached.Step(3)
		fresh.Step(3)
		w, p := cached.World(), cached.Snapshot()
		if !reflect.DeepEqual(prevW, wantW) || !reflect.DeepEqual(prevP, wantP) {
			t.Fatalf("update %d: an earlier view was rewritten", i)
		}
		wantW, wantP = fresh.World(), fresh.Snapshot()
		if !reflect.DeepEqual(w, wantW) {
			t.Fatalf("update %d: cached World differs from a fresh one", i)
		}
		if !reflect.DeepEqual(p, wantP) {
			t.Fatalf("update %d: cached Snapshot differs from a fresh one", i)
		}
		prevW, prevP = w, p
	}
	logs, post := 0, 0
	for _, a := range prevW.Animals {
		logs += len(a.Log)
	}
	for _, p := range prevP {
		logs += len(p.Log)
		post += len(p.Postponed)
	}
	t.Logf("stains %d, log lines %d, postponed lines %d, pawns %d", len(prevW.Stains), logs, post, len(prevP))
}
