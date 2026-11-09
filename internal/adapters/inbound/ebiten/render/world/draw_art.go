package world

import (
	"cmp"
	"image/color"
	"slices"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
)

// Draw path for the original generated art. Order (assets/AGENTS.md):
// terrain -> zones/blueprints -> objects north-to-south -> overlays.

const (
	// SoilVariants is how many variants each terrain tile has.
	SoilVariants = env.SoilVariants
	// GrassEvery is the hashed grass rule's spacing without simulation
	// grass data.
	GrassEvery = env.GrassEvery
)

// ArtDraw is one queued object draw: a closure (cave mouths), or (Fn nil)
// a plain record — a linked sprite (rock) or an index into the frame's
// lists — so a frame queues thousands without allocating (NG-1 §6.7).
type ArtDraw struct {
	Row, Col float64
	Fn       func()
	Ld       sprite.LinkedDraw
	Kind     ObjKind
	I        int32   // index into the list kind names
	wx, wy   float64 // creatures: where they are drawn
}

// ObjKind is what a record ArtDraw draws.
type ObjKind uint8

const (
	objLinked    ObjKind = iota // Ld (rock)
	objBuilding                 // w.Buildings[i]
	objFrame                    // w.Frames[i]
	objPlant                    // w.Plants[i]
	objPlantMark                // w.Plants[i] as a map-bake mark
	objItem                     // w.Items[i]
	objForbidden                // the forbidden mark over w.Items[i]
	objAnimal                   // w.Animals[i] at (wx, wy)
	objPawn                     // v.Pawns[i] at (wx, wy)
)

// drawKey is a queued draw's sort key: its row, column and queue place.
type drawKey struct {
	row, col float64
	seq      int32
}

// drawSorter orders the object queue through small keys instead of moving
// the queue's records around (GAR-161): the keys sort, then the records
// move once. Its buffers are reused frame to frame.
type drawSorter struct {
	keys []drawKey
	tmp  []ArtDraw
}

// sort orders draws north to south, then west to east, keeping the queue
// order of ties: the order slices.SortStableFunc gives on (Row, Col).
func (s *drawSorter) sort(draws []ArtDraw) {
	if len(draws) < 2 {
		return
	}
	keys, sorted := s.keys[:0], true
	for i := range draws {
		d := &draws[i]
		keys = append(keys, drawKey{d.Row, d.Col, int32(i)})
		if i > 0 && sorted && cmpKey(keys[i-1], keys[i]) > 0 {
			sorted = false
		}
	}
	s.keys = keys[:0]
	if sorted {
		return
	}
	slices.SortFunc(keys, cmpKey)
	tmp := append(s.tmp[:0], draws...)
	for i, k := range keys {
		draws[i] = tmp[k.seq]
	}
	clear(tmp) // drop the closures until the next frame
	s.tmp = tmp[:0]
}

// cmpKey orders keys by row, column, then queue place.
func cmpKey(a, b drawKey) int {
	if c := cmp.Compare(a.row, b.row); c != 0 {
		return c
	}
	if c := cmp.Compare(a.col, b.col); c != 0 {
		return c
	}
	return cmp.Compare(a.seq, b.seq)
}

// Draw draws the world of this frame onto screen at the view's size and
// tile (render.View, filled by the renderer before it calls Draw).
func (p *Pipeline) Draw(screen *ebiten.Image) {
	p.DrawArt(screen, p.v.SW, p.v.SH, p.v.Tile)
}

// prepare makes everything the frame's draws would otherwise make between
// them: the chunk bakes, the snow and wet masks, the fog chunks. The draws
// to the screen that follow only stamp (PERF-009, GAR-145).
func (p *Pipeline) prepare(w *game.WorldView, sw, sh int, tile float64) {
	v := p.v
	if p.LOD.Tier != Live {
		p.Bk.prepareGround(w)
	}
	p.Env.PrepareCover(v, *w, sw, sh, tile, p.Index)
	p.Overlays.PrepareFog(v, *w, sw, sh, tile)
}

