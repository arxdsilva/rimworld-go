package animal

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
)

// Ecology (SIM-024..026): what a species eats, where it lives wild, when it
// sleeps, and what it gives. Biome membership follows where each real
// animal lives (our originals follow their design); every number is our
// own formula over size, not a copied table.

// Diet is what an animal eats.
type Diet uint8

const (
	Herbivore  Diet = iota // grass, browse, crops, raw veg, straw
	Carnivore              // carcasses and meat; hunts when there are none
	Omnivore               // both
	Dendrovore             // living trees
	NoDiet                 // hive insects: no needs
	// StrictCarnivore eats only meat and carcasses: never kibble,
	// pemmican or meals (FEED-001).
	StrictCarnivore
)

var dietNames = [...]string{"plants", "meat", "plants and meat", "trees", "", "meat only"}

func (d Diet) String() string {
	if int(d) < len(dietNames) {
		return dietNames[d]
	}
	return ""
}

// Plants reports a diet that grazes and browses; Meat one that eats
// carcasses.
func (d Diet) Plants() bool { return d == Herbivore || d == Omnivore }

// Meat reports a diet that eats carcasses and raw meat.
func (d Diet) Meat() bool { return d == Carnivore || d == Omnivore || d == StrictCarnivore }

// MeatOnly reports a carnivore, strict or not: meat is its only raw food.
func (d Diet) MeatOnly() bool { return d == Carnivore || d == StrictCarnivore }

// Product is what a tame animal gives without being killed.
type Product uint8

const (
	NoProduct Product = iota
	Milk              // adult females
	Wool
	EmberOil
)

var productNames = [...]string{"", "milk", "wool", "ember oil"}

func (p Product) String() string {
	if int(p) < len(productNames) {
		return productNames[p]
	}
	return ""
}

// Item is the item kind a product drops as.
func (p Product) Item() item.Kind {
	switch p {
	case Milk:
		return item.Milk
	case Wool:
		return item.Wool
	case EmberOil:
		return item.EmberOil
	}
	return 0
}

// EggKind is how a species lays eggs.
type EggKind uint8

const (
	NoEggs      EggKind = iota // live birth
	FertileEggs                // lays only when mated; the eggs hatch
	PlainEggs                  // lays an edible egg a day, fertile ones when mated
)

// Ecology is a species' ecological row.
type Ecology struct {
	Diet Diet
	// Core biomes it lives in wild; Marginal ones where it is rarer.
	// Both empty: never wild (domestic, pets, event and hive animals).
	Core, Marginal biome.Set
	Nocturnal      bool
	Product        Product
	Eggs           EggKind
}

// biome shorthand for the table below.
const (
	tf = biome.TemperateForest
	bf = biome.BorealForest
	tu = biome.Tundra
	cb = biome.ColdBog
	is = biome.IceSheet
	si = biome.SeaIce
	ts = biome.TemperateSwamp
	ps = biome.TropicalSwamp
	rf = biome.TropicalRainforest
	as = biome.AridShrubland
	de = biome.Desert
	xd = biome.ExtremeDesert
)

func in(ids ...biome.ID) biome.Set { return biome.Of(ids...) }

