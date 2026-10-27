package game

import (
	"strings"

	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
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
	// PLAY-023 storage settings: Priority is the stockpile's priority
	// name ("Critical" .. "Low"; "" for zones without one), Corpses and
	// Chunks a dumping stockpile's filters.
	Priority        string
	Corpses, Chunks bool
}

// Zone kinds in ZoneView.Kind and CanZoneAt.
const (
	ZoneStockpile = "stockpile"
	ZoneGrow      = "grow"
	ZonePen       = "pen"
	ZoneDump      = "dump" // a dumping stockpile (PLAY-023)
)

var zoneKindNames = map[zone.Kind]string{zone.KindStockpile: ZoneStockpile, zone.KindGrow: ZoneGrow, zone.KindPen: ZonePen,
	zone.KindDump: ZoneDump}

func zoneKindOf(name string) (zone.Kind, bool) {
	switch name {
	case ZoneStockpile:
		return zone.KindStockpile, true
	case ZoneGrow:
		return zone.KindGrow, true
	case ZonePen:
		return zone.KindPen, true
	case ZoneDump:
		return zone.KindDump, true
	}
	return 0, false
}

// StockpileVariantDump is the DesignateStockpile command value
// (Command.Build) that paints a dumping stockpile instead of a stockpile.
const StockpileVariantDump = 1

// stockpileProto is the settings a freshly painted storage zone starts
// with: a stockpile takes food, wood and stone (and goods); a dumping
// stockpile takes animal corpses and stone chunks only.
func stockpileProto(variant uint8) (zone.Kind, zone.Zone) {
	if variant == StockpileVariantDump {
		return zone.KindDump, zone.Zone{Corpses: true, Chunks: true}
	}
	return zone.KindStockpile, zone.Zone{Food: true, Wood: true, Stone: true}
}

// StoragePriorities lists the storage priority names, highest first (the
// zone inspector's picker; the index is SetZonePriority's Index).
func StoragePriorities() []string {
	out := make([]string, len(zone.Priorities))
	for i, p := range zone.Priorities {
		out[i] = p.String()
	}
	return out
}

// pen is a legacy animal pen rectangle (inclusive), read from older saves.
type pen struct{ X1, Y1, X2, Y2 int }

// zoneSave is the saved zone map (absent in older saves). Zones carry the
// PLAY-020 settings (NoSow, NoCut, Hidden, Name) as gob fields of
// zone.Zone: an older save lacks them and loads with the defaults, and
// zero values are skipped by the state hash.
type zoneSave struct {
	W, H  int
	Cells []zone.ID
	Zones []zone.Zone
	Next  zone.ID
	// StoredAt (PLAY-023): when each stack was set down in a storage zone
	// below the best one taking it (the re-haul minimum stay); absent in
	// older saves and empty while every zone has the same priority.
	StoredAt map[entity.EntityID]uint64
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
	out := zoneSave{W: s.W, H: s.H, Cells: s.Cells, Zones: s.Zones, Next: s.Next}
	if len(g.haul.storedAt) > 0 {
		out.StoredAt = g.haul.storedAt
	}
	return out
}

// importZones restores the zone map; an older save (no map) paints its
// stockpile, grow zone and pen rectangles in saved order, so overlapping
// rectangles resolve first-wins and touching ones merge.
func (g *Game) importZones(s zoneSave) {
	g.haul.storedAt = s.StoredAt
	if len(s.Cells) > 0 || len(s.Zones) > 0 || s.Next > 0 {
		g.zones = zone.Import(zone.Saved{W: s.W, H: s.H, Cells: s.Cells, Zones: s.Zones, Next: s.Next})
		return
	}
	g.zones = zone.NewMap(g.Width(), g.Height())
	for _, r := range s.legacy.stockpiles {
		g.paintLegacyZone(zone.KindStockpile, zone.Zone{Food: r.AllowFood, Wood: r.AllowWood, Stone: r.AllowStone}, r.MinX, r.MinY, r.MaxX, r.MaxY)
	}
	for _, r := range s.legacy.growZones {
		g.paintLegacyZone(zone.KindGrow, zone.Zone{Crop: r.Crop}, r.MinX, r.MinY, r.MaxX, r.MaxY)
	}
	for _, p := range s.legacy.pens {
		g.paintLegacyZone(zone.KindPen, zone.Zone{}, p.X1, p.Y1, p.X2, p.Y2)
	}
}

