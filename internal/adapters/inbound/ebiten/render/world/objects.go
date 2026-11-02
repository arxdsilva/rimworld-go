package world

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
)

// runDraw draws one queued object.
func (o *Objects) runDraw(dst *ebiten.Image, w *game.WorldView, d *ArtDraw, tile float64, sw, sh int) {
	if d.Fn != nil || d.Kind != objPlant {
		o.Env.FlushCrops(dst) // queued crop units draw before whatever comes next
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
		o.DrawForbiddenMark(dst, it.X, it.Y, tile, sw, sh)
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
		if b.Kind == TroughKind {
			key = TroughKey(*b)
			if !o.art().Has(key) {
				o.drawTroughFallback(dst, *b, tile)
				break
			}
		}
		if sprite.IsFenceKind(b.Kind) && !o.art().Has(key) {
			sx, sy := o.worldToScreen(b.X, b.Y)
			sprite.DrawFenceVector(dst, sx, sy, float32(tile), b.Kind, mask, open, sprite.IsStoneStuff(b.Stuff), 255)
			break
		}
		t := none
		if (b.Kind == "bed" || b.Kind == "double bed" || b.Kind == "armchair" || b.Kind == "couch" || b.Kind == "animal flap") && len(l.cloth) > 0 { // A2: couch, flap
			t = o.art().Tint("cloth", l.cloth[art.Mix64(uint64(b.ID))%uint64(len(l.cloth))])
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
		chD := o.OverlayChunks()
		o.Overlays.Wiring.DrawDefenseParts(v, o.Pass(), &chD, dst, b, key, tile, sw, sh) // gun, beacon light
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
	o.Env.FlushCrops(dst)
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

// drawItemObj draws a ground stack (corpses and bones their own way).
func (o *Objects) drawItemObj(dst *ebiten.Image, it *game.ItemView, tile float64, sw, sh int) {
	if it.Kind == "human corpse" || it.Kind == "bones" {
		o.Creatures.DrawCorpseItem(o.v, o.Pass(), dst, *it, tile, sw, sh)
		return
	}
	key := SizedItemKey(ItemSpriteSpecies(it.Kind, it.Animal, it.X, it.Y), it.Qty)
	cs := sprite.ItemCS(*it)
	if key, cs = o.StoneItem(key, it.Kind, cs); !o.art().Has(key) {
		pad := float32(tile) * 0.3
		sx, sy := o.worldToScreen(it.X, it.Y)
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
