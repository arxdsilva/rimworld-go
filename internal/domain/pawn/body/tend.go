package body

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
)

// Tend quality, wound healing and scars (MED-02). A tend has a quality in
// permille, from the doctor's Medicine skill, hands and eyes, the medicine
// used, the bed and a random spread. The quality slows an infection
// (infection.StepRace) and speeds healing; a tended wound stops bleeding.
// Wounds heal one by one at the wiki's daily rates, faster tended and
// lying down, and a healed wound may leave a scar, likelier the deeper it
// was.

// Tend quality constants, in permille.
const (
	// tendSkillCap caps the manipulation and sight factors (the stat's
	// 140 % limit). wiki: Medical Tend Quality.
	tendSkillCap = 1400
	// tendSightWeight is how much sight counts (70 % importance: the
	// factor is 1 + 0.7×(sight − 1)). wiki: Medical Tend Quality.
	tendSightWeight = 700
	// SelfTendFactor scales a tend a pawn gives itself. wiki: Medical
	// Tend Quality.
	SelfTendFactor = 700
	// tendRollMin and tendRollSpan are the random factor, uniform in
	// 750..1250 (±25 %). wiki: Medical Tend Quality.
	tendRollMin  = 750
	tendRollSpan = 501
	// HospitalBedTendOffset is added to a tend on a patient in a hospital
	// bed. wiki: Hospital bed (+10 % medical tend quality offset).
	HospitalBedTendOffset = 100
)

// TendInput is what goes into one tend: the doctor's Medicine level, its
// manipulation and sight (permille), the medicine used (any non-medicine
// kind is none), the bed's offset (permille), whether the doctor tends
// itself, and the random source (Roll(n) in [0, n)).
type TendInput struct {
	Skill     int
	Manip     int32
	Sight     int32
	Medicine  item.Kind
	BedOffset int32
	Self      bool
	Roll      func(int) int
}

// tendBase is the Medical Tend Quality stat from the Medicine level: 20 %
// plus 10 % a level, through the stat's curve (0,0) (1,1) (2,1.5) (4,2),
// so 100 % at level 8, 150 % at 18 and 155 % at 20. wiki: Medical Tend
// Quality.
func tendBase(lvl int) int32 {
	switch {
	case lvl < 0:
		lvl = 0
	case lvl > 20:
		lvl = 20
	}
	switch {
	case lvl <= 8:
		return int32(200 + 100*lvl)
	case lvl <= 18:
		return int32(1000 + 50*(lvl-8))
	}
	return int32(1500 + 25*(lvl-18))
}

// Potency is medicine k's potency and the best tend quality it allows
// (permille): none 300/700, herbal 600/700, industrial 1000/1000, nanomed
// kit 1600/1300. wiki: Medicine (glitterworld medicine for the nanomed
// kit, MED-05).
func Potency(k item.Kind) (pot, maxQ int32) {
	switch k {
	case item.HerbalMedicine:
		return 600, 700
	case item.IndustrialMedicine:
		return 1000, 1000
	case item.NanomedKit:
		return 1600, 1300
	}
	return 300, 700
}

// TendQuality is the quality of a tend (permille): the Medicine stat ×
// the manipulation and sight factors (each capped at 140 %) × the
// medicine's potency, plus the bed offset, ×0.7 for a self-tend, × a
// uniform 75–125 %, clamped to [0, the medicine's max quality].
// wiki: Medical Tend Quality.
func TendQuality(in TendInput) int32 {
	manip := clamp32(in.Manip, 0, tendSkillCap)
	sight := clamp32(1000+(in.Sight-1000)*tendSightWeight/1000, 0, tendSkillCap)
	pot, maxQ := Potency(in.Medicine)
	q := int64(tendBase(in.Skill)) * int64(manip) / 1000 * int64(sight) / 1000 * int64(pot) / 1000
	q += int64(in.BedOffset)
	if in.Self {
		q = q * SelfTendFactor / 1000
	}
	roll := int64(1000)
	if in.Roll != nil {
		roll = int64(tendRollMin + in.Roll(tendRollSpan))
	}
	q = q * roll / 1000
	return clamp32(int32(min(q, int64(maxQ))), 0, maxQ)
}

func clamp32(v, lo, hi int32) int32 {
	switch {
	case v < lo:
		return lo
	case v > hi:
		return hi
	}
	return v
}

