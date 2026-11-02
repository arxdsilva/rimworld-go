package world

// The bookcases' art (A7, AR3's keys): the shelves fill as books go in.

// Bookcase building kinds (construction names).
const (
	BookcaseKind      = "bookcase"
	SmallBookcaseKind = "small bookcase"
)

// BookcaseKey is a bookcase's sprite holding stored books: empty, half
// (a bookcase 1..5, a small one 1..2) or full. The bookcase faces; the
// small one does not.
func BookcaseKey(kind, facing string, stored int) string {
	half, base, suffix := 5, "furniture/bookcase", "_"+FacingOr(facing)
	if kind == SmallBookcaseKind {
		half, base, suffix = 2, "furniture/small_bookcase", ""
	}
	switch {
	case stored > half:
		base += "_full"
	case stored > 0:
		base += "_half"
	}
	return base + suffix
}
