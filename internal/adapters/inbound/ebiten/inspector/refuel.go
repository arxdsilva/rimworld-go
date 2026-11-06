package inspector

import (
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/command"
)

// AutoRefuelID is the Auto refuel toggle's id (REF-1).
const AutoRefuelID = "bldg.autorefuel"

// autoRefuelCommand is the Auto refuel toggle of a fuel burner: SetAutoRefuel
// for the building. The icon is the fuel it burns.
func autoRefuelCommand(tr lang.Tr, b game.BuildingView) Button {
	icon := "wood"
	if k, ok := construction.Parse(b.Kind); ok && (k == construction.ChemfuelGenerator || k == construction.PodLauncher) {
		icon = "chemfuel"
	}
	return Button{ID: AutoRefuelID, Label: tr.T("Auto refuel"), Icon: icon, Toggle: true, On: b.AutoRefuel,
		Desc: tr.T("Haulers keep it fuelled. Off: only a colonist you order refuels it."),
		Cmds: []command.Command{{Kind: command.SetAutoRefuel, Target: uint64(b.ID), Enabled: !b.AutoRefuel}}}
}
