package ebiten

import (
	"cmp"
	"image/color"
	"math"
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

// artDraw is one queued object draw: a closure, or (fn nil) a linked
// sprite, which mountains queue by the hundred without allocating.
type artDraw struct {
	row, col float64
	fn       func()
	ld       linkedDraw
}

// colorScale is a per-draw RGBA multiplier (ghosts, blueprints).
type colorScale [4]float32

var (
	noCS        = colorScale{1, 1, 1, 1}
	blueprintCS = colorScale{0.55, 0.85, 1.25, 0.5}
	ghostOKCS   = colorScale{0.7, 1.25, 0.7, 0.65}
	ghostBadCS  = colorScale{1.3, 0.45, 0.4, 0.6}
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
	sp := r.art.sprite(key)
	if sp == nil {
		return
	}
	sx, sy := r.worldToScreenF(wx, wy)
	x, y, s := artOrigin(sp.meta, float64(sx), float64(sy), tile)
	if k != 1 {
		px := float64(sx) + float64(sp.meta.Footprint[0])*tile/2
		py := float64(sy) + float64(sp.meta.Footprint[1])*tile
		x, y, s = px+(x-px)*k, py+(y-py)*k, s*k
	}
	w, h := float64(sp.meta.Canvas[0])*s, float64(sp.meta.Canvas[1])*s
	if !visible(float32(x), float32(y), float32(w), float32(h), sw, sh) {
		return
	}
	img := r.art.image(key, tint)
	if img == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(x, y)
	op.Filter = ebiten.FilterLinear
	if !strings.HasPrefix(key, "overlay/") { // zones and markers stay unlit
		r.drawLit(screen, img, op.GeoM, cs, int(math.Floor(wx)), int(math.Floor(wy)))
		return
	}
	if cs != noCS {
		op.ColorScale.Scale(cs[0]*cs[3], cs[1]*cs[3], cs[2]*cs[3], cs[3])
	}
	screen.DrawImage(img, op)
}

// drawPlacementGhost previews a build tool: the real sprite per cell,
// green when placeable, red when not. Walls autotile among the ghosts.
func (r *Renderer) drawPlacementGhost(screen *ebiten.Image, kind, stuff, facing string, cells []ghostCell) {
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	tile := r.tilePx()
	link := buildingLink(kind)
	var p map[[2]int]linkFlag
	if r.art != nil && link.flags != 0 { // ghosts join built walls, rock, frames and each other
		w := r.drawW
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
		if !c.ok {
			cs = ghostBadCS
		}
		if r.art == nil {
			sx, sy := r.worldToScreen(c.x, c.y)
			clr := color.RGBA{90, 200, 90, 120}
			if !c.ok {
				clr = color.RGBA{220, 70, 60, 120}
			}
			vector.FillRect(screen, sx, sy, float32(tile), float32(tile), clr, false)
			continue
		}
		mask, corners := 0, 0
		if p != nil {
			mask = r.links.plannedMask(p, link.accepts, c.x, c.y)
			corners = r.links.plannedCorners(p, link.accepts, c.x, c.y)
		}
		if kind == "wall" && r.drawLinked(screen, wallPrefix(stuff), c.x, c.y, mask, corners, tint, cs, tile, sw, sh) {
			continue
		}
		key := r.buildKey(kind, stuff, facing, mask, mask&(wallE|wallW) != 0, false)
		if !r.art.has(key) {
			sx, sy := r.worldToScreen(c.x, c.y)
			if isFenceKind(kind) {
				drawFenceVector(screen, sx, sy, float32(tile), kind, mask, false, isStoneStuff(stuff), 170)
				continue
			}
			clr := color.RGBA{90, 200, 90, 120}
			if !c.ok {
				clr = color.RGBA{220, 70, 60, 120}
			}
			vector.FillRect(screen, sx, sy, float32(tile), float32(tile), clr, false)
			continue
		}
		r.drawSpriteCS(screen, key, color.NRGBA{}, float64(c.x), float64(c.y), tile, sw, sh, mulCS(tintCS(tint), cs))
	}
}

// buildKey is buildSpriteStuff with a fallback to the wood sprite when a
// material's art (steel doors, stone fences) is missing.
func (r *Renderer) buildKey(kind, stuff, facing string, mask int, ew, open bool) string {
	key := buildSpriteStuff(kind, stuff, facing, mask, ew, open)
	if r.art != nil && stuffMaterial(stuff) != "wood" && !r.art.has(key) {
		if alt := buildSprite(kind, facing, mask, ew, open); alt != "" && r.art.has(alt) {
			return alt
		}
	}
	return key
}

// layer marks the start of a ground layer for the draw-order test.
func (r *Renderer) layer(name string) {
	if r.layerTrace != nil {
		r.layerTrace(name)
	}
}

func (r *Renderer) drawArt(screen *ebiten.Image, sw, sh int, tile float64) {
	none := color.NRGBA{}
	w := r.game.World()
	r.drawW, r.drawPawns = w, r.game.Snapshot()
	r.lit.light = lightAt(w.Hour, w.DayOfYear, w.Weather, w.WeatherIntensity)
	r.setView(sw, sh, tile)
	r.links.sync(w, r.game.Width(), r.game.Height()) // linkgrid.go: rebuilt only on StructVer

	r.drawTerrainArt(screen, w, sw, sh, tile)
	r.drawGroundCover(screen, w, sw, sh, tile)
	r.drawFloorLayers(screen, w, sw, sh, tile)
	r.layer("zones")
	r.drawZones(screen, w, sw, sh, tile) // zones_draw.go
	for _, f := range w.Frames {
		r.drawSprite(screen, "overlay/blueprint", none, float64(f.X), float64(f.Y), tile, sw, sh)
	}

	for _, b := range w.Buildings { // floor markings and carpets lie on the floor
		switch b.Kind {
		case "sleeping spot":
			r.drawSprite(screen, buildSprite(b.Kind, b.Facing, 0, false, false), none, float64(b.X), float64(b.Y), tile, sw, sh)
		case "carpet":
			r.drawSprite(screen, buildSpriteStuff(b.Kind, b.Stuff, "", 0, false, false), none, float64(b.X), float64(b.Y), tile, sw, sh)
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

	r.drawShadows(screen, w, sw, sh, tile) // shadow.go: under every object
	r.drawEaves(screen, w, sw, sh, tile)   // roof_draw.go

	occupied := map[[2]int]bool{}
	for _, p := range r.game.Snapshot() {
		occupied[[2]int{p.X, p.Y}] = true
	}
	creatures := creatureCells(w, r.drawPawns)
	r.drawPaths(screen, sw, sh, tile) // paths_draw.go
	draws := r.draws[:0]
	add := func(row, col float64, fn func()) { draws = append(draws, artDraw{row: row, col: col, fn: fn}) }
	draws = r.addRockDraws(w, draws)
	draws = r.addCaveMouths(w, draws, screen, tile, sw, sh) // ng1_art.go
	cloth := r.colonyCloths()
	for _, b := range w.Buildings {
		b := b
		if b.Floor || onFloor(b.Kind) {
			continue
		}
		mask, corners := 0, 0
		open := occupied[[2]int{b.X, b.Y}]
		switch {
		case b.Kind == "wall":
			mask, corners = r.links.mask(b.X, b.Y), r.links.corners(b.X, b.Y)
		case b.Kind == "door":
			mask = r.links.mask(b.X, b.Y)
		case isFenceKind(b.Kind):
			mask, open = r.links.mask(b.X, b.Y), creatures[[2]int{b.X, b.Y}]
		}
		key := r.buildKey(b.Kind, b.Stuff, b.Facing, mask, mask&(wallE|wallW) != 0, open)
		if b.Kind == troughKind {
			key = troughKey(b)
		}
		stuffCS := tintCS(r.stuffTint(b.Stuff)) // stone walls, fences and floors take their stone's tint
		wallDraw, linked := linkedDraw{}, false
		if b.Kind == "wall" {
			wallDraw, linked = r.linkedSprite(wallPrefix(b.Stuff), b.X, b.Y, mask, corners, r.stuffTint(b.Stuff), noCS)
		}
		t := none
		if (b.Kind == "bed" || b.Kind == "double bed") && len(cloth) > 0 {
			t = r.art.tint("cloth", cloth[mix64(uint64(b.ID))%uint64(len(cloth))])
		}
		add(float64(b.Y+max(b.H, 1)-1), float64(b.X), func() {
			switch b.Kind {
			case "grave":
				r.drawGrave(screen, b, tile, sw, sh)
			case "funeral pyre":
				r.drawPyre(screen, b, tile, sw, sh)
			default:
				if linked {
					r.drawLinkedSprite(screen, wallDraw, tile, sw, sh)
					break
				}
				if b.Kind == troughKind && !r.art.has(key) {
					r.drawTroughFallback(screen, b, tile)
					break
				}
				if isFenceKind(b.Kind) && !r.art.has(key) {
					sx, sy := r.worldToScreen(b.X, b.Y)
					drawFenceVector(screen, sx, sy, float32(tile), b.Kind, mask, open, isStoneStuff(b.Stuff), 255)
					break
				}
				r.drawSpriteCS(screen, key, t, float64(b.X), float64(b.Y), tile, sw, sh, stuffCS)
			}
			if b.DryQty > 0 {
				r.drawRackLoad(screen, b, tile, sw, sh)
			}
			if b.Deconstruct {
				r.drawSprite(screen, "overlay/designation_deconstruct", none, float64(b.X), float64(b.Y), tile, sw, sh)
			}
		})
	}
	// planned walls and fences autotile with each other, built walls and
	// rock (an O(frames) planned set, linkgrid.go)
	planned := r.links.plan(w.Frames, nil)
	for _, f := range w.Frames {
		f := f
		if f.Floor || f.Kind == "carpet" {
			continue
		}
		pm, pc := 0, 0
		if c := buildingLink(f.Kind); c.flags != 0 {
			pm = r.links.plannedMask(planned, c.accepts, f.X, f.Y)
			if f.Kind == "wall" {
				pc = r.links.plannedCorners(planned, c.accepts, f.X, f.Y)
			}
		}
		key := r.buildKey(f.Kind, f.Stuff, f.Facing, pm, pm&(wallE|wallW) != 0, false)
		tint := r.stuffTint(f.Stuff)
		wallDraw, linked := linkedDraw{}, false
		if f.Kind == "wall" {
			wallDraw, linked = r.linkedSprite(wallPrefix(f.Stuff), f.X, f.Y, pm, pc, tint, blueprintCS)
		}
		add(float64(f.Y+max(f.H, 1)-1), float64(f.X), func() {
			if f.Wood > 0 {
				r.drawSprite(screen, "structure/frame_1x1", none, float64(f.X), float64(f.Y), tile, sw, sh)
			}
			if linked {
				r.drawLinkedSprite(screen, wallDraw, tile, sw, sh)
				return
			}
			if isFenceKind(f.Kind) && !r.art.has(key) {
				sx, sy := r.worldToScreen(f.X, f.Y)
				drawFenceVector(screen, sx, sy, float32(tile), f.Kind, pm, false, isStoneStuff(f.Stuff), 140)
				return
			}
			r.drawSpriteCS(screen, key, none, float64(f.X), float64(f.Y), tile, sw, sh, mulCS(tintCS(tint), blueprintCS))
		})
	}
	for _, p := range w.Plants {
		p := p
		key, scale := plantSprite(p)
		key = r.seasonKey(key, w)
		amount, kind := 0.035, kindBush
		switch p.Species {
		case "tree", "pine", "jungle_tree":
			amount, kind = 0.022, kindTree
		case "wheat", "corn":
			amount, kind = 0.07, kindCrop
		case "radish", "lettuce", "potato", "beans", "pumpkin", "healroot", "cotton":
			amount, kind = 0.04, kindCrop
		}
		add(float64(p.Y), float64(p.X), func() {
			ps := r.plantPose(p.X, p.Y, kind, amount, scale)
			r.drawSpritePose(screen, key, none, float64(p.X), float64(p.Y), tile, ps)
		})
	}
	for _, it := range w.Items {
		it := it
		if it.Carried {
			continue
		}
		key := sizedItemKey(itemSpriteSpecies(it.Kind, it.Animal, it.X, it.Y), it.Qty)
		if it.Forbidden {
			add(float64(it.Y)+0.001, float64(it.X), func() { r.drawForbiddenMark(screen, it.X, it.Y, tile, sw, sh) })
		}
		add(float64(it.Y), float64(it.X), func() {
			if it.Kind == "human corpse" || it.Kind == "bones" {
				r.drawCorpseItem(screen, it, tile, sw, sh)
				return
			}
			cs := itemCS(it)
			if key, cs = r.stoneItem(key, it.Kind, cs); !r.art.has(key) {
				pad := float32(tile) * 0.3
				sx, sy := r.worldToScreen(it.X, it.Y)
				vector.FillRect(screen, sx+pad, sy+pad, float32(tile)-2*pad, float32(tile)-2*pad, color.RGBA{200, 90, 40, 255}, false)
				return
			}
			r.drawSpriteScaled(screen, key, none, float64(it.X), float64(it.Y), tile, sw, sh, cs, r.itemPop(it.ID))
		})
	}
	r.addAnimalDraws(w, add, screen, tile, sw, sh)
	for _, p := range r.drawPawns {
		p := p
		if p.Embodied {
			continue // the body is a corpse item now
		}
		wx, wy := r.interpWorld(p)
		row := wy + 0.01
		if p.Lying && p.InBed {
			if b, ok := bedUnder(w, p.X, p.Y); ok {
				row = float64(b.Y+max(b.H, 1)-1) + 0.02 // over its bed
			}
		}
		add(row, wx, func() { r.drawPawnArt(screen, p, wx, wy, tile, sw, sh) })
	}
	slices.SortStableFunc(draws, func(a, b artDraw) int {
		if c := cmp.Compare(a.row, b.row); c != 0 {
			return c
		}
		return cmp.Compare(a.col, b.col)
	})
	for i := range draws {
		if d := &draws[i]; d.fn != nil {
			d.fn()
		} else {
			r.drawLinkedSprite(screen, d.ld, tile, sw, sh)
		}
	}
	clear(draws) // drop this frame's closures; keep the buffer
	r.draws = draws[:0]
	r.drawWorkGlow(screen, tile)
	r.drawFires(screen, w, tile)
	r.drawStackCounts(screen, w, tile)
	r.drawBuildingDamage(screen, w, tile)
	r.drawChatBubbles(screen, tile)
	r.drawBreakClouds(screen, tile)
	r.drawOrderRoute(screen, tile)
	r.drawParticles(screen, tile)
	r.drawArrivals(screen, w, tile) // arrival_draw.go
	r.drawWeather(screen, w, sw, sh, tile)
	r.drawFloaters(screen, tile)

	for _, p := range w.Plants {
		switch {
		case p.Cut:
			r.drawSprite(screen, "overlay/designation_cut", none, float64(p.X), float64(p.Y), tile, sw, sh)
		case p.Designated:
			r.drawSprite(screen, "overlay/designation_harvest", none, float64(p.X), float64(p.Y), tile, sw, sh)
		}
	}
	for _, m := range w.Mines {
		r.drawSprite(screen, "overlay/designation_mine", none, float64(m[0]), float64(m[1]), tile, sw, sh)
	}
	smoothKey, smoothCS := smoothMarkKey, noCS
	if !r.art.has(smoothKey) { // older art: the mine mark, cooled blue
		smoothKey, smoothCS = "overlay/designation_mine", smoothFallbackCS
	}
	for _, m := range w.Smooths {
		r.drawSpriteCS(screen, smoothKey, none, float64(m[0]), float64(m[1]), tile, sw, sh, smoothCS)
	}
	r.drawAnimalOverlays(screen, w, tile, sw, sh)
	if r.hasSelect {
		for _, p := range r.game.Snapshot() {
			if p.ID == r.selected {
				wx, wy := r.interpWorld(p)
				k := 1 + 0.045*math.Sin(float64(r.present)/7)
				r.drawSpritePose(screen, "overlay/selection", none, wx, wy, tile, pose{px: 0.5, py: 0.5, sx: k, sy: k})
			}
		}
	}
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
	return "item/" + kind
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

// look returns the stable appearance for a pawn.
func (r *Renderer) look(p game.PawnView) pawnLook {
	l := lookFor(uint64(p.ID), r.art.tintNames("skin"), r.art.tintNames("hair"), r.colonyCloths())
	if p.Hostile { // raiders wear dark maroon
		l.Shirt, l.Vest, l.Pants = raiderCloth, raiderCloth, "slate"
	}
	return l
}

// colonyCloths lists the cloth tints colonists and their beds may use; the
// raider tint is held back so hostiles stand out.
func (r *Renderer) colonyCloths() []string {
	var cloths []string
	for _, c := range r.art.tintNames("cloth") {
		if c != raiderCloth {
			cloths = append(cloths, c)
		}
	}
	return cloths
}

// raiderCloth is the tint reserved for raider clothing.
const raiderCloth = "raider"

// pawnLayers lists (sprite key, tint) in draw order for one facing.
func (r *Renderer) pawnLayers(l pawnLook, f facing, apparel ...string) []struct {
	key  string
	tint color.NRGBA
} {
	fs := f.String()
	out := []struct {
		key  string
		tint color.NRGBA
	}{
		{"pawn/body_" + fs, r.art.tint("skin", l.Skin)},
		{"pawn/pants_basic_" + fs, r.art.tint("cloth", l.Pants)},
		{"pawn/shirt_basic_" + fs, r.art.tint("cloth", l.Shirt)},
	}
	if l.Vest != "" {
		out = append(out, struct {
			key  string
			tint color.NRGBA
		}{"pawn/vest_utility_" + fs, r.art.tint("cloth", l.Vest)})
	}
	type layer = struct {
		key  string
		tint color.NRGBA
	}
	hat := ""
	for _, a := range apparel { // outer garments go over shirt and vest
		switch a {
		case "parka":
			out = append(out, layer{"pawn/parka_leather_" + fs, color.NRGBA{}})
		case "wool parka": // the leather parka layer, tinted cream
			out = append(out, layer{"pawn/parka_leather_" + fs, color.NRGBA{R: 238, G: 228, B: 200, A: 255}})
		case "duster":
			out = append(out, layer{"pawn/duster_leather_" + fs, color.NRGBA{}})
		case "tuque":
			hat = "pawn/tuque_knit_" + fs
		}
	}
	out = append(out, layer{"pawn/" + l.HairStyle + "_" + fs, r.art.tint("hair", l.Hair)})
	if hat != "" {
		out = append(out, layer{hat, r.art.tint("cloth", l.Pants)})
	}
	return out
}

// drawPawnPortrait draws the layered pawn (facing south) fitted into a
// w x h box for HUD chrome.
func (r *Renderer) drawPawnPortraitArt(screen *ebiten.Image, p game.PawnView, x, y, w, h float64) bool {
	body := r.art.sprite("pawn/body_south")
	if body == nil {
		return false
	}
	cw, ch := float64(body.meta.Canvas[0]), float64(body.meta.Canvas[1])
	s := math.Min(w/cw, h/ch)
	ox := x + (w-cw*s)/2
	oy := y + (h-ch*s)/2
	for _, ly := range r.pawnLayers(r.look(p), faceSouth, p.Apparel...) {
		img := r.art.image(ly.key, ly.tint)
		if img == nil {
			continue
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(s, s)
		op.GeoM.Translate(ox, oy)
		op.Filter = ebiten.FilterLinear
		screen.DrawImage(img, op)
	}
	return true
}

// drawTile draws an opaque, tileable 1x1 sprite snapped to whole pixels
// with a 1 px overlap, so fractional zoom never shows seams between tiles.
func (r *Renderer) drawTile(screen *ebiten.Image, key string, wx, wy, tile float64, sw, sh int, cs colorScale) {
	sx, sy := r.worldToScreenF(wx, wy)
	x0, y0 := math.Floor(float64(sx)), math.Floor(float64(sy))
	size := math.Ceil(float64(sx)+tile) - x0 + 1
	if !visible(float32(x0), float32(y0), float32(size), float32(size), sw, sh) {
		return
	}
	img := r.art.image(key, color.NRGBA{})
	if img == nil {
		return
	}
	k := size / float64(img.Bounds().Dx())
	geo := ebiten.GeoM{}
	geo.Scale(k, k)
	geo.Translate(x0, y0)
	r.drawLit(screen, img, geo, cs, int(math.Floor(wx)), int(math.Floor(wy)))
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
// the tile, the soft edges of higher-priority neighbours, and grass.
// Stone ground (rough, smooth, under rock) takes its stone's tint.
func (r *Renderer) drawTerrainArt(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	none := color.NRGBA{}
	grassCS := grassTint(w.BiomeKey)
	grassScratch = grassMask(w, grassScratch) // grass_draw.go
	x0, y0, x1, y1 := r.visibleTiles()
	x0, y0 = max(x0-1, 0), max(y0-1, 0)
	x1, y1 = min(x1+1, r.game.Width()-1), min(y1+1, r.game.Height()-1)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			code, ok := terrainCode(w, x, y)
			t := "soil"
			if ok && int(code) < len(terrainNames) {
				t = terrainNames[code]
			} else {
				code, ok = 0, false
			}
			key, tcs := r.tileKeyOf(code, int(tileHash(x, y)%soilVariants))
			if !ok {
				key, tcs = r.terrainTileKey("soil", int(tileHash(x, y)%soilVariants))
			} else if stoneGroundCode[code] {
				tcs = mulCS(tcs, tintCS(r.stoneTintAt(w, x, y)))
			}
			if ok && isWaterCode[code] {
				r.drawWaterTile(screen, terrainNames[code], x, y, tile, sw, sh, tcs) // water_draw.go
			} else {
				r.drawTile(screen, key, float64(x), float64(y), tile, sw, sh, tcs)
			}
			pri := int(terrainPriByCode[code])
			var nbPri [4]int // N, E, S, W; -100 off the map
			higher := false
			for i, n := range [...]struct{ dx, dy int }{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
				nbPri[i] = -100
				if nc, ok := terrainCode(w, x+n.dx, y+n.dy); ok {
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
				if !r.art.has(ek) && terrainByPri[p] == "stone_smooth" { // older art: the rough edge
					ek = terrainEdgeKeys[terrainPriority["stone"]][mask]
				}
				if n := terrainByPri[p]; n == "stone" || n == "stone_smooth" { // stone edges take the neighbouring stone's tint
					ecs = tintCS(r.stoneTintAt(w, sx, sy))
				}
				r.drawSpriteCS(screen, ek, none, float64(x), float64(y), tile, sw, sh, ecs)
			}
			if ok && isWaterCode[code] {
				r.drawWaterEdges(screen, terrainNames[code], nbPri, x, y, tile, sw, sh) // water_draw.go
			}
			if grassVisible(grassScratch, w, t, x, y) {
				gp := r.plantPose(x, y, kindGrass, 0.05, 1)
				gp.cs = grassCS
				r.drawSpritePose(screen, grassKey(t, tileHash(x, y)), none, float64(x), float64(y), tile, gp)
				if grassScratch[y*w.TerrainW+x]&grassMarked != 0 {
					r.drawSprite(screen, "overlay/designation_cut", none, float64(x), float64(y), tile, sw, sh)
				}
			}
		}
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
			ore := ""
			if i < len(w.Ore) && int(w.Ore[i]) < len(oreNames) {
				ore = oreNames[w.Ore[i]]
			}
			var stone uint8
			if i < len(w.StoneType) {
				stone = w.StoneType[i]
			}
			prefix := rockPrefix(terrainNames[w.Terrain[i]], ore)
			ld, ok := r.linkedSprite(prefix, x, y, r.links.mask(x, y), r.links.corners(x, y), r.stoneTint(stone), noCS)
			if ok {
				draws = append(draws, artDraw{row: float64(y), col: float64(x), ld: ld})
			}
		}
	}
	return draws
}

// smoothMarkKey is the smoothing designation; older art falls back to the
// mine mark cooled blue.
const smoothMarkKey = "overlay/designation_smooth"

var smoothFallbackCS = colorScale{0.65, 0.9, 1.35, 1}

// stoneItem is the key and colour scale of a ground stack: chunks and
// blocks take their stone's tint; a chunk without art draws as blocks and
// precious metals without art as steel bars.
func (r *Renderer) stoneItem(key, kind string, cs colorScale) (string, colorScale) {
	if fb, ok := itemFallback[key]; ok && !r.art.has(key) {
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

// grassKeys are the tuft sprites: short tufts (some flowering) on soil,
// tall meadow grass mixed in on rich soil (ART §9).
var (
	grassKeys     = keysOf("plant/grass_", 8)
	tallGrassKeys = keysOf("plant/grass_tall_", 4)
)

func keysOf(prefix string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = prefix + twoDigits(i)
	}
	return out
}

// grassKey picks a tile's tuft by its hash; rich soil grows tall grass
// on half its tufts.
func grassKey(terrain string, h uint64) string {
	h >>= 12
	if terrain == "rich_soil" && h%2 == 0 {
		return tallGrassKeys[(h/2)%uint64(len(tallGrassKeys))]
	}
	return grassKeys[h%uint64(len(grassKeys))]
}

// plantSprite picks the sprite and growth scale for a plant: tree
// species, bush and crop variants by tileHash, crops by drawn stage
// (ART §9, §28). Keys come from a prebuilt table; seasonKey then picks the
// seasonal look and falls back to older art.
func plantSprite(p game.PlantView) (string, float64) {
	h := tileHash(p.X, p.Y)
	if ks, ok := plantKeys.crops[p.Species]; ok {
		return ks[cropStage(p)][h%3], 1
	}
	if k, sc, ok := newPlantKey(p, h); ok {
		return k, sc
	}
	switch p.Species {
	case "tree":
		k := 0.45 + 0.55*float64(p.Growth)/100
		return plantKeys.trees[treeSpeciesAt(h)][h%3], k
	case "berry_bush":
		return plantKeys.berry[h%3][boolIdx(p.Harvestable)], bushScale(p)
	default:
		return plantKeys.bushes[h%4][boolIdx(p.Harvestable)], bushScale(p)
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

// onFloor reports a building kind drawn flat in the floor-marking pass,
// under pawns and furniture, not in the object pass.
func onFloor(kind string) bool { return kind == "sleeping spot" || kind == "carpet" }

// drawFloorLayers draws the ground under zones: built floors, then the
// tracked-in dirt (dirt_draw.go, ART §16.3), then the blood and ichor
// stains (blood_draw.go, ART §16.2), so blood always lies over dirt. The
// dirt and stains layers are traced only when there is some.
func (r *Renderer) drawFloorLayers(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.layer("floors")
	for _, b := range w.Buildings {
		if b.Floor {
			r.drawTile(screen, r.buildKey("floor", b.Stuff, "", 0, false, false), float64(b.X), float64(b.Y), tile, sw, sh, tintCS(r.stuffTint(b.Stuff)))
		}
	}
	if len(w.Dirt) > 0 {
		r.layer("dirt")
		r.drawDirt(screen, w, sw, sh, tile)
	}
	if len(w.Stains) > 0 {
		r.layer("stains")
		r.drawStains(screen, w, sw, sh, tile)
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
