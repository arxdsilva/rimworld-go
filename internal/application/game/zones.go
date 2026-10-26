package game

import (
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
)

// Zones (ZONE-001): stockpiles, grow zones and animal pens live in one
// per-tile zone map (zone.Map). A tile is in at most one zone; painting
// skips zoned tiles, touching zones of the same kind (grow zones: the same
// crop) merge, and cancelling tiles shrinks or splits a zone.

// ZoneView is one zone for the UI: its tiles and settings.
type ZoneView struct {
	ID    uint16
	Kind  string // "stockpile" | "grow" | "pen"
	Crop  string // grow zones: the species name ("tree" for tree zones)
	Food  bool   // stockpile filters
	Wood  bool
	Stone bool
	Cells [][2]int // row-major
	// The bounding box, for labels and panels.
	MinX, MinY, MaxX, MaxY int
	// PLAY-020 settings: sowing or cutting off, hidden overlay, the
	// player's name ("" = the default).
	NoSow, NoCut, Hidden bool
	Name                 string
}

// Zone kinds in ZoneView.Kind and CanZoneAt.
const (
	ZoneStockpile = "stockpile"
	ZoneGrow      = "grow"
	ZonePen       = "pen"
)

var zoneKindNames = map[zone.Kind]string{zone.KindStockpile: ZoneStockpile, zone.KindGrow: ZoneGrow, zone.KindPen: ZonePen}

func zoneKindOf(name string) (zone.Kind, bool) {
	switch name {
	case ZoneStockpile:
		return zone.KindStockpile, true
	case ZoneGrow:
		return zone.KindGrow, true
	case ZonePen:
		return zone.KindPen, true
	}
	return 0, false
}

// pen is a legacy animal pen rectangle (inclusive), read from older saves.
type pen struct{ X1, Y1, X2, Y2 int }

// zoneSave is the saved zone map (absent in older saves).
type zoneSave struct {
	W, H  int
	Cells []zone.ID
	Zones []zone.Zone
	Next  zone.ID
	// legacy rectangles of an older save (not encoded: Load fills them
	// from the old saveState fields for migration).
	legacy legacyZones
}

type legacyZones struct {
	stockpiles []zone.Stockpile
	growZones  []zone.GrowZone
	pens       []pen
}

func (g *Game) exportZones() zoneSave {
	s := g.zones.Export()
	return zoneSave{W: s.W, H: s.H, Cells: s.Cells, Zones: s.Zones, Next: s.Next}
}

// importZones restores the zone map; an older save (no map) paints its
// stockpile, grow zone and pen rectangles in saved order, so overlapping
// rectangles resolve first-wins and touching ones merge.
func (g *Game) importZones(s zoneSave) {
	if len(s.Cells) > 0 || len(s.Zones) > 0 || s.Next > 0 {
		g.zones = zone.Import(zone.Saved{W: s.W, H: s.H, Cells: s.Cells, Zones: s.Zones, Next: s.Next})
		return
	}
	g.zones = zone.NewMap(g.Width(), g.Height())
	for _, r := range s.legacy.stockpiles {
		g.paintZone(zone.KindStockpile, zone.Zone{Food: r.AllowFood, Wood: r.AllowWood, Stone: r.AllowStone}, r.MinX, r.MinY, r.MaxX, r.MaxY)
	}
	for _, r := range s.legacy.growZones {
		g.paintZone(zone.KindGrow, zone.Zone{Crop: r.Crop}, r.MinX, r.MinY, r.MaxX, r.MaxY)
	}
	for _, p := range s.legacy.pens {
		g.paintZone(zone.KindPen, zone.Zone{}, p.X1, p.Y1, p.X2, p.Y2)
	}
}

