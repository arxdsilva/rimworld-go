package zone

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

// TestProfileTable: slots and default priority per profile equal the gap
// document; every ceiling lies inside Storable; a plain shelf is unchanged; the
// dump spot alone takes corpses, bones and chunks; the ceilings hold exactly the
// kinds the Accepted items column names.
func TestProfileTable(t *testing.T) {
	t.Parallel()
	shelfSet := Universe(KindShelf)
	rows := []struct {
		p      Profile
		stacks int
		prio   Priority
		in     []item.Kind
		out    []item.Kind
	}{
		{ProfileDisplayCase, 1, PriorityPreferred, []item.Kind{item.Knife, item.Parka, item.Novel, item.Cloth, item.Steel}, []item.Kind{item.Meal, item.Wood, item.Meat}},
		{ProfileRoundPedestal, 1, PriorityPreferred, []item.Kind{item.Knife, item.Gold}, []item.Kind{item.Meal, item.Stone}},
		{ProfileSmallDisplayShelf, 3, PriorityPreferred, []item.Kind{item.Rifle}, []item.Kind{item.Potatoes}},
		{ProfileDisplayShelf, 3, PriorityPreferred, []item.Kind{item.Rifle}, []item.Kind{item.Potatoes}},
		{ProfileWeaponRack, 3, PriorityPreferred, []item.Kind{item.Knife, item.Rifle, item.FragGrenades}, []item.Kind{item.Parka, item.Steel}},
		{ProfileNarrowWardrobe, 4, PriorityPreferred, []item.Kind{item.Parka, item.SimpleHelmet, item.Cloth, item.Leather}, []item.Kind{item.Knife, item.Meal}},
		{ProfileWardrobe, 4, PriorityPreferred, []item.Kind{item.Parka, item.Wool}, []item.Kind{item.Knife, item.Meal}},
		{ProfileNarrowDisplayCabinet, 4, PriorityPreferred, []item.Kind{item.Meal, item.HerbalMedicine, item.Knife}, []item.Kind{item.Potatoes, item.Wood}},
		{ProfileDisplayCabinet, 4, PriorityPreferred, []item.Kind{item.Pemmican, item.NanomedKit, item.Gold}, []item.Kind{item.Meat, item.Stone}},
		{ProfileOpenShelf, 6, PriorityPreferred, []item.Kind{item.Wood, item.Meal}, []item.Kind{item.GraniteChunk, item.CorpseHuman, item.Bones}},
		{ProfileTwoTierDisplayShelf, 6, PriorityPreferred, []item.Kind{item.Rifle}, []item.Kind{item.Potatoes}},
		{ProfileNarrowTallShelf, 6, PriorityPreferred, []item.Kind{item.Wood}, []item.Kind{item.GraniteChunk}},
		{ProfileTallShelf, 6, PriorityPreferred, []item.Kind{item.Wood}, []item.Kind{item.GraniteChunk}},
		{ProfileSafe, 6, PriorityPreferred, []item.Kind{item.Gold, item.Components, item.MindCore, item.HerbalMedicine, item.Beer, item.PegLeg}, []item.Kind{item.Wood, item.Heart, item.Knife, item.Novel}},
		{ProfileDrum, 6, PriorityPreferred, []item.Kind{item.Chemfuel, item.Wort, item.Hops, item.Milk, item.Beer}, []item.Kind{item.Wood, item.Egg, item.AshleafTea}},
		{ProfileHorizontalWeaponRack, 6, PriorityPreferred, []item.Kind{item.Longsword}, []item.Kind{item.Parka}},
		{ProfileVerticalWeaponRack, 6, PriorityPreferred, []item.Kind{item.Longsword}, []item.Kind{item.Parka}},
		{ProfileTextileRack, 6, PriorityPreferred, []item.Kind{item.Cloth, item.Straw, item.Carpets()[0]}, []item.Kind{item.Parka, item.Wood}},
		{ProfileMeatHook, 6, PriorityPreferred, []item.Kind{item.Meat, item.HumanMeat, item.InsectMeat, item.Fish}, []item.Kind{item.Milk, item.Potatoes, item.CorpseAnimal}},
		{ProfileMiniPallet, 6, PriorityNormal, []item.Kind{item.Wood}, []item.Kind{item.SlateChunk}},
		{ProfileHalfPallet, 6, PriorityNormal, []item.Kind{item.Wood}, []item.Kind{item.SlateChunk}},
		{ProfilePallet, 6, PriorityNormal, []item.Kind{item.Wood}, []item.Kind{item.SlateChunk}},
		{ProfileSmallCrate, 6, PriorityNormal, []item.Kind{item.Wood}, []item.Kind{item.SlateChunk}},
		{ProfileCrate, 6, PriorityNormal, []item.Kind{item.Wood}, []item.Kind{item.SlateChunk}},
		{ProfileLargeCrate, 6, PriorityNormal, []item.Kind{item.Wood}, []item.Kind{item.SlateChunk}},
		{ProfileTallCrate, 6, PriorityNormal, []item.Kind{item.Wood}, []item.Kind{item.SlateChunk}},
		{ProfileCorpseDumpSpot, 12, PriorityNormal, []item.Kind{item.CorpseHuman, item.CorpseAnimal, item.CorpseDustbuck, item.Bones, item.GraniteChunk}, []item.Kind{item.Wood, item.Meat, item.Knife}},
	}
	if len(rows) != 27 || int(profileEnd) != 28 || rows[len(rows)-1].p != profileEnd-1 {
		t.Fatalf("%d rows, profileEnd %d", len(rows), profileEnd)
	}
	dumpKinds := filterOf(item.CorpseHuman, item.CorpseAnimal, item.CorpseDustbuck, item.CorpseRidgewolf, item.Bones,
		item.SandstoneChunk, item.GraniteChunk, item.LimestoneChunk, item.SlateChunk, item.MarbleChunk)
	for i, r := range rows {
		if r.p != Profile(i+1) {
			t.Fatalf("row %d is profile %d", i, r.p) // the profile numbers are saved
		}
		z := NewProfile(r.p)
		u := UniverseOf(KindShelf, r.p)
		if r.p.Stacks() != r.stacks || z.Slots() != r.stacks || z.Priority != r.prio || z.Kind != KindShelf || !z.Hidden || z.Profile != r.p {
			t.Errorf("profile %d: stacks %d slots %d zone %+v, want %d at %s", r.p, r.p.Stacks(), z.Slots(), z, r.stacks, r.prio)
		}
		if u == (Filter{}) || !sets.storable.Covers(u) || z.Effective() != u {
			t.Errorf("profile %d: ceiling %d kinds, effective %d, not inside Storable or empty", r.p, u.Len(), z.Effective().Len())
		}
		if r.p == ProfileCorpseDumpSpot {
			if u != dumpKinds {
				t.Errorf("the dump spot takes %v", u.Names())
			}
			if !z.AllowsStack(item.Stack{Kind: item.CorpseHuman, Qty: 1, Rot: 1 << 30}) {
				t.Error("the dump spot refuses a rotten body")
			}
		} else if u.Any(dumpKinds) {
			t.Errorf("profile %d takes corpses, bones or chunks", r.p)
		}
		for _, k := range r.in {
			if !z.Allows(k) {
				t.Errorf("profile %d refuses %s", r.p, k)
			}
		}
		for _, k := range r.out {
			if z.Allows(k) {
				t.Errorf("profile %d takes %s", r.p, k)
			}
		}
	}
	// the Shelf set really is the shelf's ceiling, and Display is a subset of Cabinet
	for _, p := range []Profile{ProfileOpenShelf, ProfilePallet, ProfileTallCrate} {
		if UniverseOf(KindShelf, p) != shelfSet {
			t.Errorf("profile %d is not the shelf set", p)
		}
	}
	if !UniverseOf(KindShelf, ProfileDisplayCabinet).Covers(UniverseOf(KindShelf, ProfileDisplayCase)) {
		t.Error("a cabinet takes what a display takes")
	}
	// the plain shelf and every other kind are unchanged
	if (Zone{Kind: KindShelf}).Slots() != ShelfStacks || NewShelf().Slots() != ShelfStacks || (Zone{Kind: KindBookcase}).Slots() != BookcaseStacks ||
		(Zone{Kind: KindHopper}).Slots() != 1 || (Zone{}).Slots() != 1 || (Zone{Kind: KindStockpile}).Slots() != 1 {
		t.Fatal("slots of the older zones moved")
	}
	for _, k := range []Kind{KindStockpile, KindDump, KindShelf, KindHopper, KindGrave, KindBookcase, KindGrow} {
		if UniverseOf(k, ProfileNone) != Universe(k) {
			t.Errorf("kind %d: UniverseOf differs from Universe", k)
		}
	}
	if UniverseOf(KindHopper, ProfileSafe) != Universe(KindHopper) || (Zone{Kind: KindHopper, Profile: ProfileSafe}).Slots() != 1 {
		t.Fatal("a profile only means something on a shelf zone")
	}
	if p := profileEnd; p.Stacks() != 0 || UniverseOf(KindShelf, p) != shelfSet || NewProfile(p).Profile != ProfileNone {
		t.Fatal("an unknown profile is a plain shelf")
	}
	if z := NewProfile(ProfileNone); z.Effective() != NewShelf().Effective() || z.Priority != NewShelf().Priority || z.Slots() != ShelfStacks {
		t.Fatal("NewProfile(ProfileNone) is a new shelf")
	}
}

