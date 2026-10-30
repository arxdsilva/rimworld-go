// Package raid names how a raid arrives and how it fights (DEF-4, wave E
// contract). The values are saved: append only, never reorder.
package raid

// Arrival is how a raid reaches the map. wiki: Raid (walk in from the
// edge, or drop pods on the colony, at the edge or scattered); the
// arrival weights stay with DEF-4.
type Arrival uint8

const (
	WalkInEdge     Arrival = iota // on foot from one map edge
	DropPodCentre                 // pods land on the colony itself
	DropPodEdge                   // pods land together near an edge
	DropPodScatter                // pods land spread over the map
	NumArrivals
)

var arrivalNames = [NumArrivals]string{"walk in", "drop pods (centre)", "drop pods (edge)", "drop pods (scattered)"}

func (a Arrival) String() string {
	if a >= NumArrivals {
		return "walk in"
	}
	return arrivalNames[a]
}

// DropPod reports an arrival by drop pod.
func (a Arrival) DropPod() bool { return a >= DropPodCentre && a < NumArrivals }

// Strategy is what a raid does once it is on the map. The first three
// keep the order and names of game's raid plans (assault, siege, sappers),
// so the saved plan byte reads the same when DEF-4 moves to this type.
type Strategy uint8

const (
	ImmediateAssault Strategy = iota // attack at once
	Siege                            // camp out of range, dig in, bombard, then storm
	Sappers                          // tunnel through walls and rock to the colony
	StageThenAssault                 // gather near the edge, then attack together
	Steal                            // grab valuables and leave
	Burn                             // set the colony alight
	NumStrategies
)

var strategyNames = [NumStrategies]string{"assault", "siege", "sappers", "staged assault", "steal", "burn"}

func (s Strategy) String() string {
	if s >= NumStrategies {
		return "assault"
	}
	return strategyNames[s]
}
