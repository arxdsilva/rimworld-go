package zone

import "rimworld-go/internal/domain/economy/item"

// Profile is a storage piece's zone settings (NST-*, Neat Storage): stacks per
// cell, the item ceiling and the priority a new one starts with. It is saved in
// Zone.Profile, so values only ever append. Zero is a plain shelf.
type Profile uint8

// The storage pieces, in construction.NeatProfile's order: profile n is the
// n-th kind from construction.DisplayCase (game/store checks they agree).
const (
	ProfileNone Profile = iota
	ProfileDisplayCase
	ProfileRoundPedestal
	ProfileSmallDisplayShelf
	ProfileDisplayShelf
	ProfileWeaponRack
	ProfileNarrowWardrobe
	ProfileWardrobe
	ProfileNarrowDisplayCabinet
	ProfileDisplayCabinet
	ProfileOpenShelf
	ProfileTwoTierDisplayShelf
	ProfileNarrowTallShelf
	ProfileTallShelf
	ProfileSafe
	ProfileDrum
	ProfileHorizontalWeaponRack
	ProfileVerticalWeaponRack
	ProfileTextileRack
	ProfileMeatHook
	ProfileMiniPallet
	ProfileHalfPallet
	ProfilePallet
	ProfileSmallCrate
	ProfileCrate
	ProfileLargeCrate
	ProfileTallCrate
	ProfileCorpseDumpSpot
	profileEnd
)

// The item ceilings the pieces share (NEAT_STORAGE_GAP_ANALYSIS.md, Accepted
// items; our own tuning). The sets themselves are worked out once in sets.
type profileSet uint8

const (
	setShelf profileSet = iota // what a plain shelf takes
	setDisplay
	setWeapons
	setWardrobe
	setCabinet
	setValuables
	setDrum
	setTextiles
	setMeat
	setDump
)

// profileRows is the Per cell column of the gap document (the mod's published
// numbers: AGENTS.md, NST exception) with our item ceilings and default
// priorities: Preferred for furniture, like the shelf; Normal for pallets,
// crates and the dump spot, so they don't out-rank stockpiles. Only read.
var profileRows = [profileEnd]struct {
	stacks int
	prio   Priority
	set    profileSet
}{
	ProfileNone:                 {0, PriorityPreferred, setShelf},
	ProfileDisplayCase:          {1, PriorityPreferred, setDisplay},
	ProfileRoundPedestal:        {1, PriorityPreferred, setDisplay},
	ProfileSmallDisplayShelf:    {3, PriorityPreferred, setDisplay},
	ProfileDisplayShelf:         {3, PriorityPreferred, setDisplay},
	ProfileWeaponRack:           {3, PriorityPreferred, setWeapons},
	ProfileNarrowWardrobe:       {4, PriorityPreferred, setWardrobe},
	ProfileWardrobe:             {4, PriorityPreferred, setWardrobe},
	ProfileNarrowDisplayCabinet: {4, PriorityPreferred, setCabinet},
	ProfileDisplayCabinet:       {4, PriorityPreferred, setCabinet},
	ProfileOpenShelf:            {6, PriorityPreferred, setShelf},
	ProfileTwoTierDisplayShelf:  {6, PriorityPreferred, setDisplay},
	ProfileNarrowTallShelf:      {6, PriorityPreferred, setShelf},
	ProfileTallShelf:            {6, PriorityPreferred, setShelf},
	ProfileSafe:                 {6, PriorityPreferred, setValuables},
	ProfileDrum:                 {6, PriorityPreferred, setDrum},
	ProfileHorizontalWeaponRack: {6, PriorityPreferred, setWeapons},
	ProfileVerticalWeaponRack:   {6, PriorityPreferred, setWeapons},
	ProfileTextileRack:          {6, PriorityPreferred, setTextiles},
	ProfileMeatHook:             {6, PriorityPreferred, setMeat},
	ProfileMiniPallet:           {6, PriorityNormal, setShelf},
	ProfileHalfPallet:           {6, PriorityNormal, setShelf},
	ProfilePallet:               {6, PriorityNormal, setShelf},
	ProfileSmallCrate:           {6, PriorityNormal, setShelf},
	ProfileCrate:                {6, PriorityNormal, setShelf},
	ProfileLargeCrate:           {6, PriorityNormal, setShelf},
	ProfileTallCrate:            {6, PriorityNormal, setShelf},
	ProfileCorpseDumpSpot:       {12, PriorityNormal, setDump},
}