// No roster species is on the wiki's strict-carnivore list (warg, ghoul,
// vulture, wolverine: rimworldwiki.com/wiki/Animals); the dire wolf, our
// ice-age hypercarnivore, is strict by our own design (FEED-001).
var ecology = [numSpecies]Ecology{
	Deer:             {Diet: Herbivore, Core: in(tf, bf, ts), Marginal: in(cb)},
	TimberWolf:       {Diet: Carnivore, Core: in(tf, bf), Marginal: in(cb, tu)},
	Alpaca:           {Diet: Herbivore, Core: in(as), Marginal: in(tu, tf), Product: Wool},
	Bison:            {Diet: Herbivore, Core: in(tf, as), Marginal: in(bf), Product: Wool},
	Emberbuck:        {Diet: Herbivore, Core: in(tf, as), Marginal: in(ts, de), Product: EmberOil},
	Capybara:         {Diet: Herbivore, Core: in(rf, ps), Marginal: in(ts)},
	Caribou:          {Diet: Herbivore, Core: in(tu, bf, cb), Marginal: in(is), Product: Milk},
	Cow:              {Diet: Herbivore, Product: Milk},
	Donkey:           {Diet: Herbivore, Core: in(as, de), Marginal: in(tf)},
	Dromedary:        {Diet: Herbivore, Core: in(de, xd, as), Product: Milk},
	Elephant:         {Diet: Herbivore, Core: in(rf, ps, as)},
	Elk:              {Diet: Herbivore, Core: in(bf, tf, cb), Marginal: in(tu), Product: Milk},
	Gazelle:          {Diet: Herbivore, Core: in(as, de), Marginal: in(tf, xd)},
	Goat:             {Diet: Herbivore, Product: Milk},
	Horse:            {Diet: Herbivore, Core: in(tf, as), Marginal: in(bf)},
	Ibex:             {Diet: Herbivore, Core: in(tu, bf), Marginal: in(tf, cb)},
	GroundSloth:      {Diet: Herbivore, Marginal: in(tf, bf, ts, as, rf), Product: Wool},
	Woolhorn:         {Diet: Herbivore, Core: in(tu, bf, cb, is), Marginal: in(si), Product: Wool},
	Pig:              {Diet: Omnivore},
	Rhinoceros:       {Diet: Herbivore, Core: in(as, rf), Marginal: in(ps)},
	Sheep:            {Diet: Herbivore, Product: Wool},
	Yak:              {Diet: Herbivore, Core: in(tu), Marginal: in(bf, cb), Product: Milk},
	Timbergnaw:       {Diet: Dendrovore},
	ArcticFox:        {Diet: Carnivore, Core: in(tu, is, si), Marginal: in(bf), Nocturnal: true},
	ArcticWolf:       {Diet: Carnivore, Core: in(tu, is), Marginal: in(si, bf)},
	CinderRat:        {Diet: Omnivore, Core: in(tf, as), Marginal: in(ts), Nocturnal: true},
	Cat:              {Diet: Carnivore},
	Chinchilla:       {Diet: Herbivore, Core: in(as), Marginal: in(tu), Nocturnal: true},
	Cougar:           {Diet: Carnivore, Core: in(tf, bf, as), Marginal: in(de, cb, ts), Nocturnal: true},
	FennecFox:        {Diet: Omnivore, Core: in(de, xd, as), Nocturnal: true},
	GrizzlyBear:      {Diet: Omnivore, Core: in(bf, tf), Marginal: in(tu, cb)},
	GuineaPig:        {Diet: Herbivore, Marginal: in(rf, as)},
	Hare:             {Diet: Herbivore, Core: in(tf, as), Marginal: in(bf, ts)},
	Husky:            {Diet: Carnivore},
	Labrador:         {Diet: Omnivore},
	Lynx:             {Diet: Carnivore, Core: in(bf), Marginal: in(tu, cb), Nocturnal: true},
	Monkey:           {Diet: Omnivore, Core: in(rf, ps)},
	Panther:          {Diet: Carnivore, Core: in(rf, ps), Nocturnal: true},
	PolarBear:        {Diet: Carnivore, Core: in(si, is), Marginal: in(tu)},
	Raccoon:          {Diet: Omnivore, Core: in(tf, ts), Nocturnal: true},
	Rat:              {Diet: Omnivore, Core: in(tf, ts, ps, rf), Marginal: in(as), Nocturnal: true},
	RedFox:           {Diet: Omnivore, Core: in(tf, bf), Marginal: in(as, tu), Nocturnal: true},
	Snowhare:         {Diet: Herbivore, Core: in(tu, bf, cb)},
	Squirrel:         {Diet: Herbivore, Core: in(tf, bf)},
	DireWolf:         {Diet: StrictCarnivore, Core: in(tu), Marginal: in(bf)},
	WildBoar:         {Diet: Omnivore, Core: in(tf, ts), Marginal: in(rf)},
	YorkshireTerrier: {Diet: Omnivore},
	Cassowary:        {Diet: Omnivore, Core: in(rf, ps), Eggs: FertileEggs},
	Chicken:          {Diet: Omnivore, Eggs: PlainEggs},
	Duck:             {Diet: Omnivore, Core: in(ts), Marginal: in(ps, cb), Eggs: PlainEggs},
	Emu:              {Diet: Omnivore, Core: in(as), Marginal: in(de), Eggs: FertileEggs},
	Goose:            {Diet: Herbivore, Core: in(ts, cb), Marginal: in(tu), Eggs: PlainEggs},
	Ostrich:          {Diet: Omnivore, Core: in(as, de), Eggs: FertileEggs},
	Turkey:           {Diet: Omnivore, Core: in(tf), Eggs: FertileEggs},
	Cobra:            {Diet: Carnivore, Core: in(rf, as), Marginal: in(de), Eggs: FertileEggs},
	Iguana:           {Diet: Herbivore, Core: in(rf, ps), Marginal: in(de), Eggs: FertileEggs},
	Tortoise:         {Diet: Herbivore, Core: in(as, de), Marginal: in(rf, xd), Eggs: FertileEggs},
	HiveBeetle:       {Diet: NoDiet},
	HiveBrute:        {Diet: NoDiet},
	HiveCrawler:      {Diet: NoDiet},
	Glimmerstag:      {Diet: Herbivore},
	// Wave E mechanoids: machines, no needs.
	Harrow:  {Diet: NoDiet},
	Spindle: {Diet: NoDiet},
	Bulwark: {Diet: NoDiet},
	Quill:   {Diet: NoDiet},
}

