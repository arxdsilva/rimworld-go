package animal

import "rimworld-go/internal/domain/sim/clock"

// Life, breeding and products (SIM-026, plan part 4). All numbers are our
// own.

const (
	// MateFood: both partners need at least this much food to mate.
	MateFood = 5000
	// MateRadius: a male looks this far (tiles, Chebyshev) for a female.
	MateRadius = 12
	// MateChance is the percent chance of the once-a-day check.
	MateChance = 35
	// MateTicks is how long a mating takes once he stands beside her;
	// MateWalkLimit how long he has to get there before giving up.
	MateTicks     = 200
	MateWalkLimit = 1500
	// MatedEggs: an egg layer that mated lays this many fertile eggs (an
	// egg every EggDays); a plain layer lays PlainMatedEggs eggs of which
	// every third is fertile (FertileEgg reports which).
	MatedEggs      = 3
	PlainMatedEggs = 9
	// LayFood: no eggs below this much food.
	LayFood = 3000
	// ProductFull is a full product (milk, wool, ember oil), permille; it
	// fills at the full rate from ProductFedFood, at half below it and not
	// at all when the animal starves (Food 0).
	ProductFull    = 1000
	ProductFedFood = 2500
	// CollectWork is a handler's milking or shearing time.
	CollectWork = 200
	// OldAgeFrom: from this percent of its lifespan an animal may die of
	// old age, a daily OldAgeChance roll; the chance reaches OldAgeMax
	// permille at the full lifespan.
	OldAgeFrom = 80
	OldAgeMax  = 100
	// Incubation (tenths of a degree C): a fertile egg incubates while
	// HatchMin <= temperature <= HatchMax and dies (a plain egg) outside
	// it (wiki: Eggs, 0-50 C).
	HatchMin = 0
	HatchMax = 500
	// FollowNear: a young animal keeps within this many tiles of its
	// mother; MotherRadius is how far it looks for her.
	FollowNear   = 2
	MotherRadius = 16
	// YoungMeal: a young animal's meal counts this many times over (it
	// eats half portions: the same food fills it twice as far).
	YoungMeal = 2
)

// OldAgeChance is the permille chance an animal of s, days old, dies of
// old age today: 0 before OldAgeFrom percent of its lifespan, rising
// linearly to OldAgeMax at the lifespan (and on past it, capped at 1000).
func OldAgeChance(s Species, days int) int {
	l := LifeOf(s).LifeDays
	from := l * OldAgeFrom / 100
	if l <= 0 || days < from {
		return 0
	}
	span := max(l-from, 1)
	return min(1000, (days-from)*OldAgeMax/span)
}

// Layer reports a species that lays eggs.
func Layer(s Species) bool { return s < numSpecies && layers[s] }

// Breeds reports a species that mates and has young (no hive insects or
// legendaries).
func Breeds(s Species) bool { return !NeedsFree(s) }

// Produces reports an animal that grows its species' product: adults
// only, and only females give milk.
func Produces(s Species, female, adult bool) bool {
	p := EcologyOf(s).Product
	return p != NoProduct && adult && (p != Milk || female)
}

// ProductPerDay is the permille a producing animal's product fills per
// day at food f: 1000/ProductDays when fed, half when hungry, a quarter
// when ravenously hungry (below half of ProductFedFood), nothing when
// malnourished (wiki: Saturation: 100 / 50 / 25 / 0 %).
func ProductPerDay(s Species, f int) int {
	d := LifeOf(s).ProductDays
	switch {
	case d <= 0 || f <= 0:
		return 0
	case f < ProductFedFood/2:
		return ProductFull / d / 4
	case f < ProductFedFood:
		return ProductFull / d / 2
	}
	return ProductFull / d
}

// MatedEggsOf is the egg count one mating is good for.
func MatedEggsOf(s Species) int {
	if EcologyOf(s).Eggs == PlainEggs {
		return PlainMatedEggs
	}
	return MatedEggs
}

