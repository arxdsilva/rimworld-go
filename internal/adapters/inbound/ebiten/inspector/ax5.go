package inspector

import (
	"fmt"

	"rimworld-go/internal/application/game"
)

// AX-5 fire foam and security lines on the building inspector, read from
// BuildingView.AX (game/axview): a foam turret's or rocket battery's
// charges, and whether a foam popper or buried charge is armed or already
// sparking.

// AX-5 building kinds (construction names).
const (
	foamTurretKind    = "foam turret"
	rocketBatteryKind = "rocket battery"
	foamPopperKind    = "foam popper"
)

// ax5Armed are the kinds that show whether they are armed: the foam
// popper and every buried charge.
var ax5Armed = map[string]bool{foamPopperKind: true, "blast charge": true, "fire charge": true, "pulse charge": true,
	"smoke charge": true, "foam charge": true, "warhead charge": true}

// ax5Lines are a chemfuel turret's load and a popper's or charge's state.
func ax5Lines(b game.BuildingView) []string {
	switch {
	case b.Kind == foamTurretKind && b.AX.ChargeCap > 0:
		return []string{fmt.Sprintf("Foam charges  %d / %d", b.AX.Charges, b.AX.ChargeCap), chemfuelNote(b)}
	case b.Kind == rocketBatteryKind && b.AX.ChargeCap > 0:
		if b.AX.Charges < b.AX.ChargeCap {
			return []string{fmt.Sprintf("Rockets  %d / %d", b.AX.Charges, b.AX.ChargeCap), "Needs a full reload of chemfuel to fire"}
		}
		return []string{fmt.Sprintf("Rockets  %d / %d", b.AX.Charges, b.AX.ChargeCap)}
	case ax5Armed[b.Kind]:
		if b.AX.Armed {
			return []string{"Armed"}
		}
		return []string{"Sparking: about to go off"}
	}
	return nil
}

// chemfuelNote is the refill line under a foam turret's charges.
func chemfuelNote(b game.BuildingView) string {
	if b.AX.Charges == 0 {
		return "Out of foam: waiting for chemfuel"
	}
	return "Refilled with chemfuel by colonists"
}