// DrawArt draws the world in its fixed order: the ground (live, or the
// chunk or map bakes), foundations, floors, conduits, zones and plans,
// blueprints, floor markings, colonist shadows, the objects of the view
// by row (rock, cave mouths, buildings, plants, items, frames, animals,
// pawns), then the power wires, work glow, fires, labels, badges, bubbles, particles,
// weather and fog, floaters, designations, animal overlays and the
// selection ring.
func (p *Pipeline) DrawArt(screen *ebiten.Image, sw, sh int, tile float64) {
	none := color.NRGBA{}
	v := p.v
	w := p.src.World()
	v.W, v.Pawns = w, p.src.Snapshot() // this frame's world and pawns, for every layer
	p.Lit.StartFrame() // this frame's lit-draw counts (light.Stats)
	p.Lit.Light = light.LightAt(w.Hour, w.DayOfYear, w.Weather, w.WeatherIntensity)
	p.setView(sw, sh, tile)
	p.Links.Sync(w, v.MapW, v.MapH) // rebuilt only on StructVer
	l := p.LOD
	l.Tier = TierFor(render.LODTile(tile, v.Dev)) // GAR-155: by the cells in view
	l.Crops.Begin()
	p.Bk.syncFrameChunks() // lod.go: the chunk lists in view
	l.Chunks.Frame++
	views := l.Views
	lt := l.Tier

	p.prepare(&w, sw, sh, tile) // every bake and pixel write of the frame, before the first draw
	if lt == Live {
		p.drawTerrainArt(screen, w, sw, sh, tile)
	} else {
		p.Bk.drawGroundLOD(screen, &w, sw, sh, tile)                       // lod.go: the planned chunk bakes
		p.Env.DrawPuddleVeil(v, screen, w, sw, sh, tile, p.Stamp, p.Index) // render/env (PLAY-022): under the tufts
		if lt == Chunk {
			p.Env.DrawFurrows(v, screen, w, tile, p.frameViews(&w)) // render/env: tilled ground under crops
			p.drawTufts(screen, w, sw, sh, tile)
		}
	}
	v.Date = p.src.Date()
	p.Env.DrawGroundCover(v, screen, w, sw, sh, tile, p.Stamp, p.Index)
	chF := p.OverlayChunks()
	p.Overlays.DrawFoundation(v, p.Pass(), &chF, screen, w, sw, sh, tile) // render/overlay: over the water, under floors
	p.DrawFloorLayers(screen, w, sw, sh, tile)
	chC := p.OverlayChunks()
	p.Overlays.Wiring.DrawConduits(v, p.Pass(), &chC, p.Links, screen, w, sw, sh, tile) // over floors, under zones and objects
	p.Overlays.Wiring.Parts.Prune(v.Present)
	p.layer("zones")
	chZ := p.OverlayChunks()
	p.Overlays.Zones.DrawZones(v, p.Pass(), &chZ, screen, w, sw, sh, tile)
	p.setView(sw, sh, tile)
	p.Overlays.DrawPlanOverlay(v, screen, w, sw, sh, tile)
	p.setView(sw, sh, tile)
	p.Overlays.DrawSnowClearOverlay(v, screen, w, sw, sh, tile) // PLAY-015
	for _, f := range w.Frames {
		p.drawSprite(screen, "overlay/blueprint", none, float64(f.X), float64(f.Y), tile, sw, sh)
	}

	if lt == Live { // floor markings and carpets lie on the floor (baked below LiveMin)
		for _, cv := range views {
			for _, i := range cv.Buildings {
				if int(i) >= len(w.Buildings) {
					continue
				}
				switch b := &w.Buildings[i]; b.Kind {
				case "sleeping spot":
					p.drawSprite(screen, BuildSprite(b.Kind, b.Facing, 0, false, false), none, float64(b.X), float64(b.Y), tile, sw, sh)
				case "carpet":
					p.drawSprite(screen, BuildSpriteStuff(b.Kind, b.Stuff, "", 0, false, false), none, float64(b.X), float64(b.Y), tile, sw, sh)
				}
			}
		}
	}
	for _, f := range w.Frames {
		if f.Kind == "carpet" {
			p.drawSpriteCS(screen, BuildSpriteStuff(f.Kind, f.Stuff, "", 0, false, false), none, float64(f.X), float64(f.Y), tile, sw, sh, sprite.BlueprintCS)
		} else if f.Floor {
			cs := sprite.MulCS(sprite.TintCS(p.floorTint(f.Kind, f.Stuff, f.AX.Colour)), sprite.BlueprintCS) // floor_keys.go
			p.drawSpriteCS(screen, FloorSprite(f.Kind, f.Stuff), none, float64(f.X), float64(f.Y), tile, sw, sh, cs)
		}
	}

	if lt != Map { // no shadows on the whole-map bake
		p.setView(sw, sh, tile)
		p.Shadows.Feet = p.ShadowFeet(&w, v.Pawns, tile, p.Shadows.Feet)
		p.Shadows.Draw(p.lightView(), p.Lit.Light, screen, p.Shadows.Feet, w.DayOfYear) // under every object
	}

	l.refreshCreatureCells(v.Pawns, w.Animals)
	p.Env.Trees.SweepTreeFades(v)
	p.Overlays.DrawPaths(v, screen, sw, sh, tile, p.interpWorld)
	draws := p.draws[:0]
	if lt == Live { // rock is baked below LiveMin
		draws = p.addRockDraws(w, draws)
	}
	if lt != Map {
		draws = p.addCaveMouths(w, draws, screen, tile, sw, sh) // ng1_art.go
	}
	l.cloth = p.ColonyCloths()
	// planned walls and fences autotile with each other, built walls and
	// rock (an O(frames) planned set)
	l.planned = p.Links.Plan(w.Frames, nil)
	// objects of the chunks in view, culled by position before queueing:
	// sprites reach up to three tiles north and a tile or two sideways
	for _, cv := range views {
		if lt != Map { // buildings and plants are in the map bake
			for _, i := range cv.Buildings {
				if int(i) >= len(w.Buildings) {
					continue
				}
				if b := &w.Buildings[i]; !b.Floor && !OnFloor(b.Kind) && l.Near(b.X, b.Y, 3, 4) {
					draws = append(draws, ArtDraw{Row: float64(b.Y + max(b.H, 1) - 1), Col: float64(b.X), Kind: objBuilding, I: i})
				}
			}
			for _, i := range cv.Plants {
				if int(i) >= len(w.Plants) {
					continue
				}
				if pl := &w.Plants[i]; l.Near(pl.X, pl.Y, 2, 4) {
					draws = append(draws, ArtDraw{Row: float64(pl.Y), Col: float64(pl.X), Kind: objPlant, I: i})
				}
			}
			for _, i := range cv.Items {
				if int(i) >= len(w.Items) {
					continue
				}
				it := &w.Items[i]
				if it.Carried || !l.Near(it.X, it.Y, 2, 2) { // an XL corpse reaches 1.66 tiles (PLAY-022)
					continue
				}
				row := p.Ob.itemRow(&w, it) // objects.go: a stored stack over its piece
				if it.Forbidden {
					draws = append(draws, ArtDraw{Row: row + 0.001, Col: float64(it.X), Kind: objForbidden, I: i})
				}
				draws = append(draws, ArtDraw{Row: row, Col: float64(it.X), Kind: objItem, I: i})
			}
		}
		for _, i := range cv.Frames {
			if int(i) >= len(w.Frames) {
				continue
			}
			if f := &w.Frames[i]; !f.Floor && f.Kind != "carpet" && !overlay.IsConduitKind(f.Kind) && l.Near(f.X, f.Y, 3, 4) {
				draws = append(draws, ArtDraw{Row: float64(f.Y + max(f.H, 1) - 1), Col: float64(f.X), Kind: objFrame, I: i})
			}
		}
	}
	ct := LodTile(tile)               // creatures keep a minimum size (lod.go)
	apad, apadS := p.AnimalPads(tile) // lod.go: the largest animal canvas (PLAY-022)
	l.apad, l.apadS, l.csize, l.padSet = apad, apadS, max(ct/max(tile, 1e-9), 1), true
	for i := range w.Animals {
		an := &w.Animals[i]
		if !l.Near(an.X, an.Y, apad, apadS) {
			continue
		}
		wx, wy := p.Creatures.AnimalAt(v, *an)
		wx, wy = CreatureAt(wx, wy, tile, ct)
		draws = append(draws, ArtDraw{Row: wy + 0.01, Col: wx, Kind: objAnimal, I: int32(i), wx: wx, wy: wy})
	}
	for i := range v.Pawns {
		pw := &v.Pawns[i]
		if pw.Embodied || !l.Near(pw.X, pw.Y, 3, 4) {
			continue // an embodied pawn's body is a corpse item now
		}
		wx, wy := p.interpWorld(*pw)
		row := wy + 0.01
		if pw.Lying && pw.InBed {
			if b, ok := creature.BedUnder(w, pw.X, pw.Y); ok {
				row = float64(b.Y+max(b.H, 1)-1) + 0.02 // over its bed
			}
		}
		wx, wy = CreatureAt(wx, wy, tile, ct)
		draws = append(draws, ArtDraw{Row: row, Col: wx, Kind: objPawn, I: int32(i), wx: wx, wy: wy})
	}
	p.sorter.sort(draws)
	chW := p.Bk.CordChunks()
	if lt != Map { // buildings are baked there
		p.Overlays.Wiring.DrawCordStubs(v, &chW, screen, &w, sw, sh, tile) // overlay/power_cords.go: under the objects
	}
	p.layer("objects")
	for i := range draws {
		p.Ob.runDraw(screen, &w, &draws[i], tile, sw, sh)
	}
	p.Env.FlushCrops(screen) // render/env: the last run of units
	if lt != Map {
		p.Overlays.Wiring.DrawCords(v, &chW, screen, &w, sw, sh, tile) // overlay/power_cords.go: over objects, under weather and fog
	}
	clear(draws) // drop this frame's closures; keep the buffer
	p.draws = draws[:0]
	p.Creatures.DrawWorkGlow(v, screen, tile)
	p.Overlays.Fires.DrawFires(v, screen, w, tile)
	p.Overlays.Fires.DrawWorldFires(v, screen, w, tile) // PLAY-015 flames
	// render/env: the duskmoss glow
	p.Env.DrawDuskGlow(v, screen, w, tile, l.Tier == Live, l.Views)
	p.Overlays.Zones.DrawZoneLabels(v, screen, w, sw, sh, tile) // over the stacks and crops
	if lt != Map {                                              // no item sprites, so no counts either
		p.Lb.drawStackCounts(screen, w, tile)
	}
	p.Lb.drawBuildingDamage(screen, w, tile)
	clW := p.CreatureCull()
	p.Creatures.DrawWorkProgress(v, &clW, screen, v.Pawns, tile) // bars over the work spots
	chB := p.OverlayChunks()
	p.Overlays.Wiring.DrawNoPowerBadges(v, &chB, screen, &w, tile)
	p.Lb.DrawChatBubbles(screen, tile)
	p.Lb.drawBreakClouds(screen, tile)
	p.src.DrawOrderRoute(screen, tile)
	clP := p.CreatureCull()
	p.Creatures.DrawParticles(v, &clP, screen, tile)
	p.Overlays.DrawArrivals(v, screen, w, tile)
	p.Overlays.DrawLaunch(v, screen, w, tile)        // overlay/ship_draw.go (AX-8): the launch flare
	p.Overlays.Combat.DrawBlasts(v, screen, w, tile) // blast sprites by kind (CBT-5)
	p.Overlays.Combat.DrawShields(v, screen, tile)   // shield bubbles
	p.Lb.DrawSteamPuffs(screen, w, tile)             // features_draw.go
	p.Env.DrawWeather(v, screen, w, sw, sh, tile)
	p.Overlays.Fires.ShowStrikes(v, w)
	p.Env.DrawLightning(v, screen, sw, sh)
	p.layer("fog")
	p.Overlays.DrawFog(v, screen, w, sw, sh, tile) // unexplored tiles over everything
	gfx.FogDrawn()                                 // world_scale.go: later labels show over the fog
	p.Creatures.DrawFloaters(v, screen, tile)

	for _, cv := range views {
		for _, i := range cv.Plants {
			if int(i) >= len(w.Plants) {
				continue
			}
			switch pl := &w.Plants[i]; {
			case pl.Cut:
				p.drawSprite(screen, "overlay/designation_cut", none, float64(pl.X), float64(pl.Y), tile, sw, sh)
			case pl.Designated:
				p.drawSprite(screen, "overlay/designation_harvest", none, float64(pl.X), float64(pl.Y), tile, sw, sh)
			}
		}
	}
	if lt != Map { // the map bake draws no item sprites, so no marks over them
		p.drawHaulMarks(screen, &w, views, tile, sw, sh)
	}
	for _, m := range w.Mines {
		if l.Near(m[0], m[1], 1, 1) {
			p.drawSprite(screen, "overlay/designation_mine", none, float64(m[0]), float64(m[1]), tile, sw, sh)
		}
	}
	smoothKey, smoothCS := SmoothMarkKey, sprite.NoCS
	if !p.art().Has(smoothKey) { // older art: the mine mark, cooled blue
		smoothKey, smoothCS = "overlay/designation_mine", smoothFallbackCS
	}
	for _, m := range w.Smooths {
		if l.Near(m[0], m[1], 1, 1) {
			p.drawSpriteCS(screen, smoothKey, none, float64(m[0]), float64(m[1]), tile, sw, sh, smoothCS)
		}
	}
	clM := p.CreatureCull()
	clM.Pad, clM.PadS = p.AnimalPads(tile)
	clM.White = l.WhiteImg()
	p.Creatures.DrawAnimalOverlays(v, p.Pass(), &clM, screen, w, tile)
	creature.DrawFarPawns(p.Creatures, v, &clM, screen, w) // OPT-016: dots and silhouettes at the map tier
	p.drawPawnNames(&clM, screen)
	if v.HasSelect {
		for i := range v.Pawns {
			if pw := &v.Pawns[i]; pw.ID == v.Selected {
				wx, wy := p.interpWorld(*pw)
				wx, wy = CreatureAt(wx, wy, tile, ct)
				sx, sy := v.WorldToScreenF(wx, wy)
				overlay.DrawSelectBrackets(screen, l.WhiteImg(), float64(sx), float64(sy), ct, ct, ct) // UIS-092: the cell it is drawn on
			}
		}
	}
}

