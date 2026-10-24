package social

// ID is a person's entity id.
type ID = uint64

// Kind is a relation (append only, saved).
type Kind uint8

const (
	Parent Kind = iota
	Child
	Sibling
	HalfSibling
	Grandparent
	Grandchild
	GreatGrandparent
	GreatGrandchild
	AuntUncle
	NieceNephew
	GrandAuntUncle
	GrandNieceNephew
	Cousin
	SecondCousin
	Kin
	StepParent
	StepChild
	ParentInLaw
	ChildInLaw
	Lover
	Fiance
	Spouse
	ExLover
	ExSpouse
	NumKinds
)

// Label is relation k's word for the other person ("mother", "brother").
func Label(k Kind, otherMale bool) string {
	return "" // TODO(M3)
}

// Bonus is relation k's opinion bonus, centi-points.
func Bonus(k Kind) int32 {
	return 0 // TODO(M3)
}
