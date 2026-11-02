package travel

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/world/caravan"
)

// humanKg is a person's mass aboard (wiki: Human, 60 kg).
const humanKg = caravan.HumanMassKg

// animalKg is an animal's mass aboard (caravan.AnimalMassKg by body
// class, original tuning).
func animalKg(s animal.Species) int { return caravan.AnimalMassKg(animal.RosterOf(s).Size) }

// animalValue is an animal's worth as a gift (original tuning by body
// class: the trade screens price no live animals).
func animalValue(s animal.Species) int {
	switch animal.RosterOf(s).Size {
	case animal.SizeXS:
		return 40
	case animal.SizeS:
		return 90
	case animal.SizeM:
		return 220
	case animal.SizeL:
		return 450
	}
	return 800
}
