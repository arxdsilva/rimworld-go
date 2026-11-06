package overlay

import "rimworld-go/internal/adapters/inbound/ebiten/lang"

// TrOf is the first catalog of an optional trailing argument, English when
// there is none or it is nil.
func TrOf(tr []lang.Tr) lang.Tr {
	if len(tr) > 0 && tr[0] != nil {
		return tr[0]
	}
	return lang.English{}
}