// drawPawnNames puts every colonist's name under the cell it is drawn on
// (UIS-088), at every zoom and never decluttered; off-view ones are
// skipped.
func (p *Pipeline) drawPawnNames(cl *creature.Cull, screen *ebiten.Image) {
	v := p.v
	for i := range v.Pawns {
		pw := &v.Pawns[i]
		if pw.Name == "" || !light.Colonist(pw) || !p.LOD.InView(pw.X, pw.Y, 2, 2) {
			continue
		}
		wx, wy := p.interpWorld(*pw)
		creature.DrawPawnName(p.Creatures, v, cl, screen, pw.Name, wx+0.5, wy+1)
	}
}

// frameViews is the frame's chunk lists when w is the world they index,
// else nil (env.Layer.DrawFurrows walks w then).
func (b *base) frameViews(w *game.WorldView) []game.ChunkView {
	if !b.Index.FrameLists(env.ListsOf(w)) {
		return nil
	}
	return b.LOD.Views
}

// resetCells empties a reused per-frame tile set.
// refreshCreatureCells keeps the occupied and creature cell sets for the
// pawns and animals of this frame, rebuilding them only when a creature
// moved, appeared or left since the last frame (GAR-153).
func (l *LOD) refreshCreatureCells(pawns []game.PawnView, animals []game.AnimalView) {
	cells := l.cells[:0]
	for _, pw := range pawns {
		cells = append(cells, [2]int{pw.X, pw.Y})
	}
	for i := range animals {
		cells = append(cells, [2]int{animals[i].X, animals[i].Y})
	}
	same := l.occupied != nil && l.cellPawns == len(pawns) && slices.Equal(cells, l.prevCells)
	l.cells, l.prevCells, l.cellPawns = l.prevCells, cells, len(pawns)
	if same {
		return
	}
	l.occupied = resetCells(l.occupied)
	l.Creatures = resetCells(l.Creatures)
	for i, c := range cells {
		if i < len(pawns) {
			l.occupied[c] = true
		}
		l.Creatures[c] = true
	}
}

