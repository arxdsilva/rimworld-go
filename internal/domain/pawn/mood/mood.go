// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package mood

import (
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/pkg/randx"
)

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
	// FOOD-0 contract (append only, saved; nothing remembers them before
	// FOOD-1 and FOOD-4): the better meals and kibble (meal-kind group),
	// eating human or insect meat, butchering a human (the butcher) and
	// knowing one was butchered (everyone else), and the hunger stages
	// below Hungry (state thoughts).
	AteFineMeal
	AteLavishMeal
	AteKibble
	AteHumanMeat
	AteInsectMeat
	ButcheredHuman
	ObservedButchery
	Ravenous
	Malnourished
	// FOOD-4 (append only, saved): a cooked meal with human or insect
	// meat among its ingredients (item.Stack.Ingredients).
	AteHumanMeatCooked
	AteInsectMeatCooked
	// FOOD-6 (append only, saved): under the open sky in toxic fallout
	// (a state thought).
	ToxicFallout
	// TEMP-0 contract (append only, saved; nothing thinks them before
	// TEMP-3 and TEMP-4): the four-stage cold and hot ladders (state
	// thoughts; a pawn holds at most one rung of each, groupTempCold and
	// groupTempHot) and wearing nothing on torso or legs. TEMP-3 moves the
	// temperature thoughts (rooms.go) from the two-stage Cold, Freezing,
	// Hot and Sweltering, which stay valid kinds for older saves, to these.
	Chilly
	Shivering
	NumbingCold
	AbsolutelyFreezing
	Sweaty
	Overheated
	Baking
	BlisteringHot
	Naked
	// TurnedAwayWanderer (QST-173, append only, saved): the colony turned a
	// wanderer's offer to join away.
	TurnedAwayWanderer
	// RescuedByColony (QST-057, QST-068, append only, saved): the colony
	// came for this pawn, down on a site or held in a cell, and took them in.
	RescuedByColony
	BrawlerRanged // Brawler equipped a ranged weapon (state thought)
	ExpectationsExtremelyLow
	ExpectationsVeryLow
	ExpectationsLow
	ExpectationsModerate
	ExpectationsHigh
	ExpectationsSkyHigh
	BlissCircuit
	AugmentFan
	NaturalPurist
	AnestheticWoozy // anaesthetic recovery state (append-only)
	RecreationDeprived
	RecreationSatisfied
	FatalOrganHarvest
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
	groupTempCold // TEMP-0
	groupTempHot
)

// day: remembered thoughts last one in-game day.
const day = uint64(clock.TicksPerDay)

// Definitions are maintained in definitions.json and compiled into static Go.
//go:generate python3 ../../../../scripts/gen_mood_defs.py

// needHour is how long a need thought outlasts its last refresh (BED-3).
const needHour = uint64(clock.TicksPerHour)

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
const Base = 32

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

// Memories holds a pawn's timed thoughts; subject-specific kinds may stack.
type Memories []Memory

// Add remembers k at tick now, replacing an existing one of the same kind
// and any other thought of the same group.
func (m Memories) Add(k Kind, now uint64) Memories {
	d := DefOf(k)
	if d.StackLimit > 1 {
		return m.AddLabeledUntil(k, 0, 0, "", now, now+d.Duration)
	}
	m = m.compactFor(k, d.Group, now)
	for i := range m {
		if m[i].Kind == k {
			m[i].Expires = now + d.Duration
			return m
		}
	}
	return append(m, Memory{Kind: k, Expires: now + d.Duration})
}

func (m Memories) compactFor(k Kind, group uint8, now uint64) Memories {
	out := m[:0]
	for _, x := range m {
		if x.Expires <= now || group != groupNone && x.Kind != k && DefOf(x.Kind).Group == group {
			continue
		}
		out = append(out, x)
	}
	clear(m[len(out):])
	return out
}

// AddFor remembers k about subject at tick now. Memories of the same kind
// and subject stack up to Def.StackLimit; beyond it the oldest is
// refreshed. Group replacement works as in Add.
func (m Memories) AddFor(k Kind, subject uint64, now uint64) Memories {
	return m.AddLabeled(k, subject, 0, "", now)
}

// AddValue is AddFor with a computed mood effect (grief and the like).
func (m Memories) AddValue(k Kind, subject uint64, value int, now uint64) Memories {
	return m.AddLabeled(k, subject, value, "", now)
}

// AddLabeled is AddValue with the line's own label ("": the def's).
func (m Memories) AddLabeled(k Kind, subject uint64, value int, label string, now uint64) Memories {
	return m.AddLabeledUntil(k, subject, value, label, now, now+DefOf(k).Duration)
}

