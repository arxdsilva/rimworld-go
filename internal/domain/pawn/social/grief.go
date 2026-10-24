package social

import "rimworld-go/internal/domain/pawn/trait"

// GriefDef is a death's mood effect on someone close and how long it lasts.
type GriefDef struct {
	Mood int
	Days int
}

// NoKind is passed to GriefFor when there is no family relation.
const NoKind = NumKinds

func relationGrief(k Kind) (int, bool) {
	switch k {
	case Spouse:
		return -20, true
	case Fiance:
		return -18, true
	case Lover:
		return -16, true
	case Child:
		return -20, true
	case Sibling:
		return -14, true
	case Grandchild:
		return -12, true
	case Parent:
		return -8, true
	case HalfSibling, NieceNephew:
		return -5, true
	case AuntUncle, Grandparent, Cousin, Kin,
		GreatGrandparent, GreatGrandchild, GrandAuntUncle, GrandNieceNephew, SecondCousin:
		return -4, true
	}
	return 0, false
}

// GriefFor is the grief over a person with relation k (friend, rival: the
// opinion bands; lost: missing rather than dead, which the spec scores
// the same as dying). Pass NoKind for none.
func GriefFor(k Kind, friend, rival, lost bool) (GriefDef, bool) {
	var d GriefDef
	if m, ok := relationGrief(k); ok {
		d = GriefDef{Mood: m, Days: grievanceDays}
	} else if friend {
		d = GriefDef{Mood: griefFriendMood, Days: griefFriendDays}
	} else if rival {
		d = GriefDef{Mood: griefRivalMood, Days: griefRivalDays}
	} else {
		return GriefDef{}, false
	}
	return d, true
}

// ExemptFromGrief is true for cold-hearted people.
func ExemptFromGrief(traits []trait.ID) bool {
	for _, t := range traits {
		if t == trait.ColdHearted {
			return true
		}
	}
	return false
}

// KillerOpinion is the opinion penalty toward whoever killed a person with
// relation k, centi-points (0: none).
func KillerOpinion(k Kind) int32 {
	switch k {
	case Parent, Child, Sibling:
		return killerChildSibling
	case Spouse, Fiance:
		return killerSpouseLover
	case Lover:
		return killerLover
	case ExLover, ExSpouse:
		return 0
	}
	if k < NumKinds {
		return killerDistant
	}
	return 0
}

// KillerMemory is the opinion memory a killing leaves, by the victim's
// relation to the grieving person.
func KillerMemory(k Kind) (MemKind, bool) {
	switch k {
	case Parent, Child:
		return MemKilledMyParentChild, true
	case Sibling:
		return MemKilledMySibling, true
	case Spouse, Fiance:
		return MemKilledMySpouse, true
	case Lover:
		return MemKilledMyLover, true
	case ExLover, ExSpouse:
		return 0, false
	}
	if k < NumKinds {
		return MemKilledMyKin, true
	}
	return 0, false
}
