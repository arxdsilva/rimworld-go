package mood

import "rimworld-go/internal/domain/sim/clock"

// Kind is a thought type.
type Kind uint8

const (
	SleptOutside Kind = iota
	SleptOnGround
	SleptInBed
	SleptInAwfulRoom
	SleptInNiceRoom
	AteMeal
	AteRaw
	AteAtTable
	AteWithoutTable
	Hungry
	Tired
	Cold
	Freezing
	Hot
	Sweltering
	RecreationStarved
	Bored
	Entertained
	Catharsis
	// Sick: fighting an infection. InPain: badly hurt.
	Sick
	InPain
	// Death and burial (SIM-017) and weather (SIM-016).
	ColonistDied
	UnburiedCorpse
	LaidToRest
	Soaked
	SnowDay
	// NG-1: arrival by drop pod, doing passion work, the difficulty's mood
	// offset (value comes from settings; 0 hides it).
	Landed
	LovesWork
	Difficulty
	// Social and host systems (ENG-003, append only, saved): social
	// interactions, romance, gatherings, drugs, animal bonds, prisoners,
	// the body and grief. Grief and PartnerOpinion carry their magnitude in
	// Memory.Value (the def's Value is 0).
	Insulted
	KindWords
	RebuffedMood
	HadToRebuff
	CheatedOn
	ProposalRejected
	GotMarried
	AttendedWedding
	AttendedParty
	BrokenUpWith
	Divorced
	Lovin
	PartnerOpinion
	SleptAlone
	Hungover
	Drunk
	Withdrawal
	HighOnDrug
	BondedMaster
	BondedNotMaster
	BondedDied
	BondedLost
	BondedGivenAway
	BondedReleased
	Imprisoned
	ColonistLost
	DisfiguredSelf
	MissingPart
	Surgery
	Grief
	// Organ harvesting (MED-06, append only, saved): the donor's own
	// memory, and what the colony thinks of it: an outsider's organ
	// (a prisoner or guest) or a colonist's.
	OrganHarvested
	KnowOrganHarvested
	KnowColonistOrganHarvested
	// Room impressiveness (BED-2, append only, saved): the stage's value
	// and label ride in Memory.Value and Memory.Label (game/rest
	// RoomMood); sharing a bed carries its opinion-scaled value.
	SleptInBedroom
	SleptInBarracks
	SleptInHospital
	AteInDiningRoom
	EnjoyedRecRoom
	SharedBed
	// Apparel condition (PRD-2, append only, saved): state thoughts of
	// the worst worn piece's wear and of tainted pieces (craft/garment.go;
	// TaintedApparel's value comes by count).
	RattyApparel
	TatteredApparel
	TaintedApparel
	// Comfort, beauty and outdoors needs and the temperature slept in
	// (BED-3, append only, saved): the need thoughts carry their stage's
	// value and label in Memory.Value and Memory.Label (game/rest
	// needs.go) and are refreshed while the stage holds.
	ComfortMood
	BeautyMood
	OutdoorsMood
	SleptCold
	SleptHot
	// AtePaste (STO-2, append only): ate a nutrient paste meal (wiki:
	// -4 for a day, in the meal-kind group).
	AtePaste
	numKinds
)

// Def is a thought's label, mood effect and duration (0 = state thought,
// recomputed every tick instead of remembered).
//
// StackLimit and StackFactor (ENG-003) are how many memories of the kind
// may stack and the permille weight of each further one; zero means the
// default, 1 and 1000 (no stacking).
type Def struct {
	Label       string
	Value       int
	Duration    uint64
	Group       uint8 // thoughts sharing a non-zero group replace each other
	StackLimit  uint8
	StackFactor int32
}

const (
	groupNone uint8 = iota
	groupSleepPlace
	groupSleepRoom
	groupMealKind
	groupMealTable
	groupLaidToRest
	groupSoaked
	groupSnowDay
	groupLovesWork
	groupDiningRoom // BED-2
	groupRecRoom
	groupSharedBed
	groupComfort // BED-3
	groupBeauty
	groupOutdoors
	groupSleptTemp
)

// day: remembered thoughts last one in-game day.
const day = uint64(clock.TicksPerDay)