// EcologyOf is s's ecological row.
func EcologyOf(s Species) Ecology {
	if s >= numSpecies {
		return ecology[Deer]
	}
	return ecology[s]
}

// Wild reports whether s ever lives wild in biome b.
func Wild(s Species, b biome.ID) bool { return Weight(s, b) > 0 }

// commonality weights (ours): small, timid animals are many; big ones and
// hunters are few.
var (
	sizeWeight      = [...]int{SizeXS: 100, SizeS: 80, SizeM: 50, SizeL: 30, SizeXL: 15}
	behaviourWeight = [...]int{Grazer: 100, Skittish: 100, Docile: 100, Domestic: 100, Pet: 100,
		Territorial: 60, Predator: 25, PackPredator: 30, Hive: 0, Legendary: 0, Mechanoid: 0}
)

// Fit of a biome for a species, percent.
const (
	coreFit     = 100
	marginalFit = 35
)

// Weight is how common s is in biome b's wildlife, relative to the other
// species there (0: never wild in b). The season factor applies on top.
func Weight(s Species, b biome.ID) int {
	e, r := EcologyOf(s), RosterOf(s)
	fit := 0
	switch {
	case e.Core.Has(b):
		fit = coreFit
	case e.Marginal.Has(b):
		fit = marginalFit
	}
	if fit == 0 || r.HerdMin == 0 {
		return 0
	}
	return sizeWeight[r.Size] * behaviourWeight[r.Behaviour] * fit / 10000
}

// Life is a species' life numbers: the size defaults below (our own),
// overridden by the species' published row (breeding.go, FOOD-6).
type Life struct {
	JuvenileDays         int // the baby stage ends (FOOD-6)
	AdultDays, LifeDays  int
	GestCenti            int // gestation, hundredths of a day (0: lays eggs)
	LitterMin, LitterMax int
	ProductDays          int // days to fill the product once
	ProductYield         int // items per collection
}

var lifeBySize = [...]Life{
	SizeXS: {AdultDays: 6, LifeDays: 120, GestCenti: 500, LitterMin: 1, LitterMax: 4},
	SizeS:  {AdultDays: 10, LifeDays: 240, GestCenti: 800, LitterMin: 1, LitterMax: 3},
	SizeM:  {AdultDays: 18, LifeDays: 480, GestCenti: 1400, LitterMin: 1, LitterMax: 2},
	SizeL:  {AdultDays: 25, LifeDays: 720, GestCenti: 2000, LitterMin: 1, LitterMax: 1},
	SizeXL: {AdultDays: 30, LifeDays: 900, GestCenti: 2500, LitterMin: 1, LitterMax: 1},
}

