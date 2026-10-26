package drug

import (
	"slices"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/clock"
)

// Values follow the published RimWorld wiki by explicit user decision (ENG-003 balance override).

// numDrugs is how many drugs there are (beer, ashleaf tea, spark tablet;
// MED-07: numbroot tincture and prophyl tablet).
const numDrugs = 5

// LegacyDrugs is the length of the per-drug arrays saves held before
// MED-00 (Tolerance and LastDose are slices now, so drugs can be appended).
const LegacyDrugs = 3

// Tuning (permille unless noted).
const (
	toleranceMax       = 1000
	toleranceDecayHour = 10 // per hour
	needDecayHour      = 21 // 1000 -> 0 in ~48 hours
	recoveryDays       = 10
	hangoverTicks      = clock.TicksPerDay / 2
	dayTicks           = clock.TicksPerDay
	hourTicks          = clock.TicksPerHour
)

// Def describes one drug.
type Def struct {
	Kind          item.Kind
	Short         string // short UI name
	Alcohol       int32  // alcohol severity per dose (permille)
	Joy           int32  // joy gained per dose (permille of the joy bar)
	HighHours     int    // how long the high lasts
	RestBoost     int32  // rest restored per dose (permille of the rest bar)
	AddictChance  int32  // addiction chance per dose before tolerance (permille)
	ToleranceGain int32  // tolerance added per dose (permille)
	WithdrawFight int32  // fight multiplier while in withdrawal (permille)
	Recreational  bool   // taken for joy (policies gate these)

	// Medical drugs (MED-07). Painkiller: taken when in pain; the game
	// puts its effect on the body (body.CondPainkiller). Protects: while
	// its high lasts the colonist cannot catch plague, malaria or sleeping
	// sickness (Protected). ScheduleDays: a policy that allows it has it
	// retaken every so many days. OverdoseMin..OverdoseMax is the overdose
	// severity one dose adds (permille; 0: none), rolled by the game.
	Painkiller               bool
	Protects                 bool
	ScheduleDays             int
	OverdoseMin, OverdoseMax int32
}

// Medical drug tuning (MED-07).
const (
	// PainkillerAt is the pain (permille) at which a policy has a
	// colonist take a painkiller. It is taken again only once the last
	// dose has worn off and the pain is still this high (the effect's
	// duration is the hysteresis). Original tuning (MED-07 plan).
	PainkillerAt = 400
	// ProphylDays is how long a prophyl tablet protects, and how often a
	// policy has it retaken. wiki: Penoxycyline (every 5 days).
	ProphylDays = 5
)

var defs = [numDrugs]Def{
	{Kind: item.Beer, Short: "beer", Alcohol: 150, Joy: 200, AddictChance: 10, ToleranceGain: 100, WithdrawFight: 1000, Recreational: true},
	{Kind: item.AshleafTea, Short: "ashleaf", Joy: 300, HighHours: 12, AddictChance: 30, ToleranceGain: 150, WithdrawFight: 3000, Recreational: true},
	// MED-07: the spark tablet is a hard drug, so a dose risks an
	// overdose: +180..350 severity. wiki: Overdose (hard drugs).
	{Kind: item.SparkTablet, Short: "spark", Joy: 150, HighHours: 6, RestBoost: 400, AddictChance: 60, ToleranceGain: 200, WithdrawFight: 2000, Recreational: true,
		OverdoseMin: 180, OverdoseMax: 350},
	// MED-07: numbroot tincture, a mild, non-addictive painkiller (pain
	// x0.5 for 12 h, body.CondPainkiller; original tuning: the wiki gives
	// no plain painkiller numbers). Prophyl tablet: protection for five
	// days, retaken on schedule, +80..140 overdose per dose. wiki:
	// Penoxycyline, Overdose.
	{Kind: item.NumbrootTincture, Short: "numbroot", Painkiller: true},
	{Kind: item.ProphylTablet, Short: "prophyl", HighHours: ProphylDays * 24, Protects: true, ScheduleDays: ProphylDays,
		OverdoseMin: 80, OverdoseMax: 140},
}

