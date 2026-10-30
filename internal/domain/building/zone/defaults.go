package zone

// Default settings of a new storage zone or storage building (STO-2).
// Every one is an explicit (Custom) filter, so the older switches only
// ever apply to zones painted before.

// NewStockpile is a freshly painted stockpile: every storable kind but
// corpses, bones and chunks, rotten stacks refused, Normal priority (the
// vanilla default stockpile).
func NewStockpile() Zone {
	z := Zone{Kind: KindStockpile, Special: Special{NoRotten: true}}
	z.SetAllowed(sets.storable.AndNot(sets.corpses).AndNot(sets.chunks), true)
	return z
}

// NewDump is a freshly painted dumping stockpile: corpses (human and
// animal), bones and chunks, rotten ones too (wiki: Dumping stockpile
// zone: corpses and chunks by default, nothing else).
func NewDump() Zone {
	z := Zone{Kind: KindDump}
	z.SetAllowed(sets.corpses.Or(sets.chunks), true)
	return z
}

// NewShelf is a new shelf's storage (wiki: Shelf): Preferred priority,
// every storable kind but stone chunks and corpses, its overlay hidden.
func NewShelf() Zone {
	z := Zone{Kind: KindShelf, Hidden: true, Priority: PriorityPreferred}
	z.SetAllowed(Universe(KindShelf), true)
	return z
}

// NewHopper is a new hopper's storage (wiki: Hopper): raw food, one
// stack, Important priority, its overlay hidden.
func NewHopper() Zone {
	z := Zone{Kind: KindHopper, Hidden: true, Priority: PriorityImportant}
	z.SetAllowed(Universe(KindHopper), true)
	return z
}

// NewGrave is a new grave's or (sarcophagus) sarcophagus's settings:
// human corpses and bones (wiki: Grave: the default only allows human
// corpses); a sarcophagus colonists only (wiki: Sarcophagus). Important
// priority is our tuning (the wiki names none), so a grave is filled
// before a body goes anywhere else.
func NewGrave(sarcophagus bool) Zone {
	z := Zone{Kind: KindGrave, Hidden: true, Priority: PriorityImportant, Special: Special{NoStranger: sarcophagus}}
	z.SetAllowed(sets.bodies, true)
	return z
}
