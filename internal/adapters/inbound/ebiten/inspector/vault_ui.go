package inspector

// The sleeper casket's pane commands (the casket sprites and the memo that
// cues its opening are render/overlay, vault_draw.go).

import (
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// casketCommands are the commands of building b (none unless it is a
// casket).
func (a Building) CasketCommands(b game.BuildingView) []Button {
	return a.casketCommandsFor(b, overlay.CasketSealed(b))
}

func (a Building) casketCommandsFor(b game.BuildingView, sealed bool) []Button {
	if b.Kind != overlay.CasketKind {
		return nil
	}
	refusal := ""
	if sealed {
		refusal = b.OpenReason // the game's refusal ("" when it would accept)
	}
	return []Button{CasketCommand(sealed, refusal)}
}

// applyCasketClick sends OpenCasket for the Open casket button.
func (a Building) ApplyCasketClick(id string, b game.BuildingView) bool {
	return a.ApplyCasketClickFor(id, b, overlay.CasketSealed(b))
}

func (a Building) ApplyCasketClickFor(id string, b game.BuildingView, sealed bool) bool {
	if id != CasketBtnID || b.Kind != overlay.CasketKind {
		return false
	}
	if sealed {
		a.shell.Enqueue(command.Command{Kind: command.OpenCasket, Target: uint64(b.ID)})
	}
	return true
}

const (
	CasketBtnID   = "bldg.casket.open"
	casketBtnTip  = "Opens every casket in this vault"
	casketOpenMsg = "This casket is already open."
)

// casketCommand is the Open casket command of a casket: disabled with a
// reason once opened or when the game refuses. The click goes through
// applyBuildingClick (applyCasketClick).
func CasketCommand(sealed bool, refusal string) Button {
	c := Button{ID: CasketBtnID, Label: "Open casket", Icon: "open_casket", Desc: casketBtnTip}
	switch {
	case !sealed:
		c.Disabled, c.Reason = true, casketOpenMsg
	case refusal != "":
		c.Disabled, c.Reason = true, refusal
	}
	return c
}
