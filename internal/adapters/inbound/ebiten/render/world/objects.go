package world

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
	"rimworld-go/pkg/randx"
)

// Reach of an object's art past its cells, in tiles (west and east, north,
// south): the culls of DrawArt, a tile wider. A crop unit whose box misses
// the reach keeps waiting in its batch (flushCropsPad).
type reach struct{ side, up, down float64 }

var (
	plantPad    = reach{3, 5, 3}
	buildingPad = reach{4, 5, 5}
	itemPad     = reach{3, 3, 3}
	pawnPad     = reach{4, 5, 5}
)

// flushCropsPad draws the queued crop units if the screen box of the w x h
// cells at (x, y), grown by r, touches one; otherwise the object and the
// units commute and the batch stays open (PERF-009).
func (o *Objects) flushCropsPad(dst *ebiten.Image, x, y, w, h float64, r reach) {
	x0, y0 := o.v.WorldToScreenF(x-r.side, y-r.up)
	x1, y1 := o.v.WorldToScreenF(x+w+r.side, y+h+r.down)
	o.Env.FlushCropsOver(dst, x0, y0, x1, y1)
}

// flushCropsFor flushes the queued crop units before d draws, unless d
// cannot reach them.
func (o *Objects) flushCropsFor(dst *ebiten.Image, w *game.WorldView, d *ArtDraw, tile float64) {
	l := o.LOD
	switch {
	case d.Fn != nil: // a closure: no bounds
	case d.Kind == objLinked:
		x, y := d.Ld.At()
		o.flushCropsPad(dst, float64(x), float64(y), 1, 1, buildingPad)
		return
	case d.Kind == objBuilding:
		b := &w.Buildings[d.I]
		o.flushCropsPad(dst, float64(b.X), float64(b.Y), float64(max(b.W, 1)), float64(max(b.H, 1)), buildingPad)
		return
	case d.Kind == objFrame:
		f := &w.Frames[d.I]
		o.flushCropsPad(dst, float64(f.X), float64(f.Y), float64(max(f.W, 1)), float64(max(f.H, 1)), buildingPad)
		return
	case d.Kind == objPlant || d.Kind == objPlantMark:
		pl := &w.Plants[d.I]
		o.flushCropsPad(dst, float64(pl.X), float64(pl.Y), 1, 1, plantPad)
		return
	case d.Kind == objItem || d.Kind == objForbidden:
		it := &w.Items[d.I]
		o.flushCropsPad(dst, float64(it.X), float64(it.Y), 1, 1, itemPad)
		return
	case d.Kind == objAnimal && l.padSet:
		r := reach{float64(l.apad + 1), float64(l.apadS + 1), float64(l.apad + 1)}
		o.flushCropsPad(dst, d.wx, d.wy, 1, 1, r)
		return
	case d.Kind == objPawn && l.padSet:
		r := pawnPad
		r.side, r.up, r.down = r.side*l.csize, r.up*l.csize, r.down*l.csize
		o.flushCropsPad(dst, d.wx, d.wy, 1, 1, r)
		return
	}
	o.Env.FlushCrops(dst)
}

// runDraw draws one queued object.
func (o *Objects) runDraw(dst *ebiten.Image, w *game.WorldView, d *ArtDraw, tile float64, sw, sh int) {
	if d.Fn != nil || d.Kind != objPlant {
		o.flushCropsFor(dst, w, d, tile) // queued crop units draw before what they overlap
	}
	if d.Fn != nil {
		d.Fn()
		return
	}
	switch d.Kind {
	case objLinked:
		o.drawLinkedSprite(dst, d.Ld, tile, sw, sh)
	case objBuilding:
		o.DrawBuildingObj(dst, &w.Buildings[d.I], tile, sw, sh)
	case objFrame:
		o.drawFrameObj(dst, &w.Frames[d.I], tile, sw, sh)
	case objPlant:
		o.DrawPlantObj(dst, w, &w.Plants[d.I], tile)
	case objPlantMark:
		o.Bk.drawPlantMark(dst, &w.Plants[d.I], tile)
	case objItem:
		o.drawItemObj(dst, &w.Items[d.I], tile, sw, sh)
	case objForbidden:
		it := &w.Items[d.I]
		if p := neatPieceOf(it.Store); p == nil || !p.hides(it.Slot) { // no mark on a stack nobody sees
			o.DrawForbiddenMark(dst, it.X, it.Y, tile, sw, sh)
		}
	case objAnimal:
		o.Creatures.DrawAnimalFigure(o.v, o.Pass(), dst, w.Animals[d.I], d.wx, d.wy, LodTile(tile))
	case objPawn:
		o.DrawPawnArt(dst, o.v.Pawns[d.I], d.wx, d.wy, LodTile(tile), sw, sh)
	}
}

