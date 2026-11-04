package world

import (
	"math"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/sim/entity"
)

var neatFacings = [...]string{"south", "east", "north", "west"}

// neatFixed are the 9 pieces of the doc's table with one sprite.
var neatFixed = map[string]bool{
	"round pedestal": true, "safe": true, "drum": true, "mini pallet": true, "pallet": true,
	"small crate": true, "large crate": true, "tall crate": true, "corpse dump spot": true,
}

// neatClosed are the pieces that show nothing they hold.
var neatClosed = map[string]bool{
	"safe": true, "drum": true, "small crate": true, "crate": true, "large crate": true, "tall crate": true,
}

// TestNeatKeys is every piece in every facing: the art contract's key, the
// same through BuildSprite and the same as the index draws from.
func TestNeatKeys(t *testing.T) {
	seen := 0
	for k := construction.Kind(0); k.Valid(); k++ {
		if !k.IsNeatStorage() {
			continue
		}
		seen++
		name := k.String()
		slug := "furniture/" + strings.ReplaceAll(name, " ", "_")
		p := neatPieceOf(uint8(k))
		if p == nil || p.name != name {
			t.Fatalf("%s: piece %+v", name, p)
		}
		for _, f := range neatFacings {
			want := slug + "_" + f
			if neatFixed[name] {
				want = slug
			}
			if got := NeatKey(name, f); got != want {
				t.Errorf("NeatKey(%s, %s) = %s, want %s", name, f, got, want)
			}
			if got := BuildSprite(name, f, 0, false, false); got != want {
				t.Errorf("BuildSprite(%s, %s) = %s, want %s", name, f, got, want)
			}
			if got := p.keys[neatFacing(f)]; got != want {
				t.Errorf("%s %s: table key %s, want %s", name, f, got, want)
			}
		}
		if !neatFixed[name] && NeatKey(name, "") != slug+"_south" {
			t.Errorf("%s: an unset facing is not south", name)
		}
		want := showFlat
		switch {
		case neatClosed[name]:
			want = showNone
		case k == construction.CorpseDumpSpot:
			want = showHeap
		case k == construction.VerticalWeaponRack:
			want = showOnEnd
		}
		if p.show != want {
			t.Errorf("%s: shows %d, want %d", name, p.show, want)
		}
		if n, ok := construction.NeatProfile(k); ok {
			if stacks := zone.Profile(n).Stacks(); stacks != 0 && stacks != p.slots {
				t.Errorf("%s: %d slots here, %d in the zone profile", name, p.slots, stacks)
			}
		}
	}
	if seen != 27 || len(neatLooks) != 27 {
		t.Fatalf("%d kinds, %d looks: want 27", seen, len(neatLooks))
	}
	if NeatKey("shelf", "south") != "" {
		t.Fatal("a plain shelf is not a neat piece")
	}
	if neatPieceOf(0) != nil || neatPieceOf(uint8(construction.Shelf)) != nil {
		t.Fatal("loose stacks and plain shelves have no neat piece")
	}
}

// TestNeatSlotCounts is the doc's Counts table: 1 slot: 2, 3: 3, 4: 4,
// 6: 17, 12: 1.
func TestNeatSlotCounts(t *testing.T) {
	got := map[int]int{}
	for i := range neatLooks {
		got[neatLooks[i].slots]++
	}
	for slots, want := range map[int]int{1: 2, 3: 3, 4: 4, 6: 17, 12: 1} {
		if got[slots] != want {
			t.Errorf("%d pieces with %d slots, want %d", got[slots], slots, want)
		}
	}
}