// paintLegacyZone paints an older save's rectangle with the ground rules
// only (zoneCellRefusal): zones the player already had keep their edge
// and fogged cells.
func (g *Game) paintLegacyZone(kind zone.Kind, proto zone.Zone, x1, y1, x2, y2 int) {
	g.zones.Paint(kind, proto, zone.RectOf(x1, y1, x2, y2), func(x, y int) bool { return g.zoneCellRefusal(kind, x, y) == "" })
}

// Zone placement refusals (PLAY-023, as RimWorld): ZoneRefusal names
// why a zone of a kind can't claim a cell, "" when it can.
const (
	ZoneRefuseEdge     = "Too close to the map edge"
	ZoneRefuseFog      = "Unexplored"
	ZoneRefuseTaken    = "Another zone"
	ZoneRefuseRock     = "Rock"
	ZoneRefuseWater    = "Water"
	ZoneRefuseNotSoil  = "Not soil"
	ZoneRefuseFloor    = "Floor"
	ZoneRefuseBridge   = "Bridge"
	ZoneRefuseBuilding = "Building"
	ZoneRefusePlanned  = "Planned building"
	ZoneRefuseKind     = "Unknown zone"
)

// zoneEdge is how many cells along each map edge no zone may claim (an
// original value: the outermost ring, where travellers enter and leave).
const zoneEdge = 1

// zoneRefusal is why a zone of kind can't claim tile (x, y), "" when it
// can: off or on the map's edge ring, fogged, already in a zone, or a
// ground or building refusal (zoneCellRefusal).
func (g *Game) zoneRefusal(kind zone.Kind, x, y int) string {
	if x < zoneEdge || y < zoneEdge || x >= g.Width()-zoneEdge || y >= g.Height()-zoneEdge {
		return ZoneRefuseEdge
	}
	if g.fogged(x, y) {
		return ZoneRefuseFog
	}
	if _, taken := g.zones.At(x, y); taken {
		return ZoneRefuseTaken
	}
	return g.zoneCellRefusal(kind, x, y)
}

// zoneCellRefusal is the part of zoneRefusal a change under a zone
// re-checks (zoneCellsChanged): the ground and what stands or is planned
// there. A growing zone needs bare soil (GrowSoil terrain, no floor, no
// bridge deck); a stockpile or dumping stockpile takes any dry or shallow
// ground. No zone goes on rock, deep water, an impassable building or
// wall, or the blueprint of one.
func (g *Game) zoneCellRefusal(kind zone.Kind, x, y int) string {
	t := g.terr.At(x, y)
	if terrain.IsRockWall(t) {
		return ZoneRefuseRock
	}
	bridged := g.bridged(x, y)
	if kind == zone.KindGrow {
		switch {
		case terrain.IsWater(t) && !bridged:
			return ZoneRefuseWater
		case bridged:
			return ZoneRefuseBridge // bridge.go: nothing grows on a deck
		case !t.GrowSoil():
			return ZoneRefuseNotSoil
		case g.layerBldgAt(x, y, construction.LayerFloor) != 0 || g.layerFrameAt(x, y, construction.LayerFloor) != 0:
			return ZoneRefuseFloor
		}
	} else if t == terrain.DeepWater && !bridged {
		return ZoneRefuseWater // shallow water takes a stockpile (RimWorld)
	}
	if g.layerBlockerAt(x, y) != 0 || !g.grid.Walkable(x, y) {
		return ZoneRefuseBuilding // walls, impassable buildings, filled graves
	}
	if g.blockingFrameAt(x, y) != 0 {
		return ZoneRefusePlanned
	}
	return ""
}

// zoneable: a zone of kind may claim tile (x, y) (zoneRefusal is "").
func (g *Game) zoneable(kind zone.Kind, x, y int) bool { return g.zoneRefusal(kind, x, y) == "" }

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
			NoSow: z.NoSow, NoCut: z.NoCut, Hidden: z.Hidden, Name: z.Name,
		}
		if z.Kind.Storage() {
			v.Priority, v.Corpses, v.Chunks = z.Priority.String(), z.Corpses, z.Chunks
		}
		if z.Kind == zone.KindGrow {
			v.Crop = plant.Species(z.Crop).String()
		}
		out = append(out, v)
	}
	return out
}

