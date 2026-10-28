package ebiten

import (
	"cmp"
	"image/color"
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"slices"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
)

// Draw path for the original generated art. Order (assets/AGENTS.md):
// terrain -> zones/blueprints -> objects north-to-south -> overlays.

const (
	soilVariants = 4
	pawnBobPx    = 0.035 // of a tile
	grassEvery   = 5
)

// artDraw is one queued object draw: a closure (cave mouths), or (fn nil)
// a plain record — a linked sprite (rock) or an index into the frame's
// lists — so a frame queues thousands without allocating (NG-1 §6.7).
type artDraw struct {
	row, col float64
	fn       func()
	ld       linkedDraw
	kind     objKind
	i        int32   // index into the list kind names
	wx, wy   float64 // creatures: where they are drawn
}

// objKind is what a record artDraw draws.
type objKind uint8

const (
	objLinked    objKind = iota // ld (rock)
	objBuilding                 // w.Buildings[i]
	objFrame                    // w.Frames[i]
	objPlant                    // w.Plants[i]
	objPlantMark                // w.Plants[i] as a map-bake mark
	objItem                     // w.Items[i]
	objForbidden                // the forbidden mark over w.Items[i]
	objAnimal                   // w.Animals[i] at (wx, wy)
	objPawn                     // r.view.Pawns[i] at (wx, wy)
)

// sortDraws orders draws north to south, then west to east, keeping the
// queue order of ties.
func sortDraws(draws []artDraw) {
	slices.SortStableFunc(draws, func(a, b artDraw) int {
		if c := cmp.Compare(a.row, b.row); c != 0 {
			return c
		}
		return cmp.Compare(a.col, b.col)
	})
}

// colorScale is a per-draw RGBA multiplier (ghosts, blueprints).
type colorScale [4]float32

var (
	noCS        = colorScale{1, 1, 1, 1}
	blueprintCS = colorScale{0.55, 0.85, 1.25, 0.5}
	ghostOKCS   = colorScale{0.7, 1.25, 0.7, 0.65}
	ghostBadCS  = colorScale{1.3, 0.45, 0.4, 0.6}
	ghostSameCS = colorScale{0.9, 0.9, 0.9, 0.5}
	// driedCS browns and darkens food that came off a drying rack;
	// rottenCS sickens a rotten corpse.
	driedCS  = colorScale{0.86, 0.68, 0.48, 1}
	rottenCS = colorScale{0.62, 0.74, 0.5, 1}
)

// drawRackLoad lays a drying batch along the rack's slats, browning as it
// dries.
func (r *Renderer) drawRackLoad(screen *ebiten.Image, b game.BuildingView, tile float64, sw, sh int) {
	key := itemSprite(b.DryKind)
	cs := lerpCS(noCS, driedCS, float64(b.DryPct)/100)
	w, h := max(b.W, 1), max(b.H, 1)
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			r.drawSpriteScaled(screen, key, color.NRGBA{}, float64(b.X+dx), float64(b.Y+dy)-rackSlatLift, tile, sw, sh, cs, 0.55)
		}
	}
}

// rackSlatLift raises rack contents (in tiles) to sit on the slats.
const rackSlatLift = 0.3

// itemCS is the colour treatment for a ground stack.
func itemCS(it game.ItemView) colorScale {
	switch {
	case it.Dried:
		return driedCS
	case it.Rotten:
		return rottenCS
	}
	return noCS
}

// lerpCS blends from a to b by t in [0, 1].
func lerpCS(a, b colorScale, t float64) colorScale {
	var out colorScale
	for i := range out {
		out[i] = a[i] + (b[i]-a[i])*float32(t)
	}
	return out
}

func (r *Renderer) drawSprite(screen *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, sw, sh int) {
	r.drawSpriteCS(screen, key, tint, wx, wy, tile, sw, sh, noCS)
}

func (r *Renderer) drawSpriteCS(screen *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, sw, sh int, cs colorScale) {
	r.drawSpriteScaled(screen, key, tint, wx, wy, tile, sw, sh, cs, 1)
}

// drawSpriteScaled draws a sprite scaled by k about its footprint's
// bottom-centre (ground origin), e.g. young trees.
func (r *Renderer) drawSpriteScaled(screen *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, sw, sh int, cs colorScale, k float64) {
	sp := r.art.Sprite(key)
	if sp == nil {
		return
	}
	sx, sy := r.worldToScreenF(wx, wy)
	x, y, s := artOrigin(sp.Meta, float64(sx), float64(sy), tile)
	if k != 1 {
		px := float64(sx) + float64(sp.Meta.Footprint[0])*tile/2
		py := float64(sy) + float64(sp.Meta.Footprint[1])*tile
		x, y, s = px+(x-px)*k, py+(y-py)*k, s*k
	}
	w, h := float64(sp.Meta.Canvas[0])*s, float64(sp.Meta.Canvas[1])*s
	if !visible(float32(x), float32(y), float32(w), float32(h), sw, sh) {
		return
	}
	img := r.art.Image(key, tint)
	if img == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(x, y)
	op.Filter = ebiten.FilterLinear
	if !strings.HasPrefix(key, "overlay/") { // zones and markers stay unlit
		if isGroundKey(key) { // terrain edges: the lamp light ramps like the tile under them
			r.drawLitGround(screen, img, op.GeoM, cs, wx, wy)
			return
		}
		r.drawLit(screen, img, op.GeoM, cs, wx, wy)
		return
	}
	if cs != noCS {
		op.ColorScale.Scale(cs[0]*cs[3], cs[1]*cs[3], cs[2]*cs[3], cs[3])
	}
	screen.DrawImage(img, op)
}