func resetCells(m map[[2]int]bool) map[[2]int]bool {
	if m == nil {
		return map[[2]int]bool{}
	}
	clear(m)
	return m
}

// CreatureAt is where a creature at (wx, wy) is drawn at tile size ct
// when the camera's tile is smaller (CreatureMinTile): the same
// footprint centre and bottom, so it stays on its tile.
func CreatureAt(wx, wy, tile, ct float64) (float64, float64) {
	if ct <= tile || tile <= 0 {
		return wx, wy
	}
	k := ct / tile
	return wx + 0.5 - 0.5*k, wy + 1 - k
}

// drawTerrainArt draws the ground of the visible tiles (padded by one):
// the tile, the soft edges of higher-priority neighbours; then, over the
// whole view's ground, the grass tufts (so a neighbour's ground never
// paints over a tuft). Stone ground (rough, smooth, under rock) takes its
// stone's tint.
func (p *Pipeline) drawTerrainArt(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v := p.v
	x0, y0, x1, y1 := p.visibleTiles()
	x0, y0 = max(x0-1, 0), max(y0-1, 0)
	x1, y1 = min(x1+1, v.MapW-1), min(y1+1, v.MapH-1)
	p.Lit.BatchGround(true) // one call for the lit tiles and edges (GAR-159)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			p.drawGroundTile(screen, &w, x, y, tile, sw, sh)
		}
	}
	p.Lit.BatchGround(false)
	p.Env.DrawFurrows(v, screen, w, tile, p.frameViews(&w))            // render/env: tilled ground under crops
	p.Env.DrawPuddleVeil(v, screen, w, sw, sh, tile, p.Stamp, p.Index) // render/env (PLAY-022): under the tufts
	p.drawTufts(screen, w, sw, sh, tile)
}

