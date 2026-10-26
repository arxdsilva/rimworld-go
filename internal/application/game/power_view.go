package game

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-021 power read models (POWERSIM owns this file): the BuildingView
// power fields and WorldView.PowerNets, SolarFlare and PowerVer. Read
// only, from cached state: filling them never balances or rebuilds a net.

// PowerNetView is one power net for the overlay and the inspector: its id
// (BuildingView.PowerNet), W generated and W drawn now, the batteries'
// stored energy and capacity in Wh, and whether consumers are shed.
type PowerNetView struct {
	ID, Gen, Load, StoredWh, CapWh int
	Brownout                       bool
}

// Why a consumer has no power (BuildingView.NoPowerReason).
const (
	NoPowerNotConnected = "not connected"
	NoPowerBrownout     = "brownout"
	NoPowerSwitchedOff  = "switched off"
	NoPowerSolarFlare   = "solar flare"
)

// fillPowerBuilding fills building id's power fields (PowerRole through
// ThermoTarget; Glow comes from light.go).
func (g *Game) fillPowerBuilding(bv *BuildingView, id entity.EntityID, b construction.Building) {
	// TODO(POWERSIM)
}

// fillPowerView fills the world's power nets, the solar flare flag and
// PowerVer.
func (g *Game) fillPowerView(w *WorldView) {
	// TODO(POWERSIM)
}