func index(k item.Kind) int {
	switch k {
	case item.Beer:
		return 0
	case item.AshleafTea:
		return 1
	case item.SparkTablet:
		return 2
	case item.NumbrootTincture:
		return 3
	case item.ProphylTablet:
		return 4
	}
	return -1
}

// DefOf is the definition of drug k.
func DefOf(k item.Kind) (Def, bool) {
	i := index(k)
	if i < 0 {
		return Def{}, false
	}
	return defs[i], true
}

// All lists every drug definition in a fixed order.
func All() []Def { return append([]Def(nil), defs[:]...) }

// High is a drug's effect wearing off.
type High struct {
	Drug     item.Kind
	Severity int32 // permille
	Until    uint64
}

// Addiction is a dependence on a drug and its unmet need.
type Addiction struct {
	Drug       item.Kind
	Severity   int32 // permille
	Need       int32 // permille; 1000 just dosed, 0 in withdrawal
	Withdrawal bool
	Seeking    bool   // craving hysteresis flag
	CleanSince uint64 // tick of the last dose
}

// State is one colonist's drug state (saved). Tolerance and LastDose are
// per drug in definition order, grown lazily (a missing slot reads 0):
// read them through ToleranceOf and LastDoseOf.
type State struct {
	Alcohol    int32 // permille
	Highs      []High
	Addictions []Addiction
	Tolerance  []int32 // permille per drug (nil: none)
	Hangover   uint64  // tick the hangover ends (0: none)

	Doses    int32    // alcohol doses since last sober
	Peaked   bool     // alcohol reached the drunk band
	HangPend bool     // a hangover began and was not yet read
	LastDose []uint64 // tick of last dose per drug, 0 = never

	// Policy is the colonist's drug policy. The zero value is NoDrugs, so
	// whoever creates a State sets the default (SocialDrinking).
	Policy Preset
	// Retry: no autonomous drug search before this tick (the game's
	// cooldown when no allowed drug was in reach).
	Retry uint64
}

// LegacyLists are the Tolerance and LastDose slices of an older save's
// fixed-size arrays (nil when all zero; trailing zero slots dropped).
func LegacyLists(tol [LegacyDrugs]int32, last [LegacyDrugs]uint64) ([]int32, []uint64) {
	return trimZeros(tol[:]), trimZeros(last[:])
}

// trimZeros is a copy of a without its trailing zeros (nil when none are
// left).
func trimZeros[T int32 | uint64](a []T) []T {
	n := len(a)
	for n > 0 && a[n-1] == 0 {
		n--
	}
	if n == 0 {
		return nil
	}
	return append([]T(nil), a[:n]...)
}

// grow extends a with zeros so slot i exists.
func grow[T int32 | uint64](a []T, i int) []T {
	if i < len(a) {
		return a
	}
	return append(a, make([]T, i+1-len(a))...)
}

// slot is a[i], 0 when a is too short.
func slot[T int32 | uint64](a []T, i int) T {
	if i < 0 || i >= len(a) {
		return 0
	}
	return a[i]
}

// ToleranceOf is the tolerance to drug k (permille; 0 for a non-drug).
func (s *State) ToleranceOf(k item.Kind) int32 { return slot(s.Tolerance, index(k)) }

// LastDoseOf is the tick of the last dose of drug k (0: never).
func (s *State) LastDoseOf(k item.Kind) uint64 { return slot(s.LastDose, index(k)) }

// Ingest takes one dose of drug k at tick now. roll(n) returns a value in
// [0, n) from the caller's drug RNG stream.
func (s *State) Ingest(k item.Kind, roll func(int) int, now uint64) {
	i := index(k)
	if i < 0 {
		return
	}
	d := defs[i]
	if d.Alcohol > 0 {
		s.Alcohol += d.Alcohol
		s.Doses++
		if s.Alcohol >= drunkAt {
			s.Peaked = true
		}
	}
	if d.HighHours > 0 {
		until := now + uint64(d.HighHours)*hourTicks
		found := false
		for j := range s.Highs {
			if s.Highs[j].Drug == k {
				s.Highs[j].Until, s.Highs[j].Severity, found = until, 1000, true
			}
		}
		if !found {
			s.Highs = append(s.Highs, High{Drug: k, Severity: 1000, Until: until})
		}
	}
	s.LastDose = grow(s.LastDose, i)
	s.LastDose[i] = max(now, 1)
	// Already hooked: the dose satisfies the need.
	if a := s.addiction(k); a != nil {
		a.Need, a.Withdrawal, a.Seeking, a.CleanSince = 1000, false, false, now
	} else if roll != nil && d.AddictChance > 0 {
		chance := d.AddictChance * (1000 + slot(s.Tolerance, i)) / 1000
		if int32(roll(1000)) < chance {
			s.Addictions = append(s.Addictions, Addiction{Drug: k, Severity: 1000, Need: 1000, CleanSince: now})
		}
	}
	if d.ToleranceGain > 0 {
		s.Tolerance = grow(s.Tolerance, i)
		s.Tolerance[i] = min(s.Tolerance[i]+d.ToleranceGain, toleranceMax)
	}
}

