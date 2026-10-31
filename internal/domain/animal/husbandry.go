package animal

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/clock"
)

// Husbandry (FOOD-3, ENG-003 extended to food: published RimWorld wiki
// numbers, original names). Each of our species takes the numbers of the
// vanilla animal in its role: the woolhorn a muffalo's, the emberbuck a
// boomalope's, the timbergnaw an alphabeaver's, the ground sloth a
// megasloth's, the cinder rat a boomrat's, the glimmerstag a thrumbo's,
// the dire wolf a warg's, and the hive beetle, crawler and brute a
// megascarab's, spelopede's and megaspider's.

// husbandryRow is a species' published numbers.
type husbandryRow struct {
	size      int // body size, hundredths (wiki: Body size)
	wildness  int // permille (wiki: Wildness)
	revenge   int // percent a hunted animal turns on its hunter (wiki: List of animals)
	tameRev   int // percent a failed taming enrages it (wiki: List of animals)
	noLeather bool
	leather   int // leather base per body size (0: 40; wiki: Leather amount)
}

var husbandry = [numSpecies]husbandryRow{
	Deer: {size: 120, wildness: 750}, TimberWolf: {size: 85, wildness: 850, revenge: 100, tameRev: 30},
	Alpaca: {size: 100, wildness: 250}, Bison: {size: 240, wildness: 600, revenge: 10},
	Emberbuck: {size: 200, wildness: 600, revenge: 10, tameRev: 10}, Capybara: {size: 75, wildness: 750},
	Caribou: {size: 100, wildness: 750, revenge: 10}, Cow: {size: 240}, Donkey: {size: 140},
	Dromedary: {size: 210, wildness: 250}, Elephant: {size: 400, wildness: 750, revenge: 50},
	Elk: {size: 210, wildness: 750}, Gazelle: {size: 70, wildness: 750}, Goat: {size: 75},
	Horse: {size: 240, wildness: 350}, Ibex: {size: 100, wildness: 550},
	GroundSloth: {size: 400, wildness: 970, revenge: 50, tameRev: 30},
	Woolhorn:    {size: 240, wildness: 600, revenge: 10}, Pig: {size: 170},
	Rhinoceros: {size: 300, wildness: 900, revenge: 50, tameRev: 30}, Sheep: {size: 75},
	Yak: {size: 210, wildness: 200}, Timbergnaw: {size: 45, wildness: 750, revenge: 50, tameRev: 10},
	ArcticFox: {size: 55, wildness: 750}, ArcticWolf: {size: 85, wildness: 850, revenge: 100, tameRev: 30},
	CinderRat: {size: 20, wildness: 500}, Cat: {size: 32}, Chinchilla: {size: 35, wildness: 600},
	Cougar: {size: 100, wildness: 800, revenge: 50, tameRev: 30}, FennecFox: {size: 55, wildness: 750},
	GrizzlyBear: {size: 215, wildness: 800, revenge: 50, tameRev: 30}, GuineaPig: {size: 20, wildness: 600},
	Hare: {size: 20, wildness: 750}, Husky: {size: 86}, Labrador: {size: 75},
	Lynx: {size: 60, wildness: 800, revenge: 50, tameRev: 20}, Monkey: {size: 35, wildness: 600},
	Panther:   {size: 100, wildness: 800, revenge: 50, tameRev: 30},
	PolarBear: {size: 215, wildness: 850, revenge: 50, tameRev: 30}, Raccoon: {size: 40, wildness: 750},
	Rat: {size: 20, wildness: 500}, RedFox: {size: 55, wildness: 750}, Snowhare: {size: 20, wildness: 750},
	Squirrel: {size: 20, wildness: 750}, DireWolf: {size: 140, wildness: 600, revenge: 100, tameRev: 40},
	WildBoar: {size: 85, wildness: 500}, YorkshireTerrier: {size: 32},
	Cassowary: {size: 60, wildness: 800, revenge: 100, tameRev: 10}, Chicken: {size: 30, noLeather: true},
	Duck: {size: 30, noLeather: true}, Emu: {size: 60, wildness: 950, revenge: 100, tameRev: 100},
	Goose: {size: 60, wildness: 600, leather: 36}, Ostrich: {size: 100, wildness: 950, revenge: 100, tameRev: 10},
	Turkey: {size: 60, wildness: 450}, Cobra: {size: 25, wildness: 750, revenge: 50, tameRev: 10},
	Iguana: {size: 40, wildness: 500}, Tortoise: {size: 50, wildness: 750},
	HiveBeetle:  {size: 20, wildness: 200, revenge: 50, noLeather: true},
	HiveBrute:   {size: 120, wildness: 400, revenge: 50, noLeather: true},
	HiveCrawler: {size: 80, wildness: 300, revenge: 50, noLeather: true},
	Glimmerstag: {size: 400, wildness: 980, revenge: 100},
	// mechanoids: no body to butcher (they leave a wreck, mech.go)
}

