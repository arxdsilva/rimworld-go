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

// SurgeryChance is the permille success chance of an operation.
func SurgeryChance(medicineSkill int, hospitalBed bool, medicine item.Kind, manip, sight int32) int32 {
	return 0 // TODO(M1)
}
