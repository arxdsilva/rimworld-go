package body

import "rimworld-go/internal/domain/economy/item"

// WoundKind is how a wound was made (append only, saved).
type WoundKind uint8

const (
	Cut WoundKind = iota
	Bruise
	Bite
	Scratch
	Burn
	Surgical
)

// scarDmg is the residual damage a scar keeps.
const scarDmg = 1

// Wound is damage on one part; Permanent wounds are scars.
//
// MED-00 appends the medical fields, all zero-safe (an older save and a
// fresh wound read them as zero): TendQ is the tend quality in permille
// (0: untended or tended before quality existed), TendAt the tick it was
// tended, InfectAt the tick its infection roll is due (0: none due) and
// Bleed its cached bleed rate (permille of blood per day).
//
// MED-02 appends HealAcc, the healing accumulated towards the next HP
// (milli-HP × hours a day, tend.go), and Size, the damage the wound was
// made with (scar odds; 0 for wounds older than MED-02).
type Wound struct {
	Part      Part
	Dmg       int
	Kind      WoundKind
	Tended    bool
	At        uint64
	Permanent bool
	TendQ     int16
	TendAt    uint64
	InfectAt  uint64
	Bleed     int32
	HealAcc   int32
	Size      int32
}

// Body is one body's wounds, missing parts (a bit per Part),
// installed implants, blood loss and conditions (saved). A part replaced by
// an implant stays Missing; the implant stands in for it.
//
// Fitted holds the implant on each part, indexed by Part and grown lazily
// (nil: no implants), so appending parts never resizes a saved array; read
// and write it through Implant and SetImplant. BloodLoss is permille of
// blood lost and BloodStage its last stage (the hysteresis memory of
// BloodStage, bleed.go); Conds are the body's conditions (condition.go).
// Fresh marks parts torn off by a hit and not yet tended (a bit per Part,
// MED-01): the stump hurts and bleeds until a tend closes it.
//
// MED-13 appends the body plan: Anatomy picks the part table (zero: the
// humanlike body, so a human's body and save are unchanged) and Scale
// the part hit points in permille of the plan's human-scale numbers (0:
// 1000). Pain reads damage against Scale, so a wound hurts a small animal
// as much as the same share of a man's hit points. Both are zero-safe:
// an older save and a human body read them as zero.
type Body struct {
	Wounds     []Wound
	Missing    uint32
	Fitted     []item.Kind
	BloodLoss  int32
	Conds      []Condition
	BloodStage uint8
	Fresh      uint32
	Anatomy    Anatomy
	Scale      int32
}

// Implant is the implant fitted on p (0: none).
func (b *Body) Implant(p Part) item.Kind {
	if int(p) < len(b.Fitted) {
		return b.Fitted[p]
	}
	return 0
}

// SetImplant puts kind k on p as it is, with no checks (0 clears it):
// Install and Remove are the checked ways. Fitted grows to reach p and
// shrinks back to nil once no part carries anything.
func (b *Body) SetImplant(p Part, k item.Kind) {
	if p >= b.numParts() {
		return
	}
	if k != 0 {
		b.Fresh &^= bit(p) // the operation closed the stump
	}
	if int(p) >= len(b.Fitted) {
		if k == 0 {
			return
		}
		b.Fitted = append(b.Fitted, make([]item.Kind, int(p)+1-len(b.Fitted))...)
	}
	b.Fitted[p] = k
	n := len(b.Fitted)
	for n > 0 && b.Fitted[n-1] == 0 {
		n--
	}
	if n == 0 {
		b.Fitted = nil
		return
	}
	b.Fitted = b.Fitted[:n]
}

// FittedFrom is a Fitted slice holding the implants of a per-part array
// (an older save's fixed-size Implants), nil when it holds none.
func FittedFrom(a []item.Kind) []item.Kind {
	var b Body
	for p, k := range a {
		if k != 0 {
			b.SetImplant(Part(p), k)
		}
	}
	return b.Fitted
}

// fitted reports an implant on p.
func (b *Body) fitted(p Part) bool { return b.Implant(p).IsImplant() }

// partDmg is the wound damage on p.
func (b *Body) partDmg(p Part) int {
	n := 0
	for _, w := range b.Wounds {
		if w.Part == p {
			n += w.Dmg
		}
	}
	return n
}

// Hurt deals dmg of kind to a part chosen with roll; it returns the part.
// A part whose damage reaches its MaxHP is destroyed with its children.
func (b *Body) Hurt(roll func(int) int, dmg int, kind WoundKind, now uint64) Part {
	p, ok := b.PickPart(roll)
	if !ok || dmg <= 0 {
		return p
	}
	b.hurtPart(p, dmg, kind, now)
	return p
}

// hurtPart wounds p, destroying it when its damage reaches MaxHP.
func (b *Body) hurtPart(p Part, dmg int, kind WoundKind, now uint64) {
	if b.isMissing(p) || dmg <= 0 {
		return
	}
	if b.partDmg(p)+dmg >= b.maxHP(p) {
		b.destroy(p)
		if kind != Burn { // a burn sears its stump shut
			b.Fresh |= bit(p)
		}
		return
	}
	b.Wounds = append(b.Wounds, Wound{Part: p, Dmg: dmg, Kind: kind, At: now, Size: int32(dmg)})
}

// destroy removes p and everything under it, with their wounds and implants.
func (b *Body) destroy(p Part) {
	for q := Part(0); q < b.numParts(); q++ {
		if b.under(q, p) {
			b.Missing |= bit(q)
			b.Fresh &^= bit(q) // a stump below is part of this loss
			b.SetImplant(q, 0)
		}
	}
	kept := b.Wounds[:0]
	for _, w := range b.Wounds {
		if !b.under(w.Part, p) {
			kept = append(kept, w)
		}
	}
	b.Wounds = kept
}

// VitalLost reports a vital part (brain, heart, torso) destroyed with no
// implant standing in (a pump or valve heart keeps the patient alive), or
// an organ failure (MED-05, OrganFailure: a capacity the body cannot live
// without at 0).
func (b *Body) VitalLost() bool {
	t := b.table()
	for p := range t {
		if t[p].Vital && b.isMissing(Part(p)) && !b.fitted(Part(p)) {
			return true
		}
	}
	return b.OrganFailure()
}

// replaced reports p covered by an implant on itself or an ancestor.
func (b *Body) replaced(p Part) bool {
	if b.Fitted == nil {
		return false
	}
	for q := Part(0); q < b.numParts(); q++ {
		if b.fitted(q) && b.under(p, q) {
			return true
		}
	}
	return false
}

// Damage is the total wound damage plus the value of lost parts (it feeds
// health.Record.Injury, 0..100, downed at 70). Losing a vital part reports
// at least KillDamage.
func (b *Body) Damage() int {
	n := 0
	for _, w := range b.Wounds {
		n += w.Dmg
	}
	t := b.table()
	for p := Part(0); p < Part(len(t)); p++ {
		if !b.isMissing(p) || b.replaced(p) {
			continue
		}
		if q := t[p].Parent; q != p && b.isMissing(q) {
			continue // counted with its parent
		}
		n += b.maxHP(p)
	}
	if b.VitalLost() && n < KillDamage {
		return KillDamage
	}
	return n
}
