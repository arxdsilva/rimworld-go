package world

import "rimworld-go/internal/application/game"

// The escape ship's art keys (AX-8, AR4's starship family): the beam,
// the casket and the engine are faced; the core, the reactor and the
// sensor cluster face south only. A casket with a sleeper draws its
// occupied sprite.

// ShipSprite is a ship part kind's sprite key ("" for any other kind).
func ShipSprite(kind, facing string) string {
	switch kind {
	case "ship structural beam":
		return "structure/ship_beam_" + FacingOr(facing)
	case "ship stasis casket":
		return "structure/ship_casket_" + FacingOr(facing)
	case "ship engine":
		return "structure/ship_engine_" + FacingOr(facing)
	case "ship computer core":
		return "structure/ship_core"
	case "ship reactor":
		return "structure/ship_reactor"
	case "ship sensor cluster":
		return "structure/ship_sensor"
	}
	return ""
}

// ShipKey is ship part b's sprite: a casket with a sleeper is occupied.
func ShipKey(b game.BuildingView) string {
	if b.Kind == "ship stasis casket" && b.AX.Sleeper != "" {
		return "structure/ship_casket_occupied_" + FacingOr(b.Facing)
	}
	return ShipSprite(b.Kind, b.Facing)
}