// drawPlacementGhost previews a build tool: the real sprite per cell,
// green when placeable, red when not. Walls and conduits autotile among
// the ghosts.
func (r *Renderer) drawPlacementGhost(screen *ebiten.Image, kind, stuff, facing string, cells []ghostCell) {
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	tile := r.tilePx()
	link, power := buildingLink(kind), false
	if c := powerLink(kind); c.accepts != 0 { // conduits link on the power layer (linkgrid.go)
		link, power = c, true
	}
	var p map[[2]int]linkFlag
	if r.art != nil && link.flags != 0 { // ghosts join built walls, rock, frames and each other
		w := r.view.W
		if w.TerrainW == 0 {
			w = r.world()
		}
		r.links.sync(w, r.game.Width(), r.game.Height())
		ghosts := make([]plannedCell, len(cells))
		for i, c := range cells {
			ghosts[i] = plannedCell{x: c.x, y: c.y, flags: link.flags}
		}
		p = r.links.plan(w.Frames, ghosts)
	}
	tint := r.stuffTint(stuff)
	for _, c := range cells {
		cs := ghostOKCS
		switch {
		case c.same:
			cs = ghostSameCS
		case !c.ok:
			cs = ghostBadCS
		}
		if r.art == nil {
			sx, sy := r.worldToScreen(c.x, c.y)
			vector.FillRect(screen, sx, sy, float32(tile), float32(tile), ghostFill(c), false)
			continue
		}
		mask, corners := 0, 0
		switch {
		case p != nil && power:
			mask = r.links.powerMask(link.accepts, c.x, c.y, p)
		case p != nil:
			mask = r.links.plannedMask(p, link.accepts, c.x, c.y)
			corners = r.links.plannedCorners(p, link.accepts, c.x, c.y)
		}
		if kind == "wall" && r.drawLinked(screen, wallPrefix(stuff), c.x, c.y, mask, corners, tint, cs, tile, sw, sh) {
			continue
		}
		cf := ghostFacing(c, facing)
		key := r.buildKey(kind, stuff, cf, mask, doorAxisEW(cf), false)
		if !r.art.Has(key) {
			sx, sy := r.worldToScreen(c.x, c.y)
			if isFenceKind(kind) {
				drawFenceVector(screen, sx, sy, float32(tile), kind, mask, false, isStoneStuff(stuff), 170)
				continue
			}
			vector.FillRect(screen, sx, sy, float32(tile), float32(tile), ghostFill(c), false)
			continue
		}
		r.drawSpriteCS(screen, key, color.NRGBA{}, float64(c.x), float64(c.y), tile, sw, sh, mulCS(tintCS(tint), cs))
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

// doorAxisEW is the door sprite axis for a stored facing: South/North
// doors span east-west ("ew"), East/West doors north-south ("ns").
func doorAxisEW(facing string) bool { return facing != "east" && facing != "west" }

// buildKey is buildSpriteStuff with a fallback to the wood sprite when a
// material's art (steel doors, stone fences) is missing.
func (r *Renderer) buildKey(kind, stuff, facing string, mask int, ew, open bool) string {
	key := buildSpriteStuff(kind, stuff, facing, mask, ew, open)
	if r.art != nil && stuffMaterial(stuff) != "wood" && !r.art.Has(key) {
		if alt := buildSprite(kind, facing, mask, ew, open); alt != "" && r.art.Has(alt) {
			return alt
		}
	}
	return key
}

// layer marks the start of a ground layer for the draw-order test.
func (r *Renderer) layer(name string) {
	r.view.LayerDrawn(name)
}

func (r *Renderer) drawArt(screen *ebiten.Image, sw, sh int, tile float64) {
	none := color.NRGBA{}
	w := r.game.World()
	r.view.W, r.view.Pawns = w, r.game.Snapshot() // this frame's world and pawns, for every layer
	r.lit.Light = lightAt(w.Hour, w.DayOfYear, w.Weather, w.WeatherIntensity)
	r.setView(sw, sh, tile)
	r.links.sync(w, r.game.Width(), r.game.Height()) // linkgrid.go: rebuilt only on StructVer
	r.lod.tier = lodFor(tile)
	r.lod.crops.begin()
	r.syncFrameChunks() // lod.go: the chunk lists in view
	r.lod.chunks.frame++
	views := r.lod.views
	lt := r.lod.tier

	if lt == lodLive {
		r.drawTerrainArt(screen, w, sw, sh, tile)
	} else {
		r.drawGroundLOD(screen, &w, sw, sh, tile) // lod.go: chunk or map bakes
		r.drawPuddleVeil(screen, w, sw, sh, tile) // puddle_veil.go (PLAY-022): under the tufts
		if lt == lodChunk {
			r.drawFurrows(screen, w, tile) // crop_batch.go: tilled ground under crops
			r.drawTufts(screen, w, sw, sh, tile)
		}
	}
	r.drawGroundCover(screen, w, sw, sh, tile)
	r.drawFoundation(screen, w, sw, sh, tile) // bridge_draw.go: over the water, under floors
	r.drawFloorLayers(screen, w, sw, sh, tile)
	r.drawConduits(screen, w, sw, sh, tile) // power_draw.go: over floors, under zones and objects
	r.power.prune(r.present)
	r.layer("zones")
	r.drawZones(screen, w, sw, sh, tile) // zones_draw.go
	r.drawPlanOverlay(screen, w, sw, sh, tile)
	r.drawSnowClearOverlay(screen, w, sw, sh, tile) // snow_clear_draw.go (PLAY-015)
	for _, f := range w.Frames {
		r.drawSprite(screen, "overlay/blueprint", none, float64(f.X), float64(f.Y), tile, sw, sh)
	}

	if lt == lodLive { // floor markings and carpets lie on the floor (baked below 48 px)
		for _, cv := range views {
			for _, i := range cv.Buildings {
				if int(i) >= len(w.Buildings) {
					continue
				}
				switch b := &w.Buildings[i]; b.Kind {
				case "sleeping spot":
					r.drawSprite(screen, buildSprite(b.Kind, b.Facing, 0, false, false), none, float64(b.X), float64(b.Y), tile, sw, sh)
				case "carpet":
					r.drawSprite(screen, buildSpriteStuff(b.Kind, b.Stuff, "", 0, false, false), none, float64(b.X), float64(b.Y), tile, sw, sh)
				}
			}
		}
	}
	for _, f := range w.Frames {
		if f.Kind == "carpet" {
			r.drawSpriteCS(screen, buildSpriteStuff(f.Kind, f.Stuff, "", 0, false, false), none, float64(f.X), float64(f.Y), tile, sw, sh, blueprintCS)
		} else if f.Floor {
			cs := mulCS(tintCS(r.stuffTint(f.Stuff)), blueprintCS)
			r.drawSpriteCS(screen, r.buildKey("floor", f.Stuff, "", 0, false, false), none, float64(f.X), float64(f.Y), tile, sw, sh, cs)
		}
	}

	if lt != lodMap { // no shadows on the whole-map bake
		r.drawShadows(screen, w, sw, sh, tile) // shadow.go: under every object
		r.drawEaves(screen, w, sw, sh, tile)   // roof_draw.go
	}

	l := &r.lod
	l.occupied = resetCells(l.occupied)
	l.creatures = resetCells(l.creatures)
	for _, p := range r.view.Pawns {
		l.occupied[[2]int{p.X, p.Y}] = true
		l.creatures[[2]int{p.X, p.Y}] = true
	}
	for i := range w.Animals {
		l.creatures[[2]int{w.Animals[i].X, w.Animals[i].Y}] = true
	}
	r.sweepTreeFades()
	r.drawPaths(screen, sw, sh, tile) // paths_draw.go
	draws := r.draws[:0]
	if lt == lodLive { // rock is baked below 48 px
		draws = r.addRockDraws(w, draws)
	}
	if lt != lodMap {
		draws = r.addCaveMouths(w, draws, screen, tile, sw, sh) // ng1_art.go
	}
	l.cloth = r.colonyCloths()
	// planned walls and fences autotile with each other, built walls and
	// rock (an O(frames) planned set, linkgrid.go)
	l.planned = r.links.plan(w.Frames, nil)
	// objects of the chunks in view, culled by position before queueing:
	// sprites reach up to three tiles north and a tile or two sideways
	for _, cv := range views {
		if lt != lodMap { // buildings and plants are in the map bake
			for _, i := range cv.Buildings {
				if int(i) >= len(w.Buildings) {
					continue
				}
				if b := &w.Buildings[i]; !b.Floor && !onFloor(b.Kind) && l.near(b.X, b.Y, 3, 4) {
					draws = append(draws, artDraw{row: float64(b.Y + max(b.H, 1) - 1), col: float64(b.X), kind: objBuilding, i: i})
				}
			}
			for _, i := range cv.Plants {
				if int(i) >= len(w.Plants) {
					continue
				}
				if p := &w.Plants[i]; l.near(p.X, p.Y, 2, 4) {
					draws = append(draws, artDraw{row: float64(p.Y), col: float64(p.X), kind: objPlant, i: i})
				}
			}
			for _, i := range cv.Items {
				if int(i) >= len(w.Items) {
					continue
				}
				it := &w.Items[i]
				if it.Carried || !l.near(it.X, it.Y, 2, 2) { // an XL corpse reaches 1.66 tiles (PLAY-022)
					continue
				}
				if it.Forbidden {
					draws = append(draws, artDraw{row: float64(it.Y) + 0.001, col: float64(it.X), kind: objForbidden, i: i})
				}
				draws = append(draws, artDraw{row: float64(it.Y), col: float64(it.X), kind: objItem, i: i})
			}
		}
		for _, i := range cv.Frames {
			if int(i) >= len(w.Frames) {
				continue
			}
			if f := &w.Frames[i]; !f.Floor && f.Kind != "carpet" && !isConduitKind(f.Kind) && l.near(f.X, f.Y, 3, 4) {
				draws = append(draws, artDraw{row: float64(f.Y + max(f.H, 1) - 1), col: float64(f.X), kind: objFrame, i: i})
			}
		}
	}
	ct := lodTile(tile)               // creatures keep a minimum size (lod.go)
	apad, apadS := r.animalPads(tile) // lod.go: the largest animal canvas (PLAY-022)
	for i := range w.Animals {
		an := &w.Animals[i]
		if !l.near(an.X, an.Y, apad, apadS) {
			continue
		}
		wx, wy := r.interpAnimal(*an)
		wx, wy = creatureAt(wx, wy, tile, ct)
		draws = append(draws, artDraw{row: wy + 0.01, col: wx, kind: objAnimal, i: int32(i), wx: wx, wy: wy})
	}
	for i := range r.view.Pawns {
		p := &r.view.Pawns[i]
		if p.Embodied || !l.near(p.X, p.Y, 3, 4) {
			continue // an embodied pawn's body is a corpse item now
		}
		wx, wy := r.interpWorld(*p)
		row := wy + 0.01
		if p.Lying && p.InBed {
			if b, ok := bedUnder(w, p.X, p.Y); ok {
				row = float64(b.Y+max(b.H, 1)-1) + 0.02 // over its bed
			}
		}
		wx, wy = creatureAt(wx, wy, tile, ct)
		draws = append(draws, artDraw{row: row, col: wx, kind: objPawn, i: int32(i), wx: wx, wy: wy})
	}
	sortDraws(draws)
	for i := range draws {
		r.runDraw(screen, &w, &draws[i], tile, sw, sh)
	}
	r.flushCrops(screen) // crop_batch.go: the last run of units
	clear(draws)         // drop this frame's closures; keep the buffer
	r.draws = draws[:0]
	r.drawWorkGlow(screen, tile)
	r.drawFires(screen, w, tile)
	r.drawWorldFires(screen, w, tile) // fire_world_draw.go: PLAY-015 flames
	// cave_plants_draw.go: the duskmoss glow
	r.drawDuskGlow(screen, w, tile)
	r.drawZoneLabels(screen, w, sw, sh, tile) // zones_draw.go: over the stacks and crops
	if lt != lodMap {                         // no item sprites, so no counts either
		r.drawStackCounts(screen, w, tile)
	}
	r.drawBuildingDamage(screen, w, tile)
	r.drawWorkProgress(screen, r.view.Pawns, tile) // work_progress.go: bars over the work spots
	r.drawNoPowerBadges(screen, &w, tile)          // power_draw.go
	r.drawChatBubbles(screen, tile)
	r.drawBreakClouds(screen, tile)
	r.drawOrderRoute(screen, tile)
	r.drawParticles(screen, tile)
	r.drawArrivals(screen, w, tile)   // arrival_draw.go
	r.drawSteamPuffs(screen, w, tile) // features_draw.go
	r.drawWeather(screen, w, sw, sh, tile)
	r.drawFog(screen, w, sw, sh, tile) // fog_draw.go: unexplored tiles over everything
	gfx.FogDrawn()                     // world_scale.go: later labels show over the fog
	r.drawFloaters(screen, tile)

	for _, cv := range views {
		for _, i := range cv.Plants {
			if int(i) >= len(w.Plants) {
				continue
			}
			switch p := &w.Plants[i]; {
			case p.Cut:
				r.drawSprite(screen, "overlay/designation_cut", none, float64(p.X), float64(p.Y), tile, sw, sh)
			case p.Designated:
				r.drawSprite(screen, "overlay/designation_harvest", none, float64(p.X), float64(p.Y), tile, sw, sh)
			}
		}
	}
	if lt != lodMap { // the map bake draws no item sprites, so no marks over them
		r.drawHaulMarks(screen, &w, views, tile, sw, sh)
	}
	for _, m := range w.Mines {
		if l.near(m[0], m[1], 1, 1) {
			r.drawSprite(screen, "overlay/designation_mine", none, float64(m[0]), float64(m[1]), tile, sw, sh)
		}
	}
	smoothKey, smoothCS := smoothMarkKey, noCS
	if !r.art.Has(smoothKey) { // older art: the mine mark, cooled blue
		smoothKey, smoothCS = "overlay/designation_mine", smoothFallbackCS
	}
	for _, m := range w.Smooths {
		if l.near(m[0], m[1], 1, 1) {
			r.drawSpriteCS(screen, smoothKey, none, float64(m[0]), float64(m[1]), tile, sw, sh, smoothCS)
		}
	}
	r.drawAnimalOverlays(screen, w, tile, sw, sh)
	if r.hasSelect {
		for _, p := range r.view.Pawns {
			if p.ID == r.selected {
				wx, wy := r.interpWorld(p)
				wx, wy = creatureAt(wx, wy, tile, ct)
				k := 1 + 0.045*math.Sin(float64(r.present)/7)
				r.drawSpritePose(screen, "overlay/selection", none, wx, wy, ct, pose{Px: 0.5, Py: 0.5, Sx: k, Sy: k})
			}
		}
	}
}

// resetCells empties a reused per-frame tile set.
func resetCells(m map[[2]int]bool) map[[2]int]bool {
	if m == nil {
		return map[[2]int]bool{}
	}
	clear(m)
	return m
}

// creatureAt is where a creature at (wx, wy) is drawn at tile size ct
// when the camera's tile is smaller (lod.go creatureMinTile): the same
// footprint centre and bottom, so it stays on its tile.
func creatureAt(wx, wy, tile, ct float64) (float64, float64) {
	if ct <= tile || tile <= 0 {
		return wx, wy
	}
	k := ct / tile
	return wx + 0.5 - 0.5*k, wy + 1 - k
}

// runDraw draws one queued object.
func (r *Renderer) runDraw(dst *ebiten.Image, w *game.WorldView, d *artDraw, tile float64, sw, sh int) {
	if d.fn != nil || d.kind != objPlant {
		r.flushCrops(dst) // queued crop units draw before whatever comes next
	}
	if d.fn != nil {
		d.fn()
		return
	}
	switch d.kind {
	case objLinked:
		r.drawLinkedSprite(dst, d.ld, tile, sw, sh)
	case objBuilding:
		r.drawBuildingObj(dst, &w.Buildings[d.i], tile, sw, sh)
	case objFrame:
		r.drawFrameObj(dst, &w.Frames[d.i], tile, sw, sh)
	case objPlant:
		r.drawPlantObj(dst, w, &w.Plants[d.i], tile)
	case objPlantMark:
		r.drawPlantMark(dst, &w.Plants[d.i], tile)
	case objItem:
		r.drawItemObj(dst, &w.Items[d.i], tile, sw, sh)
	case objForbidden:
		it := &w.Items[d.i]
		r.drawForbiddenMark(dst, it.X, it.Y, tile, sw, sh)
	case objAnimal:
		r.drawAnimalFigure(dst, w.Animals[d.i], d.wx, d.wy, lodTile(tile))
	case objPawn:
		r.drawPawnArt(dst, r.view.Pawns[d.i], d.wx, d.wy, lodTile(tile), sw, sh)
	}
}

// drawBuildingObj draws a building in the object pass (or the map bake):
// linked walls, doors and fences, troughs, graves, pyres and the rest,
// with its power parts (power_draw.go), a drying rack's load and the
// deconstruct mark.
func (r *Renderer) drawBuildingObj(dst *ebiten.Image, b *game.BuildingView, tile float64, sw, sh int) {
	none := color.NRGBA{}
	l := &r.lod
	mask, corners := 0, 0
	open := l.occupied[[2]int{b.X, b.Y}]
	switch {
	case b.Kind == "wall":
		mask, corners = r.links.mask(b.X, b.Y), r.links.corners(b.X, b.Y)
	case isFenceKind(b.Kind):
		mask, open = r.links.mask(b.X, b.Y), l.creatures[[2]int{b.X, b.Y}]
	case b.Kind == powerSwitchKind:
		open = !b.PowerOn // the actual state: an open switch is off
	}
	switch b.Kind {
	case "grave":
		r.drawGrave(dst, *b, tile, sw, sh)
	case "funeral pyre":
		r.drawPyre(dst, *b, tile, sw, sh)
	default:
		if b.Kind == "wall" {
			if wd, ok := r.linkedSprite(wallPrefix(b.Stuff), b.X, b.Y, mask, corners, r.stuffTint(b.Stuff), noCS); ok {
				r.drawLinkedSprite(dst, wd, tile, sw, sh)
				break
			}
		}
		key := r.buildKey(b.Kind, b.Stuff, b.Facing, mask, doorAxisEW(b.Facing), open)
		if b.Kind == casketKind {
			key = casketSprite(*b) // vault_draw.go: closed or open
		}
		if b.Kind == troughKind {
			key = troughKey(*b)
			if !r.art.Has(key) {
				r.drawTroughFallback(dst, *b, tile)
				break
			}
		}
		if isFenceKind(b.Kind) && !r.art.Has(key) {
			sx, sy := r.worldToScreen(b.X, b.Y)
			drawFenceVector(dst, sx, sy, float32(tile), b.Kind, mask, open, isStoneStuff(b.Stuff), 255)
			break
		}
		t := none
		if (b.Kind == "bed" || b.Kind == "double bed") && len(l.cloth) > 0 {
			t = r.art.Tint("cloth", l.cloth[mix64(uint64(b.ID))%uint64(len(l.cloth))])
		}
		// stone walls, fences and floors take their stone's tint
		r.drawSpriteCS(dst, key, t, float64(b.X), float64(b.Y), tile, sw, sh, tintCS(r.stuffTint(b.Stuff)))
		r.drawPowerParts(dst, b, key, tile, sw, sh)   // power_draw.go: glow, blades, flame, charge
		r.drawDefenseParts(dst, b, key, tile, sw, sh) // turret_draw.go: gun, beacon light
	}
	r.drawRuinOverlay(dst, b, tile, sw, sh) // features_draw.go
	if b.DryQty > 0 {
		r.drawRackLoad(dst, *b, tile, sw, sh)
	}
	if b.Deconstruct {
		r.drawSprite(dst, "overlay/designation_deconstruct", none, float64(b.X), float64(b.Y), tile, sw, sh)
	}
}

// drawFrameObj draws a planned building (blueprint), autotiled with the
// frame's planned set (l.planned).
func (r *Renderer) drawFrameObj(dst *ebiten.Image, f *game.FrameView, tile float64, sw, sh int) {
	none := color.NRGBA{}
	pm, pc := 0, 0
	if c := buildingLink(f.Kind); c.flags != 0 {
		pm = r.links.plannedMask(r.lod.planned, c.accepts, f.X, f.Y)
		if f.Kind == "wall" {
			pc = r.links.plannedCorners(r.lod.planned, c.accepts, f.X, f.Y)
		}
	}
	tint := r.stuffTint(f.Stuff)
	if f.Wood > 0 {
		r.drawSprite(dst, "structure/frame_1x1", none, float64(f.X), float64(f.Y), tile, sw, sh)
	}
	if f.Kind == "wall" {
		if wd, ok := r.linkedSprite(wallPrefix(f.Stuff), f.X, f.Y, pm, pc, tint, blueprintCS); ok {
			r.drawLinkedSprite(dst, wd, tile, sw, sh)
			return
		}
	}
	key := r.buildKey(f.Kind, f.Stuff, f.Facing, pm, doorAxisEW(f.Facing), false)
	if isFenceKind(f.Kind) && !r.art.Has(key) {
		sx, sy := r.worldToScreen(f.X, f.Y)
		drawFenceVector(dst, sx, sy, float32(tile), f.Kind, pm, false, isStoneStuff(f.Stuff), 140)
		return
	}
	r.drawSpriteCS(dst, key, none, float64(f.X), float64(f.Y), tile, sw, sh, mulCS(tintCS(tint), blueprintCS))
}

// drawPlantObj draws a plant swaying (plantPose) in its season's look. A
// crop queues its units instead (crop_batch.go); cold or heat damage
// tints, shrinks and fades any plant (plantDamageLook).
func (r *Renderer) drawPlantObj(dst *ebiten.Image, w *game.WorldView, p *game.PlantView, tile float64) {
	if isCropPlant(p) {
		r.drawCropObj(dst, w, p, tile)
		return
	}
	r.flushCrops(dst)
	key, scale := plantSprite(*p)
	key = r.seasonKey(key, *w)
	amount, kind := 0.035, kindBush
	switch p.Species {
	case "tree", "pine", "jungle_tree":
		amount, kind = 0.022, kindTree
	}
	ps := r.plantPose(p.X, p.Y, kind, amount, scale)
	dcs, dk := plantDamageLook(*p)
	ps.Cs = dcs
	ps.Sx, ps.Sy = ps.Sx*dk, ps.Sy*dk
	if kind == kindTree {
		if trunk, canopy, ok := r.treePartKeys(key); ok {
			// PLAY-010 2.9: the trunk stays opaque, the canopy turns
			// see-through over creatures behind it; one pose for both
			r.layer("tree_trunk")
			r.drawSpritePose(dst, trunk, color.NRGBA{}, float64(p.X), float64(p.Y), tile, ps)
			if a := r.treeCanopyAlpha(p.X, p.Y); a < 1 {
				ps.Cs = mulCS(ps.Cs, colorScale{1, 1, 1, a})
			}
			r.layer("tree_canopy")
			r.drawSpritePose(dst, canopy, color.NRGBA{}, float64(p.X), float64(p.Y), tile, ps)
			return
		}
	}
	r.drawSpritePose(dst, key, color.NRGBA{}, float64(p.X), float64(p.Y), tile, ps)
}

// drawItemObj draws a ground stack (corpses and bones their own way).
func (r *Renderer) drawItemObj(dst *ebiten.Image, it *game.ItemView, tile float64, sw, sh int) {
	if it.Kind == "human corpse" || it.Kind == "bones" {
		r.drawCorpseItem(dst, *it, tile, sw, sh)
		return
	}
	key := sizedItemKey(itemSpriteSpecies(it.Kind, it.Animal, it.X, it.Y), it.Qty)
	cs := itemCS(*it)
	if key, cs = r.stoneItem(key, it.Kind, cs); !r.art.Has(key) {
		pad := float32(tile) * 0.3
		sx, sy := r.worldToScreen(it.X, it.Y)
		vector.FillRect(dst, sx+pad, sy+pad, float32(tile)-2*pad, float32(tile)-2*pad, color.RGBA{200, 90, 40, 255}, false)
		return
	}
	r.drawSpriteScaled(dst, key, color.NRGBA{}, float64(it.X), float64(it.Y), tile, sw, sh, cs, r.itemPop(it.ID))
}

func itemSprite(kind string) string {
	switch kind {
	case "wood":
		return "item/wood_pile"
	case "food":
		return "item/ration_pack"
	case "berries":
		return "item/berries_basket"
	case "potatoes":
		return "item/potatoes"
	case "corn":
		return "item/corn_pile"
	case "wheat":
		return "item/wheat_sheaf"
	case "beans":
		return "item/beans_sack"
	case "lettuce":
		return "item/lettuce_heads"
	case "radishes":
		return "item/radish_bunch"
	case "pumpkins":
		return "item/pumpkins"
	case "herbal medicine":
		return "item/herbal_medicine"
	case "components":
		return "item/components_m" // sizedItemKey picks the pile by quantity
	case "industrial medicine":
		return "item/industrial_medicine_m"
	case "meat":
		return "item/meat_pile"
	case "leather":
		return "item/leather_hides"
	case "parka":
		return "item/parka_folded"
	case "tuque":
		return "item/tuque"
	case "duster":
		return "item/duster_folded"
	case "meal":
		return "item/meal_simple"
	case "steel":
		return "item/steel_bars"
	case "club":
		return "item/club_wood"
	case "rifle":
		return "item/rifle_ground"
	case "dustbuck corpse":
		return "item/corpse_dustbuck"
	case "ridgewolf corpse":
		return "item/corpse_ridgewolf"
	case "stone", "stone_blocks":
		return "item/stone_blocks"
	case "silver":
		return "item/silver"
	case "gold":
		return "item/gold"
	case "cloth":
		return "item/cloth_bolt"
	case "straw":
		return "item/straw_bundle"
	case "straw carpet", "cloth carpet", "leather carpet", "wood carpet", "wool carpet":
		return "item/carpet_" + strings.TrimSuffix(kind, " carpet") + "_rolled"
	case "human corpse":
		return corpseCarrySprite
	case "bones":
		return bonesSprite(0, 0)
	}
	switch {
	case strings.HasSuffix(kind, " chunk"):
		return "item/stone_chunk"
	case strings.HasSuffix(kind, " blocks"):
		return "item/stone_blocks"
	}
	if k := biomeItemSprite(kind); k != "" {
		return k
	}
	return "item/" + strings.ReplaceAll(kind, " ", "_") // sprite names use underscores ("servo arm" -> item/servo_arm)
}

func artKey(family, name string, variant int) string {
	return family + "/" + name + "_" + twoDigits(variant)
}

func twoDigits(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

// colonyCloths lists the cloth tints colonists and their beds may use; the
// raider tint is held back so hostiles stand out.
func (r *Renderer) colonyCloths() []string {
	var cloths []string
	for _, c := range r.art.TintNames("cloth") {
		if c != raiderCloth {
			cloths = append(cloths, c)
		}
	}
	return cloths
}

// raiderCloth is the tint reserved for raider clothing.
const raiderCloth = "raider"

// pawnFigureLayers lists p's sprites bottom first in art facing fs: its
// own look (build, beard, hair; PawnView.Look) dressed in its apparel, a
// raider's maroon clothes, and a disfigured pawn's face scar over the
// hair (under a hat) when the art has one.
func (r *Renderer) pawnFigureLayers(p game.PawnView, fs string) []portraitLayer {
	return figureLayers(r.art, p.Look, fs, figureDress{apparel: p.Apparel, scarred: p.Disfigured, raider: p.Hostile})
}

// pawnPortraitLayers is p's south-facing portrait, bottom first: the
// colony figure's layers (pawnFigureLayers).
func (r *Renderer) pawnPortraitLayers(p game.PawnView) []portraitLayer {
	return r.pawnFigureLayers(p, "south")
}

// faceScarSouth is the south-facing face scar of a disfigured pawn.
const faceScarSouth = "pawn/face_scar_south"

// drawTile draws an opaque, tileable 1x1 sprite snapped to whole pixels
// with a 1 px overlap, so fractional zoom never shows seams between tiles.
// It is ground (terrain, floors, water): lit by drawLitGround.
func (r *Renderer) drawTile(screen *ebiten.Image, key string, wx, wy, tile float64, sw, sh int, cs colorScale) {
	sx, sy := r.worldToScreenF(wx, wy)
	x0, y0 := math.Floor(float64(sx)), math.Floor(float64(sy))
	size := math.Ceil(float64(sx)+tile) - x0 + 1
	if !visible(float32(x0), float32(y0), float32(size), float32(size), sw, sh) {
		return
	}
	img := r.art.Image(key, color.NRGBA{})
	if img == nil {
		return
	}
	k := size / float64(img.Bounds().Dx())
	geo := ebiten.GeoM{}
	geo.Scale(k, k)
	geo.Translate(x0, y0)
	r.drawLitGround(screen, img, geo, cs, wx, wy)
}

// terrainPriority: a higher index draws its soft edge over lower ones
// (the order the terrain edge art is made for, ART-015). Rock draws its
// ground as rough stone, smoothed rock as smooth stone.
//
// Water is the lowest of all (NG-1 §7.3): deep -2, shallow -1, so every
// ground's edge overlaps it and shallow water's own edge overlaps deep.
// A track is bare dirt like soil; mud draws over the ground it lies on.
var terrainPriority = map[string]int{"soil": 0, "rich_soil": 1, "gravel": 2, "stone": 3, "rock": 3,
	"stone_smooth": 4, "rock_smooth": 4, "sand": 5, "marsh": 6, "ice": 7,
	"mud": 8, "track": 0, "shallow_water": -1, "deep_water": -2}

var terrainSprite = map[string]string{"soil": "soil", "rich_soil": "soil_rich", "gravel": "gravel", "stone": "stone", "rock": "stone",
	"sand": "sand", "marsh": "marsh", "ice": "ice", "stone_smooth": "stone_smooth", "rock_smooth": "stone_smooth",
	"mud": "mud", "track": "track", "shallow_water": "shallow_water", "deep_water": "deep_water"}

// terrainByPri lists the edge-drawing terrains by priority (index = priority;
// water and track draw no ground edge of their own here, waterEdge does).
var terrainByPri = []string{"soil", "rich_soil", "gravel", "stone", "stone_smooth", "sand", "marsh", "ice", "mud"}

// terrainNames, terrainPriByCode and stoneGroundCode index the read
// model's terrain codes once (no per-tile map lookups).
var (
	terrainNames     = game.TerrainNames()
	terrainPriByCode = func() (out [256]int8) {
		for i, n := range terrainNames {
			out[i] = int8(terrainPriority[n])
		}
		return out
	}()
	isWaterCode = func() (out [256]bool) {
		for i, n := range terrainNames {
			out[i] = n == "shallow_water" || n == "deep_water"
		}
		return out
	}()
	stoneGroundCode = func() (out [256]bool) {
		for i, n := range terrainNames {
			out[i] = n == "stone" || n == "stone_smooth" || isRockWallName(n)
		}
		return out
	}()
	// terrainEdgeKeys[p][mask] is terrain/<sprite>_edge_<NN> of priority p.
	terrainEdgeKeys = func() [][16]string {
		out := make([][16]string, len(terrainByPri))
		for p, n := range terrainByPri {
			for m := 1; m < 16; m++ {
				out[p][m] = "terrain/" + terrainSprite[n] + "_edge_" + twoDigits(m)
			}
		}
		return out
	}()
)

func (r *Renderer) terrainName(w game.WorldView, x, y int) string {
	c, ok := terrainCode(w, x, y)
	if !ok || int(c) >= len(terrainNames) {
		return ""
	}
	return terrainNames[c]
}

// terrainCode is the read model's terrain code at (x, y).
func terrainCode(w game.WorldView, x, y int) (uint8, bool) {
	if w.TerrainW <= 0 || x < 0 || y < 0 || x >= w.TerrainW {
		return 0, false
	}
	i := y*w.TerrainW + x
	if i >= len(w.Terrain) {
		return 0, false
	}
	return w.Terrain[i], true
}

// tileKeyOf is the cached ground tile and colour scale of a terrain code
// and variant (terrainTileKey without the per-tile string building).
func (r *Renderer) tileKeyOf(code uint8, v int) (string, colorScale) {
	c := r.stoneCache()
	if c.tiles == nil {
		c.tiles = make([][soilVariants]tileKeyEntry, len(terrainNames))
	}
	if int(code) >= len(c.tiles) {
		return r.terrainTileKey("", v)
	}
	e := &c.tiles[code][v]
	if !e.done {
		e.key, e.cs = r.terrainTileKey(terrainNames[code], v)
		e.done = true
	}
	return e.key, e.cs
}

// drawTerrainArt draws the ground of the visible tiles (padded by one):
// the tile, the soft edges of higher-priority neighbours; then, over the
// whole view's ground, the grass tufts (so a neighbour's ground never
// paints over a tuft). Stone ground (rough, smooth, under rock) takes its
// stone's tint.
func (r *Renderer) drawTerrainArt(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	x0, y0, x1, y1 := r.visibleTiles()
	x0, y0 = max(x0-1, 0), max(y0-1, 0)
	x1, y1 = min(x1+1, r.game.Width()-1), min(y1+1, r.game.Height()-1)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			r.drawGroundTile(screen, &w, x, y, tile, sw, sh)
		}
	}
	r.drawFurrows(screen, w, tile)            // crop_batch.go: tilled ground under crops
	r.drawPuddleVeil(screen, w, sw, sh, tile) // puddle_veil.go (PLAY-022): under the tufts
	r.drawTufts(screen, w, sw, sh, tile)
}

// drawTufts draws the visible grass over the ground (grass_batch.go): the
// tuft batch, every copy from lodLiveMin and at most grassMidCap from
// grassLiveMin (below that the chunk images hold the tufts), then the
// cut marks.
func (r *Renderer) drawTufts(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	vx0, vy0, vx1, vy1 := r.visibleTiles()
	x0, y0, x1, y1 := r.grassReach(vx0, vy0, vx1, vy1)
	r.grassFrameMask(&w, x0, y0, x1, y1) // grass_draw.go
	r.lod.grass.quads = 0
	if tile >= grassLiveMin {
		limit := 9
		if tile < lodLiveMin {
			limit = grassMidCap
		}
		r.grassBatchOf(screen, &w, x0, y0, x1, y1, tile, sw, sh, limit)
	}
	r.drawGrassCutMarks(screen, &w, vx0, vy0, vx1, vy1, tile, sw, sh)
}

// drawGroundTile draws tile (x, y)'s ground: its terrain tile (water
// through water_draw.go), the soft edges of higher-priority neighbours
// and water edges. Stone ground (rough, smooth, under rock) takes its
// stone's tint.
func (r *Renderer) drawGroundTile(screen *ebiten.Image, w *game.WorldView, x, y int, tile float64, sw, sh int) {
	none := color.NRGBA{}
	code, ok := terrainCode(*w, x, y)
	if !ok || int(code) >= len(terrainNames) {
		code, ok = 0, false
	}
	key, tcs := r.tileKeyOf(code, int(tileHash(x, y)%soilVariants))
	if !ok {
		key, tcs = r.terrainTileKey("soil", int(tileHash(x, y)%soilVariants))
	} else if stoneGroundCode[code] {
		tcs = mulCS(tcs, tintCS(r.stoneTintAt(*w, x, y)))
	}
	if ok && isWaterCode[code] {
		r.lit.Calls++
		r.drawWaterTile(screen, terrainNames[code], x, y, tile, sw, sh, tcs) // water_draw.go
	} else {
		r.drawTile(screen, key, float64(x), float64(y), tile, sw, sh, tcs)
	}
	pri := int(terrainPriByCode[code])
	var nbPri [4]int // N, E, S, W; -100 off the map
	higher := false
	for i, n := range [...]struct{ dx, dy int }{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
		nbPri[i] = -100
		if nc, ok := terrainCode(*w, x+n.dx, y+n.dy); ok {
			nbPri[i] = int(terrainPriByCode[nc])
			higher = higher || nbPri[i] > pri
		}
	}
	for p := max(pri+1, 0); higher && p < len(terrainByPri); p++ {
		mask, sx, sy := 0, -1, -1
		for i, n := range [...]struct{ dx, dy, bit int }{{0, -1, wallN}, {1, 0, wallE}, {0, 1, wallS}, {-1, 0, wallW}} {
			if nbPri[i] == p {
				mask |= n.bit
				sx, sy = x+n.dx, y+n.dy
			}
		}
		if mask == 0 || p == 0 { // soil has no edge art: it is the plain ground
			continue
		}
		ek, ecs := terrainEdgeKeys[p][mask], noCS
		if !r.art.Has(ek) && terrainByPri[p] == "stone_smooth" { // older art: the rough edge
			ek = terrainEdgeKeys[terrainPriority["stone"]][mask]
		}
		if n := terrainByPri[p]; n == "stone" || n == "stone_smooth" { // stone edges take the neighbouring stone's tint
			ecs = tintCS(r.stoneTintAt(*w, sx, sy))
		}
		r.drawSpriteCS(screen, ek, none, float64(x), float64(y), tile, sw, sh, ecs)
	}
	if ok && isWaterCode[code] {
		r.drawWaterEdges(screen, terrainNames[code], nbPri, x, y, tile, sw, sh) // water_draw.go
	}
}

// addRockDraws queues the natural rock walls in view, padded one column
// each side, one row north and two south (tall sprites reach north):
// linked (linkgrid.go) among rock and walls, tinted by their stone, ore
// cells with their vein sprite.
func (r *Renderer) addRockDraws(w game.WorldView, draws []artDraw) []artDraw {
	if w.TerrainW <= 0 {
		return draws
	}
	x0, y0, x1, y1 := r.visibleTiles()
	x0, y0 = max(x0-1, 0), max(y0-1, 0)
	x1, y1 = min(x1+1, r.game.Width()-1, w.TerrainW-1), min(y1+2, r.game.Height()-1)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			i := y*w.TerrainW + x
			if i >= len(w.Terrain) || !isRockWallCode(w.Terrain[i]) {
				continue
			}
			if ld, ok := r.rockDraw(&w, x, y); ok {
				draws = append(draws, artDraw{row: float64(y), col: float64(x), ld: ld})
			}
		}
	}
	return draws
}

