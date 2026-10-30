package inspector

import (
	"fmt"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// A drafted colonist's combat commands (DEF-4): Fire at will and Hold
// position toggles, and Throw grenade (shown while grenades are carried),
// which enters the map targeting mode (orders/mortar_aim.go). All read
// PawnView; the toggles send typed commands, the throw hands the aim to the
// shell. None has a hotkey.

// Button ids (journeys press them by these).
const (
	FireAtWillID   = "cmd.pawn.fire.at.will"
	HoldPositionID = "cmd.pawn.hold.position"
	ThrowGrenadeID = "cmd.pawn.grenade"
)

// combatCommands are the drafted colonist's fire, hold and grenade buttons.
func (a Pawn) combatCommands(p game.PawnView) []Button {
	if !p.Drafted || p.Status != "alive" {
		return nil
	}
	id := p.ID
	out := []Button{
		{ID: FireAtWillID, Label: "Fire at will", Icon: "order_attack", Toggle: true, On: p.FireAtWill,
			Desc: "Shoot at hostiles in range without being ordered to. Turn it off to fire only on your orders.",
			Cmds: []command.Command{{Kind: command.SetFireAtWill, Pawn: uint64(id), Enabled: !p.FireAtWill}}},
		{ID: HoldPositionID, Label: "Hold position", Icon: "draft", Toggle: true, On: p.HoldPosition,
			Desc: "Stay on this tile and do not chase targets.",
			Cmds: []command.Command{{Kind: command.SetHoldPosition, Pawn: uint64(id), Enabled: !p.HoldPosition}}},
	}
	if p.Grenades > 0 {
		out = append(out, Button{ID: ThrowGrenadeID, Label: "Throw grenade", Icon: "order_attack",
			Desc: fmt.Sprintf("Click a tile on the map to throw a grenade there (%d carried). Esc or a right click cancels.", p.Grenades),
			Run:  func() { a.shell.Do(ports.AimGrenade{Pawn: id}) }})
	}
	return out
}
