package game

import (
	"reflect"
	"slices"
	"strings"

	"rimworld-go/internal/application/game/store"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
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
	// PRD-4 storage filters (stockpile_filter.go): Filter is the allowed
	// filter-tree nodes, compact (zone.Filter.Names); Special the special
	// filters on and the narrowed ranges (zone.Special.Names). Shelf is a
	// shelf's storage zone's building id (0: no shelf).
	Filter, Special []string
	Shelf           uint64
	// STO/CBT contract (0 until STO-2): storage Group id, Stored stacks of Capacity.
	Group            uint64
	Stored, Capacity int
}

// Zone kinds in ZoneView.Kind and CanZoneAt.
const (
	ZoneStockpile = "stockpile"
	ZoneGrow      = "grow"
	ZonePen       = "pen"
	ZoneDump      = "dump"   // a dumping stockpile (PLAY-023)
	ZoneShelf     = "shelf"  // a shelf's storage (PRD-4): laid by the shelf, never painted
	ZoneHopper    = "hopper" // STO-2, laid by their buildings: a hopper's raw food,
	ZoneGrave     = "grave"  // a grave's or sarcophagus's settings (Shelf names the building)
)

var zoneKindNames = map[zone.Kind]string{zone.KindStockpile: ZoneStockpile, zone.KindGrow: ZoneGrow, zone.KindPen: ZonePen,
	zone.KindDump: ZoneDump, zone.KindShelf: ZoneShelf, zone.KindHopper: ZoneHopper, zone.KindGrave: ZoneGrave}

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
// with: the vanilla defaults (STO-2, zone.NewStockpile, zone.NewDump).
func stockpileProto(variant uint8) (zone.Kind, zone.Zone) {
	if variant == StockpileVariantDump {
		return zone.KindDump, zone.NewDump()
	}
	return zone.KindStockpile, zone.NewStockpile()
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
	// Store (STO-2): the stacks marked to haul urgently (game/store);
	// absent in older saves and empty while none is.
	Store store.Saved
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
	out := zoneSave{W: s.W, H: s.H, Cells: s.Cells, Zones: s.Zones, Next: s.Next, Store: g.haul.store.Export()}
	if len(g.haul.storedAt) > 0 {
		out.StoredAt = g.haul.storedAt
	}
	return out
}

