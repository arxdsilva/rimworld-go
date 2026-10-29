package ebiten

import (
	"image/color"
	spritepkg "rimworld-go/internal/adapters/inbound/ebiten/render/sprite"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/application/game"
)

// The world draw order lives in render/world (ARCH-004 wave 4.4,
// world_compat.go forwards the old entry points). What stays here is
// bound to root types: the placement ghost takes the Architect's
// ghostCell, and the figure layers are the portraits' (portrait.go).

// drawPlacementGhost previews a build tool: the real sprite per cell,
// green when placeable, red when not. Walls and conduits autotile among
// the ghosts.
func (r *Renderer) drawPlacementGhost(screen *ebiten.Image, kind, stuff, facing string, cells []ghostCell) {
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	tile := r.cam.Tile()
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
		r.links.Sync(w, r.game.Width(), r.game.Height())
		ghosts := make([]spritepkg.PlannedCell, len(cells))
		for i, c := range cells {
			ghosts[i] = spritepkg.PlannedCell{X: c.x, Y: c.y, Flags: link.Flags}
		}
		p = r.links.Plan(w.Frames, ghosts)
	}
	tint := r.stuffTint(stuff)
	for _, c := range cells {
		cs := spritepkg.GhostOKCS
		switch {
		case c.same:
			cs = spritepkg.GhostSameCS
		case !c.ok:
			cs = spritepkg.GhostBadCS
		}
		if r.art == nil {
			sx, sy := r.worldToScreen(c.x, c.y)
			vector.FillRect(screen, sx, sy, float32(tile), float32(tile), ghostFill(c), false)
			continue
		}
		mask, corners := 0, 0
		switch {
		case p != nil && power:
			mask = r.links.PowerMask(link.Accepts, c.x, c.y, p)
		case p != nil:
			mask = r.links.PlannedMask(p, link.Accepts, c.x, c.y)
			corners = r.links.PlannedCorners(p, link.Accepts, c.x, c.y)
		}
		if kind == "wall" && r.drawLinked(screen, spritepkg.WallPrefix(stuff), c.x, c.y, mask, corners, tint, cs, tile, sw, sh) {
			continue
		}
		cf := ghostFacing(c, facing)
		key := r.pipe().BuildKey(kind, stuff, cf, mask, spritepkg.DoorAxisEW(cf), false)
		if !r.art.Has(key) {
			sx, sy := r.worldToScreen(c.x, c.y)
			if spritepkg.IsFenceKind(kind) {
				spritepkg.DrawFenceVector(screen, sx, sy, float32(tile), kind, mask, false, spritepkg.IsStoneStuff(stuff), 170)
				continue
			}
			vector.FillRect(screen, sx, sy, float32(tile), float32(tile), ghostFill(c), false)
			continue
		}
		r.drawSpriteCS(screen, key, color.NRGBA{}, float64(c.x), float64(c.y), tile, sw, sh, spritepkg.MulCS(spritepkg.TintCS(tint), cs))
	}
}

// ghostFacing is the facing a ghost cell draws with: the sim's
// auto-orientation for that cell, else the tool's.
func ghostFacing(c ghostCell, tool string) string {
	if c.facing != "" {
		return c.facing
	}
	return tool
}

// ghostFill is the plain fill of a ghost cell without art: green, red
// when blocked, grey for a same-material cell that does nothing.
func ghostFill(c ghostCell) color.RGBA {
	switch {
	case c.same:
		return color.RGBA{150, 150, 150, 110}
	case !c.ok:
		return color.RGBA{220, 70, 60, 120}
	}
	return color.RGBA{90, 200, 90, 120}
}

// pawnFigureLayers lists p's sprites bottom first in art facing fs: its
// own look (build, beard, hair; PawnView.Look) dressed in its apparel, a
// raider's maroon clothes, and a disfigured pawn's face scar over the
// hair (under a hat) when the art has one.
func (r *Renderer) pawnFigureLayers(p game.PawnView, fs string) []portraitLayer {
	return figureLayers(r.art, p.Look, fs, figureDress{Apparel: p.Apparel, Scarred: p.Disfigured, Raider: p.Hostile})
}

// pawnPortraitLayers is p's south-facing portrait, bottom first: the
// colony figure's layers (pawnFigureLayers).
func (r *Renderer) pawnPortraitLayers(p game.PawnView) []portraitLayer {
	return r.pawnFigureLayers(p, "south")
}
