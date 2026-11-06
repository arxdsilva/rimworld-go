package hud

import (
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/application/game"
)

// The alert builders take the catalog (the HUD builds them with its shell);
// these forms build in English, for the tests and tools that have no shell.
var en lang.English

// BuildAlerts is BuildAlertsIn in English.
func BuildAlerts(pawns []game.PawnView, w game.WorldView) []Alert {
	return BuildAlertsIn(en, pawns, w)
}

func raiseAlerts(pawns []game.PawnView, w game.WorldView) []Alert {
	return raiseAlertsIn(en, pawns, w)
}

// FireAlert is FireAlertIn in English.
func FireAlert(w game.WorldView) (Alert, bool) { return FireAlertIn(en, w) }

// WaitingFramesAlert is WaitingFramesAlertIn in English.
func WaitingFramesAlert(w game.WorldView) (Alert, bool) { return WaitingFramesAlertIn(en, w) }

// AnimalAlerts is AnimalAlertsIn in English.
func AnimalAlerts(w game.WorldView) []Alert { return AnimalAlertsIn(en, w) }

// AllyAlerts is AllyAlertsIn in English.
func AllyAlerts(groups []game.AllyGroupView) []Alert { return AllyAlertsIn(en, groups) }

// AllyAlertsFrom is AllyAlertsFromIn in English.
func AllyAlertsFrom(src any) []Alert { return AllyAlertsFromIn(en, src) }

// TradeAlertsFrom is TradeAlertsFromIn in English.
func TradeAlertsFrom(src any) []Alert { return TradeAlertsFromIn(en, src) }

// FishDyingAlert is FishDyingAlertIn in English.
func FishDyingAlert(w game.WorldView) (Alert, bool) { return FishDyingAlertIn(en, w) }

// NotRecoveringAlert is NotRecoveringAlertIn in English.
func NotRecoveringAlert(colonists []game.PawnView) (Alert, bool) {
	return NotRecoveringAlertIn(en, colonists)
}

// NeedWarmClothesAlert is NeedWarmClothesAlertIn in English.
func NeedWarmClothesAlert(pawns []game.PawnView, w game.WorldView) (Alert, bool) {
	return NeedWarmClothesAlertIn(en, pawns, w)
}

// ColdAnimalAlert is ColdAnimalAlertIn in English.
func ColdAnimalAlert(w game.WorldView) (Alert, bool) { return ColdAnimalAlertIn(en, w) }

// AlertRowLabel is AlertRowLabelIn in English.
func AlertRowLabel(r AlertRow) string { return AlertRowLabelIn(en, r) }