// CanZoneAt reports whether a zone of kind could claim tile (x, y)
// (ZoneRefusal is ""). The Architect's drag ghost shows it per cell.
func (g *Game) CanZoneAt(kind string, x, y int) bool { return g.ZoneRefusal(kind, x, y) == "" }

// ZoneRefusal is why a zone of kind ("stockpile", "dump", "grow", "pen")
// can't claim tile (x, y), "" when it can: one of the ZoneRefuse reasons.
// The Architect's ghost names it under the cursor.
func (g *Game) ZoneRefusal(kind string, x, y int) string {
	k, ok := zoneKindOf(kind)
	if !ok {
		return ZoneRefuseKind
	}
	return g.zoneRefusal(k, x, y)
}

// Stockpiles.

// allows: storage zone s takes items of kind k. A stockpile goes by its
// food, wood and stone filters and welcomes goods; a dumping stockpile
// takes animal corpses (Corpses) and stone chunks (Chunks), and
// otherwise only what its food, wood and stone filters name (all off by
// default), never goods.
func allows(s zone.Zone, k item.Kind) bool {
	if s.Kind == zone.KindDump {
		switch {
		case isCarcass(k) && k != item.Bones:
			return s.Corpses
		case item.IsChunk(k):
			return s.Chunks
		case stockCats[k] == stockGoods:
			return false
		}
	}
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
		k == item.Components, k == item.IndustrialMedicine, k == item.NanomedKit, k.IsOrgan(),
		k.IsDrug(), k.IsImplant(), k == item.AshleafLeaf: // ENG-003
		return stockGoods
	}
	return stockNone
}

// filterMatches: some storage zone takes kind k.
func (g *Game) filterMatches(k item.Kind) bool {
	found := false
	g.zones.Each(func(z zone.Zone) bool {
		found = z.Kind.Storage() && allows(z, k)
		return !found
	})
	return found
}

// storageAt is the storage zone (stockpile or dumping stockpile) on tile
// (x, y).
func (g *Game) storageAt(x, y int) (zone.Zone, bool) {
	id, ok := g.zones.At(x, y)
	if !ok {
		return zone.Zone{}, false
	}
	z, _ := g.zones.Zone(id)
	return z, z.Kind.Storage()
}

func (g *Game) inMatchingStockpile(st item.Stack) bool {
	return g.inMatchingStockpileAt(st.Kind, st.X, st.Y)
}

// inMatchingStockpileAt reports a storage zone on (x, y) that takes kind k.
func (g *Game) inMatchingStockpileAt(k item.Kind, x, y int) bool {
	z, ok := g.storageAt(x, y)
	return ok && allows(z, k)
}

// noPriority is below every storage priority: storageCell's floor for
// "any zone".
const noPriority = zone.PriorityLow - 1

// stockpileCell is where a load of kind k goes: in the highest-priority
// storage zones taking k that have room (PLAY-023), nearest (nearX, nearY)
// first (ties: lower y, then x), a tile whose same-kind stack still has
// room (topping up), else an empty walkable tile with no item, planned or
// built object, or reservation.
func (g *Game) stockpileCell(k item.Kind, nearX, nearY int) (x, y int, ok bool) {
	return g.storageCell(k, nearX, nearY, nil, noPriority)
}

// stockpileCellWhere is stockpileCell over the cells keep admits (nil:
// all; area_dest.go keeps a hauler to its allowed area).
func (g *Game) stockpileCellWhere(k item.Kind, nearX, nearY int, keep func(x, y int) bool) (x, y int, ok bool) {
	return g.storageCell(k, nearX, nearY, keep, noPriority)
}

