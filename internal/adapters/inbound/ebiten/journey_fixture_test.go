package ebiten

import (
	"bytes"
	"sync"
	"testing"

	"rimworld-go/internal/application/game"
)

// worldFix is the wizard colony of worldColony, played once per process and
// kept as a save: every journey on the tiny world loads it instead of
// clicking the New Colony wizard again.
var worldFix struct {
	once sync.Once
	save []byte
	hash uint64
}

// worldFixtureGame is a fresh Load of the wizard colony.
func worldFixtureGame(t *testing.T) *game.Game {
	t.Helper()
	worldFix.once.Do(func() {
		j := newMenuJourney(t)
		wizardToColony(t, j)
		var buf bytes.Buffer
		if err := j.g.Save(&buf, "fixture"); err != nil {
			t.Errorf("saving the wizard colony: %v", err)
			return
		}
		worldFix.hash = j.g.StateHash()
		worldFix.save = buf.Bytes()
	})
	if worldFix.save == nil {
		t.Fatal("the wizard colony fixture failed in the test that built it")
	}
	g, _, err := game.Load(bytes.NewReader(worldFix.save))
	if err != nil {
		t.Fatalf("loading the wizard colony: %v", err)
	}
	return g
}

// TestWorldFixtureMatchesTheWizard: a loaded fixture is the colony the wizard
// builds (same state hash), so it cannot drift from the New Colony flow.
func TestWorldFixtureMatchesTheWizard(t *testing.T) {
	t.Parallel()
	j := newMenuJourney(t)
	wizardToColony(t, j)
	g := worldFixtureGame(t)
	if want, got := j.g.StateHash(), g.StateHash(); want != got {
		t.Fatalf("the loaded fixture hashes %x, the wizard's colony %x", got, want)
	}
	if g.StateHash() != worldFix.hash {
		t.Fatalf("a load of the fixture hashes %x, the saved colony %x", g.StateHash(), worldFix.hash)
	}
}
