package store

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// neatRows pairs each of the 27 pieces with the profile the gap document gives
// it: the two tables' orders are saved numbers, so they are checked name by name.
var neatRows = []struct {
	k construction.Kind
	p zone.Profile
}{
	{construction.DisplayCase, zone.ProfileDisplayCase},
	{construction.RoundPedestal, zone.ProfileRoundPedestal},
	{construction.SmallDisplayShelf, zone.ProfileSmallDisplayShelf},
	{construction.DisplayShelf, zone.ProfileDisplayShelf},
	{construction.WeaponRack, zone.ProfileWeaponRack},
	{construction.NarrowWardrobe, zone.ProfileNarrowWardrobe},
	{construction.Wardrobe, zone.ProfileWardrobe},
	{construction.NarrowDisplayCabinet, zone.ProfileNarrowDisplayCabinet},
	{construction.DisplayCabinet, zone.ProfileDisplayCabinet},
	{construction.OpenShelf, zone.ProfileOpenShelf},
	{construction.TwoTierDisplayShelf, zone.ProfileTwoTierDisplayShelf},
	{construction.NarrowTallShelf, zone.ProfileNarrowTallShelf},
	{construction.TallShelf, zone.ProfileTallShelf},
	{construction.Safe, zone.ProfileSafe},
	{construction.Drum, zone.ProfileDrum},
	{construction.HorizontalWeaponRack, zone.ProfileHorizontalWeaponRack},
	{construction.VerticalWeaponRack, zone.ProfileVerticalWeaponRack},
	{construction.TextileRack, zone.ProfileTextileRack},
	{construction.MeatHook, zone.ProfileMeatHook},
	{construction.MiniPallet, zone.ProfileMiniPallet},
	{construction.HalfPallet, zone.ProfileHalfPallet},
	{construction.Pallet, zone.ProfilePallet},
	{construction.SmallCrate, zone.ProfileSmallCrate},
	{construction.Crate, zone.ProfileCrate},
	{construction.LargeCrate, zone.ProfileLargeCrate},
	{construction.TallCrate, zone.ProfileTallCrate},
	{construction.CorpseDumpSpot, zone.ProfileCorpseDumpSpot},
}

// TestZoneKindNeat: every piece lays a shelf zone of its own profile, name and
// settings, its readout counts its own slots and it finds its building; the
// older buildings stay at profile zero.
func TestZoneKindNeat(t *testing.T) {
	t.Parallel()
	if len(neatRows) != 27 {
		t.Fatalf("%d rows", len(neatRows))
	}
	for _, r := range neatRows {
		zk, z, ok := ZoneKind(r.k)
		if !ok || zk != zone.KindShelf || z.Kind != zone.KindShelf || z.Profile != r.p || !z.Hidden || z.Effective() != zone.UniverseOf(zone.KindShelf, r.p) {
			t.Errorf("%s: %v %+v %v", r.k, zk, z, ok)
		}
		if k, ok := ProfileKind(r.p); !ok || k != r.k {
			t.Errorf("profile %d is kind %v, want %s", r.p, k, r.k)
		}
		if ProfileName(r.p) != construction.DefOf(r.k).Name {
			t.Errorf("profile %d is named %q", r.p, ProfileName(r.p))
		}
		rows := ProfileRows(construction.DefOf(r.k).Name, z.Effective().Names())
		if len(rows) == 0 {
			t.Errorf("%s: no window rows", r.k)
		}
		for _, row := range rows {
			if !row.On {
				t.Errorf("%s: a new piece's window has %q off", r.k, row.Path)
			}
		}
		w := newWorld()
		id := w.build(r.k, 5, 5)
		var s Buildings
		lz, ok := s.ZoneOf(w.zs, w, id)
		if !ok || lz.Profile != r.p || s.Of(w.zs, w, lz) != id {
			t.Errorf("%s: laid %+v (%v), of %d", r.k, lz, ok, s.Of(w.zs, w, lz))
		}
		cells, _, _ := construction.Footprint(r.k, construction.South, 5, 5)
		if _, _, c := s.Fill(w.zs, w, lz, false); c != len(cells)*r.p.Stacks() || lz.Cells != len(cells) {
			t.Errorf("%s: capacity %d, cells %d", r.k, c, lz.Cells)
		}
	}
	for _, k := range []construction.Kind{construction.Shelf, construction.SmallShelf, construction.EggBox, construction.Bookcase, construction.SmallBookcase, construction.Hopper, construction.Grave, construction.Sarcophagus} {
		if _, z, ok := ZoneKind(k); !ok || z.Profile != zone.ProfileNone {
			t.Errorf("%s: %+v %v", k, z, ok)
		}
	}
	for _, p := range []zone.Profile{zone.ProfileNone, zone.ProfileCorpseDumpSpot + 1, 255} {
		if _, ok := ProfileKind(p); ok || ProfileName(p) != "" {
			t.Errorf("profile %d names a piece", p)
		}
	}
	if ProfileRows("shelf", nil) != nil || ProfileRows("nonsense", nil) != nil {
		t.Fatal("rows for a non-piece")
	}
}

