package body

// Cryptosleep sickness (AX-9; wiki: Cryptosleep sickness): whoever comes
// out of a stasis casket, after any stay, is dazed for 10000 ticks:
// consciousness x0.8, moving x0.9, manipulation x0.9 (as offsets on a full
// capacity, the engine's only kind), and a vomit every 3 hours on average.
// It runs by time like the anaesthetic: one stage, its severity the share
// still to run, gone after CryptoSickTicks; no immunity, no tending, no
// daily rate, so it never flaps. Free functions: Body is at its method
// budget.
var condsCrypto = []CondEntry{{CondCryptosleepSickness, CondDef{
	Label:      "cryptosleep sickness",
	NoImmunity: true,
	Stages: []Stage{{At: 0, Label: "", Mods: CapMods{ // wiki: Cryptosleep sickness
		off(Consciousness, -200), off(Moving, -100), off(Manipulation, -100)}}},
}}}

const (
	// CryptoSickTicks is how long it lasts (wiki: 10000 ticks).
	CryptoSickTicks = 10000
	// CryptoVomitEvery is the mean hours between vomits (wiki: MTB 3 h).
	CryptoVomitEvery = 3
)

// CryptoSicken gives b fresh cryptosleep sickness at tick now (a second
// one starts over).
func CryptoSicken(b *Body, now uint64) {
	fresh := Condition{Kind: CondCryptosleepSickness, Part: Torso, Sev: 1000, Since: now}
	if c, ok := b.Cond(CondCryptosleepSickness); ok {
		*c = fresh
		return
	}
	b.Conds = append(b.Conds, fresh)
}

// CryptoSick reports b dazed by cryptosleep sickness.
func CryptoSick(b *Body) bool {
	if b == nil {
		return false
	}
	_, ok := b.Cond(CondCryptosleepSickness)
	return ok
}

// StepCryptoSickness brings it up to tick now: its severity the share
// left to run, gone after CryptoSickTicks. It reports a change.
func StepCryptoSickness(b *Body, now uint64) bool {
	c, ok := b.Cond(CondCryptosleepSickness)
	if !ok {
		return false
	}
	var el uint64
	if now > c.Since {
		el = now - c.Since
	}
	if el >= CryptoSickTicks {
		b.RemoveCond(CondCryptosleepSickness)
		return true
	}
	sev := max(int32(1000-el*1000/CryptoSickTicks), 1)
	changed := c.Sev != sev
	c.Sev = sev
	return changed
}

// VomitEvery is the mean hours between b's vomits: its food poisoning's
// stage, or cryptosleep sickness's, the more frequent (0: none).
func VomitEvery(b *Body) int32 {
	every := int32(0)
	if c, ok := b.Poisoned(); ok {
		every = FoodVomitEvery(c.Stage)
	}
	if CryptoSick(b) && (every == 0 || every > CryptoVomitEvery) {
		every = CryptoVomitEvery
	}
	return every
}