var defs = [numKinds]Def{
	SleptOutside:      {"slept outside", -6, day, groupSleepPlace, 0, 0},
	SleptOnGround:     {"slept on the ground", -4, day, groupSleepPlace, 0, 0},
	SleptInBed:        {"slept in a bed", 2, day, groupSleepPlace, 0, 0},
	SleptInAwfulRoom:  {"slept in an awful room", -4, day, groupSleepRoom, 0, 0},
	SleptInNiceRoom:   {"slept in a nice room", 5, day, groupSleepRoom, 0, 0},
	AteMeal:           {"ate a cooked meal", 4, day, groupMealKind, 0, 0},
	AteRaw:            {"ate raw food", -5, day, groupMealKind, 0, 0},
	AteAtTable:        {"ate at a table", 3, day, groupMealTable, 0, 0},
	AteWithoutTable:   {"ate without a table", -3, day, groupMealTable, 0, 0},
	Hungry:            {"hungry", -8, 0, groupNone, 0, 0},
	Tired:             {"tired", -5, 0, groupNone, 0, 0},
	Cold:              {"cold", -4, 0, groupNone, 0, 0},
	Freezing:          {"freezing", -10, 0, groupNone, 0, 0},
	Hot:               {"hot", -4, 0, groupNone, 0, 0},
	Sweltering:        {"sweltering", -10, 0, groupNone, 0, 0},
	RecreationStarved: {"recreation-starved", -12, 0, groupNone, 0, 0},
	Bored:             {"bored", -5, 0, groupNone, 0, 0},
	Entertained:       {"entertained", 4, 0, groupNone, 0, 0},
	Catharsis:         {"catharsis", 30, day, groupNone, 0, 0},
	Sick:              {"sick", -8, 0, groupNone, 0, 0},
	InPain:            {"in pain", -6, 0, groupNone, 0, 0},
	ColonistDied:      {"a colonist died", -3, 6 * day, groupNone, 0, 0},
	UnburiedCorpse:    {"unburied corpse nearby", -2, 0, groupNone, 0, 0},
	LaidToRest:        {"laid a friend to rest", 3, day, groupLaidToRest, 0, 0},
	Soaked:            {"soaked", -3, day / 4, groupSoaked, 0, 0},
	SnowDay:           {"snow day", 2, day, groupSnowDay, 0, 0},
	Landed:            {"rough landing", -4, day, groupNone, 0, 0},
	LovesWork:         {"doing work I love", 2, 0, groupLovesWork, 0, 0},
	Difficulty:        {"the way of the world", 0, 0, groupNone, 0, 0},
	// ENG-003: values follow the published RimWorld wiki by explicit user
	// decision (balance override); labels are original. "stack" kinds
	// stack up to 5 at 900 permille each.
	Insulted:         {"insulted", -5, 2 * day, groupNone, stackLimit, stackFactor},
	KindWords:        {"kind words", 5, 2 * day, groupNone, stackLimit, stackFactor},
	RebuffedMood:     {"rebuffed", -5, 3 * day, groupNone, 0, 0},
	HadToRebuff:      {"had to turn someone down", -3, 3 * day, groupNone, 0, 0},
	CheatedOn:        {"cheated on", -20, 25 * day, groupNone, 0, 0},
	ProposalRejected: {"proposal turned down", -18, 25 * day, groupNone, 0, 0},
	GotMarried:       {"got married", 40, 30 * day, groupNone, 0, 0},
	AttendedWedding:  {"attended a wedding", 20, 10 * day, groupNone, 0, 0},
	AttendedParty:    {"attended a party", 8, 10 * day, groupNone, 0, 0},
	BrokenUpWith:     {"was dumped", -15, 25 * day, groupNone, 0, 0},
	Divorced:         {"divorced", -20, 30 * day, groupNone, 0, 0},
	Lovin:            {"intimacy", 8, 3 * day, groupNone, stackLimit, stackFactor},
	PartnerOpinion:   {"feelings for my partner", 0, 0, groupNone, 0, 0},
	SleptAlone:       {"sleeping alone", -4, 0, groupNone, 0, 0},
	Hungover:         {"hungover", -5, day / 2, groupNone, 0, 0},
	Drunk:            {"tipsy", 6, 0, groupNone, 0, 0},
	Withdrawal:       {"withdrawal", -15, 0, groupNone, 0, 0},
	HighOnDrug:       {"high", 10, 0, groupNone, 0, 0},
	BondedMaster:     {"my bonded animal", 5, 0, groupNone, 0, 0},
	BondedNotMaster:  {"kept from my bonded animal", -3, 0, groupNone, 0, 0},
	BondedDied:       {"bonded animal died", -8, 20 * day, groupNone, stackLimit, stackFactor},
	BondedLost:       {"bonded animal lost", -8, 20 * day, groupNone, stackLimit, stackFactor},
	BondedGivenAway:  {"gave my bonded animal away", -10, 60 * day, groupNone, stackLimit, stackFactor},
	BondedReleased:   {"released my bonded animal", -5, 15 * day, groupNone, stackLimit, stackFactor},
	Imprisoned:       {"imprisoned", -10, 0, groupNone, 0, 0},
	ColonistLost:     {"a colonist went missing", -3, 6 * day, groupNone, 0, 0},
	DisfiguredSelf:   {"disfigured", 0, 0, groupNone, 0, 0},
	MissingPart:      {"missing a body part", -5, 0, groupNone, 0, 0},
	Surgery:          {"had surgery", -3, day, groupNone, 0, 0},
	Grief:            {"grief", 0, 0, groupNone, 0, 0},
	// MED-06: wiki: Human resources (donor −30 for 7 days; the colony −5,
	// −6 for a colonist's organ, stacking 5 at 75 %). The 6-day memory
	// of the colony is original tuning.
	OrganHarvested:             {"my organ was harvested", -30, 7 * day, groupNone, 0, 0},
	KnowOrganHarvested:         {"someone's organ was harvested", -5, 6 * day, groupNone, harvestStack, harvestFactor},
	KnowColonistOrganHarvested: {"a colonist's organ was harvested", -6, 6 * day, groupNone, harvestStack, harvestFactor},
	// BED-2, wiki: Thoughts (room stage thoughts and sharing a bed last a
	// day). Values come per memory; SharedBed's -5 is the opinion-0 value.
	SleptInBedroom:  {"slept in my bedroom", 0, day, groupSleepRoom, 0, 0},
	SleptInBarracks: {"slept in barracks", 0, day, groupSleepRoom, 0, 0},
	SleptInHospital: {"lay in a hospital", 0, day, groupSleepRoom, 0, 0},
	AteInDiningRoom: {"ate in a fine dining room", 0, day, groupDiningRoom, 0, 0},
	EnjoyedRecRoom:  {"relaxed in a fine recreation room", 0, day, groupRecRoom, 0, 0},
	SharedBed:       {"had to share a bed", -5, day, groupSharedBed, 0, 0},
	// PRD-2, wiki: Apparel (ratty -3, tattered -5; tainted -5/-8/-11/-14
	// by count, the value set per thought).
	RattyApparel:    {"ratty apparel", -3, 0, groupNone, 0, 0},
	TatteredApparel: {"tattered apparel", -5, 0, groupNone, 0, 0},
	TaintedApparel:  {"wearing a dead man's clothes", -5, 0, groupNone, 0, 0},
	// BED-3: need thoughts last an hour past their last refresh (values
	// and labels per memory, wiki: Comfort, Beauty, Outdoors); slept in
	// the cold or heat -4 for a day (wiki: Thoughts).
	ComfortMood:  {"comfort", 0, needHour, groupComfort, 0, 0},
	BeautyMood:   {"surroundings", 0, needHour, groupBeauty, 0, 0},
	OutdoorsMood: {"time indoors", 0, needHour, groupOutdoors, 0, 0},
	SleptCold:    {"slept in the cold", -4, day, groupSleptTemp, 0, 0},
	SleptHot:     {"slept in the heat", -4, day, groupSleptTemp, 0, 0},
	AtePaste:     {"ate nutrient paste", -4, day, groupMealKind, 0, 0},
}

