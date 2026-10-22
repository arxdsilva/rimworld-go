package animal

import (
	"strings"

	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Species is a kind of animal. Values are saved: append new species
// before numSpecies, never reorder (Deer and TimberWolf took the slots of
// the former dustbuck and ridgewolf).
type Species uint8

const (
	Deer Species = iota
	TimberWolf
	Alpaca
	Bison
	Emberbuck
	Capybara
	Caribou
	Cow
	Donkey
	Dromedary
	Elephant
	Elk
	Gazelle
	Goat
	Horse
	Ibex
	GroundSloth
	Woolhorn
	Pig
	Rhinoceros
	Sheep
	Yak
	Timbergnaw
	ArcticFox
	ArcticWolf
	CinderRat
	Cat
	Chinchilla
	Cougar
	FennecFox
	GrizzlyBear
	GuineaPig
	Hare
	Husky
	Labrador
	Lynx
	Monkey
	Panther
	PolarBear
	Raccoon
	Rat
	RedFox
	Snowhare
	Squirrel
	DireWolf
	WildBoar
	YorkshireTerrier
	Cassowary
	Chicken
	Duck
	Emu
	Goose
	Ostrich
	Turkey
	Cobra
	Iguana
	Tortoise
	HiveBeetle
	HiveBrute
	HiveCrawler
	Glimmerstag
	numSpecies

	// Dustbuck and Ridgewolf are the former names of Deer and TimberWolf.
	Dustbuck  = Deer
	Ridgewolf = TimberWolf
)

// Archetype is the body plan the art builds a species from.
type Archetype uint8

const (
	ArchQuadruped Archetype = iota
	ArchPrimate
	ArchBird
	ArchSnake
	ArchLizard
	ArchTortoise
	ArchBeetle
	ArchBrute
	ArchCrawler
)

// Size is a species' body class (SIM-019 formulas key off it).
type Size uint8

const (
	SizeXS Size = iota
	SizeS
	SizeM
	SizeL
	SizeXL
)

// Behaviour is how a species lives: what it does when calm and when
// threatened, and how easily it is tamed.
type Behaviour uint8

const (
	Grazer Behaviour = iota
	Skittish
	Docile
	Domestic
	Pet
	Territorial
	Predator
	PackPredator
	Hive
	Legendary
)

// Habitat is a bit set of the climates a species lives in.
type Habitat uint8

const (
	HabTemperate Habitat = 1 << iota
	HabCold
	HabHot
	HabAny = HabTemperate | HabCold | HabHot
)

// AttackKind is a species' natural weapon; its name is the action and the
// art's attack frame set ("bite1".."bite4", ...).
type AttackKind uint8

const (
	AttackBite AttackKind = iota
	AttackClaw
	AttackKick
	AttackGore
	AttackHeadbutt
	AttackPeck
)

var attackNames = [...]string{"bite", "claw", "kick", "gore", "headbutt", "peck"}

func (a AttackKind) String() string {
	if int(a) < len(attackNames) {
		return attackNames[a]
	}
	return "bite"
}

// Speed is a species' pace class over its size's base step.
type Speed uint8

const (
	SpeedNormal Speed = iota
	SpeedFast
	SpeedSlow
)

// Roster is a species' identity and classes (the binding roster of
// SIM-019 / ART-011); Def's numbers derive from it.
type Roster struct {
	Key       string // snake_case: the art key and the save-independent name
	Archetype Archetype
	Size      Size
	SizeTrim  int // percent over or under the size class (S+ = +15)
	Behaviour Behaviour
	Habitat   Habitat
	Attack    AttackKind
	HerdMin   int // 0: never spawns as a wild herd (hive insects)
	HerdMax   int
	Speed     Speed
	Dimorphic bool // separate female art (renderer picks by id)
	Explosive bool // bursts when it dies
	SmallPrey bool // a predator of small animals only
}

var roster = [numSpecies]Roster{
	Deer:             {Key: "deer", Archetype: ArchQuadruped, Size: SizeM, Behaviour: Skittish, Habitat: HabTemperate, Attack: AttackKick, HerdMin: 2, HerdMax: 4, Dimorphic: true},
	TimberWolf:       {Key: "timber_wolf", Archetype: ArchQuadruped, Size: SizeM, Behaviour: PackPredator, Habitat: HabTemperate, Attack: AttackBite, HerdMin: 3, HerdMax: 5},
	Alpaca:           {Key: "alpaca", Archetype: ArchQuadruped, Size: SizeM, Behaviour: Domestic, Habitat: HabCold | HabTemperate, Attack: AttackKick, HerdMin: 2, HerdMax: 4},
	Bison:            {Key: "bison", Archetype: ArchQuadruped, Size: SizeL, Behaviour: Territorial, Habitat: HabTemperate, Attack: AttackGore, HerdMin: 3, HerdMax: 6},
	Emberbuck:        {Key: "emberbuck", Archetype: ArchQuadruped, Size: SizeM, Behaviour: Grazer, Habitat: HabTemperate, Attack: AttackKick, HerdMin: 1, HerdMax: 3, Explosive: true},
	Capybara:         {Key: "capybara", Archetype: ArchQuadruped, Size: SizeM, SizeTrim: -15, Behaviour: Grazer, Habitat: HabHot | HabTemperate, Attack: AttackBite, HerdMin: 2, HerdMax: 5},
	Caribou:          {Key: "caribou", Archetype: ArchQuadruped, Size: SizeL, Behaviour: Grazer, Habitat: HabCold, Attack: AttackGore, HerdMin: 3, HerdMax: 6},
	Cow:              {Key: "cow", Archetype: ArchQuadruped, Size: SizeL, Behaviour: Domestic, Habitat: HabTemperate, Attack: AttackHeadbutt, HerdMin: 1, HerdMax: 3, Speed: SpeedSlow},
	Donkey:           {Key: "donkey", Archetype: ArchQuadruped, Size: SizeM, SizeTrim: 15, Behaviour: Domestic, Habitat: HabTemperate | HabHot, Attack: AttackKick, HerdMin: 1, HerdMax: 2},
	Dromedary:        {Key: "dromedary", Archetype: ArchQuadruped, Size: SizeL, Behaviour: Domestic, Habitat: HabHot, Attack: AttackKick, HerdMin: 1, HerdMax: 3},
	Elephant:         {Key: "elephant", Archetype: ArchQuadruped, Size: SizeXL, Behaviour: Territorial, Habitat: HabHot, Attack: AttackGore, HerdMin: 2, HerdMax: 4},
	Elk:              {Key: "elk", Archetype: ArchQuadruped, Size: SizeL, Behaviour: Grazer, Habitat: HabTemperate | HabCold, Attack: AttackGore, HerdMin: 2, HerdMax: 5, Dimorphic: true},
	Gazelle:          {Key: "gazelle", Archetype: ArchQuadruped, Size: SizeS, SizeTrim: 15, Behaviour: Skittish, Habitat: HabHot, Attack: AttackKick, HerdMin: 3, HerdMax: 6, Speed: SpeedFast},
	Goat:             {Key: "goat", Archetype: ArchQuadruped, Size: SizeS, SizeTrim: 15, Behaviour: Domestic, Habitat: HabAny, Attack: AttackHeadbutt, HerdMin: 2, HerdMax: 4},
	Horse:            {Key: "horse", Archetype: ArchQuadruped, Size: SizeL, Behaviour: Domestic, Habitat: HabTemperate, Attack: AttackKick, HerdMin: 1, HerdMax: 3, Speed: SpeedFast},
	Ibex:             {Key: "ibex", Archetype: ArchQuadruped, Size: SizeM, Behaviour: Grazer, Habitat: HabCold | HabTemperate, Attack: AttackHeadbutt, HerdMin: 2, HerdMax: 4},
	GroundSloth:      {Key: "ground_sloth", Archetype: ArchQuadruped, Size: SizeXL, Behaviour: Territorial, Habitat: HabTemperate | HabCold, Attack: AttackClaw, HerdMin: 1, HerdMax: 1, Speed: SpeedSlow},
	Woolhorn:         {Key: "woolhorn", Archetype: ArchQuadruped, Size: SizeL, Behaviour: Domestic, Habitat: HabCold | HabTemperate, Attack: AttackHeadbutt, HerdMin: 3, HerdMax: 6},
	Pig:              {Key: "pig", Archetype: ArchQuadruped, Size: SizeM, Behaviour: Domestic, Habitat: HabTemperate, Attack: AttackBite, HerdMin: 1, HerdMax: 3, Speed: SpeedSlow},
	Rhinoceros:       {Key: "rhinoceros", Archetype: ArchQuadruped, Size: SizeXL, Behaviour: Territorial, Habitat: HabHot, Attack: AttackGore, HerdMin: 1, HerdMax: 2},
	Sheep:            {Key: "sheep", Archetype: ArchQuadruped, Size: SizeS, SizeTrim: 15, Behaviour: Domestic, Habitat: HabTemperate | HabCold, Attack: AttackHeadbutt, HerdMin: 3, HerdMax: 6},
	Yak:              {Key: "yak", Archetype: ArchQuadruped, Size: SizeL, Behaviour: Domestic, Habitat: HabCold, Attack: AttackGore, HerdMin: 2, HerdMax: 4},
	Timbergnaw:       {Key: "timbergnaw", Archetype: ArchQuadruped, Size: SizeM, Behaviour: Grazer, Habitat: HabTemperate | HabCold, Attack: AttackBite, HerdMin: 3, HerdMax: 5},
	ArcticFox:        {Key: "arctic_fox", Archetype: ArchQuadruped, Size: SizeS, Behaviour: Predator, Habitat: HabCold, Attack: AttackBite, HerdMin: 1, HerdMax: 1, SmallPrey: true, Speed: SpeedFast},
	ArcticWolf:       {Key: "arctic_wolf", Archetype: ArchQuadruped, Size: SizeM, Behaviour: PackPredator, Habitat: HabCold, Attack: AttackBite, HerdMin: 3, HerdMax: 5},
	CinderRat:        {Key: "cinder_rat", Archetype: ArchQuadruped, Size: SizeXS, Behaviour: Skittish, Habitat: HabTemperate, Attack: AttackBite, HerdMin: 2, HerdMax: 4, Explosive: true},
	Cat:              {Key: "cat", Archetype: ArchQuadruped, Size: SizeS, SizeTrim: -15, Behaviour: Pet, Habitat: HabAny, Attack: AttackClaw, HerdMin: 1, HerdMax: 1},
	Chinchilla:       {Key: "chinchilla", Archetype: ArchQuadruped, Size: SizeXS, Behaviour: Grazer, Habitat: HabCold | HabTemperate, Attack: AttackBite, HerdMin: 2, HerdMax: 4},
	Cougar:           {Key: "cougar", Archetype: ArchQuadruped, Size: SizeM, Behaviour: Predator, Habitat: HabTemperate | HabHot, Attack: AttackClaw, HerdMin: 1, HerdMax: 1, Speed: SpeedFast},
	FennecFox:        {Key: "fennec_fox", Archetype: ArchQuadruped, Size: SizeXS, Behaviour: Predator, Habitat: HabHot, Attack: AttackBite, HerdMin: 1, HerdMax: 1, SmallPrey: true, Speed: SpeedFast},
	GrizzlyBear:      {Key: "grizzly_bear", Archetype: ArchQuadruped, Size: SizeL, Behaviour: Predator, Habitat: HabTemperate | HabCold, Attack: AttackClaw, HerdMin: 1, HerdMax: 1},
	GuineaPig:        {Key: "guinea_pig", Archetype: ArchQuadruped, Size: SizeXS, Behaviour: Domestic, Habitat: HabTemperate, Attack: AttackBite, HerdMin: 2, HerdMax: 4},
	Hare:             {Key: "hare", Archetype: ArchQuadruped, Size: SizeS, SizeTrim: -15, Behaviour: Skittish, Habitat: HabTemperate, Attack: AttackBite, HerdMin: 1, HerdMax: 2, Speed: SpeedFast},
	Husky:            {Key: "husky", Archetype: ArchQuadruped, Size: SizeM, SizeTrim: -15, Behaviour: Pet, Habitat: HabCold, Attack: AttackBite, HerdMin: 1, HerdMax: 1},
	Labrador:         {Key: "labrador", Archetype: ArchQuadruped, Size: SizeM, SizeTrim: -15, Behaviour: Pet, Habitat: HabTemperate, Attack: AttackBite, HerdMin: 1, HerdMax: 1},
	Lynx:             {Key: "lynx", Archetype: ArchQuadruped, Size: SizeS, SizeTrim: 15, Behaviour: Predator, Habitat: HabCold, Attack: AttackClaw, HerdMin: 1, HerdMax: 1},
	Monkey:           {Key: "monkey", Archetype: ArchPrimate, Size: SizeS, Behaviour: Skittish, Habitat: HabHot, Attack: AttackClaw, HerdMin: 2, HerdMax: 5},
	Panther:          {Key: "panther", Archetype: ArchQuadruped, Size: SizeM, Behaviour: Predator, Habitat: HabHot, Attack: AttackClaw, HerdMin: 1, HerdMax: 1},
	PolarBear:        {Key: "polar_bear", Archetype: ArchQuadruped, Size: SizeL, SizeTrim: 15, Behaviour: Predator, Habitat: HabCold, Attack: AttackClaw, HerdMin: 1, HerdMax: 1},
	Raccoon:          {Key: "raccoon", Archetype: ArchQuadruped, Size: SizeS, Behaviour: Skittish, Habitat: HabTemperate, Attack: AttackBite, HerdMin: 1, HerdMax: 2},
	Rat:              {Key: "rat", Archetype: ArchQuadruped, Size: SizeXS, Behaviour: Skittish, Habitat: HabAny, Attack: AttackBite, HerdMin: 2, HerdMax: 5},
	RedFox:           {Key: "red_fox", Archetype: ArchQuadruped, Size: SizeS, Behaviour: Predator, Habitat: HabTemperate, Attack: AttackBite, HerdMin: 1, HerdMax: 1, SmallPrey: true, Speed: SpeedFast},
	Snowhare:         {Key: "snowhare", Archetype: ArchQuadruped, Size: SizeS, SizeTrim: -15, Behaviour: Skittish, Habitat: HabCold, Attack: AttackBite, HerdMin: 1, HerdMax: 2, Speed: SpeedFast},
	Squirrel:         {Key: "squirrel", Archetype: ArchQuadruped, Size: SizeXS, Behaviour: Skittish, Habitat: HabTemperate, Attack: AttackBite, HerdMin: 1, HerdMax: 3},
	DireWolf:         {Key: "dire_wolf", Archetype: ArchQuadruped, Size: SizeL, SizeTrim: -15, Behaviour: PackPredator, Habitat: HabCold, Attack: AttackBite, HerdMin: 2, HerdMax: 4},
	WildBoar:         {Key: "wild_boar", Archetype: ArchQuadruped, Size: SizeM, Behaviour: Territorial, Habitat: HabTemperate, Attack: AttackGore, HerdMin: 2, HerdMax: 4},
	YorkshireTerrier: {Key: "yorkshire_terrier", Archetype: ArchQuadruped, Size: SizeXS, Behaviour: Pet, Habitat: HabTemperate, Attack: AttackBite, HerdMin: 1, HerdMax: 1},
	Cassowary:        {Key: "cassowary", Archetype: ArchBird, Size: SizeM, SizeTrim: 15, Behaviour: Territorial, Habitat: HabHot, Attack: AttackKick, HerdMin: 1, HerdMax: 1},
	Chicken:          {Key: "chicken", Archetype: ArchBird, Size: SizeXS, Behaviour: Domestic, Habitat: HabAny, Attack: AttackPeck, HerdMin: 3, HerdMax: 6, Dimorphic: true},
	Duck:             {Key: "duck", Archetype: ArchBird, Size: SizeXS, Behaviour: Domestic, Habitat: HabTemperate, Attack: AttackPeck, HerdMin: 2, HerdMax: 5},
	Emu:              {Key: "emu", Archetype: ArchBird, Size: SizeM, Behaviour: Skittish, Habitat: HabHot, Attack: AttackKick, HerdMin: 2, HerdMax: 4},
	Goose:            {Key: "goose", Archetype: ArchBird, Size: SizeS, SizeTrim: -15, Behaviour: Domestic, Habitat: HabTemperate, Attack: AttackPeck, HerdMin: 2, HerdMax: 5},
	Ostrich:          {Key: "ostrich", Archetype: ArchBird, Size: SizeL, Behaviour: Skittish, Habitat: HabHot, Attack: AttackKick, HerdMin: 1, HerdMax: 3, Speed: SpeedFast},
	Turkey:           {Key: "turkey", Archetype: ArchBird, Size: SizeS, Behaviour: Domestic, Habitat: HabTemperate, Attack: AttackPeck, HerdMin: 2, HerdMax: 4},
	Cobra:            {Key: "cobra", Archetype: ArchSnake, Size: SizeS, Behaviour: Territorial, Habitat: HabHot, Attack: AttackBite, HerdMin: 1, HerdMax: 1},
	Iguana:           {Key: "iguana", Archetype: ArchLizard, Size: SizeS, Behaviour: Skittish, Habitat: HabHot, Attack: AttackBite, HerdMin: 1, HerdMax: 2},
	Tortoise:         {Key: "tortoise", Archetype: ArchTortoise, Size: SizeS, Behaviour: Docile, Habitat: HabHot | HabTemperate, Attack: AttackBite, HerdMin: 1, HerdMax: 1, Speed: SpeedSlow},
	HiveBeetle:       {Key: "hive_beetle", Archetype: ArchBeetle, Size: SizeXS, SizeTrim: 15, Behaviour: Hive, Habitat: HabAny, Attack: AttackBite, HerdMin: 0, HerdMax: 0},
	HiveBrute:        {Key: "hive_brute", Archetype: ArchBrute, Size: SizeL, Behaviour: Hive, Habitat: HabAny, Attack: AttackClaw, HerdMin: 0, HerdMax: 0},
	HiveCrawler:      {Key: "hive_crawler", Archetype: ArchCrawler, Size: SizeM, Behaviour: Hive, Habitat: HabAny, Attack: AttackBite, HerdMin: 0, HerdMax: 0},
	Glimmerstag:      {Key: "glimmerstag", Archetype: ArchQuadruped, Size: SizeXL, Behaviour: Legendary, Habitat: HabTemperate, Attack: AttackGore, HerdMin: 1, HerdMax: 1},
}

// RosterOf is s's roster row.
func RosterOf(s Species) Roster {
	if s >= numSpecies {
		return roster[Deer]
	}
	return roster[s]
}

// All lists every species in value order.
func All() []Species {
	out := make([]Species, numSpecies)
	for i := range out {
		out[i] = Species(i)
	}
	return out
}

func (s Species) String() string {
	if s >= numSpecies {
		return "animal"
	}
	return roster[s].Key
}

// Label is the species' display name ("arctic fox").
func (s Species) Label() string { return strings.ReplaceAll(s.String(), "_", " ") }

// ParseSpecies maps a key (or a former name) to a species.
func ParseSpecies(name string) (Species, bool) {
	switch name {
	case "dustbuck":
		return Deer, true
	case "ridgewolf":
		return TimberWolf, true
	}
	for i := range roster {
		if roster[i].Key == name {
			return Species(i), true
		}
	}
	return 0, false
}

// FleeRelease widens the threat radius while an animal is already
// fleeing, so it runs well clear instead of stopping one tile outside
// FleeRadius and turning straight back (no flee/graze flapping).
const FleeRelease = 4

// Def is a species definition, derived from its roster row (buildDef).
// Step ticks are per tile moved.
type Def struct {
	MaxHP int
	// Corpse is the item left on death (meat comes from butchering it).
	Corpse    item.Kind
	StepTicks int
	FleeStep  int
	// Meat and Leather are what butchering the corpse yields.
	Meat    int
	Leather int
	// Attack is the natural weapon; Damage, Cooldown and Accuracy its
	// numbers for this species' size (they override the weapon profile's).
	Attack   weapon.Kind
	Damage   int
	Cooldown int
	Accuracy int
	Predator bool
	// Skittish animals flee: from threats within FleeRadius (when it is
	// above 0) and from whoever hurts them.
	Skittish   bool
	FleeRadius int
	// AggroRadius: hungry predators attack people this close.
	AggroRadius int
	// HungerTicks until a predator starts hunting.
	HungerTicks uint64
	// HuntRadius: how far a hungry predator looks for prey.
	HuntRadius   int
	WanderRadius int
	// TameChance: percent chance a taming attempt succeeds.
	TameChance int
	// Revenge: percent chance a wild animal hurt by a colonist goes
	// manhunter; TameRevenge the same for a botched taming.
	Revenge     int
	TameRevenge int
	// Pack predators spawn and hunt together; SmallPrey ones take only
	// XS and S animals. Hive insects are hostile from birth and never
	// tamed; a Legendary's rage calls every legendary nearby; Explosive
	// animals burst when they die.
	Pack      bool
	SmallPrey bool
	Hive      bool
	Legendary bool
	Explosive bool
}

// Taming and manhunters.
const (
	// TameWork is the ticks a handler spends coaxing an animal.
	TameWork = 300
	// ManhunterTicks is how long an enraged animal hunts colonists (a day).
	ManhunterTicks = 60000
	// TameRetryTicks: after a failed attempt, the animal is left alone this long.
	TameRetryTicks = 5000
)

// defs derive from the roster (buildDef, SIM-019).
var defs = func() (out [numSpecies]Def) {
	for i := range out {
		out[i] = buildDef(Species(i), roster[i])
	}
	return out
}()

// sizeStats are a size class's base numbers (SIM-019: our own formulas).
type sizeStats struct {
	hp, step, meat, leather, damage, cooldown, accuracy int
}

var bySize = [...]sizeStats{
	SizeXS: {hp: 12, step: 16, meat: 2, leather: 0, damage: 2, cooldown: 100, accuracy: 80},
	SizeS:  {hp: 28, step: 14, meat: 6, leather: 2, damage: 4, cooldown: 110, accuracy: 78},
	SizeM:  {hp: 55, step: 16, meat: 14, leather: 5, damage: 7, cooldown: 120, accuracy: 75},
	SizeL:  {hp: 100, step: 18, meat: 30, leather: 10, damage: 11, cooldown: 130, accuracy: 72},
	SizeXL: {hp: 170, step: 22, meat: 55, leather: 16, damage: 16, cooldown: 140, accuracy: 70},
}

// behaviourStats are a behaviour class's temperament.
type behaviourStats struct {
	tame, revenge int
	fleeRadius    int  // flees threats this close (0: only when hurt)
	flees         bool // runs when hurt
	predator      bool
	wander        int
	hpPct         int // percent of the size's hit points
}

var byBehaviour = [...]behaviourStats{
	Grazer:       {tame: 40, revenge: 10, fleeRadius: 3, flees: true, wander: 4, hpPct: 100},
	Skittish:     {tame: 30, revenge: 0, fleeRadius: 5, flees: true, wander: 4, hpPct: 100},
	Docile:       {tame: 50, revenge: 5, flees: true, wander: 3, hpPct: 100},
	Domestic:     {tame: 75, revenge: 5, flees: true, wander: 4, hpPct: 100},
	Pet:          {tame: 70, revenge: 3, flees: true, wander: 4, hpPct: 100},
	Territorial:  {tame: 15, revenge: 50, wander: 4, hpPct: 100},
	Predator:     {tame: 15, revenge: 45, predator: true, wander: 6, hpPct: 100},
	PackPredator: {tame: 15, revenge: 45, predator: true, wander: 6, hpPct: 100},
	Hive:         {tame: 0, revenge: 0, wander: 5, hpPct: 100},
	Legendary:    {tame: 5, revenge: 100, wander: 5, hpPct: 250},
}

// Speed class step adjustments; fleeing runs at 60% of the step.
const (
	fastStep    = -4
	slowStep    = 6
	fleePermill = 600
	// explosiveTame: a volatile animal is harder to coax.
	explosiveTame = 30
)

// hpOverride is the few species the size rule misjudges (percent of
// their size's hit points): a tortoise's shell.
var hpOverride = map[Species]int{Tortoise: 150}

var attackWeapon = [...]weapon.Kind{
	AttackBite: weapon.Bite, AttackClaw: weapon.Claw, AttackKick: weapon.Kick,
	AttackGore: weapon.Gore, AttackHeadbutt: weapon.Headbutt, AttackPeck: weapon.Peck,
}

// scale is v adjusted by pct percent, rounded to nearest.
func scale(v, pct int) int { return (v*(100+pct) + 50) / 100 }

// buildDef derives a species' numbers from its roster classes: size sets
// the body (hit points, pace, yields, blow), behaviour the temperament.
func buildDef(s Species, r Roster) Def {
	z, b := bySize[r.Size], byBehaviour[r.Behaviour]
	hpPct := b.hpPct
	if p, ok := hpOverride[s]; ok {
		hpPct = p
	}
	step := scale(z.step, r.SizeTrim)
	switch r.Speed {
	case SpeedFast:
		step += fastStep
	case SpeedSlow:
		step += slowStep
	}
	d := Def{
		MaxHP:  scale(z.hp, r.SizeTrim) * hpPct / 100,
		Corpse: item.CorpseAnimal, StepTicks: step, FleeStep: (step*fleePermill + 500) / 1000,
		Meat: scale(z.meat, r.SizeTrim), Leather: scale(z.leather, r.SizeTrim),
		Attack: attackWeapon[r.Attack], Damage: scale(z.damage, r.SizeTrim), Cooldown: z.cooldown, Accuracy: z.accuracy,
		Predator: b.predator, Skittish: b.flees, FleeRadius: b.fleeRadius, WanderRadius: b.wander,
		TameChance: b.tame, Revenge: b.revenge, TameRevenge: b.revenge * 2 / 3,
		Pack: r.Behaviour == PackPredator, SmallPrey: r.SmallPrey, Hive: r.Behaviour == Hive,
		Legendary: r.Behaviour == Legendary, Explosive: r.Explosive,
	}
	if d.Predator {
		d.AggroRadius, d.HungerTicks, d.HuntRadius = 5, 90000, 22
	}
	if d.Explosive {
		d.TameChance = explosiveTame
	}
	return d
}

// Yield is the meat and leather butchering one fresh corpse of s gives.
func Yield(s Species) (meat, leather int) {
	d := DefOf(s)
	return d.Meat, d.Leather
}

// DefOf returns the definition for s.
func DefOf(s Species) Def {
	if s >= numSpecies {
		return defs[Deer]
	}
	return defs[s]
}

// Mode is what an animal is doing.
type Mode uint8

const (
	Wander Mode = iota
	Flee
	Attack // the aggressive state: retaliating, guarding, manhunting
	// SIM-024 (append only, saved): eating, sleeping, stalking prey, and
	// walking off the map.
	Feed
	Rest
	Hunt
	Leave
)

func (m Mode) String() string {
	switch m {
	case Flee:
		return "fleeing"
	case Attack:
		return "attacking"
	case Feed:
		return "feeding"
	case Rest:
		return "resting"
	case Hunt:
		return "hunting"
	case Leave:
		return "leaving"
	default:
		return "wandering"
	}
}

// FeedKind is what a feeding animal is eating.
type FeedKind uint8

const (
	FeedNone    FeedKind = iota
	FeedGrass            // a grass tile
	FeedPlant            // a wild plant or a crop (browse)
	FeedTree             // a living tree (tree-eaters)
	FeedItem             // a loose raw food item
	FeedCarcass          // a corpse
	FeedTrough           // a pen trough
)

// FeedTarget is where a feeding animal is going to eat.
type FeedTarget struct {
	Kind FeedKind
	ID   entity.EntityID // item, plant or trough (0 for grass)
	X, Y int
}

// State is one living animal.
type State struct {
	Species  Species
	Injury   int
	Mode     Mode
	Target   entity.EntityID
	Attacker entity.EntityID
	// FleeUntil keeps a hurt grazer running for a while.
	FleeUntil uint64
	// Hunger counts ticks since the last meal (predators).
	Hunger     uint64
	Designated bool // marked for hunting
	NextThink  uint64
	Cooldown   int
	Path       []pathing.Point
	PathI      int
	GoalX      int
	GoalY      int
	// Tame animals belong to the colony and have a name. TameMark: marked
	// for taming; TameRetry: no new attempt before this tick. Manhunter:
	// enraged, hunting colonists until this tick (0: calm).
	Tame      bool
	Name      string
	TameMark  bool
	TameRetry uint64
	Manhunter uint64
	// RetryAt: after a failed path search, no new search before this tick.
	RetryAt uint64

	// Needs and life (SIM-024..026; zero in saves from before them, which
	// the game reads as fed, rested, adult, with the art's sex).
	Food, Rest int  // 0..Full
	Female     bool // set at spawn when HasSex
	HasSex     bool
	Born       uint64 // tick (0: an adult from an older save)
	Pregnant   uint64 // due tick (0: not pregnant)
	Mated      int    // eggs still to lay under the last mating (MatedEggs)
	Product    int    // milk/wool/oil fullness, permille
	Feed       FeedTarget
	FeedLeft   int    // ticks left eating the current target
	NextFeed   uint64 // found nothing to eat: no new search before this tick
	Starving   uint64 // tick food ran out (0: fed)
	HuntSince  uint64 // tick the current hunt started or last struck
	Herd       uint32 // spawn group: herds bed down and leave together
	ExitX      int    // Leave mode's map-edge exit
	ExitY      int
	NextDecide uint64 // next full decision (DecideEvery)
	// Leaving: walking off the map to ExitX/ExitY (a herd over capacity,
	// a crossing); a flee or a fight on the way returns to Leave after.
	Leaving bool
	// Mating (SIM-026, life.go): Mate is the partner of a mating under
	// way (both sides point at each other); MateLeft the ticks of it
	// still to go once the male stands beside her; MateUntil the tick it
	// is given up if it has not happened.
	Mate      entity.EntityID
	MateLeft  int
	MateUntil uint64
}

// Enraged reports a manhunter at tick now.
func (s State) Enraged(now uint64) bool { return s.Manhunter != 0 && now < s.Manhunter }

// HP is remaining hit points.
func (s State) HP() int {
	hp := DefOf(s.Species).MaxHP - s.Injury
	if hp < 0 {
		return 0
	}
	return hp
}

// Dead reports whether the animal has no hit points left.
func (s State) Dead() bool { return s.HP() <= 0 }

// Hungry reports whether a predator wants to hunt: its food is below
// FeedStart (or, in a save from before needs, it went HungerTicks without
// a meal).
func (s State) Hungry() bool {
	d := DefOf(s.Species)
	return d.Predator && (s.Hunger >= d.HungerTicks || s.HasSex && s.Food < FeedStart)
}

// Step is the ticks per tile for the current mode.
func (s State) Step() int {
	d := DefOf(s.Species)
	if s.Mode == Flee && d.FleeStep > 0 {
		return d.FleeStep
	}
	return d.StepTicks
}

// StepCost is the ticks for one step in the current mode: Step()
// straight, Step() * 1.414 (rounded) diagonally, so diagonals are no
// faster than straights.
func (s State) StepCost(diagonal bool) int {
	st := s.Step()
	if diagonal {
		return (st*1414 + 500) / 1000
	}
	return st
}

// FleeGoal is a point dist tiles from (x, y) directly away from the
// threat at (tx, ty). The caller clamps it to the map and finds a
// walkable tile near it.
func FleeGoal(x, y, tx, ty, dist int) (int, int) {
	dx, dy := x-tx, y-ty
	if dx == 0 && dy == 0 {
		dx = 1
	}
	ax, ay := dx, dy
	if ax < 0 {
		ax = -ax
	}
	if ay < 0 {
		ay = -ay
	}
	m := ax
	if ay > m {
		m = ay
	}
	return x + dx*dist/m, y + dy*dist/m
}

// sizeLength is the body length in tiles per size class (ART-011).
var sizeLength = [...]float64{SizeXS: 0.35, SizeS: 0.6, SizeM: 0.95, SizeL: 1.4, SizeXL: 2.0}

// BodyLength is s's body length in tiles, with its size trim.
func BodyLength(s Species) float64 {
	r := RosterOf(s)
	return sizeLength[r.Size] * float64(100+r.SizeTrim) / 100
}
