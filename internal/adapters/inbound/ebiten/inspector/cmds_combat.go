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

// Button ids of the CBT-5 gear and stance controls (journeys press them).
const (
	SmokepopID = "cmd.pawn.smokepop"
	ThreatID   = "cmd.pawn.threat"
)

// threatResponses are PawnView.ThreatResponse's values in command order
// (SetThreatResponse X: 0 default, 1 flee, 2 attack, 3 ignore) with what
// each does.
var threatResponses = [...]struct{ name, tip string }{
	{"default", "Fight when armed and flee when not, by the colonist's own judgement."},
	{"flee", "Run from threats."},
	{"attack", "Move to attack threats, armed or not."},
	{"ignore", "Carry on with work and ignore threats."},
}

// threatCommand is the colonist's Threat response button: each click moves
// to the next response (default, flee, attack, ignore). Colonists only.
func threatCommand(p game.PawnView) []Button {
	if p.Hostile || p.Prisoner || p.Visitor || p.Status != "alive" {
		return nil
	}
	cur := 0
	for i, r := range threatResponses {
		if r.name == p.ThreatResponse {
			cur = i
		}
	}
	next := (cur + 1) % len(threatResponses)
	return []Button{{ID: ThreatID, Label: "Threat: " + threatResponses[cur].name, Icon: "order_attack",
		Desc: "Threat response: " + threatResponses[cur].tip + "\nClick for " + threatResponses[next].name + ": " + threatResponses[next].tip,
		Cmds: []command.Command{{Kind: command.SetThreatResponse, Pawn: uint64(p.ID), X: next}}}}
}

// smokepopCommand is Use smokepop, for a colonist wearing a pack: the
// colonist sets it off where it stands.
func smokepopCommand(p game.PawnView) []Button {
	if p.Smokepops <= 0 || p.Hostile || p.Status != "alive" {
		return nil
	}
	return []Button{{ID: SmokepopID, Label: "Use smokepop", Icon: "order_attack",
		Desc: fmt.Sprintf("Smokepop\nPop a smokepop pack where this colonist stands: the cloud blocks shots (%d left).", p.Smokepops),
		Cmds: []command.Command{{Kind: command.PopSmokepop, Pawn: uint64(p.ID)}}}}
}