// rockDraw is the linked sprite of the rock wall at (x, y): its stone's
// tint, its ore vein's sprite.
func (r *Renderer) rockDraw(w *game.WorldView, x, y int) (linkedDraw, bool) {
	i := y*w.TerrainW + x
	ore := ""
	if i < len(w.Ore) && int(w.Ore[i]) < len(oreNames) {
		ore = oreNames[w.Ore[i]]
	}
	var stone uint8
	if i < len(w.StoneType) {
		stone = w.StoneType[i]
	}
	prefix := rockPrefix(terrainNames[w.Terrain[i]], ore)
	return r.linkedSprite(prefix, x, y, r.links.mask(x, y), r.links.corners(x, y), r.stoneTint(stone), noCS)
}

// smoothMarkKey is the smoothing designation; older art falls back to the
// mine mark cooled blue.
const smoothMarkKey = "overlay/designation_smooth"

// haulMarkKey is the designation drawn over a stack marked for hauling.
const haulMarkKey = "overlay/designation_haul"

// drawHaulMarks draws the haul designation over every marked stack of the
// chunks in view, on top of the objects like the other designations.
func (r *Renderer) drawHaulMarks(screen *ebiten.Image, w *game.WorldView, views []game.ChunkView, tile float64, sw, sh int) {
	for _, cv := range views {
		for _, i := range cv.Items {
			if int(i) >= len(w.Items) {
				continue
			}
			if it := &w.Items[i]; it.HaulMarked && !it.Carried && r.lod.near(it.X, it.Y, 1, 1) {
				r.drawSprite(screen, haulMarkKey, color.NRGBA{}, float64(it.X), float64(it.Y), tile, sw, sh)
			}
		}
	}
}