// drawTufts draws the visible grass over the ground (render/env): the
// tuft batch, every copy from DetailMin and at most env.GrassMidCap from
// env.GrassLiveMin (below that the chunk images hold the tufts), then the
// cut marks.
func (p *Pipeline) drawTufts(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v := p.v
	vx0, vy0, vx1, vy1 := p.visibleTiles()
	x0, y0, x1, y1 := p.Env.GrassReach(v, vx0, vy0, vx1, vy1)
	p.Env.GrassFrameMask(&w, x0, y0, x1, y1, p.Index)
	p.LOD.Grass.Quads = 0
	if render.LODTile(tile, v.Dev) >= env.GrassLiveMin { // the live tier (GAR-155)
		limit := 9
		if tile < DetailMin {
			limit = env.GrassMidCap
		}
		p.Env.GrassBatchOf(v, screen, &w, x0, y0, x1, y1, tile, sw, sh, limit, p.LOD.Baking)
	}
	p.Env.DrawGrassCutMarks(v, p.Pass(), screen, &w, vx0, vy0, vx1, vy1, tile, sw, sh)
}

// addRockDraws queues the natural rock walls in view, padded one column
// each side, one row north and two south (tall sprites reach north):
// linked among rock and walls, tinted by their stone, ore cells with
// their vein sprite.
func (p *Pipeline) addRockDraws(w game.WorldView, draws []ArtDraw) []ArtDraw {
	if w.TerrainW <= 0 {
		return draws
	}
	x0, y0, x1, y1 := p.visibleTiles()
	x0, y0 = max(x0-1, 0), max(y0-1, 0)
	x1, y1 = min(x1+1, p.v.MapW-1, w.TerrainW-1), min(y1+2, p.v.MapH-1)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			i := y*w.TerrainW + x
			if i >= len(w.Terrain) || !sprite.IsRockWallCode(w.Terrain[i]) {
				continue
			}
			if ld, ok := p.rockDraw(&w, x, y); ok {
				draws = append(draws, ArtDraw{Row: float64(y), Col: float64(x), Ld: ld})
			}
		}
	}
	return draws
}