// needHour is how long a need thought outlasts its last refresh (BED-3).
const needHour = uint64(clock.TicksPerHour)

// harvestStack and harvestFactor stack the colony's harvest memories:
// five at 75 % each further one. wiki: Human resources.
const (
	harvestStack  = 5
	harvestFactor = 750
)

// stackLimit and stackFactor are the "stack" kinds' StackLimit and
// StackFactor (ENG-003).
const (
	stackLimit  = 5
	stackFactor = 900
)

// DefOf returns the thought definition.
func DefOf(k Kind) Def {
	if k >= numKinds {
		return Def{}
	}
	d := defs[k]
	if d.StackLimit == 0 {
		d.StackLimit = 1
	}
	if d.StackFactor == 0 {
		d.StackFactor = 1000
	}
	return d
}

// Base is mood with no thoughts.
const Base = 50

// LowMood is the threshold below which work slows down.
const LowMood = 30

// Memory is a remembered thought that expires. Subject is the pawn or
// animal it is about (0: none) and Value overrides the def's mood effect
// (0: use Def.Value); both ENG-003, zero in older saves. Label overrides
// the def's label ("": use Def.Label), e.g. "my wife died" for a grief.
type Memory struct {
	Kind    Kind
	Expires uint64
	Subject uint64
	Value   int
	Label   string
}

// Memories is a pawn's remembered thoughts (one per kind, newest wins).
type Memories []Memory

// Add remembers k at tick now, replacing an existing one of the same kind
// and any other thought of the same group.
func (m Memories) Add(k Kind, now uint64) Memories {
	if g := DefOf(k).Group; g != groupNone {
		out := m[:0]
		for _, x := range m {
			if x.Kind == k || DefOf(x.Kind).Group != g {
				out = append(out, x)
			}
		}
		m = out
	}
	exp := now + DefOf(k).Duration
	for i := range m {
		if m[i].Kind == k {
			m[i].Expires = exp
			return m
		}
	}
	return append(m, Memory{Kind: k, Expires: exp})
}