// profileSets is the ceiling of every profile, for sets (s holds the sets
// worked out before it; this runs inside sets' initialiser, so it must not read
// sets itself).
func profileSets(s kindSets) (out [profileEnd]Filter) {
	display := pathsOf("Weapons", "Apparel", "Items", "Manufactured", "Raw resources/Metals")
	byset := map[profileSet]Filter{
		setShelf:     s.storable.AndNot(s.chunks).AndNot(s.corpses),
		setDisplay:   display,
		setWeapons:   pathsOf("Weapons"),
		setWardrobe:  pathsOf("Apparel", "Manufactured/Textiles"),
		setCabinet:   display.Or(pathsOf("Foods/Meals", "Manufactured/Medicine")),
		setValuables: pathsOf("Raw resources/Metals", "Manufactured/Components", "Items/Parts", "Manufactured/Medicine", "Manufactured/Drugs", "Manufactured/Body parts/Prosthetics"),
		setDrum:      pathsOf("Raw resources/Chemicals", "Raw resources/Brewing and leaves").Or(filterOf(item.Milk, item.Beer)),
		setTextiles:  pathsOf("Manufactured/Textiles", "Items/Carpets"),
		setMeat:      pathsOf("Foods/Raw food/Meat"),
		setDump:      s.corpses.Or(s.chunks),
	}
	for p, r := range profileRows {
		out[p] = byset[r.set]
	}
	return out
}

// pathsOf is every kind under the filter-tree category paths ps.
func pathsOf(ps ...string) (f Filter) {
	for _, p := range ps {
		g, _ := resolvePath(p)
		f = f.Or(g)
	}
	return f
}

// Stacks is how many stacks one cell of this profile holds (0 for ProfileNone
// and an unknown profile: the zone kind decides).
func (p Profile) Stacks() int {
	if p >= profileEnd {
		return 0
	}
	return profileRows[p].stacks
}

// Slots is how many stacks one cell of this zone holds.
func (z Zone) Slots() int {
	if n := z.Profile.Stacks(); n > 0 && z.Kind == KindShelf {
		return n
	}
	return z.Kind.Stacks()
}

// UniverseOf is what a zone of kind k with profile p can ever take: Universe(k)
// but for a storage piece, whose ceiling is its own.
func UniverseOf(k Kind, p Profile) Filter {
	if k != KindShelf || p == ProfileNone || p >= profileEnd {
		return Universe(k)
	}
	return sets.profile[p]
}

// NewProfile is a new storage piece's zone: a hidden shelf zone carrying the
// profile, taking its whole ceiling at its default priority. The corpse dump
// spot's Special stays unset, so rotten bodies are welcome, as in NewDump.
func NewProfile(p Profile) Zone {
	if p >= profileEnd {
		p = ProfileNone
	}
	return Zone{Kind: KindShelf, Hidden: true, Profile: p, Priority: profileRows[p].prio, Custom: true, Filter: sets.profile[p]}
}

// ProfileAt is the profile of the zone on tile (x, y), ProfileNone off any: a
// package function (Map is near its method ceiling) that copies no settings,
// for the item views' per-item lookup.
func ProfileAt(m *Map, x, y int) Profile {
	id, _ := m.At(x, y)
	if r := m.rec(id); r != nil {
		return r.z.Profile
	}
	return ProfileNone
}
