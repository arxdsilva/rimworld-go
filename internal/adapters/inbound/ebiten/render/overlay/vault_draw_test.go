package overlay

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

func TestCasketOpenedCuesOnce(t *testing.T) {
	t.Parallel()
	var m CasketMemo
	sealed := game.BuildingView{ID: 3, Kind: CasketKind, X: 4, Y: 5}
	sealed.Sealed = true
	open := sealed
	open.Sealed = false
	world := func(b game.BuildingView) game.WorldView { return game.WorldView{Buildings: []game.BuildingView{b}} }
	if got := m.CasketOpened(world(open)); len(got) != 0 { // first frame only records
		t.Errorf("first frame cued %v", got)
	}
	m = CasketMemo{}
	m.CasketOpened(world(sealed))
	if got := m.CasketOpened(world(sealed)); len(got) != 0 {
		t.Errorf("still sealed cued %v", got)
	}
	if got := m.CasketOpened(world(open)); len(got) != 1 || got[0].ID != 3 {
		t.Errorf("opening cued %v", got)
	}
	if got := m.CasketOpened(world(open)); len(got) != 0 {
		t.Errorf("an open casket cues once, got %v", got)
	}
}

func TestCasketStateHelpers(t *testing.T) {
	t.Parallel()
	if CasketSpriteFor(true) != "structure/sleeper_casket_closed" || CasketSpriteFor(false) != "structure/sleeper_casket_open" {
		t.Error("sprite by state")
	}
	if CasketTitleFor(true) != "Sleeper casket (sealed)" || CasketTitleFor(false) != "Sleeper casket (open)" {
		t.Error("title by state")
	}
}

func TestCasketOpenedInjected(t *testing.T) {
	t.Parallel()
	sealed := map[entity.EntityID]bool{3: true, 4: true}
	of := func(b game.BuildingView) bool { return sealed[b.ID] }
	w := game.WorldView{Buildings: []game.BuildingView{{ID: 3, Kind: CasketKind}, {ID: 4, Kind: CasketKind}, {ID: 9, Kind: "wall"}}}
	var m CasketMemo
	if got := m.Opened(w, of); len(got) != 0 {
		t.Errorf("first frame cued %v", got)
	}
	if got := m.Opened(w, of); len(got) != 0 {
		t.Errorf("unchanged cued %v", got)
	}
	sealed[3] = false
	if got := m.Opened(w, of); len(got) != 1 || got[0].ID != 3 {
		t.Errorf("opening cued %v", got)
	}
	if got := m.Opened(w, of); len(got) != 0 {
		t.Errorf("cues once, got %v", got)
	}
}