var smoothFallbackCS = colorScale{0.65, 0.9, 1.35, 1}

// stoneItem is the key and colour scale of a ground stack: chunks and
// blocks take their stone's tint; a chunk without art draws as blocks and
// precious metals without art as steel bars.
func (r *Renderer) stoneItem(key, kind string, cs colorScale) (string, colorScale) {
	if fb, ok := itemFallback[key]; ok && !r.art.Has(key) {
		key = fb
	}
	stone := ""
	switch {
	case kind == "stone":
		stone = defaultStone
	case strings.HasSuffix(kind, " chunk"), strings.HasSuffix(kind, " blocks"):
		stone = kind[:strings.LastIndexByte(kind, ' ')]
	}
	if stone == "" {
		return key, cs
	}
	return key, mulCS(tintCS(r.stuffTint(stone+" blocks")), cs)
}

var itemFallback = map[string]string{
	"item/stone_chunk": "item/stone_blocks",
	"item/silver":      "item/steel_bars",
	"item/gold":        "item/steel_bars",
}

// plantSprite picks the sprite and growth scale for a plant: tree
// species, bush and crop variants by tileHash, crops by drawn stage
// (ART §9, §28). Keys come from a prebuilt table; seasonKey then picks the
// seasonal look and falls back to older art.
func plantSprite(p game.PlantView) (string, float64) {
	h := tileHash(p.X, p.Y)
	if p.Species == "healroot" && p.Zone == "" {
		k, sc, _ := newPlantKey(p, h)
		return k, sc
	}
	if ks, ok := plantKeys.crops[p.Species]; ok {
		return ks[cropStage(p)][h%cropVariants], 1 // the inspector's icon and the shadow; drawCropObj composes the units
	}
	if k, sc, ok := newPlantKey(p, h); ok {
		return k, sc
	}
	switch p.Species {
	case "tree":
		k := 0.45 + 0.55*float64(p.Growth)/100
		return plantKeys.trees[treeSpeciesAt(h)][h%3], k
	default:
		return groundPick(plantKeys.ground["bush"], p, h), bushScale(p)
	}
}