// DrawBuildingObj draws a building in the object pass (or the map bake):
// linked walls, doors and fences, troughs, graves, pyres and the rest,
// with its power and defense parts (render/overlay), a drying rack's load,
// the deconstruct mark and the forbidden marker.
func (o *Objects) DrawBuildingObj(dst *ebiten.Image, b *game.BuildingView, tile float64, sw, sh int) {
	none := color.NRGBA{}
	l := o.LOD
	v := o.v
	mask, corners := 0, 0
	open := l.occupied[[2]int{b.X, b.Y}]
	switch {
	case b.Kind == "wall":
		mask, corners = o.Links.Mask(b.X, b.Y), o.Links.Corners(b.X, b.Y)
	case sprite.IsFenceKind(b.Kind):
		mask, open = o.Links.Mask(b.X, b.Y), l.Creatures[[2]int{b.X, b.Y}]
	case b.Kind == overlay.PowerSwitchKind:
		open = !b.PowerOn // the actual state: an open switch is off
	case b.Kind == "vent": // TEMP-1: open or shut (build_keys.go)
		open = b.Open
	case b.Kind == "animal flap": // A2: an animal swings it open
		open = l.Creatures[[2]int{b.X, b.Y}]
	case b.Kind == "ornate door": // A2: either leaf's cell
		open = open || l.occupied[[2]int{b.X + b.W - 1, b.Y + b.H - 1}]
	case b.Kind == "passive cooler": // TEMP-1: fuelled or empty (build_keys.go)
		open = b.FuelPct > 0
	}
	switch b.Kind {
	case "grave":
		o.Creatures.DrawGrave(v, o.Pass(), dst, *b, tile, sw, sh)
	case "funeral pyre":
		o.Creatures.DrawPyre(v, o.Pass(), dst, *b, tile, sw, sh)
	default:
		if b.Kind == "wall" {
			if wd, ok := o.Paint.LinkedSprite(sprite.WallPrefix(b.Stuff), b.X, b.Y, mask, corners, o.stuffTint(b.Stuff), sprite.NoCS); ok {
				o.drawLinkedSprite(dst, wd, tile, sw, sh)
				break
			}
		}
		key := o.BuildKey(b.Kind, b.Stuff, b.Facing, mask, sprite.DoorAxisEW(b.Facing), open)
		if b.Kind == overlay.CasketKind {
			key = overlay.CasketSprite(*b) // closed or open
		}
		if b.Kind == "ship stasis casket" {
			key = ShipKey(*b) // ship_keys.go (AX-8): empty or occupied
		}
		if b.Kind == StasisCasketKind {
			key = TravelKey(*b) // travel_keys.go (AX-9): empty or occupied
		}
		if b.Kind == BookcaseKind || b.Kind == SmallBookcaseKind {
			key = BookcaseKey(b.Kind, b.Facing, b.Stored) // book_keys.go (A7): empty, half or full
		}
		if b.Kind == TroughKind {
			key = TroughKey(*b)
			if !o.art().Has(key) {
				o.drawTroughFallback(dst, *b, tile)
				break
			}
		}
		if sprite.IsFenceKind(b.Kind) && !o.art().Has(key) {
			sx, sy := o.worldToScreen(b.X, b.Y)
			o.Lit.Flush() // the lit batch first: a raw draw lands over the sprites queued before it
			sprite.DrawFenceVector(dst, sx, sy, float32(tile), b.Kind, mask, open, sprite.IsStoneStuff(b.Stuff), 255)
			break
		}
		t := none
		if (b.Kind == "bed" || b.Kind == "double bed" || b.Kind == "armchair" || b.Kind == "couch" || b.Kind == "animal flap") && len(l.cloth) > 0 { // A2: couch, flap
			t = o.art().Tint("cloth", l.cloth[randx.Mix64(uint64(b.ID))%uint64(len(l.cloth))])
		}
		// stone walls, fences and floors take their stone's tint; the
		// sculptures and the barricade carry a stone_tint mask instead
		cs := sprite.TintCS(o.stuffTint(b.Stuff))
		if stoneMaskKind(b.Kind) {
			t, cs = o.stuffTint(b.Stuff), sprite.NoCS
		}
		o.drawSpriteCS(dst, key, t, float64(b.X), float64(b.Y), tile, sw, sh, cs)
		if b.AX.Plant != "" { // A2: a plant pot's plant
			o.drawPotPlant(dst, b, key, tile, sw, sh)
		}
		chP := o.OverlayChunks()
		o.Overlays.Wiring.DrawPowerParts(v, o.Pass(), &chP, dst, b, key, tile, sw, sh) // glow, blades, flame, charge
		o.drawTankFish(dst, b, tile, sw, sh)
		chD := o.OverlayChunks()
		o.Overlays.Wiring.DrawDefenseParts(v, o.Pass(), &chD, dst, b, key, tile, sw, sh) // gun, beacon light
		o.Overlays.Wiring.DrawShipGlow(v, &chD, dst, b, key, tile, sw, sh)               // overlay/ship_draw.go (AX-8)
	}
	o.Env.DrawRuinOverlay(v, o.Pass(), dst, b, tile, sw, sh)
	if b.DryQty > 0 {
		o.drawRackLoad(dst, *b, tile, sw, sh)
	}
	switch { // AX-1: a casket's open order rides its deconstruct mark
	case b.AX.OpenMarked:
		o.drawSprite(dst, "overlay/designation_open", none, float64(b.X), float64(b.Y), tile, sw, sh)
	case b.Deconstruct:
		o.drawSprite(dst, "overlay/designation_deconstruct", none, float64(b.X), float64(b.Y), tile, sw, sh)
	case b.AX.Uninstall:
		o.drawSprite(dst, "overlay/designation_uninstall", none, float64(b.X), float64(b.Y), tile, sw, sh)
	}
	if b.Forbidden { // STK-AL: a forbidden door or bed wears the item's marker
		o.DrawForbiddenMark(dst, b.X, b.Y, tile, sw, sh)
	}
}