// Butchering (wiki: Meat amount, Leather amount, Animals).
const (
	// meatPerSize and leatherPerSize are the yields per unit of body
	// size; HumanMeatYield and HumanLeather a human's.
	meatPerSize    = 140
	leatherPerSize = 40
	HumanMeatYield = 140
	HumanLeather   = 75
	// NotCareful is the yield of a body not carefully slaughtered
	// (permille): anything killed in a fight, by a hunter's shots or by
	// starvation. A slaughter and death of old age are careful.
	NotCareful = 660
	// Life-stage body sizes, permille of the adult's (wiki: Animals): a
	// baby bird 10 %, any other baby 20 %, a juvenile 50 %. The stages
	// are the species' juvenile and maturity ages (breeding.go).
	babyBirdSize = 100
	babySize     = 200
	juvenileSize = 500
)

// BodySize is s's adult body size in hundredths (wiki: Body size; 0 for a
// mechanoid).
func BodySize(s Species) int {
	if s >= numSpecies {
		return 0
	}
	return husbandry[s].size
}

// smallCurve is the wiki's post-process curve of the meat and leather
// amounts, in hundredths: (0,0), (5,14), (40,40), then linear.
func smallCurve(h int) int {
	switch {
	case h <= 0:
		return 0
	case h <= 500:
		return h * 14 / 5
	case h <= 4000:
		return 1400 + (h-500)*26/35
	}
	return h
}

// adultYieldH is an adult whole body's meat and leather in hundredths.
func adultYieldH(s Species) (meat, leather int) {
	if s >= numSpecies {
		return 0, 0
	}
	r := husbandry[s]
	meat = smallCurve(meatPerSize * r.size)
	if !r.noLeather {
		per := leatherPerSize
		if r.leather > 0 {
			per = r.leather
		}
		leather = smallCurve(per * r.size)
	}
	return meat, leather
}

// Yield is the meat and leather an adult whole body of s gives before
// the butcher's efficiency (wiki: Meat amount, Leather amount; a cow 336
// and 96).
func Yield(s Species) (meat, leather int) {
	m, l := adultYieldH(s)
	return (m + 50) / 100, (l + 50) / 100
}

// CorpseYield is what a body of s gives at yield permille (Stack.Yield, 0:
// whole and adult): the adult's amounts scaled, rounded to nearest.
func CorpseYield(s Species, yield int) (meat, leather int) {
	if yield <= 0 {
		yield = 1000
	}
	m, l := adultYieldH(s)
	return (m*yield/1000 + 50) / 100, (l*yield/1000 + 50) / 100
}

// StageSize is the body size of an animal of s days old, permille of its
// adult size (wiki: Animals).
func StageSize(s Species, days int) int {
	switch {
	case Adult(s, days):
		return 1000
	case Juvenile(s, days):
		return juvenileSize
	case RosterOf(s).Archetype == ArchBird:
		return babyBirdSize
	}
	return babySize
}

// PartsLeft is the share of a body's coverage still there, permille (wiki:
// Meat amount: "fraction of body parts remaining"; 1000 without a body).
func PartsLeft(b *body.Body) int {
	if b == nil {
		return 1000
	}
	left := 0
	for p := range b.PartCount() {
		if !b.PartMissing(body.Part(p)) {
			left += b.PartDef(body.Part(p)).Coverage
		}
	}
	return min(max(left, 1), 1000)
}