func bushScale(p game.PlantView) float64 {
	return min(0.6+0.4*float64(p.Growth)/100, 1)
}

func clampStage(s int) int {
	if s < 0 {
		return 0
	}
	if s > 3 {
		return 3
	}
	return s
}

// cleanlinessNote is a room's dirt for its label: empty while it is clean,
// else ", dirty (-3)" with the impressiveness it costs.
func cleanlinessNote(rm game.RoomView) string {
	if rm.Cleanliness == "" || rm.Cleanliness == "clean" {
		return ""
	}
	if rm.FilthPenalty > 0 {
		return ", " + rm.Cleanliness + " (-" + strconv.Itoa(rm.FilthPenalty) + ")"
	}
	return ", " + rm.Cleanliness
}

// onFloor reports a building kind drawn flat under pawns and furniture
// (the floor-marking, foundation or conduit pass), not in the object pass.
func onFloor(kind string) bool {
	return kind == "sleeping spot" || kind == "carpet" || kind == bridgeKind || // a bridge: the foundation pass
		isConduitKind(kind) // conduits: the conduit pass (power_draw.go)
}

// drawFloorLayers draws the ground under zones: built floors, then the
// tracked-in dirt (dirt_draw.go, ART §16.3), then the blood and ichor
// stains (blood_draw.go, ART §16.2), so blood always lies over dirt. The
// dirt and stains layers are traced only when there is some.
func (r *Renderer) drawFloorLayers(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.layer("floors")
	if r.lod.tier == lodLive { // baked below 48 px (lod.go)
		for _, cv := range r.lod.views {
			for _, i := range cv.Buildings {
				if int(i) >= len(w.Buildings) {
					continue
				}
				if b := &w.Buildings[i]; b.Floor {
					r.drawTile(screen, r.buildKey("floor", b.Stuff, "", 0, false, false), float64(b.X), float64(b.Y), tile, sw, sh, tintCS(r.stuffTint(b.Stuff)))
				}
			}
		}
	}
	if r.lod.tier == lodMap { // no decals on the whole-map bake
		return
	}
	if len(w.Dirt) > 0 {
		r.layer("dirt")
		r.drawDirt(screen, w, sw, sh, tile)
	}
	if len(w.Stains) > 0 {
		r.layer("stains")
		r.drawStains(screen, w, sw, sh, tile)
		r.drawAshStains(screen, w, sw, sh, tile) // fire_world_draw.go
	}
}

// roomLabelAt describes the room at a tile for HUD/inspectors.
func roomLabelAt(w game.WorldView, x, y int) string {
	if w.TerrainW <= 0 || x < 0 || y < 0 || x >= w.TerrainW {
		return ""
	}
	i := y*w.TerrainW + x
	if i >= len(w.RoomGrid) {
		return ""
	}
	id := w.RoomGrid[i]
	if id < 0 || id >= len(w.Rooms) {
		if id == -1 {
			return "Outdoors"
		}
		return ""
	}
	rm := w.Rooms[id]
	s := "Room: " + rm.Role + ", " + rm.Label + " (" + strconv.Itoa(rm.Score) + ")" + cleanlinessNote(rm)
	if rm.Lit {
		s += ", lit"
	}
	return s + ", roofed " + strconv.Itoa(int(math.Round(rm.Roofed*100))) + "%"
}
