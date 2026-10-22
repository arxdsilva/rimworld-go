// Package item is ground (and carried) resource stacks.
package item

import (
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Kind is a stack category.
type Kind uint8

const (
	Food Kind = iota // packaged survival ration
	Wood
	Stone    // stone blocks
	Berries  // raw, foraged
	Potatoes // raw, farmed
	Meat     // raw, hunted
	Meal     // cooked
	Club     // melee weapon
	Rifle    // ranged weapon
	// Corpses are butchered into meat at a butcher table.
	CorpseDustbuck
	CorpseRidgewolf
	// More farmed produce (raw).
	Corn
	Wheat
	Beans
	Lettuce
	Radishes
	Pumpkins
	// Leather comes from butchering; tailors make clothes from it.
	Leather
	// Clothes (apparel) are worn: see Slot and Insulation.
	Parka
	Tuque
	Duster
	// HerbalMedicine is dried healroot: doctors tend wounds with it.
	HerbalMedicine
	// Knife (stone) and Spear (wood) are melee weapons (SIM-015).
	Knife
	Spear
	// CorpseHuman is a dead colonist or raider (Stack.Of names who);
	// Bones are what a corpse rots down to (SIM-017).
	CorpseHuman
	Bones
	// CorpseAnimal is any animal's carcass (Stack.Animal names the
	// species, SIM-019); CorpseDustbuck and CorpseRidgewolf are legacy.
	CorpseAnimal
	// Cloth is picked from cotton; Straw comes with the wheat harvest (SIM-021).
	Cloth
	Straw
	// Carpets are crafted, then installed at an entrance: stepping on one
	// wipes off part of the dirt a colonist tracks in (filth.Catch).
	CarpetStraw
	CarpetCloth
	CarpetLeather
	CarpetWood
	numKinds
)

var kindNames = [numKinds]string{"food", "wood", "stone", "berries", "potatoes", "meat", "meal", "club", "rifle", "dustbuck corpse", "ridgewolf corpse",
	"corn", "wheat", "beans", "lettuce", "radishes", "pumpkins", "leather", "parka", "tuque", "duster", "herbal medicine",
	"knife", "spear", "human corpse", "bones", "animal corpse",
	"cloth", "straw", "straw carpet", "cloth carpet", "leather carpet", "wood carpet"}

// IsWeapon reports whether k can be equipped.
func (k Kind) IsWeapon() bool { return k == Club || k == Rifle || k == Knife || k == Spear }

// IsCorpse reports an animal carcass.
func (k Kind) IsCorpse() bool {
	return k == CorpseAnimal || k == CorpseDustbuck || k == CorpseRidgewolf || k == CorpseHuman
}

// nutrition is hunger restored per unit eaten (needs.Full = 10000).
// Produce values follow real energy density per portion, rounded for
// play: grains and dry beans are dense, leafy greens are light.
var nutrition = [numKinds]int{Food: 4000, Berries: 1500, Meat: 2200, Meal: 6000,
	Potatoes: 1600, Corn: 1700, Wheat: 2400, Beans: 2400, Lettuce: 450, Radishes: 500, Pumpkins: 800}

// preference ranks food choice: higher is eaten first.
// Raw grain, dry beans and raw potatoes are edible but unappealing.
var preference = [numKinds]int{Meal: 4, Food: 3, Berries: 2, Corn: 2, Lettuce: 2, Radishes: 2, Pumpkins: 2,
	Potatoes: 1, Beans: 1, Wheat: 1, Meat: 1}

// Nutrition is the hunger restored by eating one unit of k (0 if inedible).
func (k Kind) Nutrition() int {
	if k >= numKinds {
		return 0
	}
	return nutrition[k]
}

// Preference ranks edible kinds; higher is chosen first.
func (k Kind) Preference() int {
	if k >= numKinds {
		return 0
	}
	return preference[k]
}

// Raw reports uncooked food (eating it is unpleasant).
func (k Kind) Raw() bool { return k.IsFood() && k != Food && k != Meal }

// Foods lists every edible kind.
func Foods() []Kind {
	return []Kind{Food, Berries, Potatoes, Corn, Wheat, Beans, Lettuce, Radishes, Pumpkins, Meat, Meal}
}

func (k Kind) String() string {
	if k >= numKinds {
		return "unknown"
	}
	return kindNames[k]
}

// ParseKind returns the kind for a name.
func ParseKind(name string) (Kind, bool) {
	for i, n := range kindNames {
		if n == name {
			return Kind(i), true
		}
	}
	return 0, false
}

// IsFood reports whether k can be eaten.
func (k Kind) IsFood() bool { return k.Nutrition() > 0 }

// Slot is where a piece of apparel is worn.
type Slot uint8

const (
	NoSlot Slot = iota
	Body        // outer layer over the shirt: parka, duster
	Head
)

func (s Slot) String() string {
	switch s {
	case Body:
		return "body"
	case Head:
		return "head"
	}
	return ""
}

// apparel: slot, cold insulation and heat insulation in tenths of a
// degree C. Cold insulation lowers the bottom of the wearer's comfortable
// range; heat insulation raises the top (negative: the garment is hot to
// wear). A leather parka is serious winter gear but sweltering in summer;
// a knitted tuque keeps the head warm; a long leather duster shades the
// wearer in heat and cuts the wind a little.
var apparel = [numKinds]struct {
	slot       Slot
	cold, heat int
}{
	Parka:  {Body, 200, -30},
	Tuque:  {Head, 60, 0},
	Duster: {Body, 40, 80},
}

// IsApparel reports clothing that can be worn.
func (k Kind) IsApparel() bool { return k < numKinds && apparel[k].slot != NoSlot }

// Slot is where apparel k is worn (NoSlot for non-apparel).
func (k Kind) Slot() Slot {
	if k >= numKinds {
		return NoSlot
	}
	return apparel[k].slot
}

// Insulation is apparel k's cold and heat insulation (tenths of a degree).
func (k Kind) Insulation() (cold, heat int) {
	if k >= numKinds {
		return 0, 0
	}
	return apparel[k].cold, apparel[k].heat
}

// Apparel lists every wearable kind.
func Apparel() []Kind { return []Kind{Parka, Tuque, Duster} }

// IsStuff reports whether k is a building material.
func (k Kind) IsStuff() bool { return k == Wood || k == Stone }

// IsCarpet reports a crafted carpet (installed as a carpet building).
func (k Kind) IsCarpet() bool { return k >= CarpetStraw && k <= CarpetWood }

// Carpets lists every carpet kind, cheapest material first.
func Carpets() []Kind { return []Kind{CarpetStraw, CarpetWood, CarpetLeather, CarpetCloth} }

// StackLimit is how many items of one kind share a tile.
const StackLimit = 1000

// CarryLimit is how many items a colonist hauls in one trip.
const CarryLimit = 100

// Stack is a positive quantity of one kind at one location model.
type Stack struct {
	Kind    Kind
	Qty     int
	X, Y    int
	Carrier entity.EntityID // 0 = on ground or in a frame
	// Rot is spoilage accumulated so far, in ticks at full rot speed. The
	// stack spoils when Rot reaches Shelf().
	Rot int
	// Dried food (from a drying rack) keeps indefinitely.
	Dried bool
	// Of is the dead pawn a human corpse (or its bones) once was; DiedAt
	// the tick it died.
	Of     entity.EntityID
	DiedAt uint64
	// Animal is the species (animal.Species value) of a CorpseAnimal.
	Animal uint8
}

// Shelf is how many full-speed rot ticks the stack keeps (0 = forever).
func (s Stack) Shelf() int {
	if s.Dried {
		return 0
	}
	return s.Kind.ShelfTicks()
}

// Dryable reports whether s can go on a drying rack: perishable food that
// isn't dried or spoiled yet.
func (s Stack) Dryable() bool { return s.Kind.Dryable() && !s.Dried && !s.Rotten() }

// Dryable reports perishable food kinds (corpses are butchered, not dried).
func (k Kind) Dryable() bool { return k.IsFood() && k.Perishable() }

// dryDays is how long a rack batch takes to dry in warm air: thin meat
// strips and leaves dry in about a day, juicy fruit, sliced roots and
// squash take longer, and cooked meals are dried into hard travel cakes.
var dryDays = [numKinds]float64{Meat: 1, Meal: 1, Lettuce: 0.75, Berries: 2, Radishes: 1.5,
	Potatoes: 2, Corn: 1.5, Pumpkins: 2.5}

// DryTicks is how long a batch of k takes to dry at full speed.
func (k Kind) DryTicks() int {
	if k >= numKinds {
		return 0
	}
	return int(dryDays[k] * clock.TicksPerDay)
}

// DryRate is drying speed in permille at air temperature tenthsC: full
// speed from 20 C, slower in cool air, and a slow freeze-dry below 0 C.
func DryRate(tenthsC int) int {
	switch {
	case tenthsC >= 200:
		return 1000
	case tenthsC <= 0:
		return 250
	default:
		return 250 + 750*tenthsC/200
	}
}

// CanMerge reports whether a and b may combine on the same tile.
// Rotten corpses never merge with fresh ones, one person's body (or
// bones) never merges with another's, and animal corpses stack by species.
func CanMerge(a, b Stack) bool {
	return a.Kind == b.Kind && a.Carrier == 0 && b.Carrier == 0 && a.X == b.X && a.Y == b.Y &&
		a.Rotten() == b.Rotten() && a.Dried == b.Dried && a.Of == b.Of && a.Animal == b.Animal
}

// ConvertLegacy turns a save's former dustbuck or ridgewolf corpse into a
// CorpseAnimal of the species that took its slot (deer 0, timber wolf 1);
// ok is false (and s unchanged) for any other stack.
func ConvertLegacy(s Stack) (Stack, bool) {
	switch s.Kind {
	case CorpseDustbuck:
		s.Kind, s.Animal = CorpseAnimal, 0
	case CorpseRidgewolf:
		s.Kind, s.Animal = CorpseAnimal, 1
	default:
		return s, false
	}
	return s, true
}

// MergeRot is the spoilage of a stack of qa items with rot ra combined with
// qb items with rot rb: the quantity-weighted average.
func MergeRot(qa, ra, qb, rb int) int {
	if qa+qb <= 0 {
		return 0
	}
	return (qa*ra + qb*rb) / (qa + qb)
}

// shelfDays is how long food keeps at room temperature (10 C or warmer)
// before it spoils; 0 never spoils. Calibrated to real produce, squeezed for
// play: fresh meat and cooked meals go off in days, greens in a week, roots
// and squash keep a season, and dried grain and beans keep indefinitely.
var shelfDays = [numKinds]float64{Meat: 2, Meal: 4, CorpseDustbuck: 2.5, CorpseRidgewolf: 2.5, CorpseAnimal: 2.5, CorpseHuman: 3,
	Berries: 10, Lettuce: 6, Radishes: 14, Potatoes: 30, Corn: 40, Pumpkins: 50}

// ShelfTicks is how many full-speed rot ticks k survives (0 = never rots).
func (k Kind) ShelfTicks() int {
	if k >= numKinds {
		return 0
	}
	return int(shelfDays[k] * clock.TicksPerDay)
}

// Perishable reports whether k rots.
func (k Kind) Perishable() bool { return k.ShelfTicks() > 0 }

// Rotten reports a stack that has spoiled. Rotten food is destroyed; a
// rotten corpse lingers but can no longer be butchered.
func (s Stack) Rotten() bool {
	sh := s.Shelf()
	return sh > 0 && s.Rot >= sh
}

// FreshTicks is how many full-speed rot ticks remain before s spoils.
func (s Stack) FreshTicks() int {
	sh := s.Shelf()
	if sh == 0 || s.Rot >= sh {
		return 0
	}
	return sh - s.Rot
}

// RotRate is the rot speed in permille at air temperature tenthsC (tenths
// of a degree C): frozen food (0 C or below) doesn't rot, cool food rots
// proportionally slower, and from 10 C up it rots at full speed.
func RotRate(tenthsC int) int {
	switch {
	case tenthsC <= 0:
		return 0
	case tenthsC >= 100:
		return 1000
	default:
		return tenthsC * 10
	}
}

// Add returns qty after adding n, capped, and whether the result is still positive.
func Add(qty, n int) int {
	v := qty + n
	if v < 0 {
		return 0
	}
	if v > StackLimit {
		return StackLimit
	}
	return v
}

// Take removes n from qty. ok is false if qty would go negative.
func Take(qty, n int) (int, bool) {
	if n <= 0 || qty < n {
		return qty, false
	}
	return qty - n, true
}
