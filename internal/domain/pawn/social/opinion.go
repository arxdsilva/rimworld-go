package social

import "rimworld-go/internal/domain/pawn/trait"

// Person is what opinion and interaction rules read about someone.
type Person struct {
	ID              ID
	Male            bool
	AgeYears        int
	Traits          []trait.ID
	Beauty          int8
	Disfigured      bool
	Blind, Deaf     bool
	ArtificialParts int
	Addicted        bool
	Dead            bool
}

// Has reports whether p has trait t.
func (p Person) Has(t trait.ID) bool {
	for _, x := range p.Traits {
		if x == t {
			return true
		}
	}
	return false
}

// Term is one line of an opinion breakdown, centi-points.
type Term struct {
	Label string
	Value int32
}

// Inputs is everything Observer's opinion of Target depends on.
type Inputs struct {
	Observer, Target Person
	Relations        []Kind
	Memories         []OpinionMemory
	Now              uint64
}

func clampI32(v, lo, hi int32) int32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Opinion is Observer's opinion of Target in centi-points (clamped to
// +-100 points) and the terms that make it up.
func Opinion(in Inputs) (centi int32, terms []Term) {
	o, t := in.Observer, in.Target
	add := func(label string, v int32) {
		if v != 0 {
			terms = append(terms, Term{label, v})
		}
	}
	var kin []Kind
	var romance []Kind
	for _, k := range in.Relations {
		if IsRomantic(k) {
			romance = append(romance, k)
		} else {
			kin = append(kin, k)
		}
	}
	if k, ok := StrongestKin(kin); ok {
		add(Label(k, t.Male), Bonus(k))
	}
	var rk Kind
	var have bool
	for _, k := range romance {
		abs := func(v int32) int32 {
			if v < 0 {
				return -v
			}
			return v
		}
		if !have || abs(Bonus(k)) > abs(Bonus(rk)) {
			rk, have = k, true
		}
	}
	if have {
		add(Label(rk, t.Male), Bonus(rk))
	}

	gracious := o.Has(trait.Gracious)
	beauty := int32(t.Beauty) * BeautyStep
	if !(gracious || o.Has(trait.Austere) || o.Blind) && beauty != 0 {
		if !(t.Disfigured && beauty > 0) {
			if beauty > BeautyCap {
				beauty = BeautyCap
			} else if beauty < -BeautyCap {
				beauty = -BeautyCap
			}
			if beauty > 0 {
				add("pleasant looks", beauty)
			} else {
				add("unpleasant looks", beauty)
			}
		}
	}
	if t.Disfigured && !gracious {
		add("disfigured", DisfiguredPenalty)
	}
	if !gracious && !o.Deaf {
		if t.Has(trait.GratingVoice) {
			add("grating voice", VoicePenalty)
		}
		if t.Has(trait.RaspingBreath) {
			add("rasping breath", VoicePenalty)
		}
	}
	if (o.Has(trait.DistrustsMen) && t.Male) || (o.Has(trait.DistrustsWomen) && !t.Male) {
		add("distrust", DistrustPenalty)
	}
	if o.Has(trait.Tireless) && t.Has(trait.Unhurried) {
		add("too slow", TirelessUnhurried)
	}
	if o.Has(trait.Tireless) && t.Has(trait.Idle) {
		add("idle", TirelessIdle)
	}
	if o.Has(trait.Abstainer) {
		if t.Addicted {
			add("addicted", AbstainerAddicted)
		}
		if t.Has(trait.Indulgent) {
			add("indulgent", AbstainerIndulgent)
		}
		if t.Has(trait.Hedonist) {
			add("hedonist", AbstainerHedonist)
		}
	}
	if o.Has(trait.AugmentFan) && t.ArtificialParts > 0 {
		add("augmented", clampI32(int32(t.ArtificialParts)*AugmentFanPerPart, 0, AugmentFanCap))
	}
	if o.Has(trait.NaturalPurist) && t.ArtificialParts > 0 {
		add("artificial parts", clampI32(int32(t.ArtificialParts)*PuristPerPart, PuristCap, 0))
	}
	terms = append(terms, MemoryTerms(in.Memories, t.ID, in.Now)...)

	var sum int32
	for _, x := range terms {
		sum += x.Value
	}
	return clampI32(sum, OpinionMin, OpinionMax), terms
}