// moved moves building id to (x, y) as a reinstall does, lays its zone and
// reports the zone it then carries.
func moved(t *testing.T, w *world, id entity.EntityID, x, y int) zone.Zone {
	t.Helper()
	b := w.bldgs[id]
	b.X, b.Y = x, y
	w.bldgs[id] = b
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, x, y)
	Buildings{}.Lay(w.zs, w, b.Kind, cells)
	z, ok := Buildings{}.ZoneOf(w.zs, w, id)
	if !ok {
		t.Fatalf("building %d carries no zone at (%d, %d)", id, x, y)
	}
	return z
}

// TestLayKeepsItsOwnProfile: a piece moved away or onto its own cells keeps its
// settings; a Lay never adopts a stale zone of another profile, and lifts it.
func TestLayKeepsItsOwnProfile(t *testing.T) {
	t.Parallel()
	var s Buildings
	w := newWorld()
	crate := w.build(construction.Crate, 2, 2)
	z, _ := s.ZoneOf(w.zs, w, crate)
	w.zs.Update(z.ID, func(z *zone.Zone) { z.Priority, z.Name = zone.PriorityCritical, "mine" })
	far := moved(t, w, crate, 10, 10)
	if far.Priority != zone.PriorityCritical || far.Name != "mine" || far.Profile != zone.ProfileCrate {
		t.Fatalf("moved away: %+v", far)
	}
	if _, in := w.zs.At(2, 2); in {
		t.Fatal("the old cells stayed zoned")
	}
	near := moved(t, w, crate, 11, 10) // onto half of its own cells
	if near.Priority != zone.PriorityCritical || near.Name != "mine" || near.Profile != zone.ProfileCrate {
		t.Fatalf("moved onto its own cells: %+v", near)
	}
	if id, in := w.zs.At(10, 10); in {
		t.Fatalf("the old half stayed zoned: %d", id)
	}

	// a stale zone of another profile, away from the footprint: lifted, not adopted
	w = newWorld()
	rack := w.build(construction.WeaponRack, 2, 2)
	rz, _ := s.ZoneOf(w.zs, w, rack)
	w.zs.Update(rz.ID, func(z *zone.Zone) { z.Priority = zone.PriorityCritical })
	delete(w.bldgs, rack)
	w.ids = nil
	safe := w.build(construction.Safe, 10, 10)
	sz, _ := s.ZoneOf(w.zs, w, safe)
	if sz.Profile != zone.ProfileSafe || sz.Priority != zone.PriorityPreferred || sz.Effective() != zone.UniverseOf(zone.KindShelf, zone.ProfileSafe) {
		t.Fatalf("a safe adopted a stale rack's settings: %+v", sz)
	}
	if _, in := w.zs.At(2, 2); in {
		t.Fatal("the stale zone stayed")
	}

	// a stale zone under the footprint: the same
	w = newWorld()
	rack = w.build(construction.WeaponRack, 5, 5)
	rz, _ = s.ZoneOf(w.zs, w, rack)
	w.zs.Update(rz.ID, func(z *zone.Zone) { z.Priority = zone.PriorityLow })
	delete(w.bldgs, rack)
	w.ids = nil
	cr := w.build(construction.Crate, 5, 5)
	cz, _ := s.ZoneOf(w.zs, w, cr)
	if cz.Profile != zone.ProfileCrate || cz.Priority != zone.PriorityNormal {
		t.Fatalf("a crate on a stale rack's cells: %+v", cz)
	}

	// a stale piece of a linked pair leaves its group: the other is alone again
	w = newWorld()
	a, b := w.build(construction.WeaponRack, 2, 2), w.build(construction.WeaponRack, 2, 5)
	if !s.Link(w.zs, w, b, a) {
		t.Fatal("two racks link")
	}
	delete(w.bldgs, a)
	w.ids = []entity.EntityID{b}
	w.build(construction.Crate, 10, 10)
	if bz, _ := s.ZoneOf(w.zs, w, b); bz.Group != 0 {
		t.Fatalf("a group of one stayed: %+v", bz)
	}

	// a zone whose profile isn't the building's is no building's zone, and
	// LayMissing lays the right one
	w = newWorld()
	shelf := w.build(construction.Crate, 5, 5)
	cells, _, _ := construction.Footprint(construction.Crate, construction.South, 5, 5)
	w.zs.RemoveTiles(cells)
	wrong := w.zs.Paint(zone.KindShelf, zone.NewProfile(zone.ProfileSafe), zone.RectOf(5, 5, 6, 5), nil)
	wz, _ := w.zs.Zone(wrong)
	if _, ok := s.ZoneOf(w.zs, w, shelf); ok || s.Of(w.zs, w, wz) != 0 {
		t.Fatal("a crate claims a safe's zone")
	}
	s.LayMissing(w.zs, w)
	if z, ok := s.ZoneOf(w.zs, w, shelf); !ok || z.Profile != zone.ProfileCrate {
		t.Fatalf("LayMissing: %+v", z)
	}
}

