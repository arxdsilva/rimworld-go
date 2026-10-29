package inspector

import (
	"fmt"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// Defence and workbench extras on the building inspector (wave D: DEF-3,
// PRD-3). A door can be held open, a turret told to hold fire, a mortar
// given a forced target; benches show their linked facilities and work
// speed. Everything is read from BuildingView and sent as a typed command.

// Building kinds with the defence buttons (construction names).
const (
	mortarKind    = "mortar"
	doorKind      = "door"
	autodoorKind  = "powered door"
	turretKind    = "sentry turret"
	autocannon    = "autocannon"
	slugTurret    = "slug turret"
	pctPerPermile = 10
)

// Button ids (journeys press them by these).
const (
	DoorHoldID   = "cmd.door.hold"
	TurretHoldID = "cmd.turret.hold"
	MortarAimID  = "cmd.mortar.target"
	MortarClrID  = "cmd.mortar.clear"
)

func isDoorBldg(b game.BuildingView) bool { return b.Kind == doorKind || b.Kind == autodoorKind }

func isTurretBldg(b game.BuildingView) bool {
	return b.Kind == turretKind || b.Kind == autocannon || b.Kind == slugTurret
}

// defenceCommands are a door's, turret's or mortar's own buttons.
func (a Building) defenceCommands(b game.BuildingView) []Button {
	var out []Button
	switch {
	case isDoorBldg(b):
		out = append(out, Button{ID: DoorHoldID, Label: "Hold open", Toggle: true, On: b.HoldOpen,
			Desc: "Keep this door open so nobody waits at it. Turn it off to let it close again.",
			Cmds: []command.Command{{Kind: command.SetDoorHoldOpen, X: b.X, Y: b.Y, Enabled: !b.HoldOpen}}})
	case isTurretBldg(b):
		out = append(out, Button{ID: TurretHoldID, Label: "Hold fire", Toggle: true, On: b.HoldFire,
			Desc: "Stop this turret shooting until you turn it off again.",
			Cmds: []command.Command{{Kind: command.SetTurretHold, X: b.X, Y: b.Y, Enabled: !b.HoldFire}}})
	case b.Kind == mortarKind:
		bx, by, id := b.X, b.Y, b.ID
		out = append(out, Button{ID: MortarAimID, Label: "Set target", Icon: "order_attack",
			Desc: "Click a tile on the map for this mortar to shell. Esc or a right click cancels.",
			Run:  func() { a.shell.Do(ports.AimMortar{ID: id, X: bx, Y: by}) }})
		clr := Button{ID: MortarClrID, Label: "Clear target", Icon: "order_cancel",
			Desc: "Drop the forced target; the mortar picks its own again.",
			Cmds: []command.Command{{Kind: command.ClearMortarTarget, X: b.X, Y: b.Y}}}
		if !b.HasTarget {
			clr.Disabled, clr.Reason = true, "no target set"
		}
		out = append(out, clr)
	}
	return out
}

// defenceLines are the barrel, shells and target lines, and a bench's
// facility and work speed lines.
func defenceLines(b game.BuildingView) []string {
	var out []string
	if isTurretBldg(b) || b.Kind == mortarKind {
		if b.Barrel > 0 {
			out = append(out, fmt.Sprintf("Barrel  %d%%", b.Barrel/pctPerPermile))
		}
		if b.HoldFire {
			out = append(out, "Holding fire")
		}
	}
	if b.Kind == mortarKind {
		out = append(out, fmt.Sprintf("Shells loaded  %d", b.Shells))
		if b.HasTarget {
			out = append(out, fmt.Sprintf("Target  (%d, %d)", b.TargetX, b.TargetY))
		} else {
			out = append(out, "No target set")
		}
	}
	if isDoorBldg(b) && b.HoldOpen {
		out = append(out, "Held open")
	}
	for _, f := range b.Linked {
		out = append(out, "Linked  "+f)
	}
	if b.WorkSpeed > 0 {
		out = append(out, fmt.Sprintf("Work speed  %d%%", b.WorkSpeed/pctPerPermile))
	}
	return out
}
