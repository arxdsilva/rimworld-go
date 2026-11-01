package ebiten

import (
	"math"
	artpkg "rimworld-go/internal/adapters/inbound/ebiten/art"
	lightpkg "rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	spritepkg "rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/application/game"
)

// The shadow pass lives in render/light (Shadows.Draw). What casts stays
// here, because it is the renderer's to know: the sprite keys and
// footprints of buildings, plants, items and rock, and the chunk lists in
// view. shadowHost hands those to the pass as its light.Casters.
// Creatures (pawns, animals, mechanoids) cast no shadow (user decision,
// 2026-10-31).

// shadowHost is the renderer as the shadow pass's light.Casters. The pass
// runs inside the pipeline's frame (or a test's liveView), so it reads the
// pipeline and the frame view as they are.
type shadowHost struct{ r *Renderer }

// VisibleTiles is the on-map tile range in view.
func (h shadowHost) VisibleTiles() (x0, y0, x1, y1 int) { return h.r.wp.Env.VisibleTiles(&h.r.view) }

// White is the environment's white texel.
func (h shadowHost) White() *ebiten.Image { return h.r.env.WhiteImg() }

// footprintOf is a sprite's footprint in tiles (1x1 when unknown).
func footprintOf(c *artpkg.Catalog, key string) (float64, float64) {
	if sp := c.Sprite(key); sp != nil && sp.Meta.Footprint[0] > 0 && sp.Meta.Footprint[1] > 0 {
		return float64(sp.Meta.Footprint[0]), float64(sp.Meta.Footprint[1])
	}
	return 1, 1
}

// skyOpen reports tile (x, y) of w is under open sky (no built or natural
// roof); without roof data every tile is.
func skyOpen(w game.WorldView, x, y int) bool {
	return !overlay.Roofed(overlay.RoofCellAt(w, x, y))
}

// Objects lists this frame's box and blob casters (buildings, rocks,
// plants and items) standing on open-sky tiles within the view (widened by
// the longest shadow). Pawns and animals cast nothing.
func (h shadowHost) Objects(w game.WorldView, pawns []game.PawnView) []lightpkg.Caster {
	r := h.r
	wp, fv := r.wp, &r.view
	st := &r.env.Shadow
	out := st.Objects[:0]
	v := r.env.View
	m := lightpkg.ShadowMaxLen + 1
	inView := func(x, y, fw, fd float64) bool {
		return v.Empty() || x+fw >= v.X0-m && y+fd >= v.Y0-m && x <= v.X1+m && y <= v.Y1+m
	}
	addBox := func(key string, x, y, fw, fd float64, scale float64) {
		s := st.SpecOf(r.art, key)
		if s.Kind == lightpkg.ShadowNone || !inView(x, y, fw, fd) || !skyOpen(w, int(math.Floor(x)), int(math.Floor(y))) {
			return
		}
		s.W, s.H, s.D = s.W*scale, s.H*scale, s.D*scale
		if s.Kind == lightpkg.ShadowBlob {
			out = append(out, lightpkg.Caster{Spec: s, X: x + fw/2, Y: y + fd/2})
			return
		}
		bx, by := light.BoxFootprint(x, y, fw, fd, s)
		out = append(out, lightpkg.Caster{Spec: s, X: bx, Y: by})
	}
	// a cheap position test first: keys and footprints cost more than
	// most casters are worth off screen (a tree's footprint is ≤ 3 tiles)
	near := func(x, y int) bool { return inView(float64(x)-2, float64(y)-2, 5, 5) }
	building := func(b *game.BuildingView) {
		if b.Floor || b.Kind == "sleeping spot" || spritepkg.IsFenceKind(b.Kind) || !near(b.X, b.Y) {
			return // fences cast no sun shadow (art: shadow kind none)
		}
		key := world.BuildSpriteStuff(b.Kind, b.Stuff, b.Facing, 0, spritepkg.DoorAxisEW(b.Facing), false)
		addBox(key, float64(b.X), float64(b.Y), float64(max(b.W, 1)), float64(max(b.H, 1)), 1)
	}
	plant := func(p *game.PlantView) {
		if !near(p.X, p.Y) {
			return
		}
		key, scale := world.PlantSprite(*p)
		if f := env.DamageShadowFactor(*p); f < 1 { // PLAY-016: dying plants' shadows thin out
			if f <= 0 {
				return
			}
			scale *= f
		}
		key = wp.Env.SeasonKey(fv, key, w)
		fw, fd := footprintOf(r.art, key)
		addBox(key, float64(p.X), float64(p.Y), fw, fd, scale)
	}
	stack := func(it *game.ItemView) {
		if it.Carried || !near(it.X, it.Y) {
			return
		}
		key := world.ItemSpriteSpecies(it.Kind, it.Animal, it.X, it.Y)
		addBox(key, float64(it.X), float64(it.Y), 1, 1, 1)
	}
	if (envHost{r}).FrameLists(env.ListsOf(&w)) { // only the chunks in view (render/world lod.go)
		for _, cv := range wp.LOD.Views {
			for _, i := range cv.Buildings {
				if int(i) < len(w.Buildings) {
					building(&w.Buildings[i])
				}
			}
		}
		h.addRockCasters(w, addBox)
		for _, cv := range wp.LOD.Views {
			for _, i := range cv.Plants {
				if int(i) < len(w.Plants) {
					plant(&w.Plants[i])
				}
			}
		}
		for _, cv := range wp.LOD.Views {
			for _, i := range cv.Items {
				if int(i) < len(w.Items) {
					stack(&w.Items[i])
				}
			}
		}
	} else {
		for i := range w.Buildings {
			building(&w.Buildings[i])
		}
		h.addRockCasters(w, addBox)
		for i := range w.Plants {
			plant(&w.Plants[i])
		}
		for i := range w.Items {
			stack(&w.Items[i])
		}
	}
	st.Objects = out
	return out
}

// addRockCasters adds the rock tiles at an outcrop's edge (inner rock is
// covered by its neighbours' sprites and casts nothing visible).
func (h shadowHost) addRockCasters(w game.WorldView, add func(key string, x, y, fw, fd, scale float64)) {
	r := h.r
	if w.TerrainW <= 0 || len(w.Terrain) == 0 {
		return
	}
	gw, gh := r.game.Width(), r.game.Height()
	isRock := func(x, y int) bool { // rough or smoothed rock; map edges count as rock
		if x < 0 || y < 0 || x >= gw || y >= gh {
			return true
		}
		i := y*w.TerrainW + x
		return i < len(w.Terrain) && spritepkg.IsRockWallCode(w.Terrain[i])
	}
	x0, y0, x1, y1 := 0, 0, gw-1, gh-1
	if v := r.env.View; !v.Empty() {
		m := lightpkg.ShadowMaxLen + 1
		x0, y0 = max(int(v.X0-m), 0), max(int(v.Y0-m), 0)
		x1, y1 = min(int(v.X1+m), gw-1), min(int(v.Y1+m), gh-1)
	}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if !isRock(x, y) || isRock(x-1, y) && isRock(x+1, y) && isRock(x, y-1) && isRock(x, y+1) {
				continue
			}
			add("rock/rock_wall_00", float64(x), float64(y), 1, 1, 1)
		}
	}
}
