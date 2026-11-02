package world

import "rimworld-go/internal/application/game"

// The travel buildings' art keys (AX-9, AR4's transit family): the stasis
// casket, the pod launcher and the mineral scanner are faced; the
// transport pod and the hitching spot's marking face south only. A casket
// with a sleeper draws its occupied sprite (its glow layer lights it).

// Travel kinds (construction.go names).
const (
	StasisCasketKind = "stasis casket"
	TransportPodKind = "transport pod"
	PodLauncherKind  = "pod launcher"
)

// TravelSprite is a travel kind's sprite key ("" for any other kind).
func TravelSprite(kind, facing string) string {
	switch kind {
	case StasisCasketKind:
		return "furniture/stasis_casket_" + FacingOr(facing)
	case TransportPodKind:
		return "structure/transport_pod"
	case PodLauncherKind:
		return "structure/pod_launcher_" + FacingOr(facing)
	case "long-range mineral scanner":
		return "power/mineral_scanner_" + FacingOr(facing)
	case "caravan hitching spot":
		return "overlay/hitching_spot"
	}
	return ""
}

// TravelKey is travel building b's sprite: a casket with a sleeper is
// occupied.
func TravelKey(b game.BuildingView) string {
	if b.Kind == StasisCasketKind && b.AX.SleeperID != 0 {
		return "furniture/stasis_casket_occupied_" + FacingOr(b.Facing)
	}
	return TravelSprite(b.Kind, b.Facing)
}
