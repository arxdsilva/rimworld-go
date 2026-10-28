package scenario

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// TestSocialScenario builds the social colony and steps it two in-game
// days and pins what the dev hooks staged: the marriage, the engagement,
// the rivalry, the dog's bond and the jaw scar, besides the beds and the
// prisoner.
func TestSocialScenario(t *testing.T) {
	g, c, err := Social(1, 80, 60)
	if err != nil {
		t.Fatal(err)
	}
	if c.DoubleBed == 0 || c.PrisBed == 0 {
		t.Fatalf("beds not placed: %+v", c)
	}
	// The cast's relations are under test, not the wildlife: a wild wolf
	// that wanders in can kill the dog's master (PLAY-022's slower idle
	// wandering moved one such encounter onto Maren), so the map's wild
	// animals go before the two days run.
	for _, a := range g.World().Animals {
		if !a.Tame {
			g.KillAnimal(a.ID)
		}
	}
	g.Step(2 * clock.TicksPerDay)

	if v, ok := g.Prisoner(c.Prisoner); !ok || !v.ForPrisonersBed {
		t.Errorf("prisoner %d: view %+v ok %v, want a prisoner on a prisoner bed", c.Prisoner, v, ok)
	}
	dog := false
	for _, a := range g.World().Animals {
		if a.ID == c.Dog {
			dog = true
			if a.MasterID != c.Couple[0] || a.BondID != c.Couple[0] {
				t.Errorf("dog master %d bond %d, want %d", a.MasterID, a.BondID, c.Couple[0])
			}
		}
	}
	if !dog {
		t.Error("dog missing")
	}
	hasRow := func(a, b entity.EntityID, label string) (game.RelationRowView, bool) {
		s, _ := g.Social(a)
		for _, r := range s.Relations {
			if r.Other == uint64(b) && (label == "" || r.Label == label) {
				return r, true
			}
		}
		return game.RelationRowView{}, false
	}
	married := func(l string) bool { return l == "husband" || l == "wife" || l == "spouse" }
	if r, ok := hasRow(c.Couple[0], c.Couple[1], ""); !ok || !married(r.Label) {
		s, _ := g.Social(c.Couple[0])
		t.Errorf("couple not married: %+v", s.Relations)
	}
	if _, ok := hasRow(c.Engaged[0], c.Engaged[1], ""); !ok {
		t.Error("engaged pair has no relation row")
	} else if r, _ := hasRow(c.Engaged[0], c.Engaged[1], ""); !married(r.Label) && !strings.HasPrefix(r.Label, "fianc") {
		t.Errorf("engaged pair is %q, want fiance(e) (or married once wed)", r.Label)
	}
	if r, ok := hasRow(c.Rival[0], c.Rival[1], ""); !ok || r.OpinionTo >= 0 || r.OpinionFrom >= 0 {
		t.Errorf("rivals: row %+v ok %v, want negative opinions both ways", r, ok)
	}
	for _, p := range g.Snapshot() {
		if p.ID == c.Scarred && !p.Disfigured {
			t.Error("scarred colonist is not disfigured")
		}
	}
	if _, ok := g.Health(c.Couple[1]); !ok {
		t.Error("no Health view for the second partner")
	}
}

func TestSocialScenarioTooSmall(t *testing.T) {
	if _, _, err := Social(1, 40, 40); err == nil {
		t.Fatal("want an error for a small map")
	}
}
