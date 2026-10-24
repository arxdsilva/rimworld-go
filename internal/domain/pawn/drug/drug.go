package drug

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/clock"
)

// Values follow the published RimWorld wiki by explicit user decision (ENG-003 balance override).

// numDrugs is how many drugs there are (beer, ashleaf tea, spark tablet).
const numDrugs = 3

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
}

var defs = [numDrugs]Def{
	{Kind: item.Beer, Short: "beer", Alcohol: 150, Joy: 200, AddictChance: 10, ToleranceGain: 100, WithdrawFight: 1000, Recreational: true},
	{Kind: item.AshleafTea, Short: "ashleaf", Joy: 300, HighHours: 12, AddictChance: 30, ToleranceGain: 150, WithdrawFight: 3000, Recreational: true},
	{Kind: item.SparkTablet, Short: "spark", Joy: 150, HighHours: 6, RestBoost: 400, AddictChance: 60, ToleranceGain: 200, WithdrawFight: 2000, Recreational: true},
}

func index(k item.Kind) int {
	switch k {
	case item.Beer:
		return 0
	case item.AshleafTea:
		return 1
	case item.SparkTablet:
		return 2
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

// State is one colonist's drug state (saved).
type State struct {
	Alcohol    int32 // permille
	Highs      []High
	Addictions []Addiction
	Tolerance  [numDrugs]int32
	Hangover   uint64 // tick the hangover ends (0: none)

	Doses    int32            // alcohol doses since last sober
	Peaked   bool             // alcohol reached the drunk band
	HangPend bool             // a hangover began and was not yet read
	LastDose [numDrugs]uint64 // tick of last dose, 0 = never

	// Policy is the colonist's drug policy. The zero value is NoDrugs, so
	// whoever creates a State sets the default (SocialDrinking).
	Policy Preset
	// Retry: no autonomous drug search before this tick (the game's
	// cooldown when no allowed drug was in reach).
	Retry uint64
}

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
	s.LastDose[i] = max(now, 1)
	// Already hooked: the dose satisfies the need.
	if a := s.addiction(k); a != nil {
		a.Need, a.Withdrawal, a.Seeking, a.CleanSince = 1000, false, false, now
	} else if roll != nil && d.AddictChance > 0 {
		chance := d.AddictChance * (1000 + s.Tolerance[i]) / 1000
		if int32(roll(1000)) < chance {
			s.Addictions = append(s.Addictions, Addiction{Drug: k, Severity: 1000, Need: 1000, CleanSince: now})
		}
	}
	s.Tolerance[i] = min(s.Tolerance[i]+d.ToleranceGain, toleranceMax)
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
	// Expire highs.
	kept := s.Highs[:0]
	for _, h := range s.Highs {
		if h.Until > now {
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
	if len(s.Highs) > 0 {
		out = append(out, mood.HighOnDrug)
	}
	for _, a := range s.Addictions {
		if a.Withdrawal {
			out = append(out, mood.Withdrawal)
			break
		}
	}
	return out
}