// importZones restores the zone map; an older save (no map) paints its
// stockpile, grow zone and pen rectangles in saved order, so overlapping
// rectangles resolve first-wins and touching ones merge. An older save's
// graves get their storage settings (STO-2).
func (g *Game) importZones(s zoneSave) {
	g.haul.storedAt = s.StoredAt
	g.haul.store.Import(s.Store)
	if len(s.Cells) > 0 || len(s.Zones) > 0 || s.Next > 0 {
		g.zones = zone.Import(zone.Saved{W: s.W, H: s.H, Cells: s.Cells, Zones: s.Zones, Next: s.Next})
		g.haul.store.Buildings.LayMissing(g.zones, haulHost{g})
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

// zoneMemo is the view cache's last zone list, per zone map and its Ver,
// and that list with the storage readouts (STO-2), per item generation.
type zoneMemo struct {
	views, counted []ZoneView // handed out, never written
	src            *zone.Map
	ver, gen       uint64
	ok             bool
}

// zoneViews lists every zone in id order. With the view cache on the list
// is shared and read only, rebuilt only when the zone map's Ver moves;
// the readouts (Group, Stored, Capacity) when a stack moves too.
func (g *Game) zoneViews() []ZoneView {
	if !g.views.on {
		return countZones(g, g.buildZoneViews(), nil)
	}
	m := &g.views.zones
	if m.ok && m.src == g.zones && m.ver == g.zones.Ver() {
		if selfCheck() && !reflect.DeepEqual(m.views, g.buildZoneViews()) {
			panic("zoneViews: zones changed without a Ver bump")
		}
	} else {
		m.views, m.src, m.ver, m.ok, m.counted = g.buildZoneViews(), g.zones, g.zones.Ver(), true, nil
	}
	if gen := g.idx.ground.Gen(); m.counted == nil || m.gen != gen {
		m.counted, m.gen = countZones(g, m.views, m.counted), gen
	}
	return m.counted
}

// countZones is vs with each storage zone's readouts (store.Buildings.Fill),
// or old when it already reads the same (shared views stay shared).
func countZones(g *Game, vs, old []ZoneView) []ZoneView {
	out := slices.Clone(vs)
	for i := range out {
		z, _ := g.zones.Zone(zone.ID(out[i].ID))
		out[i].Group, out[i].Stored, out[i].Capacity = g.haul.store.Buildings.Fill(g.zones, haulHost{g}, z, g.graveFilled(entity.EntityID(out[i].Shelf)))
	}
	if old != nil && slices.EqualFunc(out, old, func(a, b ZoneView) bool {
		return a.Group == b.Group && a.Stored == b.Stored && a.Capacity == b.Capacity
	}) {
		return old
	}
	return out
}

// buildZoneViews is a fresh zoneViews list.
func (g *Game) buildZoneViews() []ZoneView {
	zs := g.zones.Zones()
	out := make([]ZoneView, 0, len(zs))
	for _, z := range zs {
		b, _ := g.zones.Bounds(z.ID)
		v := ZoneView{
			ID: uint16(z.ID), Kind: zoneKindNames[z.Kind], Food: z.Food, Wood: z.Wood, Stone: z.Stone,
			Cells: g.zones.Cells(z.ID), MinX: b.MinX, MinY: b.MinY, MaxX: b.MaxX, MaxY: b.MaxY,
			NoSow: z.NoSow, NoCut: z.NoCut, Hidden: z.Hidden, Name: z.Name,
		}
		if z.Kind.Settings() {
			v.Priority, v.Corpses, v.Chunks = z.Priority.String(), z.Corpses, z.Chunks
			v.Filter, v.Special, v.Shelf = z.Effective().Names(), z.Special.Names(), uint64(g.haul.store.Buildings.Of(g.zones, haulHost{g}, z))
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

// allows: storage zone s takes items of kind k (zone.Zone.Allows: its
// filter once the player edited it, else its Food, Wood, Stone, Corpses
// and Chunks switches; PRD-4).
func allows(s zone.Zone, k item.Kind) bool { return s.Allows(k) }

// filterMatches: some storage zone takes kind k.
func (g *Game) filterMatches(k item.Kind) bool {
	found := false
	g.zones.Each(func(z zone.Zone) bool {
		found = z.Allows(k)
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

// inMatchingStockpile reports st lies in a storage zone that takes it,
// special filters included (PRD-4).
func (g *Game) inMatchingStockpile(st item.Stack) bool {
	z, ok := g.storageAt(st.X, st.Y)
	return ok && z.AllowsStack(st)
}

// inMatchingStockpileAt reports a storage zone on (x, y) that takes kind k.
func (g *Game) inMatchingStockpileAt(k item.Kind, x, y int) bool {
	z, ok := g.storageAt(x, y)
	return ok && allows(z, k)
}

// noPriority is below every storage priority: storageCell's floor for
// "any zone".
const noPriority = zone.PriorityLow - 1

// stockpileCell is where stack st goes: in the highest-priority storage
// zones taking it (its kind and the special filters, STO-2) that have
// room (PLAY-023), nearest (nearX, nearY) first (ties: lower y, then x),
// a tile whose same-kind stack still has room (topping up), else an empty
// walkable tile with no item, planned or built object, or reservation. A
// shelf tile (PRD-4) holds up to zone.ShelfStacks stacks.
func (g *Game) stockpileCell(st item.Stack, nearX, nearY int) (x, y int, ok bool) {
	return g.storageCell(st, nearX, nearY, nil, noPriority)
}

// storageCell is stockpileCellWhere for stack st (its kind and the zones'
// special filters, PRD-4) over the zones whose priority is above floor
// (noPriority: every zone; a re-haul passes the priority of the zone the
// stack lies in). Priorities are tried highest first; within one it is
// the nearest top-up cell, else the nearest free cell (STO-1: the storage
// zones by priority from zone.Stores, the stacks from the ground index).
func (g *Game) storageCell(st item.Stack, nearX, nearY int, keep func(x, y int) bool, floor zone.Priority) (x, y int, ok bool) {
	return store.Search(haulHost{g}, st, nearX, nearY, keep, floor, false)
}

// haulHost as store.Cells: the storage search's reads (STO-1 indexes).
func (h haulHost) Stores(p zone.Priority) []zone.Store { return h.g.idx.stores.Of(h.g.zones, p) }
func (h haulHost) Zones() *zone.Map                    { return h.g.zones }
func (h haulHost) First(x, y int) (item.Stack, bool) {
	return h.g.stacks.Get(h.g.idx.ground.First(h, x, y))
}
func (h haulHost) Shelf(k item.Kind, x, y int) (int, bool) { return h.g.idx.ground.Shelf(h, k, x, y) }
func (h haulHost) TileTaken(x, y int) bool                 { _, taken := h.g.reserve.TileOwner(x, y); return taken }
func (h haulHost) Blocked(x, y int, building bool) bool {
	g := h.g
	return !g.grid.Walkable(x, y) || g.layerFrameAt(x, y, construction.LayerObject) != 0 || !building && g.occupiedAt(x, y)
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
	g.haul.store.Buildings.Lay(g.zones, haulHost{g}, kind, cells) // store (PRD-4, STO-2): a storage building's zone
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

// Storage filters and storage buildings (PRD-4, STO-2). The filter
// tree, the special filters, the legacy switches and storage groups live
// in domain zone; the handlers below edit them through zone.Map (Update
// reaches a whole group). Shelves, hoppers, graves and sarcophagi lay a
// zone over their footprint (game/store Buildings); the haul search
// treats a shelf tile as room for zone.ShelfStacks stacks.

// FilterRow is one line of the stockpile filter tree for the settings
// window (zone.Row: path, label, depth, leaf, On, Partial).
type FilterRow = zone.Row

// StockpileFilterRows is the filter tree in display order with each
// line's state under filter (a ZoneView.Filter). StorageFilterRows is
// the same for a zone of kind (a ZoneView.Kind): a hopper's raw food or a
// grave's bodies only. FilterRoot is the node SetStockpileFilter takes
// for "allow all" (Enabled) and "clear all".
func StockpileFilterRows(filter []string) []FilterRow { return zone.Rows(filter) }
func StorageFilterRows(kind string, filter []string) []FilterRow {
	for k, n := range zoneKindNames {
		if n == kind {
			return zone.RowsFor(k, filter)
		}
	}
	return nil
}

const FilterRoot = zone.Root

// StockpileSpecials names the special filter toggles by
// SetStockpileSpecial's Index (zone.SpecialRotten, zone.SpecialTainted);
// Index zone.SpecialHP and zone.SpecialQuality set the ranges from X..X2.
// StorageSpecials lists every toggle with its Index (STO-2; Grave: a
// corpse filter only graves and sarcophagi use).
func StockpileSpecials() []string    { return zone.SpecialNames() }
func StorageSpecials() []zone.Toggle { return zone.Toggles() }

// cmdStockpile handles the storage commands. On storage zone c.Target:
// SetStockpileFilter allows (Enabled) or forbids filter-tree node c.Name
// (FilterRoot: everything), SetStockpileSpecial sets special filter
// c.Index (a toggle by Enabled, a range by X..X2) and PasteStockpile
// copies zone c.Zone's filter, special filters and priority. On storage
// buildings (STO-2): LinkStorage links building c.Target into building
// c.Zone's group, UnlinkStorage takes it out. HaulUrgently marks
// (Enabled) or unmarks the loose stacks in X..X2, Y..Y2. Unknown targets
// are ignored.
func (g *Game) cmdStockpile(c command.Command) {
	st := &g.haul.store
	switch c.Kind {
	case command.LinkStorage:
		st.Buildings.Link(g.zones, haulHost{g}, entity.EntityID(c.Target), entity.EntityID(c.Zone))
	case command.UnlinkStorage:
		st.Buildings.Unlink(g.zones, haulHost{g}, entity.EntityID(c.Target))
	case command.HaulUrgently:
		st.Urgent.MarkRect(haulHost{g}, c.X, c.Y, c.X2, c.Y2, c.Enabled)
	}
	z, ok := g.zones.Zone(zone.ID(c.Target))
	if !ok || !z.Kind.Settings() {
		return
	}
	switch c.Kind {
	case command.SetStockpileFilter:
		if set, ok := zone.Resolve(c.Name); ok {
			g.zones.Update(z.ID, func(z *zone.Zone) { z.SetAllowed(set, c.Enabled) })
		}
	case command.SetStockpileSpecial:
		g.zones.Update(z.ID, func(z *zone.Zone) { z.Special = z.Special.Set(c.Index, c.Enabled, c.X, c.X2) })
	case command.PasteStockpile:
		if src, ok := g.zones.Zone(zone.ID(c.Zone)); ok && src.Kind.Settings() && src.ID != z.ID {
			g.zones.Update(z.ID, func(z *zone.Zone) { z.PasteFrom(src) })
		}
	}
}

// onShelf reports (x, y) lies under a shelf's storage.
func (g *Game) onShelf(x, y int) bool {
	z, ok := g.storageAt(x, y)
	return ok && z.Kind == zone.KindShelf
}

// cellFull reports tile (x, y) holds as many stacks besides id as it
// may: one anywhere, zone.ShelfStacks on a shelf (mergeOrSet spills
// from a full tile).
func (g *Game) cellFull(id entity.EntityID, x, y int) bool {
	slots := 1
	if g.onShelf(x, y) {
		slots = zone.ShelfStacks
	}
	return g.idx.ground.Others(haulHost{g}, id, x, y) >= slots
}

// liftShelf takes a removed storage building's zone off its footprint.
func (g *Game) liftShelf(b construction.Building) { g.haul.store.Buildings.Lift(g.zones, b) }

// storeView is a storage building's readout (BuildingView Group, Stored,
// Capacity; zeros for any other building).
func (g *Game) storeView(id entity.EntityID) (uint64, int, int) {
	z, ok := g.haul.store.Buildings.ZoneOf(g.zones, haulHost{g}, id)
	if !ok {
		return 0, 0, 0
	}
	return g.haul.store.Buildings.Fill(g.zones, haulHost{g}, z, g.graveFilled(id))
}

// haulHost as game/store's Machines, Larder, Items and Diners (STO-2).
func (h haulHost) BuildingIDs() []entity.EntityID { return h.g.bldgIDs }
func (h haulHost) Building(id entity.EntityID) (construction.Building, bool) {
	return h.g.bldgs.Get(id)
}
func (h haulHost) ObjectAt(x, y int) entity.EntityID {
	return h.g.layerBldgAt(x, y, construction.LayerObject)
}
func (h haulHost) Powered(id entity.EntityID) bool         { return h.g.powered(id) }
func (h haulHost) ItemAt(x, y int) entity.EntityID         { return h.g.itemAt(x, y) }
func (h haulHost) StacksAt(x, y int) int                   { return h.g.idx.ground.Others(h, 0, x, y) }
func (h haulHost) SetJob(id entity.EntityID, j job.Record) { h.g.jobs.Set(id, j) }
func (h haulHost) Claim(owner, target entity.EntityID) bool {
	return h.g.reserve.TryEntity(owner, target)
}
func (h haulHost) Reachable(id entity.EntityID, x, y int) bool { return h.g.reachable(id, x, y) }
func (h haulHost) Position(id entity.EntityID) (int, int) {
	p, _ := h.g.positions.Get(id)
	return p.X, p.Y
}
func (h haulHost) Spawn(k item.Kind, qty, x, y int) entity.EntityID {
	id, _ := h.g.SpawnStack(k, qty, x, y)
	return id
}