// SmoothMarkKey is the smoothing designation; older art falls back to the
// mine mark cooled blue.
const SmoothMarkKey = "overlay/designation_smooth"

// HaulMarkKey is the designation drawn over a stack marked for hauling.
const HaulMarkKey = "overlay/designation_haul"

// drawHaulMarks draws the haul designation over every marked stack of the
// chunks in view, on top of the objects like the other designations.
func (p *Pipeline) drawHaulMarks(screen *ebiten.Image, w *game.WorldView, views []game.ChunkView, tile float64, sw, sh int) {
	for _, cv := range views {
		for _, i := range cv.Items {
			if int(i) >= len(w.Items) {
				continue
			}
			if it := &w.Items[i]; it.HaulMarked && !it.Carried && p.LOD.Near(it.X, it.Y, 1, 1) {
				p.drawSprite(screen, HaulMarkKey, color.NRGBA{}, float64(it.X), float64(it.Y), tile, sw, sh)
			}
		}
	}
}

var smoothFallbackCS = sprite.ColorScale{0.65, 0.9, 1.35, 1}

// DrawFloorLayers draws the ground under zones: built floors, then the
// tracked-in dirt (ART §16.3), then the blood and ichor stains (ART
// §16.2), so blood always lies over dirt. The dirt and stains layers are
// traced only when there is some.
func (p *Pipeline) DrawFloorLayers(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	p.layer("floors")
	if p.LOD.Tier == Live { // baked below LiveMin (lod.go)
		for _, cv := range p.LOD.Views {
			for _, i := range cv.Buildings {
				if int(i) >= len(w.Buildings) {
					continue
				}
				if b := &w.Buildings[i]; b.Floor {
					p.DrawTile(screen, FloorSprite(b.Kind, b.Stuff), float64(b.X), float64(b.Y), tile, sw, sh, sprite.TintCS(p.floorTint(b.Kind, b.Stuff, b.AX.Colour))) // floor_keys.go
				}
			}
		}
	}
	if p.LOD.Tier == Map { // no decals on the whole-map bake
		return
	}
	v := p.v
	if len(w.Dirt) > 0 {
		p.layer("dirt")
		p.Overlays.DrawDirt(v, p.Pass(), screen, w, sw, sh, tile)
	}
	if len(w.Stains) > 0 {
		p.layer("stains")
		p.Overlays.DrawStains(v, p.Pass(), screen, w, sw, sh, tile)
		p.Overlays.Fires.DrawAshStains(v, p.Pass(), screen, w, sw, sh, tile)
	}
}

// ShadowFeet appends to out[:0] the feet of this frame's shadow casters:
// the colonists in view standing on unroofed tiles (light.Casts), where
// they are drawn (their motion, scaled up with the creatures far out).
// Nothing else casts a shadow (user decision, 2026-11-01).
func (p *Pipeline) ShadowFeet(w *game.WorldView, pawns []game.PawnView, tile float64, out []light.Foot) []light.Foot {
	out = out[:0]
	l := p.LOD
	ct := LodTile(tile)
	k := 1.0
	if ct > tile && tile > 0 {
		k = ct / tile
	}
	for i := range pawns {
		pw := &pawns[i]
		if !light.Casts(pw, w) || !l.InView(pw.X, pw.Y, 2, 2) {
			continue
		}
		wx, wy := p.interpWorld(*pw)
		wx, wy = CreatureAt(wx, wy, tile, ct)
		out = append(out, light.Foot{X: wx + creature.PawnFeetX*k, Y: wy + creature.PawnFeetY*k, Scale: k})
	}
	return out
}