// TestParseMetaSlots reads slot_scale and the slot attachments once, by
// number; a stray name or a skipped number is no slot, never an error.
func TestParseMetaSlots(t *testing.T) {
	m, err := art.ParseMeta(`id: "core:furniture/x"
image: x.png
canvas: [288, 190]
slot_scale: 0.45
attachments:
  plant: [1, 2]
  slot0: [40, 90]
  slot2: [120.5, 90]
  slot_top: [9, 9]
  slot99: [9, 9]
`)
	if err != nil {
		t.Fatal(err)
	}
	if m.SlotScale != 0.45 {
		t.Fatalf("slot scale %v", m.SlotScale)
	}
	if a, ok := m.Slot(0); !ok || a != [2]float64{40, 90} {
		t.Fatalf("slot0 %v %v", a, ok)
	}
	if a, ok := m.Slot(2); !ok || a != [2]float64{120.5, 90} {
		t.Fatalf("slot2 %v %v", a, ok)
	}
	for _, n := range []int{-1, 1, 3, 99} {
		if _, ok := m.Slot(n); ok {
			t.Errorf("slot %d should be missing", n)
		}
	}
	if len(m.Slots) != 3 || m.Attach["plant"] != [2]float64{1, 2} {
		t.Fatalf("slots %v, attach %v", m.Slots, m.Attach)
	}
	if _, err := art.ParseMeta("id: a\nimage: b\ncanvas: [1, 1]\nslot_scale: big\n"); err == nil {
		t.Fatal("a bad slot_scale must be an error")
	}
	var none art.Meta
	if _, ok := none.Slot(0); ok {
		t.Fatal("no metadata, no slot")
	}
}

// TestSlotPointPlacesTheStack: the attachment is given for the first cell;
// the stack's own cell moves it; missing it sits centred on its tile.
func TestSlotPointPlacesTheStack(t *testing.T) {
	m := &art.Meta{Origin: [2]float64{16, 46}, SlotScale: 0.5, Slots: []art.SlotPoint{{X: 16 + 64, Y: 46 + 96, Set: true}}}
	first := game.ItemView{X: 5, Y: 7}
	if ax, ay, k, ok := slotPoint(m, &first); !ok || ax != 5.5 || ay != 7.75 || k != 0.5 {
		t.Fatalf("first cell: %v %v %v %v", ax, ay, k, ok)
	}
	second := game.ItemView{X: 6, Y: 7}
	if ax, ay, _, ok := slotPoint(m, &second); !ok || ax != 6.5 || ay != 7.75 {
		t.Fatalf("second cell: %v %v %v", ax, ay, ok)
	}
	overfilled := game.ItemView{X: 5, Y: 7, Slot: 200} // a spill chain's last resort
	if ax, ay, _, ok := slotPoint(m, &overfilled); !ok || ax != 5.5 || ay != 7.75 {
		t.Fatalf("past the last slot: %v %v %v", ax, ay, ok)
	}
	gap := &art.Meta{Slots: []art.SlotPoint{{Set: true}, {}, {Set: true}}, SlotScale: 0.5}
	missing := game.ItemView{X: 5, Y: 7, Slot: 1}
	if ax, ay, k, ok := slotPoint(gap, &missing); ok || ax != 5.5 || ay != 7.5 || k != 0.5 {
		t.Fatalf("missing slot: %v %v %v %v", ax, ay, k, ok)
	}
	if ax, ay, k, ok := slotPoint(&art.Meta{}, &missing); ok || ax != 5.5 || ay != 7.5 || k != 1 {
		t.Fatalf("no slots at all: %v %v %v %v", ax, ay, k, ok)
	}
	if ax, ay, k, ok := slotPoint(nil, &missing); ok || ax != 5.5 || ay != 7.5 || k != 1 {
		t.Fatalf("no sprite: %v %v %v %v", ax, ay, k, ok)
	}
}

// TestStackOriginCentresTheSprite: drawn where stackOrigin says and scaled
// about its footprint's bottom-centre, a sprite's footprint is centred on
// the slot, at any scale and footprint.
func TestStackOriginCentresTheSprite(t *testing.T) {
	for _, c := range []struct {
		k      float64
		fw, fh int
	}{{1, 1, 1}, {0.5, 1, 1}, {0.4, 2, 1}, {0.25, 2, 2}} {
		wx, wy := stackOrigin(5.5, 7.25, c.k, c.fw, c.fh)
		pivotX, pivotY := wx+float64(c.fw)/2, wy+float64(c.fh)
		centreY := pivotY - c.k*float64(c.fh)/2
		if pivotX != 5.5 || centreY != 7.25 {
			t.Errorf("k %v, %dx%d: centre (%v, %v)", c.k, c.fw, c.fh, pivotX, centreY)
		}
	}
}

