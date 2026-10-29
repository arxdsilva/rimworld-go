package quality

// Level is an item's craftsmanship. Values are saved as uint8: append only.
// The zero value is Awful, so persisted or defaulted items that need
// "ordinary" must use Default (Normal) explicitly.
type Level uint8

const (
	Awful Level = iota
	Poor
	Normal
	Good
	Excellent
	Masterwork
	Legendary
	numLevels
)

// Default is the level of items with no quality roll (bought, found).
const Default = Normal

var labels = [numLevels]string{"awful", "poor", "normal", "good", "excellent", "masterwork", "legendary"}

// String is the lowercase player-facing label.
func (l Level) String() string {
	if l >= numLevels {
		return "unknown"
	}
	return labels[l]
}

// Valid reports whether l is a defined level.
func (l Level) Valid() bool { return l < numLevels }

// All returns every level in ascending order.
func All() []Level {
	return []Level{Awful, Poor, Normal, Good, Excellent, Masterwork, Legendary}
}

// MaxSkill is the highest crafting skill the tables cover.
const MaxSkill = 20

// Stat is a quality-scaled stat category.
type Stat uint8

const (
	MarketValue Stat = iota
	Beauty
	Deterioration // rate multiplier: lower is better
	MeleeDamage   // damage and armor penetration
	RangedAccuracy
	RangedDamage // damage and armor penetration
	ApparelProtection
	Insulation
	Comfort // furniture
	RestEffectiveness
	SurgerySuccess // beds
	Recreation
	numStats
)

// Shares is the probability of each level in basis points (1/10000) for
// Awful..Masterwork; Legendary is 0 in the base table. Rows are the wiki's
// two-decimal percentages, so a row may sum to a few points off 10000;
// Roll normalises by the row total.
// wiki: Quality (crafting quality probability by skill)
type row [numLevels - 1]uint16

var baseRows = [MaxSkill + 1]row{
	{6462, 3018, 500, 20, 0, 0},     // skill 0
	{4339, 4359, 1215, 86, 1, 0},    // skill 1
	{2024, 5317, 2356, 295, 8, 0},   // skill 2
	{912, 5076, 3345, 638, 29, 0},   // skill 3
	{477, 4523, 3944, 995, 61, 0},   // skill 4
	{227, 3468, 4720, 1465, 119, 1}, // skill 5
	{98, 2428, 5211, 2040, 221, 2},  // skill 6
	{38, 1550, 5332, 2687, 387, 6},  // skill 7
	{13, 900, 5083, 3350, 639, 15},  // skill 8
	{5, 562, 4695, 3814, 898, 26},   // skill 9
	{2, 333, 4022, 4377, 1222, 44},  // skill 10
	{0, 186, 3222, 4905, 1613, 74},  // skill 11
	{0, 98, 2455, 5264, 2064, 119},  // skill 12
	{0, 62, 1991, 5393, 2393, 161},  // skill 13
	{0, 38, 1579, 5430, 2739, 214},  // skill 14
	{0, 23, 1225, 5376, 3094, 282},  // skill 15
	{0, 13, 929, 5240, 3450, 368},   // skill 16
	{0, 8, 688, 5031, 3798, 475},    // skill 17
	{0, 4, 498, 4761, 4131, 606},    // skill 18
	{0, 2, 353, 4266, 4616, 763},    // skill 19
	{0, 1, 244, 3742, 5061, 952},    // skill 20
}

// shares returns the per-level weights for a crafter. Inspired creativity
// raises the result two levels (wiki: Quality): Normal takes base Awful,
// Good base Poor, Excellent base Normal, Masterwork base Good, and
// Legendary base Excellent + Masterwork; Awful and Poor become 0.
func shares(skill int, inspired bool) (s [numLevels]int) {
	if skill < 0 {
		skill = 0
	}
	if skill > MaxSkill {
		skill = MaxSkill
	}
	r := baseRows[skill]
	if !inspired {
		for i, v := range r {
			s[i] = int(v)
		}
		return s
	}
	for i := 0; i < 4; i++ {
		s[i+2] = int(r[i])
	}
	s[Legendary] = int(r[4]) + int(r[5])
	return s
}

// Roll picks the quality a crafter of the given skill (clamped 0..20)
// produces. roll is any uniform uint32 from the caller's seeded stream.
func Roll(skill int, roll uint32, inspired bool) Level {
	s := shares(skill, inspired)
	total := 0
	for _, v := range s {
		total += v
	}
	p := int((uint64(roll) * uint64(total)) >> 32) // 0..total-1
	for l, v := range s {
		if p < v {
			return Level(l)
		}
		p -= v
	}
	return Legendary
}

// Share is the table weight in basis points (1/10000) of level l.
func Share(skill int, inspired bool, l Level) int {
	if !l.Valid() {
		return 0
	}
	return shares(skill, inspired)[l]
}

// factors[stat][level] in permille (1000 = unchanged). wiki: Quality.
// The wiki publishes no quality factor for work speed, hit points or
// weapon cooldown, so those stats are not modelled.
var factors = [numStats][numLevels]int{
	MarketValue:       {500, 750, 1000, 1250, 1500, 2500, 5000},
	Beauty:            {-100, 500, 1000, 2000, 3000, 5000, 8000},
	Deterioration:     {2000, 1500, 1000, 800, 600, 300, 100},
	MeleeDamage:       {800, 900, 1000, 1100, 1200, 1450, 1650},
	RangedAccuracy:    {800, 900, 1000, 1100, 1200, 1350, 1500},
	RangedDamage:      {900, 1000, 1000, 1000, 1000, 1250, 1500},
	ApparelProtection: {600, 800, 1000, 1150, 1300, 1450, 1800},
	Insulation:        {800, 900, 1000, 1100, 1200, 1500, 1800},
	Comfort:           {760, 880, 1000, 1120, 1240, 1450, 1700},
	RestEffectiveness: {860, 920, 1000, 1080, 1140, 1250, 1600},
	SurgerySuccess:    {900, 950, 1000, 1050, 1100, 1150, 1300},
	Recreation:        {760, 880, 1000, 1120, 1240, 1400, 1800},
}

// Factor is the permille multiplier quality l applies to stat s; 1000 for
// an unknown level or stat.
func Factor(l Level, s Stat) int {
	if !l.Valid() || s >= numStats {
		return 1000
	}
	return factors[s][l]
}

// Apply scales base by Factor(l, s).
func Apply(base int, l Level, s Stat) int { return base * Factor(l, s) / 1000 }

// Stored is l as the one-higher byte item stacks and buildings keep
// (item.Stack.Quality, construction.Building.Quality): 0 means none.
func Stored(l Level) uint8 { return uint8(l) + 1 }

// FromStored reads a stored quality byte; ok is false for 0 (none).
func FromStored(b uint8) (Level, bool) {
	if b == 0 || !Level(b-1).Valid() {
		return Default, false
	}
	return Level(b - 1), true
}