// FertileEgg reports whether the next egg of a layer of s with mated
// eggs still to lay is fertile: every one for a fertile-egg layer, every
// third for a plain layer (the 3rd, 6th and 9th of a mating).
func FertileEgg(s Species, mated int) bool {
	if mated <= 0 {
		return false
	}
	if EcologyOf(s).Eggs == PlainEggs {
		return mated%3 == 1
	}
	return true
}

// LaysToday reports a layer of s that lays on day (EggsToday, FOOD-3:
// the wiki interval of its species, phase by id).
func LaysToday(s Species, id uint64, day int) bool { return EggsToday(s, id, day) > 0 }

// GestTicks is a live bearer's pregnancy (wiki: the species' gestation
// period; a layer's 0 never comes up, it lays eggs instead).
func GestTicks(s Species) uint64 { return uint64(LifeOf(s).GestCenti) * clock.TicksPerDay / 100 }

// awakeHours is how many hours a day every animal is up (SleepHours
// sleeps the other 8).
const awakeHours = 16

// mix scrambles an id with a salt (a fixed odd multiplier): the per-animal
// hour of the daily checks, the same every day and after a load.
func mix(id, salt uint64) uint64 {
	x := (id + salt) * 0x9E3779B97F4A7C15
	return x ^ x>>29
}

// MateTick is the tick of the day (0..TicksPerDay) an animal of s with id
// makes its daily mating check: an hour it is awake, its own minute.
func MateTick(s Species, id uint64) uint64 { return awakeTick(s, mix(id, 0x51)) }

// LayTick is the tick of the day a layer lays.
func LayTick(s Species, id uint64) uint64 { return awakeTick(s, mix(id, 0xE6)) }

func awakeTick(s Species, h uint64) uint64 {
	start := uint64(5) // diurnal animals are up 05-21
	if s < numSpecies && nocturnal[s] {
		start = 17 // nocturnal ones 17-09
	}
	hour := (start + h%awakeHours) % 24
	return hour*clock.TicksPerHour + (h>>8)%clock.TicksPerHour
}

// nocturnal and layers are EcologyOf's flags as tables (read every tick).
var nocturnal, layers = func() (n, l [numSpecies]bool) {
	for i := range n {
		e := EcologyOf(Species(i))
		n[i], l[i] = e.Nocturnal, e.Eggs != NoEggs
	}
	return n, l
}()

// youngWords is what the young of a species are called ("" : the
// archetype's word, Stage).
var youngWords = map[Species]string{
	Goat: "kid", Ibex: "kid", Sheep: "lamb", Alpaca: "cria",
	Cow: "calf", Bison: "calf", Yak: "calf", Elk: "calf", Caribou: "calf", Elephant: "calf",
	Rhinoceros: "calf", Dromedary: "calf", Woolhorn: "calf", GroundSloth: "pup",
	Deer: "fawn", Gazelle: "fawn", Emberbuck: "fawn", Glimmerstag: "fawn",
	Horse: "foal", Donkey: "foal", Pig: "piglet", WildBoar: "piglet",
	TimberWolf: "pup", ArcticWolf: "pup", DireWolf: "pup", Husky: "pup", Labrador: "pup",
	YorkshireTerrier: "pup", ArcticFox: "kit", RedFox: "kit", FennecFox: "kit",
	Cat: "kitten", Cougar: "cub", Lynx: "kitten", Panther: "cub",
	GrizzlyBear: "cub", PolarBear: "cub", Hare: "leveret", Snowhare: "leveret",
	Chicken: "chick", Duck: "duckling", Goose: "gosling", Turkey: "poult",
	Cobra: "hatchling", Iguana: "hatchling", Tortoise: "hatchling",
}

// YoungWord is one young of s ("kid" for a goat); default the body
// plan's (Stage): "chick", "hatchling", "young".
func YoungWord(s Species) string {
	if w, ok := youngWords[s]; ok {
		return w
	}
	return Stage(s, false)
}
