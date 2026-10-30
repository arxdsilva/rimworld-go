package architect

import (
	"fmt"
	"math"
	"strings"
	"sync"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// PLAY-020 zone selection and pane (zone_select.go), the command buttons
// (the root's cmds_zone.go) and the Expand and Shrink tools
// (zone_tools.go). A zone is the last thing a click can select on a tile;
// the pane and the grid are the HUD's, like every other kind's.

// Zones is the selected zone's pane and its Expand and Shrink tools, and
// the zone ghost of the Architect's zone tools.
type Zones struct {
	*Tool
	st ZoneState
}

// ---- reading the selected zone ----

// ZoneByID is zone id in the world.
func ZoneByID(w game.WorldView, id uint16) (game.ZoneView, bool) {
	for i := range w.Zones {
		if w.Zones[i].ID == id {
			return w.Zones[i], true
		}
	}
	return game.ZoneView{}, false
}

// ZoneCellAt is the zone holding tile (x, y).
func ZoneCellAt(w game.WorldView, x, y int) (game.ZoneView, bool) {
	for i := range w.Zones {
		z := &w.Zones[i]
		if z.Kind == "shelf" { // a shelf's zone is not selectable by tile (PRD-4)
			continue
		}
		if x < z.MinX || x > z.MaxX || y < z.MinY || y > z.MaxY {
			continue
		}
		if zoneHasCell(z, x, y) {
			return *z, true
		}
	}
	return game.ZoneView{}, false
}

// zoneHasCell reports whether z holds tile (x, y).
func zoneHasCell(z *game.ZoneView, x, y int) bool {
	for _, c := range z.Cells {
		if c[0] == x && c[1] == y {
			return true
		}
	}
	return false
}

// Selected is the selected zone, if it still exists. A zone that is gone
// (split, merged or deleted) is followed to the zone now holding the last
// clicked cell, else the selection clears.
func (z *Zones) Selected() (game.ZoneView, bool) {
	env := z.sh.Env()
	if !env.Sel.HasZone || env.Sim == nil {
		return game.ZoneView{}, false
	}
	w := z.sh.World()
	if zv, ok := ZoneByID(w, env.Sel.ZoneSel); ok {
		return zv, true
	}
	if st := &z.st; st.hasCell {
		if zv, ok := ZoneCellAt(w, st.cell[0], st.cell[1]); ok {
			env.Sel.ZoneSel = zv.ID
			return zv, true
		}
	}
	z.Clear()
	return game.ZoneView{}, false
}

// View is zone id, following the selection when id is the selected
// zone's (Selected).
func (z *Zones) View(id uint16) (game.ZoneView, bool) {
	env := z.sh.Env()
	if env.Sim == nil {
		return game.ZoneView{}, false
	}
	if env.Sel.HasZone && env.Sel.ZoneSel == id {
		return z.Selected()
	}
	return ZoneByID(z.sh.World(), id)
}

// Select makes zone id the selection (the other kinds are cleared by the
// selection model).
func (z *Zones) Select(id uint16) {
	env := z.sh.Env()
	env.Sel.ZoneSel, env.Sel.HasZone = id, true
	st := &z.st
	st.Kind, st.Drag, st.Fresh = ZoneToolNone, false, false
	// The cell to follow if the zone splits: the last click when it is
	// inside, else the zone's first cell.
	if env.Sim != nil {
		if zv, ok := ZoneByID(z.sh.World(), id); ok && len(zv.Cells) > 0 &&
			(!st.hasCell || !zoneHasCell(&zv, st.cell[0], st.cell[1])) {
			st.cell, st.hasCell = zv.Cells[0], true
		}
	}
}

// Clear drops the zone selection and its tool.
func (z *Zones) Clear() {
	sel := z.sh.Env().Sel
	sel.HasZone, sel.ZoneSel = false, 0
	z.st.Kind, z.st.Drag, z.st.Fresh = ZoneToolNone, false, false
}

// SelectLabel selects the zone whose name plate is under physical pixel
// (mx, my) and reports it did. (tx, ty) is the tile under the pointer.
func (z *Zones) SelectLabel(w game.WorldView, mx, my, tx, ty int) bool {
	zv, ok := z.sh.ZoneLabelAt(w, mx, my, z.sh.TilePx(), z.shownHidden())
	if !ok {
		return false
	}
	z.sh.Env().Sel.Select(selection.Cand{Kind: selection.Zone, ID: entity.EntityID(zv.ID)})
	if zoneHasCell(&zv, tx, ty) {
		z.st.cell, z.st.hasCell = [2]int{tx, ty}, true
	}
	return true
}

// shownHidden is the id of the selected zone while it is hidden: the one
// hidden zone whose label still shows (0: none).
func (z *Zones) shownHidden() uint16 {
	env := z.sh.Env()
	if env.Sel.HasZone && env.Sim != nil {
		if zv, ok := ZoneByID(z.sh.World(), env.Sel.ZoneSel); ok && zv.Hidden {
			return zv.ID
		}
	}
	return 0
}

// ---- hooks the HUD calls ----

// Click handles a "zone." click id: the pane's close button.
func (z *Zones) Click(id string) {
	if id == "zone.close" {
		z.Clear()
	}
}

// DrawSelection marks the selected zone on the map: a brighter fill and
// outline (a hidden zone shows its outline and its name while it is
// selected) and, with a tool active, its ghost, icon and hint.
func (z *Zones) DrawSelection(screen *ebiten.Image) {
	env := z.sh.Env()
	if env.Sim == nil || env.WinOpen || z.Open {
		return
	}
	zv, ok := z.Selected()
	if !ok {
		return
	}
	tile := z.sh.TilePx()
	sw, sh := z.sh.Size()
	pw, ph := int(math.Ceil(float64(sw)*z.sh.MapScale())), int(math.Ceil(float64(sh)*z.sh.MapScale())) // map pixels, like the camera
	z.sh.DrawZoneMark(screen, zv, pw, ph, tile)
	z.drawTool(screen, zv) // zone_tools.go
}

// ---- the pane ----

// ZoneKindName is the zone kind in words ("Growing zone").
func ZoneKindName(z game.ZoneView) string {
	switch z.Kind {
	case game.ZoneGrow:
		return "Growing zone"
	case game.ZonePen:
		return "Pen"
	case game.ZoneDump:
		return "Dumping stockpile"
	}
	return "Stockpile"
}

// ZoneDefaultName is the name a zone has until the player names it.
func ZoneDefaultName(z game.ZoneView) string {
	return fmt.Sprintf("%s %d", ZoneKindName(z), z.ID)
}

// zoneTitle is the zone's name, or its default.
func zoneTitle(z game.ZoneView) string {
	if z.Name != "" {
		return z.Name
	}
	return ZoneDefaultName(z)
}

// Pane is the selected zone's inspect pane.
func (z *Zones) Pane(id uint16) ZonePane {
	zv, ok := z.View(id)
	if !ok {
		return ZonePane{}
	}
	sp := ZonePane{Title: zoneTitle(zv), Close: "zone.close", Lines: z.Lines(zv)}
	if zv.Name != "" {
		sp.Subtitle = ZoneDefaultName(zv)
	}
	return sp
}

// Lines are the pane's body lines.
func (z *Zones) Lines(zv game.ZoneView) []string {
	n := len(zv.Cells)
	size := fmt.Sprintf("%d tiles", n)
	if n == 1 {
		size = "1 tile"
	}
	out := []string{size}
	switch zv.Kind {
	case game.ZoneGrow:
		out = append(out, "Plant: "+ZoneCropName(zv.Crop))
		out = append(out, z.climateLine(zv), z.plantsLine(zv))
		if zv.NoSow {
			out = append(out, "Sowing off")
		}
		if zv.NoCut {
			out = append(out, "Cutting off")
		}
	case game.ZoneStockpile, game.ZoneDump, game.ZoneShelf:
		out = append(out, "Priority: "+ZonePriorityName(zv), zoneStoredLine(zv), zoneFilterLine(zv))
	}
	return out
}

// ZonePriorityName is a storage zone's priority ("Normal" when the view
// carries none).
func ZonePriorityName(z game.ZoneView) string {
	if z.Priority == "" {
		return "Normal"
	}
	return z.Priority
}

// ZoneCropName is the crop's name for the pane and the button.
func ZoneCropName(crop string) string {
	if crop == "" {
		return "nothing"
	}
	return CropLabel(crop)
}

// zoneFilterLine is what a storage takes, read from its real filter tree:
// "everything", "nothing", or the allowed categories and items (the first
// few, then a count). A dumping stockpile reads the same tree.
func zoneFilterLine(z game.ZoneView) string {
	rows := game.StorageFilterRows(z.Kind, z.Filter)
	var names []string
	all := true
	for _, r := range rows {
		if r.Depth == 0 {
			all = all && r.On
		}
	}
	for _, name := range z.Filter {
		names = append(names, name[strings.LastIndexByte(name, '/')+1:])
	}
	switch {
	case all && len(rows) > 0:
		return "Accepts: everything"
	case len(names) == 0:
		return "Accepts: nothing"
	case len(names) > 4:
		return "Accepts: " + strings.Join(names[:4], ", ") + fmt.Sprintf(" and %d more", len(names)-4)
	}
	return "Accepts: " + strings.Join(names, ", ")
}

// zoneStoredLine is the capacity readout of a storage zone or shelf.
func zoneStoredLine(z game.ZoneView) string {
	return fmt.Sprintf("Stored: %d of %d stacks", z.Stored, z.Capacity)
}

// growTempRange is the air temperature, in tenths of a degree C, at or
// below which nothing grows and at or above which nothing grows: read off
// plant.TempFactor, so the pane never repeats the sim's numbers.
var growTempRange = sync.OnceValues(func() (lo, hi int) {
	for t := -500; t < 1500; t++ {
		if plant.TempFactor(t) > 0 {
			lo = t - 1
			break
		}
	}
	for t := 1500; t > lo; t-- {
		if plant.TempFactor(t) > 0 {
			hi = t + 1
			break
		}
	}
	return lo, hi
})

// climateLine says whether the zone's plants grow in the air at its
// middle: "Growing now", "Too cold" or "Too hot", with the temperature and
// the range plants grow in.
func (z *Zones) climateLine(zv game.ZoneView) string {
	g := z.sh.Overlays().Zones.ZoneGeomOf(zv)
	x, y := min(max(int(g.Ax), g.MinX), g.MaxX), min(max(int(g.Ay), g.MinY), g.MaxY)
	t := z.sim().TemperatureAt(x, y)
	lo, hi := growTempRange()
	switch {
	case t <= lo:
		return fmt.Sprintf("Too cold to grow: %s (needs above %s)", fmtTemp(t), fmtTemp(lo))
	case t >= hi:
		return fmt.Sprintf("Too hot to grow: %s (needs below %s)", fmtTemp(t), fmtTemp(hi))
	}
	return fmt.Sprintf("Growing now: %s (grows between %s and %s)", fmtTemp(t), fmtTemp(lo), fmtTemp(hi))
}

// zoneOwnPlant reports a plant is one its growing zone grows (as the map
// fill reads it): its crop, or any tree in a tree zone.
func zoneOwnPlant(p *game.PlantView) bool {
	return p.Zone != "" && (p.Species == p.Zone || p.Zone == "tree" && sprite.IsTreePlant(p.Species))
}

// plantsLine is "Sown 18 of 24 · ripe 6 · growth 63%" from the plants
// standing in the zone: only the ones inside its bounding box are read
// (PlantsOn, the chunk index), never every plant of the map.
func (z *Zones) plantsLine(zv game.ZoneView) string {
	g := z.sh.Overlays().Zones.ZoneGeomOf(zv)
	w := z.sh.World()
	st := &z.st
	st.plants = z.sh.PlantsOn(w.Plants, g.MinX, g.MinY, g.MaxX, g.MaxY, st.plants[:0])
	sown, ripe, growth := 0, 0, 0
	for _, i := range st.plants {
		p := &w.Plants[i]
		if !zoneOwnPlant(p) || !g.Cells[[2]int{p.X, p.Y}] {
			continue
		}
		sown++
		growth += p.Growth
		if p.Harvestable {
			ripe++
		}
	}
	if sown == 0 {
		return fmt.Sprintf("Sown 0 of %d", len(zv.Cells))
	}
	return fmt.Sprintf("Sown %d of %d · ripe %d · growth %d%%", sown, len(zv.Cells), ripe, (growth+sown/2)/sown)
}
