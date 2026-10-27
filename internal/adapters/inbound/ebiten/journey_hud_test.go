package ebiten

// journey_hud_test.go: HUD journeys (QA-001). The debts below are what this
// file still owes the guard (TestEveryInputActionHasAJourney); delete a
// name here once a journey in this file covers it.
var _ = owe("journey_hud_test.go",
	[]string{"SetForbidden", "SetHaulMark", "SetPowerWanted", "SetThermostat", "SetHomeAuto", "DismissLetter"},
	[]string{"view.screenshot", "thing.forbid", "thing.power", "overlay.roof", "overlay.power", "overlay.rooms", "overlay.detail", "overlay.search"})