// storageCell is stockpileCellWhere over the zones whose priority is above
// floor (noPriority: every zone; a re-haul passes the priority of the zone
// the stack lies in). Priorities are tried highest first; with every zone
// at one priority it is the plain two-pass search.
func (g *Game) storageCell(k item.Kind, nearX, nearY int, keep func(x, y int) bool, floor zone.Priority) (x, y int, ok bool) {
	var prios [len(zone.Priorities)]bool // by zone.Priorities index
	found := false
	g.zones.Each(func(z zone.Zone) bool {
		if z.Kind.Storage() && z.Priority > floor && z.Priority.Valid() && allows(z, k) {
			prios[priorityIndex(z.Priority)], found = true, true
		}
		return true
	})
	if !found {
		return 0, 0, false
	}
	// loose items on storage tiles, the first per tile (as itemAt; 0:
	// none), in a reused per-tile scratch (jobindex.go)
	loose := g.looseOnStockpiles()
	defer g.releaseLoose()
	for i, p := range zone.Priorities {
		if prios[i] {
			if x, y, ok = g.storageCellAt(k, p, nearX, nearY, keep, loose); ok {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

// priorityIndex is p's index in zone.Priorities (highest first).
func priorityIndex(p zone.Priority) int { return int(zone.PriorityCritical - p) }

// storageCellAt is storageCell over the storage zones of priority p.
func (g *Game) storageCellAt(k item.Kind, p zone.Priority, nearX, nearY int, keep func(x, y int) bool, loose []entity.EntityID) (x, y int, ok bool) {
	w := g.Width()
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
	g.eachStorageCell(k, p, func(cx, cy int) {
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
	g.eachStorageCell(k, p, func(cx, cy int) {
		if loose[cy*w+cx] != 0 || !better(cx, cy) || !g.grid.Walkable(cx, cy) || !free(cx, cy) || keep != nil && !keep(cx, cy) {
			return
		}
		if g.layerFrameAt(cx, cy, construction.LayerObject) != 0 || g.occupiedAt(cx, cy) {
			return
		}
		take(cx, cy)
	})
	return x, y, ok
}

// eachStorageCell visits every tile of every storage zone of priority p
// taking kind k.
func (g *Game) eachStorageCell(k item.Kind, p zone.Priority, fn func(x, y int)) {
	g.zones.Each(func(z zone.Zone) bool {
		if z.Kind.Storage() && z.Priority == p && allows(z, k) {
			g.zones.EachCell(z.ID, func(x, y int) bool { fn(x, y); return true })
		}
		return true
	})
}

// Pens: painted pen zones are legacy (older saves, the DesignatePen
// command); a load turns them into pen markers (pens.go migratePenZones),
// and nothing else reads them.

// Changes under a zone (PLAY-023): a finished wall, impassable building,
// floor or bridge, or a change of ground, takes the cells it spoils out
// of their zone (zoneCellRefusal), splitting a zone it cuts in two (the
// biggest piece keeps the id). Each zone that lost cells gets one plain
// message, no letter: "Growing zone shrank: wood floor built".

// zoneBuilt re-checks the zoned cells under a finished building of kind.
func (g *Game) zoneBuilt(kind construction.Kind, cells [][2]int) {
	g.zoneCellsChanged(cells, construction.DefOf(kind).Name+" built")
}

// zoneGroundChanged re-checks the zone on tile (x, y) after its terrain
// changed.
func (g *Game) zoneGroundChanged(x, y int) {
	g.zoneCellsChanged([][2]int{{x, y}}, "")
}

// zoneCellsChanged unzones the cells whose zone now refuses them and
// posts one message per zone that shrank; cause names what happened (""
// names the refusal: "Growing zone shrank: not soil").
func (g *Game) zoneCellsChanged(cells [][2]int, cause string) {
	if g.zones == nil {
		return
	}
	var gone [][2]int
	type lost struct {
		id   zone.ID
		x, y int
		why  string
		name string
	}
	var hits []lost
	for _, c := range cells {
		id, ok := g.zones.At(c[0], c[1])
		if !ok {
			continue
		}
		z, _ := g.zones.Zone(id)
		why := g.zoneCellRefusal(z.Kind, c[0], c[1])
		if why == "" {
			continue
		}
		gone = append(gone, c)
		seen := false
		for _, h := range hits {
			seen = seen || h.id == id
		}
		if !seen {
			hits = append(hits, lost{id, c[0], c[1], why, zoneLabel(z)})
		}
	}
	if len(gone) == 0 {
		return
	}
	g.zones.RemoveTiles(gone)
	for _, h := range hits {
		what := cause
		if what == "" {
			what = strings.ToLower(h.why)
		}
		g.message(h.x, h.y, "%s shrank: %s", h.name, what)
	}
}

// zoneLabel is a zone's name in messages: the player's name, else its
// kind ("Growing zone", "Stockpile", "Dumping stockpile").
func zoneLabel(z zone.Zone) string {
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