// AddFor remembers k about subject at tick now. Memories of the same kind
// and subject stack up to Def.StackLimit; beyond it the oldest is
// refreshed. Group replacement works as in Add.
func (m Memories) AddFor(k Kind, subject uint64, now uint64) Memories {
	return m.addMemory(k, subject, 0, "", now)
}

// AddValue is AddFor with a computed mood effect (grief and the like).
func (m Memories) AddValue(k Kind, subject uint64, value int, now uint64) Memories {
	return m.addMemory(k, subject, value, "", now)
}

// AddLabeled is AddValue with the line's own label ("": the def's).
func (m Memories) AddLabeled(k Kind, subject uint64, value int, label string, now uint64) Memories {
	return m.addMemory(k, subject, value, label, now)
}

func (m Memories) addMemory(k Kind, subject uint64, value int, label string, now uint64) Memories {
	d := DefOf(k)
	if d.Group != groupNone {
		out := m[:0]
		for _, x := range m {
			if x.Kind == k || DefOf(x.Kind).Group != d.Group {
				out = append(out, x)
			}
		}
		m = out
	}
	exp := now + d.Duration
	n, oldest := 0, -1
	for i := range m {
		if m[i].Kind != k || m[i].Subject != subject {
			continue
		}
		n++
		if oldest < 0 || m[i].Expires < m[oldest].Expires {
			oldest = i
		}
	}
	if n >= int(d.StackLimit) && oldest >= 0 {
		m[oldest].Expires = exp
		m[oldest].Value = value
		m[oldest].Label = label
		return m
	}
	return append(m, Memory{Kind: k, Expires: exp, Subject: subject, Value: value, Label: label})
}

// Approach moves the displayed mood (milli-points) toward the target over
// ticks: at most 12 points per hour up and 8 per hour down.
func Approach(curMilli, targetMilli int32, ticks uint64) int32 {
	switch {
	case targetMilli > curMilli:
		step := int64(RiseMilliPerHour) * int64(ticks) / TicksPerHour
		if int64(targetMilli-curMilli) <= step {
			return targetMilli
		}
		return curMilli + int32(step)
	case targetMilli < curMilli:
		step := int64(FallMilliPerHour) * int64(ticks) / TicksPerHour
		if int64(curMilli-targetMilli) <= step {
			return targetMilli
		}
		return curMilli - int32(step)
	}
	return curMilli
}

// Approach rates (milli-points per hour) and the hour length in ticks.
const (
	RiseMilliPerHour = 12000
	FallMilliPerHour = 8000
	TicksPerHour     = 2500
)

// ForgetGroupOf drops memories in k's group (e.g. room quality after a
// night in an ordinary room).
func (m Memories) ForgetGroupOf(k Kind) Memories {
	g := DefOf(k).Group
	out := m[:0]
	for _, x := range m {
		if g == groupNone || DefOf(x.Kind).Group != g {
			out = append(out, x)
		}
	}
	return out
}

// Prune drops expired memories.
func (m Memories) Prune(now uint64) Memories {
	out := m[:0]
	for _, x := range m {
		if x.Expires > now {
			out = append(out, x)
		}
	}
	return out
}

// Thought is one active line with its effect.
type Thought struct {
	Kind  Kind
	Label string
	Value int
}

// Active lists memories plus current state thoughts.
func Active(m Memories, states ...Kind) []Thought {
	return AppendActive(nil, m, states...)
}

// AppendActive appends Active(m, states...) to out.
func AppendActive(out []Thought, m Memories, states ...Kind) []Thought {
	var seen [numKinds]int
	for _, x := range m {
		d := DefOf(x.Kind)
		v := d.Value
		if x.Value != 0 {
			v = x.Value
		}
		if x.Kind < numKinds {
			for i := 0; i < seen[x.Kind]; i++ {
				v = v * int(d.StackFactor) / 1000
			}
			seen[x.Kind]++
		}
		label := d.Label
		if x.Label != "" {
			label = x.Label
		}
		out = append(out, Thought{x.Kind, label, v})
	}
	for _, k := range states {
		d := DefOf(k)
		out = append(out, Thought{k, d.Label, d.Value})
	}
	return out
}

// Level sums thoughts onto Base and clamps to 0..100.
func Level(ts []Thought) int {
	v := Base
	for _, t := range ts {
		v += t.Value
	}
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// Label is a player-facing word for a mood level.
func Label(v int) string {
	switch {
	case v < 20:
		return "miserable"
	case v < LowMood:
		return "unhappy"
	case v < 55:
		return "content"
	case v < 75:
		return "happy"
	}
	return "joyful"
}
