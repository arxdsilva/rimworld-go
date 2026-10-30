package game

import "testing"

// TestDraftDefaults: a colonist fires at will, holds no position and
// carries no grenades until told otherwise (DEF-4). This was the wave E
// contract's inertness test; once PRD-4 and DEF-4 handled the commands
// only the defaults remain to check (the commands have their own tests in
// raid_def4_test.go, combat/draft_test.go and the craft stockpile tests).
func TestDraftDefaults(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 7, Width: 30, Height: 30})
	if _, err := g.SpawnPawn(10, 10); err != nil {
		t.Fatal(err)
	}
	pawns := g.Snapshot()
	if len(pawns) == 0 {
		t.Fatal("no colonists")
	}
	if !pawns[0].FireAtWill || pawns[0].HoldPosition || pawns[0].Grenades != 0 {
		t.Errorf("a colonist fires at will and holds nothing: %+v", pawns[0])
	}
}
