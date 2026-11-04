package ebiten

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/domain/sim/entity"
)

// A selected id nothing ever had (no click can make one, so this is no
// journey) gives no panic and no card, docked or floating.
func TestDossierStaleSelection(t *testing.T) {
	for _, docked := range []bool{true, false} {
		j := newJourney(t)
		dossierOn(j, docked)
		j.a.renderer.Select(entity.EntityID(99999))
		j.update(2)
		j.draw(1)
		if !j.a.hud().Card.Empty() || len(cardIDs(j, "dossier.")) != 0 {
			t.Fatalf("docked %v: a card over a pawn that is not there", docked)
		}
		j.a.sel.Select(selection.Cand{Kind: selection.Animal, ID: 88888})
		j.update(2)
		j.draw(1)
		if !j.a.hud().Card.Empty() || len(cardIDs(j, "dossier.")) != 0 {
			t.Fatalf("docked %v: a card over an animal that is not there", docked)
		}
	}
}
