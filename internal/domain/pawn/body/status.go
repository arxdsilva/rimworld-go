package body

import "rimworld-go/internal/domain/economy/item"

// PartStatus is a read-only row for the health UI.
type PartStatus struct {
	Part    Part
	Label   string
	HP      int // health left
	MaxHP   int
	Wounds  int // open wounds
	Scars   int
	Tended  bool // every open wound is tended
	Missing bool
	Implant item.Kind
	Fitted  bool // Implant is valid
}

// Parts lists every part of the body's plan in order with its state.
func (b *Body) Parts() []PartStatus {
	t := b.table()
	out := make([]PartStatus, 0, len(t))
	for p := Part(0); p < Part(len(t)); p++ {
		s := PartStatus{Part: p, Label: t[p].Label, MaxHP: b.maxHP(p), Missing: b.isMissing(p), Tended: true}
		if s.Missing {
			if b.fitted(p) {
				s.Implant, s.Fitted = b.Implant(p), true
			}
			out = append(out, s)
			continue
		}
		s.HP = s.MaxHP
		for _, w := range b.Wounds {
			if w.Part != p {
				continue
			}
			s.HP -= w.Dmg
			if w.Permanent {
				s.Scars++
				continue
			}
			s.Wounds++
			s.Tended = s.Tended && w.Tended
		}
		if s.HP < 0 {
			s.HP = 0
		}
		out = append(out, s)
	}
	return out
}
