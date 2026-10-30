package body

import "slices"

// MaxSpill bounds how many parts one Spread may spread over.
const MaxSpill = 8

// Spread deals b dmg of kind on parts picked by coverage, one intn draw a
// pick (a hit with no placed part: a burn, a fall, a fixture). A part
// whose loss would kill (LossKills) is never destroyed by it: what it
// cannot take spills to the next pick, so Damage rises by exactly dmg
// and death still comes at KillDamage; other parts may be lost. What no
// part takes lands on the root (the torso, a snake's body).
func Spread(b *Body, dmg int, kind WoundKind, intn func(int) int, now uint64) {
	left := dmg
	for try := 0; left > 0 && try < MaxSpill; try++ {
		drawn := -1
		roll := func(n int) int { // one draw, replayed so Hurt picks the same part
			if drawn < 0 {
				drawn = intn(n)
			}
			return drawn
		}
		p, ok := b.PickPart(roll)
		if !ok {
			break
		}
		hit := left
		if room := b.PartDef(p).MaxHP - b.partDmg(p) - 1; hit > room && b.LossKills(p) {
			if room <= 0 {
				continue
			}
			hit = room
		}
		before := b.Damage()
		b.Hurt(roll, hit, kind, now)
		left -= b.Damage() - before
	}
	if left > 0 {
		b.Wounds = append(b.Wounds, Wound{Part: b.Root(), Dmg: left, Kind: kind, At: now, Size: int32(left)})
	}
}

// PartDamage is the wound damage on b's part p.
func PartDamage(b *Body, p Part) int { return b.partDmg(p) }

// Reconcile makes b's damage match injury where wounds allow it: more
// injury lands as a torso bruise, less trims open wounds (TrimOpen).
func Reconcile(b *Body, injury int, now uint64) {
	d := b.Damage()
	switch {
	case injury > d:
		b.Wounds = append(b.Wounds, Wound{Part: Torso, Dmg: injury - d, Kind: Bruise, At: now})
	case injury < d:
		TrimOpen(b, d-injury)
	}
}

// TrimOpen removes n points of open-wound damage, smallest wound first,
// without scarring (an outside correction, not healing).
func TrimOpen(b *Body, n int) {
	for n > 0 {
		idx := -1
		for i, w := range b.Wounds {
			if !w.Permanent && (idx < 0 || w.Dmg < b.Wounds[idx].Dmg) {
				idx = i
			}
		}
		if idx < 0 {
			return
		}
		w := &b.Wounds[idx]
		if w.Dmg > n {
			w.Dmg -= n
			return
		}
		n -= w.Dmg
		b.Wounds = slices.Delete(b.Wounds, idx, idx+1)
	}
}
