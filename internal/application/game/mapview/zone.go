package mapview

import "rimworld-go/internal/domain/building/zone"

// StoragePriorities lists the storage priority names, highest first (the
// zone inspector's picker; the index is the SetZonePriority command's
// Index).
func StoragePriorities() []string {
	out := make([]string, len(zone.Priorities))
	for i, p := range zone.Priorities {
		out[i] = p.String()
	}
	return out
}

// ZoneLabel is a zone's name in messages: the player's name, else its
// kind ("Growing zone", "Stockpile", "Dumping stockpile").
func ZoneLabel(z zone.Zone) string {
	if z.Name != "" {
		return z.Name
	}
	switch z.Kind {
	case zone.KindGrow:
		return "Growing zone"
	case zone.KindDump:
		return "Dumping stockpile"
	case zone.KindPen:
		return "Pen"
	}
	return "Stockpile"
}
