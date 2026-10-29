package ebiten

import (
	inspectorpkg "rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

func TestCasketSpriteAndTitleByState(t *testing.T) {
	t.Parallel()
	b := game.BuildingView{Kind: overlay.CasketKind}
	if overlay.CasketSprite(b) != overlay.CasketOpen || overlay.CasketTitle(b) != "Sleeper casket (open)" {
		t.Errorf("unsealed: %q %q", overlay.CasketSprite(b), overlay.CasketTitle(b))
	}
	b.Sealed = true
	if overlay.CasketSprite(b) != "structure/sleeper_casket_closed" || overlay.CasketTitle(b) != "Sleeper casket (sealed)" {
		t.Errorf("sealed: %q %q", overlay.CasketSprite(b), overlay.CasketTitle(b))
	}
	if inspectorpkg.BuildingTitle(b) != "Sleeper casket (sealed)" {
		t.Errorf("inspector title %q", inspectorpkg.BuildingTitle(b))
	}
	if overlay.CasketTitle(game.BuildingView{Kind: "wall"}) != "" {
		t.Error("only caskets get the casket title")
	}
}

func TestCasketCommand(t *testing.T) {
	t.Parallel()
	ok := inspectorpkg.CasketCommand(true, "")
	if ok.Disabled || ok.Label != "Open casket" || ok.Desc != "Opens every casket in this vault" || ok.ID != inspectorpkg.CasketBtnID || ok.Icon != "open_casket" {
		t.Errorf("sealed command = %+v", ok)
	}
	open := inspectorpkg.CasketCommand(false, "")
	if !open.Disabled || open.Reason == "" {
		t.Errorf("an open casket's command must be disabled with a reason: %+v", open)
	}
	refused := inspectorpkg.CasketCommand(true, "No colonist can reach it.")
	if !refused.Disabled || refused.Reason != "No colonist can reach it." {
		t.Errorf("refused command = %+v", refused)
	}
}

func TestOpenCasketButtonEmitsCommand(t *testing.T) {
	a, g, _ := liveColony(t, 0)
	rs := &recSim{Simulation: g}
	a.renderer.game = rs
	b := game.BuildingView{ID: 41, Kind: overlay.CasketKind, X: 3, Y: 3}
	b.Sealed = true
	if !a.insp.Building().ApplyCasketClick(inspectorpkg.CasketBtnID, b) {
		t.Fatal("click not handled")
	}
	if len(rs.cmds) != 1 || rs.cmds[0].Kind != command.OpenCasket || rs.cmds[0].Target != 41 {
		t.Fatalf("cmds = %+v", rs.cmds)
	}
	if a.insp.Building().ApplyCasketClick("bldg.close", b) {
		t.Error("other ids must not be handled")
	}
	// an opened casket sends nothing
	b.Sealed = false
	rs.cmds = nil
	a.insp.Building().ApplyCasketClick(inspectorpkg.CasketBtnID, b)
	if len(rs.cmds) != 0 {
		t.Errorf("open casket sent %+v", rs.cmds)
	}
}

// The command shows the game's own refusal (BuildingView.OpenReason).
func TestCasketCommandsAskTheGame(t *testing.T) {
	a, _, _ := liveColony(t, 0)
	b := game.BuildingView{ID: 5, Kind: overlay.CasketKind, Sealed: true, OpenReason: "Nobody can reach it."}
	got := a.insp.Building().CasketCommands(b)
	if len(got) != 1 || !got[0].Disabled || got[0].Reason != "Nobody can reach it." {
		t.Fatalf("refused: %+v", got)
	}
	b.OpenReason = ""
	if got = a.insp.Building().CasketCommands(b); got[0].Disabled {
		t.Errorf("accepted: %+v", got[0])
	}
	b.Sealed = false
	if got = a.insp.Building().CasketCommands(b); !got[0].Disabled {
		t.Errorf("opened casket must be disabled: %+v", got[0])
	}
	if a.insp.Building().CasketCommands(game.BuildingView{Kind: "wall"}) != nil {
		t.Error("walls have no casket command")
	}
}

func TestListenCasketsEmitsSound(t *testing.T) {
	r := New(&stubSim{w: 8, h: 8})
	sealed := game.BuildingView{ID: 3, Kind: overlay.CasketKind, X: 4, Y: 5}
	sealed.Sealed = true
	open := sealed
	open.Sealed = false
	r.overlays().ListenCaskets(game.WorldView{Buildings: []game.BuildingView{sealed}})
	r.overlays().ListenCaskets(game.WorldView{Buildings: []game.BuildingView{open}})
	if len(r.cueBus().Cues) != 1 || r.cueBus().Cues[0].ID != "sfx.casket_open" || r.cueBus().Cues[0].X != 4.5 {
		t.Errorf("cues = %+v", r.cueBus().Cues)
	}
}

func TestDormantAnimalLabel(t *testing.T) {
	t.Parallel()
	a := testApp(nil) // the features hang off NewApp (ARCH-005)
	an := game.AnimalView{State: "wandering"}
	if got := a.insp.Animal().AnimalStateLine(an); got == "Dormant" {
		t.Fatalf("awake animal reads %q", got)
	}
	an.Dormant = true
	if got := a.insp.Animal().AnimalStateLine(an); got != "Dormant" {
		t.Errorf("dormant animal reads %q", got)
	}
}

// Behaviour with the sealed state injected, independent of the game
// side's BuildingView.Sealed field.

func TestOpenCasketClickInjected(t *testing.T) {
	a, g, _ := liveColony(t, 0)
	rs := &recSim{Simulation: g}
	a.renderer.game = rs
	b := game.BuildingView{ID: 41, Kind: overlay.CasketKind}
	if !a.insp.Building().ApplyCasketClickFor(inspectorpkg.CasketBtnID, b, true) || len(rs.cmds) != 1 ||
		rs.cmds[0].Kind != command.OpenCasket || rs.cmds[0].Target != 41 {
		t.Fatalf("cmds = %+v", rs.cmds)
	}
	rs.cmds = nil
	a.insp.Building().ApplyCasketClickFor(inspectorpkg.CasketBtnID, b, false)
	if a.insp.Building().ApplyCasketClickFor("bldg.close", b, true) || len(rs.cmds) != 0 {
		t.Errorf("open casket or other id sent %+v", rs.cmds)
	}
}
