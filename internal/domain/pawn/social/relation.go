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

type labelPair struct{ female, male string }

var labels = [NumKinds]labelPair{
	Parent:           {"mother", "father"},
	Child:            {"daughter", "son"},
	Sibling:          {"sister", "brother"},
	HalfSibling:      {"half-sister", "half-brother"},
	Grandparent:      {"grandmother", "grandfather"},
	Grandchild:       {"granddaughter", "grandson"},
	GreatGrandparent: {"great-grandmother", "great-grandfather"},
	GreatGrandchild:  {"great-granddaughter", "great-grandson"},
	AuntUncle:        {"aunt", "uncle"},
	NieceNephew:      {"niece", "nephew"},
	GrandAuntUncle:   {"grand-aunt", "grand-uncle"},
	GrandNieceNephew: {"grand-niece", "grand-nephew"},
	Cousin:           {"cousin", "cousin"},
	SecondCousin:     {"second cousin", "second cousin"},
	Kin:              {"relative", "relative"},
	StepParent:       {"stepmother", "stepfather"},
	StepChild:        {"stepdaughter", "stepson"},
	ParentInLaw:      {"mother-in-law", "father-in-law"},
	ChildInLaw:       {"daughter-in-law", "son-in-law"},
	Lover:            {"lover", "lover"},
	Fiance:           {"fiancée", "fiancé"},
	Spouse:           {"wife", "husband"},
	ExLover:          {"ex-lover", "ex-lover"},
	ExSpouse:         {"ex-wife", "ex-husband"},
}

// Label is relation k's word for the other person ("mother", "brother").
func Label(k Kind, otherMale bool) string {
	if k >= NumKinds {
		return ""
	}
	if otherMale {
		return labels[k].male
	}
	return labels[k].female
}

var bonuses = [NumKinds]int32{
	Parent: bonusParent, Child: bonusChild, Sibling: bonusSibling, HalfSibling: bonusHalfSibling,
	Grandparent: bonusGrandparent, Grandchild: bonusGrandchild,
	GreatGrandparent: bonusGreatGrand, GreatGrandchild: bonusGreatGrand,
	AuntUncle: bonusAuntUncle, NieceNephew: bonusNieceNephew,
	GrandAuntUncle: bonusGrandAunt, GrandNieceNephew: bonusGrandAunt,
	Cousin: bonusCousin, SecondCousin: bonusSecondCousin, Kin: bonusKin,
	StepParent: bonusStep, StepChild: bonusStep, ParentInLaw: bonusInLaw, ChildInLaw: bonusInLaw,
	Lover: bonusLover, Fiance: bonusFiance, Spouse: bonusSpouse,
	ExLover: bonusExLover, ExSpouse: bonusExSpouse,
}

// Bonus is relation k's opinion bonus, centi-points.
func Bonus(k Kind) int32 {
	if k >= NumKinds {
		return 0
	}
	return bonuses[k]
}

// IsRomantic reports whether k is a partner or ex-partner relation.
func IsRomantic(k Kind) bool { return k >= Lover && k < NumKinds }