// CorpseYieldOf is the butcher yield permille of the body animal a leaves
// at tick now (Stack.Yield): its size at its age (through the small
// curve), the parts it still has, and NotCareful unless careful. At least
// 1, so 0 keeps meaning a whole adult (older saves).
func CorpseYieldOf(a State, now uint64, careful bool) int {
	am, _ := adultYieldH(a.Species)
	if am <= 0 {
		return 1000
	}
	size := husbandry[a.Species].size * StageSize(a.Species, AgeDays(a.Born, now, a.Species)) / 1000
	y := smallCurve(meatPerSize*size) * 1000 / am
	y = y * PartsLeft(a.Body) / 1000
	if !careful {
		y = y * NotCareful / 1000
	}
	return min(max(y, 1), 1000)
}

// ButcheryEfficiency is a butcher's efficiency at Cooking level l,
// permille (wiki: Butchery efficiency: 75 % + 2.5 % a level, at most
// 150 %).
func ButcheryEfficiency(l int) int { return min(750+25*max(l, 0), 1500) }

// MeatOf is the meat kind a body of s gives: insect meat from the hive
// insects, else (animal) meat.
func MeatOf(s Species) item.Kind {
	if RosterOf(s).Behaviour == Hive {
		return item.InsectMeat
	}
	return item.Meat
}

// MeatTag is the Stack.Animal of meat butchered from s: the species plus
// one, so 0 stays untagged meat (older saves, trade).
func MeatTag(s Species) uint8 { return uint8(s) + 1 }

// MeatSpecies is the species a meat stack was butchered from (false:
// untagged, or not animal meat).
func MeatSpecies(st item.Stack) (Species, bool) {
	if st.Kind != item.Meat || st.Animal == 0 || Species(st.Animal-1) >= numSpecies {
		return 0, false
	}
	return Species(st.Animal - 1), true
}

// MeatLabel is a meat stack's name: "cow meat" for tagged animal meat,
// else "" (the kind's own name).
func MeatLabel(st item.Stack) string {
	if s, ok := MeatSpecies(st); ok {
		return s.Label() + " meat"
	}
	return ""
}

// Taming and handling (wiki: Wildness, Minimum handling skill, Tame
// animal chance).

// Wildness is s's wildness, permille.
func Wildness(s Species) int {
	if s >= numSpecies {
		return 1000
	}
	return husbandry[s].wildness
}

// MinHandling is the Animals level a handler needs to tame an animal of
// wildness w (permille): 0 below 15 %, 1 at 15 % rising to 10 at 98 %,
// then to 14 at 99 %.
func MinHandling(w int) int {
	switch {
	case w < 150:
		return 0
	case w <= 980:
		return 1 + (w-150)*9/830
	}
	return min(10+(w-980)*4/10, 14)
}

// TameChance is a taming attempt's chance, permille, for a handler at
// Animals level l and an animal of wildness w: (4 % + 3 % a level) x 2 x
// (1 - wildness).
func TameChance(l, w int) int {
	return min(max((40+30*max(l, 0))*2*(1000-w)/1000, 0), 1000)
}

// tameFeedShare and tameFeedMax: a taming attempt feeds the animal 15 % of
// its full food, at most 0.3 nutrition (original tuning: the wiki says a
// handler brings food but publishes no amount).
const (
	tameFeedShare = 150
	tameFeedMax   = 3000
)

// TameFood is the nutrition (item.Kind.Nutrition units) one taming
// attempt of an animal of s costs.
func TameFood(s Species) int {
	full := Full * 100 / sizeMeal[RosterOf(s).Size] // nutrition of a full bar
	return min(full*tameFeedShare/1000, tameFeedMax)
}

// TameFeed reports food a handler may bring to tame an animal of diet d
// (wiki: Animal husbandry: raw food or kibble the animal eats; never hay,
// meals or drugs).
func TameFeed(d Diet, k item.Kind) bool {
	return k != item.Straw && k != item.Hay && FoodTier(k) != TierMeal && FeedValue(d, k) > 0
}