// TestNeatIndexRebuildsOnlyOnChange: facing is read from the building when
// the index builds; it moves only with StructVer or another map's list.
func TestNeatIndexRebuildsOnlyOnChange(t *testing.T) {
	p := neatByName["crate"]
	bs := []game.BuildingView{
		{ID: 1, Kind: "floor", Floor: true, X: 0, Y: 0, W: 1, H: 1},
		{ID: 2, Kind: "crate", X: 4, Y: 4, W: 2, H: 1, Facing: "south"},
		{ID: 3, Kind: "wall", X: 9, Y: 9, W: 1, H: 1},
	}
	it := &game.ItemView{X: 5, Y: 4}
	var ix neatIndex
	if c, ok := ix.at(1, bs, p, it); !ok || c.x != 4 || c.y != 4 || c.face != 0 {
		t.Fatalf("second cell of the crate: %+v %v", c, ok)
	}
	bs[1].Facing = "east"
	if c, _ := ix.at(1, bs, p, it); c.face != 0 {
		t.Fatal("the same StructVer rebuilt the index")
	}
	if c, _ := ix.at(2, bs, p, it); c.face != 1 {
		t.Fatal("a new StructVer did not rebuild the index")
	}
	other := []game.BuildingView{{ID: 7, Kind: "crate", X: 5, Y: 4, W: 1, H: 1, Facing: "west"}, {ID: 8, Kind: "wall"}, {ID: 9, Kind: "wall"}}
	if c, ok := ix.at(2, other, p, it); !ok || c.x != 5 || c.face != 3 {
		t.Fatalf("another map at the same StructVer: %+v %v", c, ok)
	}
	if c, ok := ix.at(2, other, neatByName["safe"], it); ok || c.x != 5 || c.y != 4 || c.h != 1 {
		t.Fatalf("a piece the tile doesn't show: %+v %v", c, ok)
	}
}

// TestItemRowOrder: loose stacks and plain shelves keep their tile's exact
// row; a stored stack sorts after its piece's bottom row, also from an upper
// cell, before the next row, front cells and later slots over the rest.
func TestItemRowOrder(t *testing.T) {
	w := game.WorldView{StructVer: 1, Buildings: []game.BuildingView{
		{ID: 1, Kind: "pallet", X: 4, Y: 4, W: 2, H: 2},
		{ID: 2, Kind: "open shelf", X: 8, Y: 4, W: 1, H: 2, Facing: "east"},
	}}
	r := newRig(t, w, 16, 16)
	o := &r.p.Ob
	row := func(it game.ItemView) float64 { return o.itemRow(&w, &it) }
	if got := row(game.ItemView{X: 3, Y: 4}); got != 4 {
		t.Fatalf("loose row %v", got)
	}
	if got := row(game.ItemView{X: 3, Y: 4, Store: uint8(construction.Shelf), Slot: 2}); got != 4 {
		t.Fatalf("plain shelf row %v", got)
	}
	pallet := uint8(construction.Pallet)
	upper := row(game.ItemView{X: 4, Y: 4, Store: pallet})
	upperLate := row(game.ItemView{X: 4, Y: 4, Store: pallet, Slot: 11})
	lower := row(game.ItemView{X: 4, Y: 5, Store: pallet})
	if !(upper > 5 && upper < upperLate && upperLate < lower && lower+0.001 < 5.01) {
		t.Fatalf("rows: upper %v, late slot %v, lower %v", upper, upperLate, lower)
	}
	if got := row(game.ItemView{X: 8, Y: 4, Store: uint8(construction.OpenShelf)}); got <= 5 || got >= 6 {
		t.Fatalf("upper cell of a turned piece: row %v", got)
	}
	if got := row(game.ItemView{X: 4, Y: 4, Store: pallet, Slot: 250}); got != row(game.ItemView{X: 4, Y: 4, Store: pallet, Slot: 5}) {
		t.Fatalf("an overfilled cell's row %v passes its last slot's", got)
	}
	if got := row(game.ItemView{X: 12, Y: 12, Store: pallet}); got <= 12 || got >= 13 {
		t.Fatalf("a piece the buildings don't show: row %v", got)
	}
}