// drawFrameObj draws a planned building (blueprint), autotiled with the
// frame's planned set.
func (o *Objects) drawFrameObj(dst *ebiten.Image, f *game.FrameView, tile float64, sw, sh int) {
	none := color.NRGBA{}
	pm, pc := 0, 0
	if c := overlay.BuildingLink(f.Kind); c.Flags != 0 {
		pm = o.Links.PlannedMask(o.LOD.planned, c.Accepts, f.X, f.Y)
		if f.Kind == "wall" {
			pc = o.Links.PlannedCorners(o.LOD.planned, c.Accepts, f.X, f.Y)
		}
	}
	tint := o.stuffTint(f.Stuff)
	if f.Wood > 0 {
		o.drawSprite(dst, "structure/frame_1x1", none, float64(f.X), float64(f.Y), tile, sw, sh)
	}
	if f.Kind == "wall" {
		if wd, ok := o.Paint.LinkedSprite(sprite.WallPrefix(f.Stuff), f.X, f.Y, pm, pc, tint, sprite.BlueprintCS); ok {
			o.drawLinkedSprite(dst, wd, tile, sw, sh)
			return
		}
	}
	key := o.BuildKey(f.Kind, f.Stuff, f.Facing, pm, sprite.DoorAxisEW(f.Facing), false)
	if sprite.IsFenceKind(f.Kind) && !o.art().Has(key) {
		sx, sy := o.worldToScreen(f.X, f.Y)
		o.Lit.Flush() // the lit batch first: a raw draw lands over the sprites queued before it
		sprite.DrawFenceVector(dst, sx, sy, float32(tile), f.Kind, pm, false, sprite.IsStoneStuff(f.Stuff), 140)
		return
	}
	o.drawSpriteCS(dst, key, none, float64(f.X), float64(f.Y), tile, sw, sh, sprite.MulCS(sprite.TintCS(tint), sprite.BlueprintCS))
}

