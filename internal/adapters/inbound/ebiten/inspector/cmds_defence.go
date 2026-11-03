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
	shelfKind     = "shelf"
	doorKind      = "door"
	autodoorKind  = "powered door"
	turretKind    = "sentry turret"
	autocannon    = "autocannon"
	slugTurret    = "slug turret"
	pctPerPermile = 10
)

// Button ids (journeys press them by these).
const (
	DoorHoldID    = "cmd.door.hold"
	TurretHoldID  = "cmd.turret.hold"
	MortarAimID   = "cmd.mortar.target"
	MortarClrID   = "cmd.mortar.clear"
	DoorClaimID   = "cmd.door.claim"
	TurretClaimID = "cmd.turret.claim"
)

func isDoorBldg(b game.BuildingView) bool {
	return b.Kind == doorKind || b.Kind == autodoorKind || b.Kind == ornateDoorKind // A2: not the animal flap
}

func isTurretBldg(b game.BuildingView) bool {
	return b.Kind == turretKind || b.Kind == autocannon || b.Kind == slugTurret || b.Kind == foamTurretKind // AX-5
}

// enemyCommands are a settlement's door's or turret's one button (MAP-17,
// MAP-21): Claim, once no defender stands (the same count the Claim order
// makes: raid.Defending).
func enemyCommands(b game.BuildingView, hostiles []game.PawnView) []Button {
	claim := Button{ID: DoorClaimID, Label: "Claim", Icon: "order_claim",
		Desc: "Make this door the colony's, so colonists can open it.",
		Cmds: []command.Command{{Kind: command.DesignateClaim, X: b.X, Y: b.Y, X2: b.X, Y2: b.Y, Enabled: true}}}
	if !isDoorBldg(b) {
		claim.ID, claim.Desc = TurretClaimID, "Make this turret the colony's: it stops firing on us and fires on our enemies, with no grid."
	}
	for _, h := range hostiles {
		if h.Status == "alive" && !h.Prisoner && !h.Fleeing {
			claim.Disabled, claim.Reason = true, "defeat the defenders first"
			break
		}
	}
	return []Button{claim}
}

// defenceCommands are a door's, turret's or mortar's own buttons.
func (a Building) defenceCommands(b game.BuildingView) []Button {
	var out []Button
	switch {
	case isDoorBldg(b):
		out = append(out, Button{ID: DoorHoldID, Label: "Hold open", Toggle: true, On: b.HoldOpen,
			Desc: "Keep this door open so nobody waits at it. Turn it off to let it close again.",
			Cmds: []command.Command{{Kind: command.SetDoorHoldOpen, X: b.X, Y: b.Y, Enabled: !b.HoldOpen}}})
	case isTurretBldg(b) && !b.Hostile: // MAP-13: a settlement's turret takes no orders
		out = append(out, Button{ID: TurretHoldID, Label: "Hold fire", Toggle: true, On: b.HoldFire,
			Desc: "Stop this turret shooting until you turn it off again.",
			Cmds: []command.Command{{Kind: command.SetTurretHold, X: b.X, Y: b.Y, Enabled: !b.HoldFire}}})
	case storageKinds[b.Kind]:
		out = append(out, a.shelfStorageCommand(b)...)
	case b.Kind == mortarKind || b.Kind == rocketBatteryKind: // AX-5: the battery is aimed like a mortar
		bx, by, id := b.X, b.Y, b.ID
		aimDesc, clrDesc := "Click a tile on the map for this mortar to shell. Esc or a right click cancels.",
			"Drop the forced target; the mortar picks its own again."
		if b.Kind == rocketBatteryKind {
			aimDesc, clrDesc = "Click a tile for the next volley; a drafted colonist mans the battery and fires it once. Esc or a right click cancels.",
				"Drop the target: the battery holds its rockets."
		}
		out = append(out, Button{ID: MortarAimID, Label: "Set target", Icon: "order_attack", Desc: aimDesc,
			Run: func() { a.shell.Do(ports.AimMortar{ID: id, X: bx, Y: by}) }})
		clr := Button{ID: MortarClrID, Label: "Clear target", Icon: "order_cancel", Desc: clrDesc,
			Cmds: []command.Command{{Kind: command.ClearMortarTarget, X: b.X, Y: b.Y}}}
		if !b.HasTarget {
			clr.Disabled, clr.Reason = true, "no target set"
		}
		out = append(out, clr)
	}
	return out
}

// HostileTurretLine is what a settlement's turret's pane says (MAP-13): it
// is no one's to run, so none of the barrel, power or hold lines apply.
const HostileTurretLine = "Hostile: it fires on us. Destroy it, or claim it once the defenders are beaten."

// HostileTurretSpentLine is the same pane once its barrel is spent (MAP-20).
const HostileTurretSpentLine = "Out of ammo: it can't fire. Claimed, a hauler can reload it."

// OffGridLine and SparkingLine are a claimed turret's and a turret's lit
// fuse's lines (MAP-21).
const (
	OffGridLine  = "Claimed: runs on its own supply. Moved, it needs power."
	SparkingLine = "Sparking: it will blow up!"
)

// EnemyDoorLine is what a settlement's door's pane says (MAP-17).
const EnemyDoorLine = "Enemy door: colonists can't pass. Break it, or claim it once the defenders are beaten."

// defenceLines are the barrel, shells and target lines, and a bench's
// facility and work speed lines.
func defenceLines(b game.BuildingView) []string {
	var out []string
	if b.Sparking {
		out = append(out, SparkingLine)
	}
	if b.Hostile {
		switch {
		case isDoorBldg(b):
			return append(out, EnemyDoorLine)
		case b.Barrel == 0:
			return append(out, HostileTurretSpentLine)
		}
		return append(out, HostileTurretLine)
	}
	if b.OffGrid {
		out = append(out, OffGridLine)
	}
	if isTurretBldg(b) || b.Kind == mortarKind {
		if b.Barrel > 0 {
			out = append(out, fmt.Sprintf("Barrel  %d%%", b.Barrel/pctPerPermile))
		}
		if b.HoldFire {
			out = append(out, "Holding fire")
		}
	}
	if b.Kind == mortarKind || b.Kind == rocketBatteryKind {
		if b.Kind == mortarKind {
			out = append(out, fmt.Sprintf("Shells loaded  %d", b.Shells))
		}
		if b.HasTarget {
			out = append(out, fmt.Sprintf("Target  (%d, %d)", b.TargetX, b.TargetY))
		} else {
			out = append(out, "No target set")
		}
	}
	out = append(out, ax5Lines(b)...) // ax5.go (AX-5)
	out = append(out, ax4Lines(b)...) // ax4.go (AX-4)
	out = append(out, ax6Lines(b)...) // ax6.go (AX-6)
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