// TestLinkEqualProfilesOnly: two pieces of one kind link; a rack, a shelf and a
// crate don't; two plain shelves and a shelf with an egg box still do.
func TestLinkEqualProfilesOnly(t *testing.T) {
	t.Parallel()
	var s Buildings
	w := newWorld()
	rack1, rack2 := w.build(construction.WeaponRack, 1, 1), w.build(construction.WeaponRack, 1, 3)
	flat := w.build(construction.HorizontalWeaponRack, 1, 5)
	shelf, egg := w.build(construction.Shelf, 1, 7), w.build(construction.EggBox, 1, 9)
	crate := w.build(construction.Crate, 1, 11)
	if s.Link(w.zs, w, rack1, shelf) || s.Link(w.zs, w, shelf, rack1) || s.Link(w.zs, w, rack1, flat) || s.Link(w.zs, w, crate, rack1) {
		t.Fatal("unequal profiles linked")
	}
	if !s.Link(w.zs, w, rack2, rack1) || !s.Link(w.zs, w, egg, shelf) {
		t.Fatal("equal profiles refuse to link")
	}
}

// TestSearchFillsToSlots: a search fills a cell to its piece's slots, a
// one-slot piece and a 12-slot dump spot alike, and refuses what the piece
// refuses.
func TestSearchFillsToSlots(t *testing.T) {
	t.Parallel()
	cases := []struct {
		k    construction.Kind
		st   item.Kind
		slot int
		no   item.Kind
	}{
		{construction.RoundPedestal, item.Knife, 1, item.Meal},
		{construction.SmallDisplayShelf, item.Knife, 3, item.Meal},
		{construction.NarrowWardrobe, item.Parka, 4, item.Knife},
		{construction.MiniPallet, item.Wood, 6, item.GraniteChunk},
		{construction.CorpseDumpSpot, item.CorpseHuman, 12, item.Wood},
	}
	for _, c := range cases {
		w := newWorld()
		w.build(c.k, 5, 5)
		st := item.Stack{Kind: c.st, Qty: 1}
		floor := zone.PriorityLow - 1
		if _, _, ok := Search(w, item.Stack{Kind: c.no, Qty: 1}, 5, 5, nil, floor, false); ok {
			t.Errorf("%s takes %s", c.k, c.no)
		}
		for i := range c.slot {
			x, y, ok := Search(w, st, 5, 5, nil, floor, false)
			if !ok || x != 5 || y != 5 {
				t.Fatalf("%s: stack %d goes to (%d, %d, %v)", c.k, i, x, y, ok)
			}
			w.put(c.st, c.st.StackLimit(), x, y)
		}
		if x, y, ok := Search(w, st, 5, 5, nil, floor, false); ok {
			t.Errorf("%s takes more than %d stacks: (%d, %d)", c.k, c.slot, x, y)
		}
	}
}

// slotGround is the item views' cell list for the fake world.
type slotGround struct{ w *world }

func (g slotGround) LooseAt(x, y int) []entity.EntityID {
	var out []entity.EntityID
	for _, id := range g.w.items {
		if st := g.w.stacks[id]; st.X == x && st.Y == y && st.Carrier == 0 {
			out = append(out, id)
		}
	}
	return out
}

