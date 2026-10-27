package ebiten

import (
	"fmt"
	"math"
	"strings"
	"sync"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// PLAY-020 zone selection and pane (zone_select.go), the command buttons
// (cmds_zone.go) and the Expand and Shrink tools (zone_tools.go). A zone is
// the last thing a click can select on a tile; the pane and the grid are
// the HUD's, like every other kind's (hud_inspect.go, hud_commands.go).

// ---- reading the selected zone ----

// zoneByID is zone id in the world.
func zoneByID(w game.WorldView, id uint16) (game.ZoneView, bool) {
	for i := range w.Zones {
		if w.Zones[i].ID == id {
			return w.Zones[i], true
		}
	}
	return game.ZoneView{}, false
}

// zoneCellAt is the zone holding tile (x, y).
func zoneCellAt(w game.WorldView, x, y int) (game.ZoneView, bool) {
	for i := range w.Zones {
		z := &w.Zones[i]
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

// selectedZone is the selected zone, if it still exists. A zone that is
// gone (split, merged or deleted) is followed to the zone now holding the
// last clicked cell, else the selection clears.
func (a *App) selectedZone() (game.ZoneView, bool) {
	if !a.hasZone || a.renderer == nil {
		return game.ZoneView{}, false
	}
	w := a.renderer.world()
	if z, ok := zoneByID(w, a.zoneSel); ok {
		return z, true
	}
	if st := a.zone; st != nil && st.hasCell {
		if z, ok := zoneCellAt(w, st.cell[0], st.cell[1]); ok {
			a.zoneSel = z.ID
			return z, true
		}
	}
	a.clearZone()
	return game.ZoneView{}, false
}

// zoneView is zone id, following the selection when id is the selected
// zone's (selectedZone).
func (a *App) zoneView(id uint16) (game.ZoneView, bool) {
	if a.renderer == nil {
		return game.ZoneView{}, false
	}
	if a.hasZone && a.zoneSel == id {
		return a.selectedZone()
	}
	return zoneByID(a.renderer.world(), id)
}

// selectZone makes zone id the selection (the other kinds are cleared by
// selectCand).
func (a *App) selectZone(id uint16) {
	a.zoneSel, a.hasZone = id, true
	st := a.zoneSt()
	st.tool, st.drag, st.fresh = zoneToolNone, false, false
	// The cell to follow if the zone splits: the last click when it is
	// inside, else the zone's first cell.
	if a.renderer != nil {
		if z, ok := zoneByID(a.renderer.world(), id); ok && len(z.Cells) > 0 &&
			(!st.hasCell || !zoneHasCell(&z, st.cell[0], st.cell[1])) {
			st.cell, st.hasCell = z.Cells[0], true
		}
	}
}

// clearZone drops the zone selection and its tool.
func (a *App) clearZone() {
	a.hasZone, a.zoneSel = false, 0
	if st := a.zone; st != nil {
		st.tool, st.drag, st.fresh = zoneToolNone, false, false
	}
}

// selectZoneLabel selects the zone whose name plate is under physical
// pixel (mx, my) and reports it did. (tx, ty) is the tile under the pointer.
func (a *App) selectZoneLabel(w game.WorldView, mx, my, tx, ty int) bool {
	z, ok := a.renderer.zoneLabelAt(w, mx, my, a.renderer.tilePx(), a.shownHiddenZone())
	if !ok {
		return false
	}
	a.selectCand(selCand{selZone, entity.EntityID(z.ID)})
	if st := a.zoneSt(); zoneHasCell(&z, tx, ty) {
		st.cell, st.hasCell = [2]int{tx, ty}, true
	}
	return true
}

// shownHiddenZone is the id of the selected zone while it is hidden: the
// one hidden zone whose label still shows (0: none).
func (a *App) shownHiddenZone() uint16 {
	if a.hasZone && a.renderer != nil {
		if z, ok := zoneByID(a.renderer.world(), a.zoneSel); ok && z.Hidden {
			return z.ID
		}
	}
	return 0
}

// ---- hooks the HUD calls ----

// applyZoneClick handles a "zone." click id: the pane's close button.
func (a *App) applyZoneClick(id string) {
	if id == "zone.close" {
		a.clearZone()
	}
}

// drawZoneSelection marks the selected zone on the map: a brighter fill
// and outline (a hidden zone shows its outline and its name while it is
// selected) and, with a tool active, its ghost, icon and hint.
func (a *App) drawZoneSelection(screen *ebiten.Image) {
	if a.renderer == nil || a.workOpen || a.archOpen {
		return
	}
	z, ok := a.selectedZone()
	if !ok {
		return
	}
	r := a.renderer
	tile := r.tilePx()
	sw, sh := a.size()
	pw, ph := int(math.Ceil(float64(sw)*a.mapScale())), int(math.Ceil(float64(sh)*a.mapScale())) // map pixels, like the camera
	r.drawZoneHighlight(screen, r.world(), z, pw, ph, tile)
	if z.Hidden {
		r.drawZoneLabel(screen, z, r.zoneGeomOf(z), pw, ph, tile)
	}
	a.drawZoneTool(screen, z) // zone_tools.go
}

// ---- the pane ----

// zoneKindName is the zone kind in words ("Growing zone").
func zoneKindName(z game.ZoneView) string {
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

// zoneDefaultName is the name a zone has until the player names it.
func zoneDefaultName(z game.ZoneView) string {
	return fmt.Sprintf("%s %d", zoneKindName(z), z.ID)
}

// zoneTitle is the zone's name, or its default.
func zoneTitle(z game.ZoneView) string {
	if z.Name != "" {
		return z.Name
	}
	return zoneDefaultName(z)
}

// zonePane is the selected zone's inspect pane.
func (a *App) zonePane(id uint16) paneSpec {
	z, ok := a.zoneView(id)
	if !ok {
		return paneSpec{}
	}
	sp := paneSpec{Title: zoneTitle(z), Close: "zone.close", Lines: a.zoneLines(z)}
	if z.Name != "" {
		sp.Subtitle = zoneDefaultName(z)
	}
	return sp
}

// zoneLines are the pane's body lines.
func (a *App) zoneLines(z game.ZoneView) []string {
	n := len(z.Cells)
	size := fmt.Sprintf("%d tiles", n)
	if n == 1 {
		size = "1 tile"
	}
	out := []string{size}
	switch z.Kind {
	case game.ZoneGrow:
		out = append(out, "Plant: "+zoneCropName(z.Crop))
		out = append(out, a.zoneClimateLine(z), a.zonePlantsLine(z))
		if z.NoSow {
			out = append(out, "Sowing off")
		}
		if z.NoCut {
			out = append(out, "Cutting off")
		}
	case game.ZoneStockpile, game.ZoneDump:
		out = append(out, "Priority: "+zonePriorityName(z), zoneFilterLine(z))
	}
	return out
}

// zonePriorityName is a storage zone's priority ("Normal" when the view
// carries none).
func zonePriorityName(z game.ZoneView) string {
	if z.Priority == "" {
		return "Normal"
	}
	return z.Priority
}

// zoneCropName is the crop's name for the pane and the button.
func zoneCropName(crop string) string {
	if crop == "" {
		return "nothing"
	}
	return cropLabel(crop)
}

// zoneFilterLine is what a stockpile takes: its filters, then anything
// else; a dumping stockpile names its corpses and chunks filters instead.
func zoneFilterLine(z game.ZoneView) string {
	if z.Kind == game.ZoneDump {
		var parts []string
		for _, f := range []struct {
			on   bool
			name string
		}{{z.Corpses, "Animal corpses"}, {z.Chunks, "Stone chunks"}, {z.Food, "Food"}, {z.Wood, "Wood"}, {z.Stone, "Stone"}} {
			if f.on {
				parts = append(parts, f.name)
			}
		}
		if len(parts) == 0 {
			return "Accepts: nothing"
		}
		return "Accepts: " + strings.Join(parts, ", ")
	}
	var parts []string
	for _, f := range []struct {
		on   bool
		name string
	}{{z.Food, "Food"}, {z.Wood, "Wood"}, {z.Stone, "Stone"}} {
		if f.on {
			parts = append(parts, f.name)
		}
	}
	return "Accepts: " + strings.Join(append(parts, "other goods"), ", ")
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

// zoneClimateLine says whether the zone's plants grow in the air at its
// middle: "Growing now", "Too cold" or "Too hot", with the temperature and
// the range plants grow in.
func (a *App) zoneClimateLine(z game.ZoneView) string {
	g := a.renderer.zoneGeomOf(z)
	x, y := min(max(int(g.ax), g.minX), g.maxX), min(max(int(g.ay), g.minY), g.maxY)
	t := a.renderer.game.TemperatureAt(x, y)
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
	return p.Zone != "" && (p.Species == p.Zone || p.Zone == "tree" && isTreePlant(p.Species))
}

// zonePlantsLine is "Sown 18 of 24 · ripe 6 · growth 63%" from the plants
// standing in the zone: only the ones inside its bounding box are read
// (plantsOn, the chunk index), never every plant of the map.
func (a *App) zonePlantsLine(z game.ZoneView) string {
	r := a.renderer
	g := r.zoneGeomOf(z)
	w := r.world()
	st := a.zoneSt()
	st.plants = r.plantsOn(&w, g.minX, g.minY, g.maxX, g.maxY, st.plants[:0])
	sown, ripe, growth := 0, 0, 0
	for _, i := range st.plants {
		p := &w.Plants[i]
		if !zoneOwnPlant(p) || !g.cells[[2]int{p.X, p.Y}] {
			continue
		}
		sown++
		growth += p.Growth
		if p.Harvestable {
			ripe++
		}
	}
	if sown == 0 {
		return fmt.Sprintf("Sown 0 of %d", len(z.Cells))
	}
	return fmt.Sprintf("Sown %d of %d · ripe %d · growth %d%%", sown, len(z.Cells), ripe, (growth+sown/2)/sown)
}
