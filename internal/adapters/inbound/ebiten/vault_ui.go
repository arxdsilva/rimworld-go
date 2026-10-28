package ebiten

// The sleeper casket's pane commands (the casket sprites and the memo that
// cues its opening are render/overlay, vault_draw.go).

import (
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// casketCommands are the commands of building b (none unless it is a
// casket).
func (a *App) casketCommands(b game.BuildingView) []cmdButton {
	return a.casketCommandsFor(b, casketSealed(b))
}

func (a *App) casketCommandsFor(b game.BuildingView, sealed bool) []cmdButton {
	if b.Kind != casketKind {
		return nil
	}
	refusal := ""
	if sealed {
		refusal = b.OpenReason // the game's refusal ("" when it would accept)
	}
	return []cmdButton{casketCommand(sealed, refusal)}
}

// applyCasketClick sends OpenCasket for the Open casket button.
func (a *App) applyCasketClick(id string, b game.BuildingView) bool {
	return a.applyCasketClickFor(id, b, casketSealed(b))
}

func (a *App) applyCasketClickFor(id string, b game.BuildingView, sealed bool) bool {
	if id != casketBtnID || b.Kind != casketKind {
		return false
	}
	if sealed {
		a.shell.Enqueue(command.Command{Kind: command.OpenCasket, Target: uint64(b.ID)})
	}
	return true
}

const (
	casketBtnID   = "bldg.casket.open"
	casketBtnTip  = "Opens every casket in this vault"
	casketOpenMsg = "This casket is already open."
)

// casketCommand is the Open casket command of a casket: disabled with a
// reason once opened or when the game refuses. The click goes through
// applyBuildingClick (applyCasketClick).
func casketCommand(sealed bool, refusal string) cmdButton {
	c := cmdButton{ID: casketBtnID, Label: "Open casket", Icon: "open_casket", Desc: casketBtnTip}
	switch {
	case !sealed:
		c.Disabled, c.Reason = true, casketOpenMsg
	case refusal != "":
		c.Disabled, c.Reason = true, refusal
	}
	return c
}