// TestItemSlot: a stack on a piece reads the piece and its index on the cell;
// a carried one, a loose one and one on a plain shelf read zero.
func TestItemSlot(t *testing.T) {
	t.Parallel()
	var s Buildings
	w := newWorld()
	w.build(construction.Crate, 5, 5)
	w.build(construction.Shelf, 5, 8)
	a, other, b := w.put(item.Wood, 5, 5, 5), w.put(item.Wood, 5, 6, 5), w.put(item.Steel, 5, 5, 5)
	g := slotGround{w}
	for _, c := range []struct {
		id    entity.EntityID
		piece uint8
		slot  uint8
	}{{a, uint8(construction.Crate), 0}, {other, uint8(construction.Crate), 0}, {b, uint8(construction.Crate), 1}} {
		if p, n := s.Slot(w.zs, g, c.id, w.stacks[c.id]); p != c.piece || n != c.slot {
			t.Errorf("stack %d: piece %d slot %d, want %d %d", c.id, p, n, c.piece, c.slot)
		}
	}
	carried := w.stacks[a]
	carried.Carrier = 7
	if p, n := s.Slot(w.zs, g, a, carried); p != 0 || n != 0 {
		t.Fatal("a carried stack has no slot")
	}
	loose := w.put(item.Wood, 5, 20, 20)
	if p, n := s.Slot(w.zs, g, loose, w.stacks[loose]); p != 0 || n != 0 {
		t.Fatal("a loose stack has no slot")
	}
	shelved := w.put(item.Wood, 5, 5, 8)
	if p, n := s.Slot(w.zs, g, shelved, w.stacks[shelved]); p != 0 || n != 0 {
		t.Fatal("a plain shelf's stack has no slot layout")
	}
}

// benchCells is the storage search's reads over a map with no stacks: what the
// benchmark measures is the walk over the zones, as store.Search does it.
type benchCells struct {
	zs     *zone.Map
	stores zone.Stores
	full   map[[2]int]int
}

func (b *benchCells) Stores(p zone.Priority) []zone.Store { return b.stores.Of(b.zs, p) }
func (b *benchCells) Zones() *zone.Map                    { return b.zs }
func (b *benchCells) First(x, y int) (item.Stack, bool) {
	if b.full[[2]int{x, y}] > 0 {
		return item.Stack{Kind: item.Steel, Qty: item.StackLimit}, true
	}
	return item.Stack{}, false
}
func (b *benchCells) Shelf(k item.Kind, x, y int) (int, bool) { return b.full[[2]int{x, y}], false }
func (b *benchCells) TileTaken(x, y int) bool                 { return false }
func (b *benchCells) Blocked(x, y int, building bool) bool    { return false }

// searchWorld is n one-cell building zones in a grid, the first half of them
// full, each laid by proto(i).
func searchWorld(n int, proto func(i int) zone.Zone) *benchCells {
	const side = 40
	b := &benchCells{zs: zone.NewMap(side*2, side*2), full: map[[2]int]int{}}
	for i := range n {
		x, y := (i%side)*2, (i/side)*2
		b.zs.Paint(zone.KindShelf, proto(i), zone.RectOf(x, y, x, y), nil)
		if i < n/2 {
			b.full[[2]int{x, y}] = 12
		}
	}
	return b
}

// BenchmarkSearchManyShelves is the storage search over 300 building zones: 300
// plain shelves against 300 pieces of mixed profile and priority. Mixed must
// cost no more than the zone count scales.
func BenchmarkSearchManyShelves(b *testing.B) {
	st := item.Stack{Kind: item.Steel, Qty: 10}
	for _, c := range []struct {
		name  string
		proto func(i int) zone.Zone
	}{
		{"shelves", func(int) zone.Zone { return zone.NewShelf() }},
		{"neat", func(i int) zone.Zone {
			z := zone.NewProfile(zone.Profile(i%27 + 1))
			z.Priority = zone.Priorities[i%len(zone.Priorities)]
			return z
		}},
		{"crates", func(int) zone.Zone { return zone.NewProfile(zone.ProfileCrate) }},
	} {
		b.Run(c.name, func(b *testing.B) {
			cells := searchWorld(300, c.proto)
			b.ReportAllocs()
			for b.Loop() {
				if _, _, ok := Search(cells, st, 40, 40, nil, zone.PriorityLow-1, false); !ok {
					b.Fatal("no cell")
				}
			}
		})
	}
}
