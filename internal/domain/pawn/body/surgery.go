package body

import "rimworld-go/internal/domain/economy/item"

// OpKind is an operation (append only, saved).
type OpKind uint8

const (
	InstallImplant OpKind = iota
	RemoveImplant
	Amputate
)

// Operation is one queued operation on a part.
type Operation struct {
	Kind OpKind
	Part Part
	Item item.Kind
}

// Outcome is how an operation went.
type Outcome uint8

const (
	Success Outcome = iota
	MinorFail
	Catastrophic
)

// MaxSurgeryChance caps SurgeryChance (permille).
const MaxSurgeryChance = 980

// SurgeryChance is the permille success chance of an operation.
func SurgeryChance(medicineSkill int, hospitalBed bool, medicine item.Kind, manip, sight int32) int32 {
	s := int32(medicineSkill)
	if s < 0 {
		s = 0
	}
	if s > 20 {
		s = 20
	}
	var c int32
	if s <= 8 {
		c = 400 + 450*s/8
	} else {
		c = 850 + 130*(s-8)/12
	}
	if hospitalBed {
		c = c * 1100 / 1000
	}
	switch medicine {
	case item.IndustrialMedicine:
		c = c * 1100 / 1000
	case item.HerbalMedicine:
	default:
		c = c * 700 / 1000
	}
	m := min32(clamp(manip), clamp(sight))
	c = c * m / 1000
	if c > MaxSurgeryChance {
		c = MaxSurgeryChance
	}
	return c
}

func clamp(v int32) int32 {
	if v < 0 {
		return 0
	}
	if v > 1000 {
		return 1000
	}
	return v
}

// RollOutcome decides an operation of success chance chance (permille);
// failures are minor 80:20 against catastrophic.
func RollOutcome(chance int32, roll func(int) int) Outcome {
	if int32(roll(1000)) < chance {
		return Success
	}
	if roll(100) < 80 {
		return MinorFail
	}
	return Catastrophic
}

// CanOperate reports the operation is valid on the current body.
func (b *Body) CanOperate(op Operation) bool {
	if op.Part >= NumParts {
		return false
	}
	switch op.Kind {
	case InstallImplant:
		return Fits(op.Item, op.Part) && b.CanReplace(op.Part)
	case RemoveImplant:
		return b.Implants[op.Part].IsImplant()
	case Amputate:
		return !b.isMissing(op.Part) && !defs[op.Part].Vital
	}
	return false
}

// Apply carries out op with the given outcome and reports whether the
// intended change happened. Failures wound the patient: minor ones with a
// 5-15 damage surgical wound (never destroying the part), catastrophic ones
// with 20-40 damage that may destroy it.
func (b *Body) Apply(op Operation, o Outcome, roll func(int) int, now uint64) bool {
	if !b.CanOperate(op) {
		return false
	}
	target := op.Part
	for b.isMissing(target) && defs[target].Parent != target {
		target = defs[target].Parent
	}
	switch o {
	case Success:
		switch op.Kind {
		case InstallImplant:
			if !b.isMissing(op.Part) {
				b.destroy(op.Part)
			}
			return b.Install(op.Part, op.Item)
		case RemoveImplant:
			_, ok := b.Remove(op.Part)
			return ok
		default:
			b.destroy(op.Part)
			return true
		}
	case MinorFail:
		dmg := 5 + roll(11)
		if room := defs[target].MaxHP - 1 - b.partDmg(target); dmg > room {
			dmg = room
		}
		if dmg > 0 {
			b.Wounds = append(b.Wounds, Wound{Part: target, Dmg: dmg, Kind: Surgical, At: now})
		}
	default:
		b.hurtPart(target, 20+roll(21), Surgical, now)
	}
	return false
}
