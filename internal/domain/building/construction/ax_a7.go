package construction

// plannedA7 is A7's kinds not live yet (bookcases; AX-0 contract): delete a
// case when its kind works end to end. This file is A7's, and so are any
// predicates it adds here for its own kinds.
func plannedA7(k Kind) bool {
	switch k {
	case Bookcase, SmallBookcase:
		return true
	}
	return false
}
