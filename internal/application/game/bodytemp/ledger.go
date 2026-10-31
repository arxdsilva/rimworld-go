package bodytemp

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/pawn/exposure"
	"rimworld-go/internal/domain/sim/entity"
)

// Sev is one creature's hypothermia and heatstroke (exposure.Full is
// 1.0).
type Sev struct{ Hypo, Heat int32 }

// Ledger holds the severities of every pawn and animal suffering any
// (saved as Exposure; a creature with none has no entry).
type Ledger map[entity.EntityID]Sev

// Set stores s for id, dropping the entry once both are gone.
func (l Ledger) Set(id entity.EntityID, s Sev) {
	if s == (Sev{}) {
		delete(l, id)
		return
	}
	l[id] = s
}

// MigratePermille moves a save's permille severities (before TEMP-3,
// when 600 collapsed and 1000 killed) to the wiki scale, keeping what
// each meant: 0–600 maps onto 0–62 % (the extreme stage, downed) and
// 600–1000 onto 62–100 %. The body stages follow on the next step.
func (l Ledger) MigratePermille() {
	for id, s := range l {
		l[id] = Sev{Hypo: fromPermille(int(s.Hypo)), Heat: fromPermille(int(s.Heat))}
	}
}

func fromPermille(p int) int32 {
	p = min(max(p, 0), 1000)
	if p <= 600 {
		return exposure.FromPermille(p * 620 / 600)
	}
	return exposure.FromPermille(620 + (p-600)*380/400)
}

// AnimalView is an animal's comfortable range (tenths of a degree C) and
// its hypothermia (or slowdown) and heatstroke in percent (all 0 for a
// machine).
func (l Ledger) AnimalView(id entity.EntityID, s animal.Species) (lo, hi, hypo, heat int) {
	lo, hi, _ = animal.Comfort(s)
	e := l[id]
	return lo, hi, int(exposure.Permille(e.Hypo)) / 10, int(exposure.Permille(e.Heat)) / 10
}