// Products: milk and wool per species from the wiki (husbandry.go
// milkRows, woolRows); ember oil (a boomalope's role) every 2 days, 6 a
// time (original tuning).
const (
	emberOilDays = 2
	emberOilQty  = 6
)

// LifeOf is s's life numbers (from a table built once: it is read every
// tick, life.go).
func LifeOf(s Species) Life {
	if s >= numSpecies {
		return lives[Deer]
	}
	return lives[s]
}

// lives is every species' Life.
var lives = func() (out [numSpecies]Life) {
	for i := range out {
		out[i] = buildLife(Species(i))
	}
	return out
}()

// buildLife works s's life numbers out from its size, body and products.
func buildLife(s Species) Life {
	l := lifeBySize[RosterOf(s).Size]
	l.JuvenileDays = l.AdultDays / 3
	l = breedLife(s, l) // breeding.go (FOOD-6): the wiki's numbers
	e := EcologyOf(s)
	switch e.Product {
	case Milk:
		l.ProductDays, l.ProductYield = milkRows[s].days, milkRows[s].qty
	case Wool:
		l.ProductDays, l.ProductYield = woolRows[s].days, woolRows[s].qty
	case EmberOil:
		l.ProductDays, l.ProductYield = emberOilDays, emberOilQty
	}
	return l
}

// Needs (0..Full, like colonists). Every gap between a start and a stop
// threshold is deliberate: animals must not flap between two states.
const (
	Full = 10000
	// FoodPerDay drains a full adult animal empty in under two days; the
	// bar holds 1/FoodPerDay of the species' daily need (sizeMeal).
	FoodPerDay = 6000
	// FeedStart: an animal looks for food below this; FeedStop: it eats
	// until this.
	FeedStart = 4000
	FeedStop  = 9500
	// GrazeFood is one grass tile's nutrition (GrazeTicks to eat); wiki:
	// Grass (0.15, FOOD-1 scale: needs.Full = 1.0).
	GrazeFood  = 1500
	GrazeTicks = 120
	// TreeTicks is how long a tree-eater takes over one tree.
	TreeTicks = 400
	// NoFoodRetry: found nothing to eat, look again after this.
	NoFoodRetry = 600
	// StarveEvery: a starving animal takes 1 injury this often.
	StarveEvery = 600
	// RestPerDay drains a full animal empty in about 1.6 days awake;
	// SleepPerHour refills it from empty in 8 hours.
	RestPerDay   = 6250
	SleepPerHour = Full / 8
	// RestStart: bed down in sleep hours below this; RestCollapse: at any
	// hour. RestWake: up at full; RestWakeDay: out of sleep hours, up from
	// this.
	RestStart    = 7000
	RestCollapse = 1500
	RestWakeDay  = 6000
	// HuntGiveUp ticks without a strike, or prey beyond HuntRadius +
	// HuntLeash, and a predator gives up; it tries again after HuntRetry.
	HuntGiveUp = 1500
	HuntLeash  = 8
	HuntRetry  = 1200
	// BadlyHurt: fighters break off below this percent of their hit points.
	BadlyHurt = 30
	// DecideEvery: a full decision this often (staggered by id); threats
	// are checked every ThreatEvery.
	DecideEvery = 60
	ThreatEvery = 15
)

// sizeMeal is the bar an adult of this size fills per unit of nutrition
// (percent): FoodPerDay*100/sizeMeal is its daily need. FOOD-1, wiki:
// Alpaca (0.44 a day, medium) and Cow (0.86, large); the other sizes are
// interpolated (original tuning): 0.12, 0.24 and 1.76 a day.
var sizeMeal = [...]int{SizeXS: 500, SizeS: 250, SizeM: 136, SizeL: 70, SizeXL: 34}