// zoneable: a zone of kind may claim tile (x, y) (zoned or not): in
// bounds, walkable and, for grow zones, fertile.
func (g *Game) zoneable(kind zone.Kind, x, y int) bool {
	if x < 0 || y < 0 || x >= g.Width() || y >= g.Height() || !g.grid.Walkable(x, y) {
		return false
	}
	t := g.terr.At(x, y)
	if terrain.IsRockWall(t) || g.isWaterAt(x, y) {
		return false // no zone on rock or on water (NG-1 §5.6); a bridge deck is dry
	}
	if kind == zone.KindGrow && g.bridged(x, y) {
		return false // bridge.go: nothing grows on a deck (stockpiles are fine)
	}
	return kind != zone.KindGrow || t.Fertility() > 0
}

// paintZone paints a dragged rectangle as kind with proto's settings.
func (g *Game) paintZone(kind zone.Kind, proto zone.Zone, x1, y1, x2, y2 int) zone.ID {
	return g.zones.Paint(kind, proto, zone.RectOf(x1, y1, x2, y2), func(x, y int) bool { return g.zoneable(kind, x, y) })
}

// zoneOf is the zone of kind on tile (x, y).
func (g *Game) zoneOf(kind zone.Kind, x, y int) (zone.Zone, bool) {
	id, ok := g.zones.At(x, y)
	if !ok {
		return zone.Zone{}, false
	}
	z, _ := g.zones.Zone(id)
	return z, z.Kind == kind
}

// zoneViews lists every zone in id order.
func (g *Game) zoneViews() []ZoneView {
	zs := g.zones.Zones()
	out := make([]ZoneView, 0, len(zs))
	for _, z := range zs {
		b, _ := g.zones.Bounds(z.ID)
		v := ZoneView{
			ID: uint16(z.ID), Kind: zoneKindNames[z.Kind], Food: z.Food, Wood: z.Wood, Stone: z.Stone,
			Cells: g.zones.Cells(z.ID), MinX: b.MinX, MinY: b.MinY, MaxX: b.MaxX, MaxY: b.MaxY,
		}
		if z.Kind == zone.KindGrow {
			v.Crop = plant.Species(z.Crop).String()
		}
		out = append(out, v)
	}
	return out
}

// CanZoneAt reports whether a zone of kind could claim tile (x, y): in
// bounds, walkable, not already in any zone and, for grow zones, fertile.
// The Architect's drag ghost shows it per cell.
func (g *Game) CanZoneAt(kind string, x, y int) bool {
	k, ok := zoneKindOf(kind)
	if !ok || !g.zoneable(k, x, y) {
		return false
	}
	_, taken := g.zones.At(x, y)
	return !taken
}

// Stockpiles.

func allows(s zone.Zone, k item.Kind) bool {
	switch stockCats[k] {
	case stockFood:
		return s.Food
	case stockWood:
		return s.Wood
	case stockStone:
		return s.Stone
	case stockGoods:
		return true
	}
	return false
}

// stockCat is the stockpile filter an item kind falls under.
type stockCat uint8

const (
	stockNone  stockCat = iota // never stored (the dead, unknown kinds)
	stockFood                  // the Food filter
	stockWood                  // the Wood filter
	stockStone                 // the Stone filter
	stockGoods                 // goods: always welcome
)

// stockCats is stockCatOf for every kind, worked out once (allows runs
// for every loose item in the hauling search).
var stockCats = func() (t [256]stockCat) {
	for k := range t {
		t[k] = stockCatOf(item.Kind(k))
	}
	return t
}()

func stockCatOf(k item.Kind) stockCat {
	switch {
	case k == item.CorpseHuman, k == item.Bones:
		return stockNone // the dead go to graves and pyres, not shelves
	case k.IsFood(), k.IsCorpse():
		return stockFood
	case k == item.Wood:
		return stockWood
	case k == item.Stone, item.IsChunk(k), item.IsStoneBlocks(k):
		return stockStone
	case k == item.FertileEgg: // SIM-026: kept with the food; they hatch in place
		return stockFood
	case k == item.Leather, k == item.HerbalMedicine, k == item.Cloth, k == item.Straw, k.IsCarpet(), k.IsApparel(), k.IsWeapon(),
		k == item.Wool, k == item.EmberOil, item.IsMetal(k),
		k == item.Components, k == item.IndustrialMedicine,
		k.IsDrug(), k.IsImplant(), k == item.AshleafLeaf: // ENG-003
		return stockGoods
	}
	return stockNone
}