// TendAll tends the body with quality q at tick now: every open untended
// wound is marked tended (it stops bleeding) with q, every fresh stump is
// closed, and every condition that takes tending and is due (now past its
// TendUntil) gets q for its next TendEvery hours. Wounds tended before
// keep their quality: one tend per wound, one per condition window. It
// returns how many it changed.
func (b *Body) TendAll(q int32, now uint64) int {
	q = clamp32(q, 0, 2000)
	n := 0
	for i := range b.Wounds {
		w := &b.Wounds[i]
		if w.Permanent || w.Tended {
			continue
		}
		w.Tended, w.TendQ, w.TendAt = true, int16(q), now
		n++
	}
	for p := Part(0); p < NumParts; p++ {
		if b.Fresh&bit(p) != 0 {
			n++
		}
	}
	b.Fresh = 0
	for i := range b.Conds {
		c := &b.Conds[i]
		d := DefOfCond(c.Kind)
		if d.TendEvery <= 0 || now < c.TendUntil {
			continue
		}
		c.TendQ, c.TendSum = q, c.TendSum+q
		c.TendUntil = now + uint64(d.TendEvery)*clock.TicksPerHour
		n++
	}
	return n
}

// TendDue reports something on the body a tend would help at tick now:
// an open untended wound, a fresh stump, or a condition that takes
// tending past its TendUntil. A tend clears it until a new wound opens or
// the condition's window runs out, so a doctor tends once per window.
func (b *Body) TendDue(now uint64) bool {
	if b == nil {
		return false
	}
	if b.Fresh != 0 {
		return true
	}
	for _, w := range b.Wounds {
		if !w.Permanent && !w.Tended {
			return true
		}
	}
	for _, c := range b.Conds {
		if d := DefOfCond(c.Kind); d.TendEvery > 0 && now >= c.TendUntil {
			return true
		}
	}
	return false
}

// Healing rates, in milli-HP per day per wound. wiki: Injury.
const (
	// HealBase is how fast any open wound heals.
	HealBase = 8000
	// HealTended is added to a tended wound, plus HealPerTendQ per
	// permille of its tend quality (+4 HP at 0 %, +12 HP at 100 %).
	HealTended   = 4000
	HealPerTendQ = 8
	// LieGround, LieBed and LieHospital are added while the patient lies
	// on the ground, in a bed and in a hospital bed.
	LieGround   = 4000
	LieBed      = 8000
	LieHospital = 14000
	// healUnit is one HP in the heal accumulator: milli-HP × hours a day,
	// so an hour of a daily rate adds exactly that rate.
	healUnit = 1000 * clock.HoursPerDay
)

// HealRate is how fast wound w heals (milli-HP per day) with lying added
// for how the patient lies (0, LieGround, LieBed or LieHospital).
func HealRate(w Wound, lying int32) int32 {
	if w.Permanent {
		return 0
	}
	r := HealBase + lying
	if w.Tended {
		r += HealTended + HealPerTendQ*int32(w.TendQ)
	}
	return r
}

// Scar odds: a healed cut, bite, scratch or burn becomes a scar with
// ScarMax permille × clamp((damage − 4) / 10, 0, 1). wiki: Injury.
const (
	ScarMax      = 20
	scarDmgFloor = 4
	scarDmgSpan  = 10
)

// ScarOdds is the permille chance a healed wound of kind k and damage dmg
// leaves a scar.
func ScarOdds(k WoundKind, dmg int) int32 {
	switch k {
	case Cut, Bite, Scratch, Burn:
	default:
		return 0
	}
	over := min(max(dmg-scarDmgFloor, 0), scarDmgSpan)
	return int32(ScarMax * over / scarDmgSpan)
}

// HealHour heals one hour of every open wound at HealRate with lying, in
// wound order. A wound that closes rolls ScarOdds on its damage when
// made (Size; Dmg for wounds older than MED-02) and either becomes a
// scar or goes. It reports the parts that scarred (nil: none).
func (b *Body) HealHour(roll func(int) int, lying int32) (scarred []Part) {
	kept := b.Wounds[:0]
	for _, w := range b.Wounds {
		if w.Permanent || w.Dmg <= 0 {
			kept = append(kept, w)
			continue
		}
		w.HealAcc += HealRate(w, lying)
		for w.HealAcc >= healUnit && w.Dmg > 0 {
			w.HealAcc -= healUnit
			w.Dmg--
		}
		if w.Dmg > 0 {
			kept = append(kept, w)
			continue
		}
		size := int(w.Size)
		if size <= 0 {
			size = 1
		}
		if odds := ScarOdds(w.Kind, size); odds > 0 && int32(roll(1000)) < odds {
			kept = append(kept, Wound{Part: w.Part, Dmg: scarDmg, Kind: w.Kind, At: w.At, Permanent: true, Size: w.Size})
			scarred = append(scarred, w.Part)
		}
	}
	clear(b.Wounds[len(kept):])
	b.Wounds = kept
	return scarred
}
