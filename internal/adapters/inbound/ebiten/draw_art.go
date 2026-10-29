package ebiten

import (
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	spritepkg "rimworld-go/internal/adapters/inbound/ebiten/render/sprite"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/application/game"
)

// The world draw order lives in render/world (ARCH-004 wave 4.4). What
// stays here are the map previews the Architect and the selection draw
// over the frame (architectMap, multiselect_render.go): the placement
// ghost takes the Architect's ghostCell; the zone, range-ring and
// power-link previews and the selection's sprites are the layers' draws
// in the live frame view.

// drawPlacementGhost previews a build tool: the real sprite per cell,
// green when placeable, red when not. Walls and conduits autotile among
// the ghosts.
func (r *Renderer) drawPlacementGhost(screen *ebiten.Image, kind, stuff, facing string, cells []architect.GhostCell) {
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	tile := r.cam.Tile()
	wp, v := r.pipe(), r.liveView()
	links := wp.Links
	link, power := overlay.BuildingLink(kind), false
	if c := overlay.PowerLink(kind); c.Accepts != 0 { // conduits link on the power layer
		link, power = c, true
	}
	var p map[[2]int]spritepkg.LinkFlag
	if r.art != nil && link.Flags != 0 { // ghosts join built walls, rock, frames and each other
		w := r.view.W
		if w.TerrainW == 0 {
			w = r.world()
		}
		links.Sync(w, r.game.Width(), r.game.Height())
		ghosts := make([]spritepkg.PlannedCell, len(cells))
		for i, c := range cells {
			ghosts[i] = spritepkg.PlannedCell{X: c.X, Y: c.Y, Flags: link.Flags}
		}
		p = links.Plan(w.Frames, ghosts)
	}
	tint := wp.Paint.StuffTint(stuff)
	for _, c := range cells {
		cs := spritepkg.GhostOKCS
		switch {
		case c.Same:
			cs = spritepkg.GhostSameCS
		case !c.OK:
			cs = spritepkg.GhostBadCS
		}
		if r.art == nil {
			sx, sy := r.worldToScreen(c.X, c.Y)
			vector.FillRect(screen, sx, sy, float32(tile), float32(tile), ghostFill(c), false)
			continue
		}
		mask, corners := 0, 0
		switch {
		case p != nil && power:
			mask = links.PowerMask(link.Accepts, c.X, c.Y, p)
		case p != nil:
			mask = links.PlannedMask(p, link.Accepts, c.X, c.Y)
			corners = links.PlannedCorners(p, link.Accepts, c.X, c.Y)
		}
		if kind == "wall" && wp.Paint.DrawLinked(v, wp.Pass(), screen, spritepkg.WallPrefix(stuff), c.X, c.Y, mask, corners, tint, cs, tile, sw, sh) {
			continue
		}
		cf := ghostFacing(c, facing)
		key := wp.BuildKey(kind, stuff, cf, mask, spritepkg.DoorAxisEW(cf), false)
		if !r.art.Has(key) {
			sx, sy := r.worldToScreen(c.X, c.Y)
			if spritepkg.IsFenceKind(kind) {
				spritepkg.DrawFenceVector(screen, sx, sy, float32(tile), kind, mask, false, spritepkg.IsStoneStuff(stuff), 170)
				continue
			}
			vector.FillRect(screen, sx, sy, float32(tile), float32(tile), ghostFill(c), false)
			continue
		}
		wp.Paint.DrawCS(v, wp.Pass(), screen, key, color.NRGBA{}, float64(c.X), float64(c.Y), tile, sw, sh, spritepkg.MulCS(spritepkg.TintCS(tint), cs))
	}
}

// ghostFacing is the facing a ghost cell draws with: the sim's
// auto-orientation for that cell, else the tool's.
func ghostFacing(c architect.GhostCell, tool string) string {
	if c.Facing != "" {
		return c.Facing
	}
	return tool
}

// ghostFill is the plain fill of a ghost cell without art: green, red
// when blocked, grey for a same-material cell that does nothing.
func ghostFill(c architect.GhostCell) color.RGBA {
	switch {
	case c.Same:
		return color.RGBA{150, 150, 150, 110}
	case !c.OK:
		return color.RGBA{220, 70, 60, 120}
	}
	return color.RGBA{90, 200, 90, 120}
}

// drawSprite draws a world sprite in the live frame (the selection's
// marks, multiselect_render.go).
func (r *Renderer) drawSprite(screen *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, sw, sh int) {
	p := r.pipe()
	p.Paint.Draw(r.liveView(), p.Pass(), screen, key, tint, wx, wy, tile, sw, sh)
}

// drawZoneHighlight outlines zone z (render/overlay Zones).
func (r *Renderer) drawZoneHighlight(screen *ebiten.Image, w game.WorldView, z game.ZoneView, sw, sh int, tile float64) {
	p := r.pipe()
	ch := p.OverlayChunks()
	p.Overlays.Zones.DrawZoneHighlight(r.liveView(), &ch, screen, w, z, sw, sh, tile)
}

// drawZoneLabel draws zone z's name plate over its geometry g.
func (r *Renderer) drawZoneLabel(screen *ebiten.Image, z game.ZoneView, g *overlay.ZoneGeom, sw, sh int, tile float64) {
	r.overlays().Zones.DrawZoneLabel(r.liveView(), screen, z, g, sw, sh, tile)
}

// zoneLabelAt is the zone whose name plate is under the physical point
// (mx, my), among the zones shown.
func (r *Renderer) zoneLabelAt(w game.WorldView, mx, my int, tile float64, shown uint16) (game.ZoneView, bool) {
	return r.overlays().Zones.ZoneLabelAt(r.liveView(), w, mx, my, tile, shown)
}

// drawRangeRing draws a turret's or a lamp's range around (x, y).
func (r *Renderer) drawRangeRing(dst *ebiten.Image, x, y int, radius float64, c color.NRGBA) {
	r.overlays().Wiring.DrawRangeRing(r.liveView(), dst, x, y, radius, c)
}

// drawPowerLink draws a planned power connection from one cell to another.
func (r *Renderer) drawPowerLink(dst *ebiten.Image, from, to [2]int, c color.NRGBA, tile float64) {
	r.overlays().Wiring.DrawPowerLink(r.liveView(), dst, from, to, c, tile)
}
