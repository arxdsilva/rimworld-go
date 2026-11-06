package inspector

// The AX-1 orders in the panes (MAIN_UI_SCREENS §17 inspector commands):
// a building's Uninstall toggle and its mark line, a filled grave's Open,
// a crated building's Reinstall and contents line, and a downed person's
// Strip toggle and mark line. Every click is a typed command (or the
// Reinstall placement intent); the panes only read views.

import (
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// Order button ids (journeys press them): new-style "cmd." commands that
// carry their command in Cmds (or Run).
const (
	UninstallBtnID = "cmd.bldg.uninstall"
	OpenGraveBtnID = "cmd.bldg.open"
	ReinstallBtnID = "cmd.item.reinstall"
	StripBtnID     = "cmd.pawn.strip"
)

// footprintCmd is kind k over building b's origin cell, marking (on).
func footprintCmd(k command.Kind, b game.BuildingView, on bool) command.Command {
	return command.Command{Kind: k, X: b.X, Y: b.Y, X2: b.X, Y2: b.Y, Enabled: on}
}

// unmarkCmd takes order o's mark off (x, y).
func unmarkCmd(o command.Order, x, y int) command.Command {
	return command.Command{Kind: command.Undesignate, Order: o, X: x, Y: y, X2: x, Y2: y}
}

// orderCommands are building b's AX-1 buttons: Uninstall (a movable
// piece) and Open (a filled grave or sarcophagus).
func orderCommands(a Building, b game.BuildingView) []Button {
	var out []Button
	if movable, _ := a.shell.MoveState(b); movable && !b.Ruin {
		u := Button{ID: UninstallBtnID, Label: a.shell.T("Uninstall"), Icon: "order_uninstall", Toggle: true, On: b.AX.Uninstall,
			Desc: a.shell.T("A builder packs it into a crate that keeps its material, quality and wear, to haul, sell or reinstall."),
			Cmds: []command.Command{footprintCmd(command.DesignateUninstall, b, true)}}
		if b.AX.Uninstall {
			u.Label, u.Desc = "Cancel uninstall", "Leave it standing."
			u.Cmds = []command.Command{unmarkCmd(command.OrderUninstall, b.X, b.Y)}
		}
		if b.Deconstruct {
			u.Disabled, u.Reason = true, "marked for deconstruction"
		}
		out = append(out, u)
	}
	if (b.Kind == "grave" || b.Kind == "sarcophagus") && b.Occupant != "" {
		o := Button{ID: OpenGraveBtnID, Label: a.shell.T("Open"), Icon: "order_open", Toggle: true, On: b.AX.OpenMarked,
			Desc: a.shell.T("A hauler opens it and lifts the body out beside it, unburied again."),
			Cmds: []command.Command{footprintCmd(command.DesignateOpen, b, true)}}
		if b.AX.OpenMarked {
			o.Label, o.Desc = "Cancel open", "Let the dead rest."
			o.Cmds = []command.Command{unmarkCmd(command.OrderOpen, b.X, b.Y)}
		}
		out = append(out, o)
	}
	return out
}

// OrderLines are building b's AX-1 mark lines.
func OrderLines(b game.BuildingView) []string {
	var out []string
	if b.AX.Uninstall {
		out = append(out, "Marked to uninstall")
	}
	if b.AX.OpenMarked {
		out = append(out, "Marked to open")
	}
	return out
}

// CrateLine is a crated building's contents line ("" for anything else).
func CrateLine(it game.ItemView) string {
	if it.AX.Building == "" {
		return ""
	}
	return "Holds a packed " + it.AX.Building + ": select it and Reinstall to set it up again."
}

// crateCommands is a crated building's Reinstall button.
func (a Thing) crateCommands(it game.ItemView) []Button {
	if it.AX.Building == "" {
		return nil
	}
	c := Button{ID: ReinstallBtnID, Label: a.shell.T("Reinstall"), Icon: "order_uninstall", Single: true,
		Desc: a.shell.T("Pick a spot for the ") + it.AX.Building + a.shell.T(": a builder carries the crate there and installs it."),
		Run:  func() { a.shell.Do(ports.ReinstallCrate{ID: it.ID}) }}
	if it.Carried {
		c.Disabled, c.Reason = true, "being carried"
	}
	return []Button{c}
}

// stripCommand is a downed person's Strip toggle (ok false: not downed).
func stripCommand(tr lang.Tr, p game.PawnView) (Button, bool) {
	if p.Status != "downed" {
		return Button{}, false
	}
	c := Button{ID: StripBtnID, Label: tr.T("Strip"), Icon: "order_strip", Toggle: true, On: p.AX.StripMarked,
		Desc: tr.T("A hauler takes off what this person wears and carries; it drops beside them."),
		Cmds: []command.Command{{Kind: command.DesignateStrip, X: p.X, Y: p.Y, X2: p.X, Y2: p.Y, Enabled: true}}}
	if p.AX.StripMarked {
		c.Label, c.Desc = "Cancel strip", "Leave them dressed."
		c.Cmds = []command.Command{unmarkCmd(command.OrderStrip, p.X, p.Y)}
	}
	return c, true
}

// StripLine is a downed person's strip mark line ("" unmarked).
func StripLine(p game.PawnView) string {
	if p.AX.StripMarked {
		return "Marked for stripping"
	}
	return ""
}

// CrateFrame reports blueprint f waiting for a crated building (its
// MovingFrom is a crate on the map).
func CrateFrame(f game.FrameView, w game.WorldView) bool {
	for _, it := range w.Items {
		if f.Moving && it.ID == f.MovingFrom && it.AX.Building != "" {
			return true
		}
	}
	return false
}