// Meal scales a food's nutrition (needs.Full = 1.0) to species s's bar
// (big bodies need more).
func Meal(s Species, nutrition int) int { return nutrition * sizeMeal[RosterOf(s).Size] / 100 }

// SleepHours reports whether hour h is when s sleeps: diurnal animals
// 21-05, nocturnal ones 09-16.
func SleepHours(s Species, h int) bool {
	if EcologyOf(s).Nocturnal {
		return h >= 9 && h < 17
	}
	return h >= 21 || h < 5
}

// AgeDays is how old an animal born at tick born is at tick now (born 0:
// an adult from before ages were kept). Born may lie before tick 0 (a
// start herd's old animals): it is then stored wrapped (BornAt), and the
// difference still comes out right in uint64 arithmetic.
func AgeDays(born, now uint64, s Species) int {
	if born == 0 {
		return LifeOf(s).AdultDays * 2
	}
	age := int64(now - born)
	if age < 0 {
		return 0
	}
	return int(age / clock.TicksPerDay)
}

// BornAt is the Born tick of an animal ageTicks old at tick now (it may
// wrap below tick 0; never 0, which means "adult from an old save").
func BornAt(now, ageTicks uint64) uint64 {
	if b := now - ageTicks; b != 0 {
		return b
	}
	return 1
}

// Adult reports an animal of age days is grown.
func Adult(s Species, days int) bool { return days >= LifeOf(s).AdultDays }

// GrowthScale is the drawn body size of an animal age days old: 0.55
// newborn up to 1 when adult.
func GrowthScale(s Species, days int) float64 {
	a := LifeOf(s).AdultDays
	if a <= 0 || days >= a {
		return 1
	}
	if days < 0 {
		days = 0
	}
	return 0.55 + 0.45*float64(days)/float64(a)
}

// Stage is the life-stage word for the inspector: "adult", or the young
// of the body plan ("chick", "hatchling", "young").
func Stage(s Species, adult bool) string {
	if adult {
		return "adult"
	}
	switch RosterOf(s).Archetype {
	case ArchBird:
		return "chick"
	case ArchSnake, ArchLizard, ArchTortoise:
		return "hatchling"
	}
	return "young"
}

// ArtFemale is the sex an animal from a save without sexes gets: the
// hash of its id the art used to pick dimorphic females.
func ArtFemale(id uint64) bool { return (id*0x9E3779B97F4A7C15)>>63 == 1 }

// Normalise fills in what a save from before needs and sexes lacks
// (HasSex false): fed, rested, the art's sex. A pre-needs hunger counter
// (Hunger, ticks since a predator's last meal) becomes Food, HungerTicks
// mapping to just under FeedStart, and is cleared.
func (s *State) Normalise(id uint64) {
	if !s.HasSex {
		s.HasSex, s.Female = true, ArtFemale(id)
		if s.Food == 0 {
			s.Food = Full
		}
		if s.Rest == 0 {
			s.Rest = Full
		}
	}
	if s.Hunger == 0 {
		return
	}
	if ht := DefOf(s.Species).HungerTicks; ht > 0 {
		f := Full - int(min(s.Hunger, 2*ht)*uint64(Full-FeedStart+1)/ht)
		s.Food = min(s.Food, max(f, 1))
	}
	s.Hunger = 0
}

// NeedsFree reports a species that has no food or rest needs (hive
// insects, legendaries, mechanoids).
func NeedsFree(s Species) bool {
	d := DefOf(s)
	return EcologyOf(s).Diet == NoDiet || d.Hive || d.Legendary
}

// Feeding numbers (our own). A browsed plant is two grazed tiles; a crop
// loses CropBite percent of its growth per meal and dies below CropDies;
// a tree is a big meal. Items take ItemTicks to eat, a carcass
// CarcassTicks.
const (
	BrowseFood   = 2 * GrazeFood
	CropFood     = 2 * GrazeFood
	CropBite     = 40
	CropDies     = 20
	TreeFood     = 6000 // 0.6 nutrition (original tuning)
	ItemTicks    = 90
	CarcassTicks = 300
	// FeedRadius bounds every food search (tiles, Chebyshev).
	FeedRadius = 30
	// HayFood is one straw's worth to a plant eater; wiki: Hay (0.05).
	HayFood = 500
)

