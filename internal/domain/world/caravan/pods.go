package caravan

import "rimworld-go/internal/domain/animal"

// Transport pods (AX-9, ENG-003: wiki Transport pod, Pod launcher,
// Human): what a pod lifts, how far its launcher's chemfuel takes it and
// how fast it flies. The flight speed and the animal masses are original
// tuning (the wiki gives neither in a usable form).
const (
	// PodCapacityKg is what one pod lifts (wiki: 150 kg).
	PodCapacityKg = 150
	// PodMaxRange is the farthest a pod flies, in world tiles (wiki: 66).
	PodMaxRange = 66
	// LauncherFuelCap is a pod launcher's chemfuel tank (wiki: 150).
	LauncherFuelCap = 150
	// PodFuelQuarters is the chemfuel a pod burns per tile, in quarters
	// (wiki: 2.25 a tile a pod).
	PodFuelQuarters = 9
	// HumanMassKg is a person's mass aboard (wiki: Human, 60 kg).
	HumanMassKg = 60
	// PodTicksPerTile is a pod's flight time per tile (original: about
	// ten tiles an hour; the wiki gives no figure).
	PodTicksPerTile = 250
)

// AnimalMassKg is an animal's mass aboard by its body class (original,
// close to the wiki's figures for a hare, a dog, a deer, a muffalo and an
// elephant-sized beast).
func AnimalMassKg(s animal.Size) int {
	switch s {
	case animal.SizeXS:
		return 4
	case animal.SizeS:
		return 15
	case animal.SizeM:
		return 45
	case animal.SizeL:
		return 120
	}
	return 250
}

// PodRange is how far pods tiles fly on fuel chemfuel per pod, capped at
// PodMaxRange (fuel*4/9: 150 chemfuel reaches 66 tiles).
func PodRange(fuel, pods int) int {
	if pods <= 0 || fuel <= 0 {
		return 0
	}
	return min(PodMaxRange, fuel*4/(PodFuelQuarters*pods))
}

// PodFuel is the chemfuel pods pods burn flying tiles tiles (rounded up).
func PodFuel(tiles, pods int) int {
	return (max(tiles, 0)*pods*PodFuelQuarters + 3) / 4
}