// TestProfileCeilingHolds: SetAllowed, PasteFrom, a filterless piece and the
// settings window's rows never go past the piece's ceiling.
func TestProfileCeilingHolds(t *testing.T) {
	t.Parallel()
	rack := NewProfile(ProfileWeaponRack)
	ceiling := UniverseOf(KindShelf, ProfileWeaponRack)
	rack.SetAllowed(sets.storable, false)
	if rack.Effective() != (Filter{}) {
		t.Fatal("clear all")
	}
	rack.SetAllowed(sets.storable, true) // "allow all"
	if rack.Effective() != ceiling {
		t.Fatalf("allow all on a rack: %v", rack.Effective().Names())
	}
	rack = NewProfile(ProfileWeaponRack)
	shelf := NewShelf()
	shelf.Priority, shelf.Special = PriorityCritical, Special{NoRotten: true}
	rack.PasteFrom(shelf)
	if rack.Effective() != ceiling || rack.Priority != PriorityCritical || !rack.Special.NoRotten || rack.Profile != ProfileWeaponRack {
		t.Fatalf("a shelf's settings pasted on a rack: %+v", rack)
	}
	dump := NewProfile(ProfileCorpseDumpSpot)
	shelf.PasteFrom(dump)
	if shelf.Effective() != (Filter{}) {
		t.Fatalf("a dump spot's settings on a shelf keep only the shelf's kinds: %v", shelf.Effective().Names())
	}
	if z := (Zone{Kind: KindShelf, Profile: ProfileMeatHook, Food: true, Wood: true}); z.Effective() != UniverseOf(KindShelf, ProfileMeatHook) ||
		z.Allows(item.Wood) || !z.Allows(item.Meat) {
		t.Fatal("a piece without a filter goes by its ceiling")
	}
	if z := (Zone{Kind: KindShelf, Food: true}); !z.Allows(item.Meat) || !z.Allows(item.Steel) {
		t.Fatal("a plain shelf's older switches still decide")
	}
	rows := RowsForProfile(KindShelf, ProfileMeatHook, NewProfile(ProfileMeatHook).Effective().Names())
	var leaves, off int
	for _, r := range rows {
		if r.Leaf {
			leaves++
		}
		if !r.On {
			off++
		}
		if r.Label == "Apparel" || r.Label == "Wood" {
			t.Fatalf("a meat hook's window shows %q", r.Label)
		}
	}
	if leaves != 4 || off != 0 {
		t.Fatalf("meat hook rows: %d leaves, %d off (%+v)", leaves, off, rows)
	}
	plain := RowsFor(KindShelf, nil)
	same := RowsForProfile(KindShelf, ProfileNone, nil)
	if len(plain) == 0 || len(plain) != len(same) {
		t.Fatal("RowsFor is the plain form")
	}
}