// AddLabeledUntil records an event with an explicit expiry tick.
func (m Memories) AddLabeledUntil(k Kind, subject uint64, value int, label string, now, expires uint64) Memories {
	d := DefOf(k)
	m = m.compactFor(k, d.Group, now)
	n, oldest := 0, -1
	for i := range m {
		if m[i].Kind != k || d.StackLimit == 1 && m[i].Subject != subject {
			continue
		}
		n++
		if oldest < 0 || m[i].Expires < m[oldest].Expires {
			oldest = i
		}
	}
	fresh := Memory{Kind: k, Expires: expires, Subject: subject, Value: value, Label: label}
	if n >= int(d.StackLimit) && oldest >= 0 {
		if d.StackLimit == 1 {
			m[oldest] = fresh
			return m
		}
		// The refreshed event is newest; stacking weights follow event order.
		copy(m[oldest:], m[oldest+1:])
		m[len(m)-1] = fresh
		return m
	}
	return append(m, fresh)
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
	clear(m[len(out):])
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
	clear(m[len(out):])
	return out
}

// ForgetOne drops one currently active memory. A fully expired set remains
// untouched until normal cleanup, keeping the original hourly save semantics.
func ForgetOne(m Memories, now uint64, roll func(int) int) Memories {
	var found bool
	for _, x := range m {
		if x.Expires > now {
			found = true
			break
		}
	}
	if !found {
		return m
	}
	active := m.Prune(now)
	i := roll(len(active))
	return append(active[:i], active[i+1:]...)
}

// ForgetSubject removes memories about one person or animal after the
// relationship event is resolved. It clears removed labels and references.
func (m Memories) ForgetSubject(k Kind, subject uint64) Memories {
	out := m[:0]
	for _, x := range m {
		if x.Kind == k && x.Subject == subject {
			continue
		}
		out = append(out, x)
	}
	clear(m[len(out):])
	return out
}

// Thought is one active line with its effect.
type Thought struct {
	Kind   Kind
	Label  string
	Value  int
	Memory bool // a live event memory, even when its definition has no default duration
}

// NullifyConditional applies late-stage memory impairment to current
// state thoughts only. Saved memories stay intact, and the decision is
// stable for each pawn, thought and hour across replay and save/load.
func NullifyConditional(thoughts []Thought, chance int, seed, pawn, hour uint64) []Thought {
	if chance == 0 {
		return thoughts
	}
	n := 0
	for _, th := range thoughts {
		if !th.Memory {
			salt := randx.SplitMix64(seed ^ randx.SplitMix64(pawn) ^ uint64(th.Kind)<<32 ^ hour)
			if int(salt%10000) < chance {
				continue
			}
		}
		thoughts[n] = th
		n++
	}
	return thoughts[:n]
}

// Active lists memories plus current state thoughts.
func Active(m Memories, states ...Kind) []Thought {
	return AppendActive(nil, m, states...)
}

// AppendActive appends Active(m, states...) to out.
func AppendActive(out []Thought, m Memories, states ...Kind) []Thought {
	return appendActive(out, m, 0, false, states...)
}

// AppendActiveAt lists thoughts whose memories are still active at now.
// It leaves the backing memory slice untouched.
func AppendActiveAt(out []Thought, m Memories, now uint64, states ...Kind) []Thought {
	return appendActive(out, m, now, true, states...)
}

const stackPrecision int64 = 1_000_000_000

func stackRounded(v int64) int {
	if v < 0 {
		return -int((-v + stackPrecision/2) / stackPrecision)
	}
	return int((v + stackPrecision/2) / stackPrecision)
}

func appendActive(out []Thought, m Memories, now uint64, checkExpiry bool, states ...Kind) []Thought {
	var counts [numKinds]uint8
	var weights, totals [numKinds]int64
	var shown [numKinds]int
	for _, x := range m {
		if checkExpiry && x.Expires <= now {
			continue
		}
		d := DefOf(x.Kind)
		v := d.Value
		if x.Value != 0 {
			v = x.Value
		}
		if x.Kind < numKinds && d.StackLimit > 1 {
			k := x.Kind
			if counts[k] >= d.StackLimit {
				v = 0 // tolerate old saves with more memories than today's cap
			} else {
				w := weights[k]
				if w == 0 {
					w = stackPrecision
				} else {
					w = w * int64(d.StackFactor) / 1000
				}
				weights[k] = w
				totals[k] += int64(v) * w
				n := stackRounded(totals[k])
				v, shown[k] = n-shown[k], n
				counts[k]++
			}
		}
		label := d.Label
		if x.Label != "" {
			label = x.Label
		}
		out = append(out, Thought{Kind: x.Kind, Label: label, Value: v, Memory: true})
	}
	for _, k := range states {
		d := DefOf(k)
		out = append(out, Thought{Kind: k, Label: d.Label, Value: d.Value})
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