// filterMatches: some stockpile takes kind k.
func (g *Game) filterMatches(k item.Kind) bool {
	found := false
	g.zones.Each(func(z zone.Zone) bool {
		found = z.Kind == zone.KindStockpile && allows(z, k)
		return !found
	})
	return found
}

func (g *Game) inMatchingStockpile(st item.Stack) bool {
	return g.inMatchingStockpileAt(st.Kind, st.X, st.Y)
}

// inMatchingStockpileAt reports a stockpile on (x, y) that takes kind k.
func (g *Game) inMatchingStockpileAt(k item.Kind, x, y int) bool {
	z, ok := g.zoneOf(zone.KindStockpile, x, y)
	return ok && allows(z, k)
}

// stockpileCell is where a load of kind k goes, nearest (nearX, nearY)
// first (ties: lower y, then x): a stockpile tile whose same-kind stack
// still has room (topping up), else an empty walkable stockpile tile with
// no item, planned or built object, or reservation.
func (g *Game) stockpileCell(k item.Kind, nearX, nearY int) (x, y int, ok bool) {
	return g.stockpileCellWhere(k, nearX, nearY, nil)
}

// stockpileCellWhere is stockpileCell over the cells keep admits (nil:
// all; area_dest.go keeps a hauler to its allowed area).
func (g *Game) stockpileCellWhere(k item.Kind, nearX, nearY int, keep func(x, y int) bool) (x, y int, ok bool) {
	w := g.Width()
	// loose items on stockpile tiles, the first per tile (as itemAt; 0:
	// none), in a reused per-tile scratch (jobindex.go)
	loose := g.looseOnStockpiles()
	defer g.releaseLoose()
	bestD := 1 << 30
	better := func(cx, cy int) bool {
		d := pathing.Dist(nearX, nearY, cx, cy)
		return !ok || d < bestD || d == bestD && (cy < y || cy == y && cx < x)
	}
	take := func(cx, cy int) { x, y, ok, bestD = cx, cy, true, pathing.Dist(nearX, nearY, cx, cy) }
	free := func(cx, cy int) bool {
		_, taken := g.reserve.TileOwner(cx, cy)
		return !taken
	}
	g.eachStockpileCell(k, func(cx, cy int) {
		oid := loose[cy*w+cx]
		if oid == 0 || !better(cx, cy) || keep != nil && !keep(cx, cy) {
			return
		}
		o, _ := g.stacks.Get(oid)
		if o.Kind == k && o.Qty+item.CarryLimit <= item.StackLimit && free(cx, cy) {
			take(cx, cy)
		}
	})
	if ok {
		return x, y, true
	}
	g.eachStockpileCell(k, func(cx, cy int) {
		if loose[cy*w+cx] != 0 || !better(cx, cy) || !g.grid.Walkable(cx, cy) || !free(cx, cy) || keep != nil && !keep(cx, cy) {
			return
		}
		if g.layerFrameAt(cx, cy, construction.LayerObject) != 0 || g.layerBldgAt(cx, cy, construction.LayerObject) != 0 {
			return
		}
		take(cx, cy)
	})
	return x, y, ok
}

// eachStockpileCell visits every tile of every stockpile taking kind k.
func (g *Game) eachStockpileCell(k item.Kind, fn func(x, y int)) {
	g.zones.Each(func(z zone.Zone) bool {
		if z.Kind == zone.KindStockpile && allows(z, k) {
			g.zones.EachCell(z.ID, func(x, y int) bool { fn(x, y); return true })
		}
		return true
	})
}

// Pens: painted pen zones are legacy (older saves, the DesignatePen
// command); a load turns them into pen markers (pens.go migratePenZones),
// and nothing else reads them.