// TestNeatLookupsAllocateNothing: the per-stack work of a frame, once the
// index is built.
func TestNeatLookupsAllocateNothing(t *testing.T) {
	w := game.WorldView{StructVer: 1, Buildings: []game.BuildingView{{ID: 1, Kind: "open shelf", X: 4, Y: 4, W: 2, H: 1}}}
	r := newRig(t, w, 16, 16, "furniture/open_shelf_south")
	r.src.w.Items = nil
	r.frame(64)
	r.v.W = w
	r.p.art().Sprites["furniture/open_shelf_south"].Meta.Slots = []art.SlotPoint{{X: 32, Y: 64, Set: true}}
	o := &r.p.Ob
	it := &game.ItemView{X: 5, Y: 4, Store: uint8(construction.OpenShelf)}
	p := neatPieceOf(it.Store)
	sink := 0.0
	if n := testing.AllocsPerRun(200, func() {
		sink += o.itemRow(&w, it)
		ax, ay, k, _ := o.stackSlot(p, it)
		sink += ax + ay + k
		_ = p.showsCount(it.Qty, 64)
		_ = BuildSprite("open shelf", "north", 0, false, false)
	}); n != 0 {
		t.Fatalf("%v allocations per stack", n)
	}
	if sink == 0 {
		t.Fatal("no work")
	}
}

// TestShowsCount: closed pieces never; many slots only at the closest tier.
func TestShowsCount(t *testing.T) {
	few, many, closed := neatByName["display shelf"], neatByName["open shelf"], neatByName["safe"]
	near, mid := float64(LiveMin), float64(LiveMin)-1
	for _, c := range []struct {
		p    *neatPiece
		qty  int
		tile float64
		want bool
	}{
		{few, 5, near, true}, {few, 5, mid, true}, {few, 1, near, false},
		{many, 5, near, true}, {many, 5, mid, false},
		{closed, 5, near, false},
	} {
		if got := c.p.showsCount(c.qty, c.tile); got != c.want {
			t.Errorf("%s qty %d at %v: %v", c.p.name, c.qty, c.tile, got)
		}
	}
}

// neatDraws draws one frame at 64 px and lists the sprite keys drawn, in order.
func neatDraws(r *rig) []string {
	byImg := map[*ebiten.Image]string{}
	for k, sp := range r.p.art().Sprites {
		byImg[sp.Img] = k
	}
	var got []string
	r.p.Lit.OnDraw = func(img *ebiten.Image, _ float32) {
		if k, ok := byImg[img]; ok && !strings.HasPrefix(k, "terrain/") {
			got = append(got, k)
		}
	}
	r.frame(64)
	r.p.Lit.OnDraw = nil
	return got
}

func indexOf(s []string, key string) int {
	for i, k := range s {
		if k == key {
			return i
		}
	}
	return -1
}

// count is how many times key is drawn.
func count(s []string, key string) int {
	n := 0
	for _, k := range s {
		if k == key {
			n++
		}
	}
	return n
}

// TestStoredStackDrawsOverItsPiece: on a 2x2 pallet a stack in an upper cell
// draws after the pallet (the piece sorts by its bottom row), and before the
// next row's objects.
func TestStoredStackDrawsOverItsPiece(t *testing.T) {
	w := rigWorld(16, 16)
	w.Items = []game.ItemView{{ID: 30, Kind: "wood", Qty: 30, X: 4, Y: 2, Store: uint8(construction.Pallet)}}
	w.Buildings = []game.BuildingView{
		{ID: 40, Kind: "pallet", X: 4, Y: 2, W: 2, H: 2},
		{ID: 41, Kind: "end table", X: 4, Y: 4, W: 1, H: 1},
	}
	itemKey := SizedItemKey(ItemSpriteSpecies("wood", "", 4, 2), 30)
	r := newRig(t, w, 16, 16, "furniture/pallet", "furniture/end_table", itemKey)
	got := neatDraws(r)
	pallet, item, table := indexOf(got, "furniture/pallet"), indexOf(got, itemKey), indexOf(got, "furniture/end_table")
	if pallet < 0 || item < pallet || table < item {
		t.Fatalf("draw order %v", got)
	}
}