// DrawPlantObj draws a plant swaying (Sky.PlantPose) in its season's
// look. A crop queues its units instead (render/env); cold or heat damage
// tints, shrinks and fades any plant (env.PlantDamageLook).
func (o *Objects) DrawPlantObj(dst *ebiten.Image, w *game.WorldView, p *game.PlantView, tile float64) {
	v := o.v
	if env.IsCropPlant(p) {
		o.Env.DrawCropObj(v, dst, w, p, tile)
		return
	}
	o.flushCropsPad(dst, float64(p.X), float64(p.Y), 1, 1, plantPad)
	key, scale := PlantSprite(*p)
	key = o.Env.SeasonKey(v, key, *w)
	amount, kind := 0.035, env.KindBush
	switch p.Species {
	case "tree", "pine", "jungle_tree":
		amount, kind = 0.022, env.KindTree
	}
	ps := o.Env.PlantPose(v, p.X, p.Y, kind, amount, scale)
	dcs, dk := env.PlantDamageLook(*p)
	ps.Cs = dcs
	ps.Sx, ps.Sy = ps.Sx*dk, ps.Sy*dk
	if kind == env.KindTree {
		if trunk, canopy, ok := env.TreePartKeys(v, key); ok {
			// PLAY-010 2.9: the trunk stays opaque, the canopy turns
			// see-through over creatures behind it; one pose for both
			o.layer("tree_trunk")
			o.DrawSpritePose(dst, trunk, color.NRGBA{}, float64(p.X), float64(p.Y), tile, ps)
			if a := o.Env.Trees.TreeCanopyAlpha(v, p.X, p.Y, o.LOD.Creatures); a < 1 {
				ps.Cs = sprite.MulCS(ps.Cs, sprite.ColorScale{1, 1, 1, a})
			}
			o.layer("tree_canopy")
			o.DrawSpritePose(dst, canopy, color.NRGBA{}, float64(p.X), float64(p.Y), tile, ps)
			return
		}
	}
	o.DrawSpritePose(dst, key, color.NRGBA{}, float64(p.X), float64(p.Y), tile, ps)
}

// itemRow is a stack's draw row: its own tile's, or for a stack in a storage
// piece the row that draws it after the piece (neat_keys.go).
func (o *Objects) itemRow(w *game.WorldView, it *game.ItemView) float64 {
	if it.Store != 0 {
		if p := neatPieceOf(it.Store); p != nil {
			c, _ := o.neat.at(w.StructVer, w.Buildings, p, it)
			return stackRow(c, it)
		}
	}
	return float64(it.Y)
}

// drawStoredItem draws a stack in storage piece p at its slot (NST): nothing
// in a closed piece or past a heap's first few, a corpse lying at its slot,
// on end where the piece stands its stacks up.
// False when the stack has no art, for the caller's fallback.
func (o *Objects) drawStoredItem(dst *ebiten.Image, p *neatPiece, it *game.ItemView, tile float64, sw, sh int) bool {
	if p.hides(it.Slot) {
		return true
	}
	ax, ay, k, _ := o.stackSlot(p, it)
	if it.Kind == "human corpse" {
		if pawn, ok := o.Creatures.DeceasedOf(o.v, *it); ok {
			ps, wx, wy := heapBody(creature.CorpsePlanFor(o.v.Art, *it).Ps, ax, ay, k)
			o.Creatures.DrawFigure(o.v, o.Pass(), dst, pawn, wx, wy, tile, ps)
			return true
		}
	}
	key := SizedItemKey(ItemSpriteSpecies(it.Kind, it.Animal, it.X, it.Y), it.Qty)
	cs := sprite.ItemCS(*it)
	if it.Kind == "bones" {
		key = creature.BonesSprite(it.X, it.Y)
	} else {
		key, cs = o.StoneItem(key, it.Kind, cs)
	}
	m := o.art().MetaOf(key)
	if m == nil {
		return false
	}
	k *= motion.PopScale((*o.Creatures.ItemBorn)[it.ID], o.v.Present)
	if p.show == showOnEnd {
		fw, fh := float64(m.Footprint[0]), float64(m.Footprint[1])
		o.DrawSpritePose(dst, key, color.NRGBA{}, ax-fw/2, ay-fh/2, tile, stackOnEnd(fw, fh, k, cs))
		return true
	}
	wx, wy := stackOrigin(ax, ay, k, m.Footprint[0], m.Footprint[1])
	o.drawSpriteScaled(dst, key, color.NRGBA{}, wx, wy, tile, sw, sh, cs, k)
	return true
}