// GatherYield is a handler's animal gather yield at Animals level l,
// permille (wiki: Animal gather yield); a milking or shearing succeeds
// with this chance (at most certain) and wastes the product otherwise.
func GatherYield(l int) int {
	switch {
	case l <= 0:
		return 600
	case l < 8:
		return [...]int{600, 700, 750, 800, 850, 900, 950, 975}[l]
	case l <= 18:
		return 1000 + 10*(l-8)
	case l == 19:
		return 1120
	}
	return 1130
}

// Revenge is the percent chance a wild s turns on a colonist who hurts it
// (wiki: List of animals, hunting); TameRevenge after a failed taming.
func Revenge(s Species) int {
	if s >= numSpecies {
		return 0
	}
	return husbandry[s].revenge
}

// TameRevenge is the percent chance a failed taming enrages an s.
func TameRevenge(s Species) int {
	if s >= numSpecies {
		return 0
	}
	return husbandry[s].tameRev
}

// Products (wiki: Milk, Wool, Eggs).
type productRow struct{ days, qty int }

var milkRows = map[Species]productRow{
	Cow: {1, 14}, Yak: {1, 11}, Elk: {1, 11}, Goat: {3, 12}, Caribou: {2, 10}, Dromedary: {2, 18},
}

var woolRows = map[Species]productRow{
	Sheep: {10, 45}, Alpaca: {10, 45}, Woolhorn: {15, 120}, Bison: {15, 120}, GroundSloth: {20, 200},
}

// eggRow is a layer's interval (thousandths of a day), mean clutch
// (tenths) and fertile egg's incubation (tenths of a day).
type eggRow struct{ milli, clutch, hatch int }

var eggRows = map[Species]eggRow{
	Chicken: {1000, 10, 35}, Duck: {1000, 10, 35}, Goose: {2000, 10, 65}, Turkey: {1333, 10, 50},
	Emu: {3330, 10, 60}, Ostrich: {3330, 10, 80}, Cassowary: {3330, 10, 70},
	Cobra: {10000, 15, 45}, Iguana: {5661, 15, 35}, Tortoise: {6660, 20, 35},
}

// eggsBy is how many eggs a layer of s with phase id has laid by the end
// of day (its lays so far times the mean clutch, whole eggs).
func eggsBy(s Species, id uint64, day int) int {
	r, ok := eggRows[s]
	if !ok || day < 0 {
		return 0
	}
	lays := (int64(day) + int64(id%uint64(max(r.milli/1000, 1)))) * 1000 / int64(r.milli)
	return int(lays * int64(r.clutch) / 10)
}

// EggsToday is how many eggs a fed layer of s with id lays on day (wiki:
// Eggs: the species' interval and mean clutch, spread over the days).
func EggsToday(s Species, id uint64, day int) int {
	return max(eggsBy(s, id, day)-eggsBy(s, id, day-1), 0)
}

// HatchTicks is how long a fertile egg of s incubates (wiki: the species'
// fertilized egg).
func HatchTicks(s Species) int {
	return eggRows[s].hatch * clock.TicksPerDay / 10
}

// Butcher is what butchering body st gives before the butcher's
// efficiency (wiki: Meat amount, Leather amount, Butcher table): its meat
// as a stack (kind, count, species tag) and its leather. A human body
// gives HumanMeatYield human meat and no leather (no human leather kind
// yet); a hive insect insect meat; any other animal its species' meat.
func Butcher(st item.Stack) (meat item.Stack, leather int) {
	if st.Kind == item.CorpseHuman {
		return item.Stack{Kind: item.HumanMeat, Qty: HumanMeatYield}, 0
	}
	s := Species(st.Animal)
	m, l := CorpseYield(s, int(st.Yield))
	meat = item.Stack{Kind: MeatOf(s), Qty: m}
	if meat.Kind == item.Meat {
		meat.Animal = MeatTag(s)
	}
	return meat, l
}
