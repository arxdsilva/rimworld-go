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

// Wound is damage on one part; Permanent wounds are scars.
type Wound struct {
	Part      Part
	Dmg       int
	Kind      WoundKind
	Tended    bool
	At        uint64
	Permanent bool
}

// Body is one humanlike's wounds, missing parts (a bit per Part) and
// installed implants (saved).
type Body struct {
	Wounds   []Wound
	Missing  uint32
	Implants [NumParts]item.Kind
}

// Hurt deals dmg of kind to a part chosen with roll; it returns the part.
func (b *Body) Hurt(roll func(int) int, dmg int, kind WoundKind, now uint64) Part {
	return Torso // TODO(M1)
}

// Heal heals amount of damage; scarred reports a wound closed as a scar.
func (b *Body) Heal(roll func(int) int, amount int) (scarred bool) {
	return false // TODO(M1)
}

// Damage is the total wound damage (it feeds health.Record.Injury).
func (b *Body) Damage() int {
	return 0 // TODO(M1)
}
