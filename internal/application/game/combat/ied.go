package combat

import (
	"fmt"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// Buried charges (IEDs, DEF-3). A hostile that steps on one lights its
// fuse (always: wiki: IED trap, 100 % for enemies); IEDFuse ticks later
// the charge bursts through the explosion service (Blasts) and is re-laid
// as a blueprint, like a sprung spike trap. Only the high-explosive charge
// has a wiki page (50 bomb damage, radius 3.9, 15-tick delay); the fire,
// pulse and smoke charges burst as the shell they are made of (original
// decision: wiki: Incendiary shell, EMP shell, Smoke shell).

// IEDFuse is the delay from trigger to burst. wiki: IED trap
const IEDFuse = 15

// isIED reports a buried-charge kind.
func isIED(k construction.Kind) bool {
	return k >= construction.IEDHighExplosive && k <= construction.IEDSmoke
}

// iedBlast is the blast charge kind k bursts with and the weapon its log
// names (a mortar shell's: the charge is made of two).
func iedBlast(k construction.Kind) (weapon.Kind, explosion.Def) {
	switch k {
	case construction.IEDIncendiary:
		return weapon.MortarIncendiary, weapon.DefOf(weapon.MortarIncendiary).Blast
	case construction.IEDEMP:
		return weapon.MortarEMP, weapon.DefOf(weapon.MortarEMP).Blast
	case construction.IEDSmoke:
		return weapon.MortarSmoke, weapon.DefOf(weapon.MortarSmoke).Blast
	}
	b := weapon.DefOf(weapon.MortarHE).Blast
	b.Radius = 3900 // wiki: IED trap
	return weapon.MortarHE, b
}

// Fuse is a lit charge: building ID of kind Kind on (X, Y) bursts at tick
// At. All fields are exported for the save (gob).
type Fuse struct {
	ID   entity.EntityID
	Kind construction.Kind
	X, Y int
	At   uint64
}

// light starts charge id's fuse (once: a charge already burning keeps its
// time).
func (t *Traps) light(id entity.EntityID, k construction.Kind, x, y int) {
	for _, f := range t.fuses {
		if f.ID == id {
			return
		}
	}
	t.fuses = append(t.fuses, Fuse{ID: id, Kind: k, X: x, Y: y, At: t.h.Now() + IEDFuse})
}

// Burning reports charge id's fuse lit.
func (t *Traps) Burning(id entity.EntityID) bool {
	for _, f := range t.fuses {
		if f.ID == id {
			return true
		}
	}
	return false
}

// phase bursts the charges whose fuse ran out, in the order they were lit.
func (t *Traps) phase(now uint64) {
	if len(t.fuses) == 0 {
		return
	}
	due := 0
	for _, f := range t.fuses {
		if f.At <= now {
			due++
		}
	}
	if due == 0 {
		return
	}
	fired := make([]Fuse, 0, due) // a burst may light other charges
	keep := t.fuses[:0]
	for _, f := range t.fuses {
		if f.At <= now {
			fired = append(fired, f)
		} else {
			keep = append(keep, f)
		}
	}
	t.fuses = keep
	for _, f := range fired {
		t.burst(f)
	}
}

// burst sets off charge f: the building goes, the blast bursts on its
// tile and a blueprint is laid in its place.
func (t *Traps) burst(f Fuse) {
	h := t.h
	b, ok := h.Building(f.ID)
	if !ok {
		return // destroyed or removed while burning
	}
	h.RemoveBuilding(f.ID)
	wk, d := iedBlast(f.Kind)
	h.Message(f.X, f.Y, fmt.Sprintf("A %s went off.", construction.DefOf(f.Kind).Name))
	t.blasts.Explode(0, wk, f.X, f.Y, d)
	h.PlaceFrame(f.Kind, b.Stuff, construction.South, f.X, f.Y) // re-lay it
}

// Struck lights the buried charge on (x, y), if any: a bullet or an
// arrow landing on it (wiki: IED trap). One object lookup.
func (t *Traps) Struck(x, y int) {
	if id, k := t.h.Object(x, y); id != 0 && isIED(k) {
		t.light(id, k, x, y)
	}
}

// chain lights charge id (of kind k on (x, y)) reached by a blast:
// explosions set off the charges in their radius in turn (DEF-4).
func (t *Traps) chain(id entity.EntityID, k construction.Kind, x, y int) bool {
	if !isIED(k) {
		return false
	}
	t.light(id, k, x, y)
	return true
}

// export is the burning fuses for the save (nil when none).
func (t *Traps) export() []Fuse {
	if len(t.fuses) == 0 {
		return nil
	}
	return append([]Fuse(nil), t.fuses...)
}

// importFuses restores saved fuses.
func (t *Traps) importFuses(s []Fuse) { t.fuses = append(t.fuses[:0], s...) }
