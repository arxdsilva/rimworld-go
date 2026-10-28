package ebiten

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

func TestCasketSpriteAndTitleByState(t *testing.T) {
	t.Parallel()
	b := game.BuildingView{Kind: casketKind}
	if casketSprite(b) != casketOpen || casketTitle(b) != "Sleeper casket (open)" {
		t.Errorf("unsealed: %q %q", casketSprite(b), casketTitle(b))
	}
	b.Sealed = true
	if casketSprite(b) != "structure/sleeper_casket_closed" || casketTitle(b) != "Sleeper casket (sealed)" {
		t.Errorf("sealed: %q %q", casketSprite(b), casketTitle(b))
	}
	if buildingTitle(b) != "Sleeper casket (sealed)" {
		t.Errorf("inspector title %q", buildingTitle(b))
	}
	if casketTitle(game.BuildingView{Kind: "wall"}) != "" {
		t.Error("only caskets get the casket title")
	}
}

func TestCasketCommand(t *testing.T) {
	t.Parallel()
	ok := casketCommand(true, "")
	if ok.Disabled || ok.Label != "Open casket" || ok.Desc != "Opens every casket in this vault" || ok.ID != casketBtnID || ok.Icon != "open_casket" {
		t.Errorf("sealed command = %+v", ok)
	}
	open := casketCommand(false, "")
	if !open.Disabled || open.Reason == "" {
		t.Errorf("an open casket's command must be disabled with a reason: %+v", open)
	}
	refused := casketCommand(true, "No colonist can reach it.")
	if !refused.Disabled || refused.Reason != "No colonist can reach it." {
		t.Errorf("refused command = %+v", refused)
	}
}

func TestOpenCasketButtonEmitsCommand(t *testing.T) {
	a, g, _ := liveColony(t, 0)
	rs := &recSim{Simulation: g}
	a.renderer.game = rs
	b := game.BuildingView{ID: 41, Kind: casketKind, X: 3, Y: 3}
	b.Sealed = true
	if !a.applyCasketClick(casketBtnID, b) {
		t.Fatal("click not handled")
	}
	if len(rs.cmds) != 1 || rs.cmds[0].Kind != command.OpenCasket || rs.cmds[0].Target != 41 {
		t.Fatalf("cmds = %+v", rs.cmds)
	}
	if a.applyCasketClick("bldg.close", b) {
		t.Error("other ids must not be handled")
	}
	// an opened casket sends nothing
	b.Sealed = false
	rs.cmds = nil
	a.applyCasketClick(casketBtnID, b)
	if len(rs.cmds) != 0 {
		t.Errorf("open casket sent %+v", rs.cmds)
	}
}

// The command shows the game's own refusal (BuildingView.OpenReason).
func TestCasketCommandsAskTheGame(t *testing.T) {
	a, _, _ := liveColony(t, 0)
	b := game.BuildingView{ID: 5, Kind: casketKind, Sealed: true, OpenReason: "Nobody can reach it."}
	got := a.casketCommands(b)
	if len(got) != 1 || !got[0].Disabled || got[0].Reason != "Nobody can reach it." {
		t.Fatalf("refused: %+v", got)
	}
	b.OpenReason = ""
	if got = a.casketCommands(b); got[0].Disabled {
		t.Errorf("accepted: %+v", got[0])
	}
	b.Sealed = false
	if got = a.casketCommands(b); !got[0].Disabled {
		t.Errorf("opened casket must be disabled: %+v", got[0])
	}
	if a.casketCommands(game.BuildingView{Kind: "wall"}) != nil {
		t.Error("walls have no casket command")
	}
}

func TestCasketOpenedCuesOnce(t *testing.T) {
	t.Parallel()
	var m casketMemo
	sealed := game.BuildingView{ID: 3, Kind: casketKind, X: 4, Y: 5}
	sealed.Sealed = true
	open := sealed
	open.Sealed = false
	world := func(b game.BuildingView) game.WorldView { return game.WorldView{Buildings: []game.BuildingView{b}} }
	if got := m.CasketOpened(world(open)); len(got) != 0 { // first frame only records
		t.Errorf("first frame cued %v", got)
	}
	m = casketMemo{}
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

func TestListenCasketsEmitsSound(t *testing.T) {
	r := New(&stubSim{w: 8, h: 8})
	sealed := game.BuildingView{ID: 3, Kind: casketKind, X: 4, Y: 5}
	sealed.Sealed = true
	open := sealed
	open.Sealed = false
	r.listenCaskets(game.WorldView{Buildings: []game.BuildingView{sealed}})
	r.listenCaskets(game.WorldView{Buildings: []game.BuildingView{open}})
	if len(r.cues) != 1 || r.cues[0].ID != "sfx.casket_open" || r.cues[0].X != 4.5 {
		t.Errorf("cues = %+v", r.cues)
	}
}

func TestDormantAnimalLabel(t *testing.T) {
	t.Parallel()
	a := &App{}
	an := game.AnimalView{State: "wandering"}
	if got := a.animalStateLine(an); got == "Dormant" {
		t.Fatalf("awake animal reads %q", got)
	}
	an.Dormant = true
	if got := a.animalStateLine(an); got != "Dormant" {
		t.Errorf("dormant animal reads %q", got)
	}
}

// Behaviour with the sealed state injected, independent of the game
// side's BuildingView.Sealed field.

func TestCasketStateHelpers(t *testing.T) {
	t.Parallel()
	if casketSpriteFor(true) != "structure/sleeper_casket_closed" || casketSpriteFor(false) != "structure/sleeper_casket_open" {
		t.Error("sprite by state")
	}
	if casketTitleFor(true) != "Sleeper casket (sealed)" || casketTitleFor(false) != "Sleeper casket (open)" {
		t.Error("title by state")
	}
}

func TestOpenCasketClickInjected(t *testing.T) {
	a, g, _ := liveColony(t, 0)
	rs := &recSim{Simulation: g}
	a.renderer.game = rs
	b := game.BuildingView{ID: 41, Kind: casketKind}
	if !a.applyCasketClickFor(casketBtnID, b, true) || len(rs.cmds) != 1 ||
		rs.cmds[0].Kind != command.OpenCasket || rs.cmds[0].Target != 41 {
		t.Fatalf("cmds = %+v", rs.cmds)
	}
	rs.cmds = nil
	a.applyCasketClickFor(casketBtnID, b, false)
	if a.applyCasketClickFor("bldg.close", b, true) || len(rs.cmds) != 0 {
		t.Errorf("open casket or other id sent %+v", rs.cmds)
	}
}

func TestCasketOpenedInjected(t *testing.T) {
	t.Parallel()
	sealed := map[entity.EntityID]bool{3: true, 4: true}
	of := func(b game.BuildingView) bool { return sealed[b.ID] }
	w := game.WorldView{Buildings: []game.BuildingView{{ID: 3, Kind: casketKind}, {ID: 4, Kind: casketKind}, {ID: 9, Kind: "wall"}}}
	var m casketMemo
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