// drawItemObj draws a ground stack (corpses and bones their own way).
func (o *Objects) drawItemObj(dst *ebiten.Image, it *game.ItemView, tile float64, sw, sh int) {
	if it.Store != 0 {
		if p := neatPieceOf(it.Store); p != nil && o.drawStoredItem(dst, p, it, tile, sw, sh) {
			return
		}
	}
	if it.Kind == "human corpse" || it.Kind == "bones" {
		o.Creatures.DrawCorpseItem(o.v, o.Pass(), dst, *it, tile, sw, sh)
		return
	}
	key := SizedItemKey(ItemSpriteSpecies(it.Kind, it.Animal, it.X, it.Y), it.Qty)
	cs := sprite.ItemCS(*it)
	if key, cs = o.StoneItem(key, it.Kind, cs); !o.art().Has(key) {
		pad := float32(tile) * 0.3
		sx, sy := o.worldToScreen(it.X, it.Y)
		o.Lit.Flush() // the lit batch first: a raw draw lands over the sprites queued before it
		vector.FillRect(dst, sx+pad, sy+pad, float32(tile)-2*pad, float32(tile)-2*pad, color.RGBA{200, 90, 40, 255}, false)
		return
	}
	o.drawSpriteScaled(dst, key, color.NRGBA{}, float64(it.X), float64(it.Y), tile, sw, sh, cs, motion.PopScale((*o.Creatures.ItemBorn)[it.ID], o.v.Present))
}

// drawRackLoad lays a drying batch along the rack's slats, browning as it
// dries.
func (o *Objects) drawRackLoad(dst *ebiten.Image, b game.BuildingView, tile float64, sw, sh int) {
	key := ItemSprite(b.DryKind)
	cs := sprite.LerpCS(sprite.NoCS, sprite.DriedCS, float64(b.DryPct)/100)
	w, h := max(b.W, 1), max(b.H, 1)
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			o.drawSpriteScaled(dst, key, color.NRGBA{}, float64(b.X+dx), float64(b.Y+dy)-rackSlatLift, tile, sw, sh, cs, 0.55)
		}
	}
}

// potPlantArt is the plant art a pot plant borrows (A2: until it has its
// own, a rose draws as the red fire lily, a daylily as the yellow
// dandelion; plants never carry an outline).
func potPlantArt(plant string) (string, bool) {
	switch plant {
	case "rose":
		return "plant/fire_lily", true
	case "daylily":
		return "plant/dandelion", true
	}
	return "", false
}

// drawPotPlant draws pot b's plant (drawn as key) rising from the soil
// (its sprite's plant attachment), young below half grown, scaled from a
// third to two thirds of a tile with its growth.
func (o *Objects) drawPotPlant(dst *ebiten.Image, b *game.BuildingView, key string, tile float64, sw, sh int) {
	base, ok := potPlantArt(b.AX.Plant)
	sp := o.art().Sprites[key]
	if !ok || sp == nil {
		return
	}
	a, ok := sp.Meta.Attach["plant"]
	if !ok {
		return
	}
	stage := "_young_v0"
	if b.AX.PlantPct >= 50 {
		stage = "_mature_v0"
	}
	k := 0.33 + 0.33*float64(b.AX.PlantPct)/100
	sx := float64(b.X) + (a[0]-sp.Meta.Origin[0])/art.TilePx - 0.5*k
	sy := float64(b.Y) + (a[1]-sp.Meta.Origin[1])/art.TilePx - 0.85*k
	o.drawSpriteScaled(dst, base+stage, color.NRGBA{}, sx, sy, tile, sw, sh, sprite.NoCS, k)
}

// rackSlatLift raises rack contents (in tiles) to sit on the slats.
const rackSlatLift = 0.3

// stoneMaskKind reports a wave D kind whose sprite has a stone_tint mask
// (PRD-3 sculptures, DEF-3 barricade): the stuff's stone colours the mask.
func stoneMaskKind(kind string) bool {
	switch kind {
	case "small sculpture", "large sculpture", "grand sculpture", "barricade", "sarcophagus",
		"column", "urn", "large stele", "grand stele": // A2 (AR1's art)
		return true
	}
	return false
}