// FeedValue is what one item of k is worth to an animal of diet d before
// its size (Meal); 0 when it won't eat it. The diet table follows the wiki
// (FEED-001, ENG-003): kibble and pemmican feed every diet but strict
// carnivores; meals and rations every diet but strict carnivores and tree
// eaters; raw vegetables and berries plant and tree eaters; straw plant
// eaters; raw meat meat eaters; eggs and milk omnivores (FOOD-3). Fertile
// eggs and medicine feed no animal.
func FeedValue(d Diet, k item.Kind) int {
	switch {
	case d == NoDiet || d > StrictCarnivore:
		return 0
	case k == item.Straw:
		if d.Plants() {
			return HayFood
		}
	case k.IsMeat(): // FOOD-3: human and insect meat too
		if d.Meat() {
			return k.Nutrition()
		}
	case k == item.Egg || k == item.Milk: // FOOD-3, wiki: Animals: omnivores eat eggs and milk
		if d == Omnivore {
			return k.Nutrition()
		}
	case k == item.Kibble || k == item.Pemmican:
		if d != StrictCarnivore {
			return k.Nutrition()
		}
	case FoodTier(k) == TierMeal:
		if d != StrictCarnivore && d != Dendrovore {
			return k.Nutrition()
		}
	case PlantFeed(k):
		if d.Plants() || d == Dendrovore {
			return k.Nutrition()
		}
	}
	return 0
}

// Tier orders a tame animal's food (FEED-001): it eats animal feed first,
// a meal only when no feed is in reach.
type Tier uint8

const (
	TierFeed Tier = iota // kibble, straw, raw food, pemmican, plants, carcasses, prey
	TierMeal             // meals and packaged rations: the colonists' food
)

// FoodTier is item kind k's tier.
func FoodTier(k item.Kind) Tier {
	switch k {
	case item.Meal, item.FineMeal, item.LavishMeal, item.SurvivalMeal, item.Food:
		return TierMeal
	}
	return TierFeed
}

// Prepared reports food made in a kitchen or at a bench (meals, rations,
// pemmican, kibble): a wild animal never takes it.
func Prepared(k item.Kind) bool {
	return FoodTier(k) == TierMeal || k == item.Kibble || k == item.Pemmican
}

// Made is what prepared food diet d eats, for the inspector: "kibble,
// pemmican and meals", "kibble and pemmican" (tree eaters) or "" (strict
// carnivores and the needless).
func (d Diet) Made() string {
	switch {
	case FeedValue(d, item.Meal) > 0:
		return "kibble, pemmican and meals"
	case FeedValue(d, item.Kibble) > 0:
		return "kibble and pemmican"
	}
	return ""
}

// PlantFeed reports raw plant food an animal eats (berries and raw
// vegetables; straw separately).
func PlantFeed(k item.Kind) bool {
	switch k {
	case item.Berries, item.Potatoes, item.Corn, item.Wheat, item.Beans, item.Lettuce, item.Radishes, item.Pumpkins:
		return true
	}
	return false
}

// Starving search (FEED-001, original tuning): below StarveSearch a tame
// animal looks for food across its whole allowed area or pen, past
// FeedRadius; it stops once it has eaten or its food is back above
// StarveSearchEnd. FarSearch is that band: was is whether it searched far
// last time.
const (
	StarveSearch    = Full * 15 / 100
	StarveSearchEnd = Full * 25 / 100
)

// FarSearch reports a tame animal with food searching far (hysteresis
// between StarveSearch and StarveSearchEnd).
func FarSearch(food int, was bool) bool {
	return food < StarveSearch || was && food < StarveSearchEnd
}

// CarcassFood is a corpse of species s as a meal (its meat).
func CarcassFood(s Species) int {
	m, _ := Yield(s)
	return max(m, 1) * item.Meat.Nutrition()
}