// TestClosedPieceDrawsNoStacks and the heap's limit: a safe shows nothing, a
// dump spot at most four of its twelve, a tall shelf all of its six.
func TestClosedPieceDrawsNoStacks(t *testing.T) {
	stacks := func(store construction.Kind, x, n int) []game.ItemView {
		out := make([]game.ItemView, n)
		for i := range out {
			out[i] = game.ItemView{ID: entity.EntityID(100 + i), Kind: "wood", Qty: 30, X: x, Y: 3, Store: uint8(store), Slot: uint8(i), Forbidden: true}
		}
		return out
	}
	itemKey := SizedItemKey(ItemSpriteSpecies("wood", "", 2, 3), 30)
	for _, c := range []struct {
		kind  construction.Kind
		name  string
		n     int
		drawn int
	}{
		{construction.Safe, "safe", 6, 0},
		{construction.SmallCrate, "small crate", 6, 0},
		{construction.CorpseDumpSpot, "corpse dump spot", 12, neatHeapBodies},
		{construction.NarrowTallShelf, "narrow tall shelf", 6, 6},
	} {
		w := rigWorld(16, 16)
		w.Items = stacks(c.kind, 2, c.n)
		w.Buildings = []game.BuildingView{{ID: 50, Kind: c.name, X: 2, Y: 3, W: 1, H: 1, Facing: "north"}}
		r := newRig(t, w, 16, 16, NeatKey(c.name, "north"), itemKey, "overlay/forbidden")
		if got := count(neatDraws(r), itemKey); got != c.drawn {
			t.Errorf("%s: %d stacks drawn, want %d", c.name, got, c.drawn)
		}
		if len(r.p.neat.heaps) != 0 {
			t.Errorf("%s: the heap tally survived the frame", c.name)
		}
	}
}

// TestOnEndPoseStandsTheSprite: the pose turns a level sprite's right side up
// at scale k and leaves its footprint's centre where it is.
func TestOnEndPoseStandsTheSprite(t *testing.T) {
	const tile, k = 64.0, 0.5
	g := stackOnEnd(1, 1, k, sprite.NoCS).Geo(100, 200, tile)
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
	if x, y := g.Apply(100+tile/2, 200+tile/2); !near(x, 132) || !near(y, 232) {
		t.Fatalf("the centre moved to (%v, %v)", x, y)
	}
	if x, y := g.Apply(100+tile, 200+tile/2); !near(x, 132) || !near(y, 232-tile/2*k) {
		t.Fatalf("the right edge went to (%v, %v), want straight up from the centre", x, y)
	}
}

// TestOnEndDrawsAndAllocatesNothing: the vertical rack's turned stack draws
// once, and the work only it does (the pose and its transform) allocates
// nothing. The draw call they lead to is the level stack's: counts taken
// through it vary by one under -race, for a level stack and a turned one alike.
func TestOnEndDrawsAndAllocatesNothing(t *testing.T) {
	itemKey := SizedItemKey(ItemSpriteSpecies("wood", "", 2, 3), 30)
	w := rigWorld(16, 16)
	w.Items = []game.ItemView{{ID: 80, Kind: "wood", Qty: 30, X: 2, Y: 3, Store: uint8(construction.VerticalWeaponRack)}}
	w.Buildings = []game.BuildingView{{ID: 81, Kind: "vertical weapon rack", X: 2, Y: 3, W: 2, H: 1}}
	r := newRig(t, w, 16, 16, NeatKey("vertical weapon rack", "south"), itemKey)
	if got := count(neatDraws(r), itemKey); got != 1 {
		t.Fatalf("stack drawn %d times", got)
	}
	m := r.p.art().MetaOf(itemKey)
	sink := 0.0
	if n := testing.AllocsPerRun(200, func() {
		g := creature.SpritePoseGeo(*m, 100, 200, 64, stackOnEnd(1, 1, 0.4, sprite.NoCS))
		x, _ := g.Apply(1, 1)
		sink += x
	}); n != 0 {
		t.Fatalf("%v allocations per turned stack", n)
	}
	if sink == 0 {
		t.Fatal("no work")
	}
}