// Protected reports a protecting drug (prophyl) still acting at tick now:
// no plague, malaria or sleeping sickness can strike (MED-07).
func (s *State) Protected(now uint64) bool {
	if s == nil {
		return false
	}
	for _, h := range s.Highs {
		if d, ok := DefOf(h.Drug); ok && d.Protects && h.Until > now {
			return true
		}
	}
	return false
}

func (s *State) addiction(k item.Kind) *Addiction {
	for i := range s.Addictions {
		if s.Addictions[i].Drug == k {
			return &s.Addictions[i]
		}
	}
	return nil
}

// Addicted reports whether the colonist is addicted to k.
func (s *State) Addicted(k item.Kind) bool { return s.addiction(k) != nil }

// Tick advances the state by hours; now is the tick at the end.
func (s *State) Tick(hours int, now uint64) {
	for h := 0; h < hours; h++ {
		s.tickAlcohol(now)
		for i := range s.Tolerance {
			s.Tolerance[i] = max(s.Tolerance[i]-toleranceDecayHour, 0)
		}
		if s.Tolerance != nil && slices.Max(s.Tolerance) == 0 {
			s.Tolerance = nil // all gone: nothing to save
		}
		for i := range s.Addictions {
			a := &s.Addictions[i]
			if a.Need > 0 {
				a.Need = max(a.Need-needDecayHour, 0)
			}
			if a.Need == 0 {
				a.Withdrawal = true
			}
			if a.Need < craveStart {
				a.Seeking = true
			} else if a.Need > craveStop {
				a.Seeking = false
			}
		}
	}
	// Expire highs; a medical one's severity is the share still to run
	// (the UI's bar).
	kept := s.Highs[:0]
	for _, h := range s.Highs {
		if h.Until > now {
			if d, ok := DefOf(h.Drug); ok && !d.Recreational && d.HighHours > 0 {
				h.Severity = int32((h.Until - now) * 1000 / (uint64(d.HighHours) * hourTicks))
			}
			kept = append(kept, h)
		}
	}
	s.Highs = kept
	// Recovery: ten days without a dose.
	ad := s.Addictions[:0]
	for _, a := range s.Addictions {
		if now >= a.CleanSince && now-a.CleanSince >= recoveryDays*dayTicks {
			continue
		}
		ad = append(ad, a)
	}
	s.Addictions = ad
}

// FightMultiplier scales social-fight chance (alcohol band x withdrawal),
// permille.
func (s *State) FightMultiplier() int32 {
	m := int32(1000)
	for _, a := range s.Addictions {
		if a.Withdrawal {
			if d, ok := DefOf(a.Drug); ok && d.WithdrawFight > m {
				m = d.WithdrawFight
			}
		}
	}
	return s.Band().FightMult() * m / 1000
}

// Thoughts are the state thoughts the drugs give (tipsy, high, withdrawal).
func (s *State) Thoughts() []mood.Kind {
	var out []mood.Kind
	if s.Band() >= Tipsy {
		out = append(out, mood.Drunk)
	}
	for _, h := range s.Highs {
		if d, ok := DefOf(h.Drug); ok && d.Recreational {
			out = append(out, mood.HighOnDrug) // a medical drug is no high
			break
		}
	}
	for _, a := range s.Addictions {
		if a.Withdrawal {
			out = append(out, mood.Withdrawal)
			break
		}
	}
	return out
}