// TestProfileOnTheMap: zones of unequal profile never link, the map keeps a
// zone's profile through every edit, and Export/Import carry it.
func TestProfileOnTheMap(t *testing.T) {
	t.Parallel()
	m := NewMap(20, 20)
	a := m.Paint(KindShelf, NewProfile(ProfileWeaponRack), RectOf(1, 1, 2, 1), nil)
	b := m.Paint(KindShelf, NewProfile(ProfileWeaponRack), RectOf(4, 1, 5, 1), nil)
	c := m.Paint(KindShelf, NewShelf(), RectOf(7, 1, 8, 1), nil)
	d := m.Paint(KindShelf, NewProfile(ProfileHorizontalWeaponRack), RectOf(10, 1, 11, 1), nil)
	if m.Link(a, c) || m.Link(c, a) || m.Link(a, d) {
		t.Fatal("unequal profiles linked")
	}
	if z, _ := m.Zone(a); z.Group != 0 {
		t.Fatalf("a refused link starts no group: %+v", z)
	}
	if !m.Link(b, a) {
		t.Fatal("equal profiles link")
	}
	m.Update(a, func(z *Zone) { z.Profile, z.Priority = ProfileSafe, PriorityLow })
	za, _ := m.Zone(a)
	zb, _ := m.Zone(b)
	if za.Profile != ProfileWeaponRack || zb.Profile != ProfileWeaponRack || zb.Priority != PriorityLow {
		t.Fatalf("update: %+v %+v", za, zb)
	}
	m.Set(a, Zone{Profile: ProfileSafe})
	if z, _ := m.Zone(a); z.Profile != ProfileWeaponRack {
		t.Fatalf("set: %+v", z)
	}
	if ProfileAt(m, 2, 1) != ProfileWeaponRack || ProfileAt(m, 7, 1) != ProfileNone || ProfileAt(m, 0, 0) != ProfileNone || ProfileAt(m, -1, 99) != ProfileNone {
		t.Fatal("ProfileAt")
	}
	if z, ok := Import(m.Export()).Zone(d); !ok || z.Profile != ProfileHorizontalWeaponRack {
		t.Fatalf("after import: %+v", z)
	}
	// touching pieces never merge: each is its own zone
	e := m.Paint(KindShelf, NewProfile(ProfileWeaponRack), RectOf(1, 2, 2, 2), nil)
	if e == a || e == 0 {
		t.Fatalf("a piece merged with its neighbour: %d", e)
	}
}