// TestHeapTalliesOneCount: a heap's stacks add to one tally per tile, closed
// pieces to none, and the pass empties it.
func TestHeapTalliesOneCount(t *testing.T) {
	r := newRig(t, rigWorld(16, 16), 16, 16)
	r.frame(64)
	lb := &r.p.Lb
	dump := neatByName["corpse dump spot"]
	for i := 0; i < 12; i++ {
		lb.storedCount(r.screen, dump, &game.ItemView{X: 2, Y: 2, Qty: 1, Store: uint8(construction.CorpseDumpSpot), Slot: uint8(i)}, 64)
	}
	if len(r.p.neat.heaps) != 1 || r.p.neat.heaps[[2]int{2, 2}] != 12 {
		t.Fatalf("tally %v", r.p.neat.heaps)
	}
	lb.storedCount(r.screen, neatByName["safe"], &game.ItemView{X: 3, Y: 2, Qty: 9}, 64)
	if len(r.p.neat.heaps) != 1 {
		t.Fatalf("a closed piece was tallied: %v", r.p.neat.heaps)
	}
	lb.heapCounts(r.screen, 64)
	if len(r.p.neat.heaps) != 0 {
		t.Fatalf("tally left: %v", r.p.neat.heaps)
	}
}

// TestNeatDrawsWithoutArt: no building sprite, no attachment, no building at
// all, a slot past the layout, a corpse with nobody to draw: no crash, and a
// stack still shows where it can.
func TestNeatDrawsWithoutArt(t *testing.T) {
	itemKey := SizedItemKey(ItemSpriteSpecies("wood", "", 2, 3), 30)
	w := rigWorld(16, 16)
	w.Items = []game.ItemView{
		{ID: 60, Kind: "wood", Qty: 30, X: 2, Y: 3, Store: uint8(construction.OpenShelf), Slot: 5}, // sprite without attachments
		{ID: 61, Kind: "wood", Qty: 30, X: 4, Y: 3, Store: uint8(construction.Wardrobe)},           // no sprite at all
		{ID: 62, Kind: "wood", Qty: 30, X: 6, Y: 3, Store: uint8(construction.TextileRack)},        // no building under it
		{ID: 63, Kind: "human corpse", Qty: 1, X: 8, Y: 3, Store: uint8(construction.CorpseDumpSpot)},
		{ID: 64, Kind: "bones", Qty: 1, X: 8, Y: 3, Store: uint8(construction.CorpseDumpSpot), Slot: 1},
		{ID: 65, Kind: "wood", Qty: 30, X: 8, Y: 3, Store: uint8(construction.CorpseDumpSpot), Slot: 2},
	}
	w.Buildings = []game.BuildingView{
		{ID: 70, Kind: "open shelf", X: 2, Y: 3, W: 2, H: 1},
		{ID: 71, Kind: "wardrobe", X: 4, Y: 3, W: 2, H: 1, Facing: "west"},
		{ID: 72, Kind: "corpse dump spot", X: 8, Y: 3, W: 1, H: 1},
	}
	r := newRig(t, w, 16, 16, "furniture/open_shelf_south", itemKey)
	if got := count(neatDraws(r), itemKey); got != 4 {
		t.Fatalf("%d stacks drawn, want 4", got)
	}
	for _, tile := range []float64{float64(LiveMin) - 1, 8} { // the other tiers
		r.frame(tile)
	}
}
