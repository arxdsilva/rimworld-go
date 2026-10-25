package ebiten

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"slices"
	"strconv"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// Crops as composed units (PLAY-016, ART §9.3). A crop tile is not one big
// sprite: it is several small unit plants (cropUnitsPerTile) on a jittered
// pattern with a slight row alignment along the field's long axis, each
// with its own variant, mirror, size trim and sway phase, scaled by the
// growth within its stage so a plant grows without popping between
// stages. Every unit queued by the object pass goes into one triangle
// batch from a runtime atlas (lit like a sprite: lit.kage, per-vertex sky
// and lamp light); the batch is flushed whenever another object comes
// next in draw order, so units never paint over what stands in front.
//
// Cold and heat (PlantView.Damage): below cropWiltDamage a unit keeps its
// sprite under a growing rime (cold) or scorch (heat) tint; from there it
// shows the wilted sprite, shrinks towards cropMinShrink and, over the
// last 30%, fades out one unit at a time, so a tile thins out visibly.
//
// Level of detail by on-screen tile size: from lodLiveMin every unit; below
// it one representative unit a tile; the map bake draws marks
// (cropMarkColor). Furrows (overlay/tilled_*) lie in the ground pass under
// every crop cell, turned a quarter on a north-south field.

const (
	// cropVariants is how many variants each crop stage has (v0..v3).
	cropVariants = 4
	// cropWiltDamage is the damage from which a crop draws its wilted
	// sprite (below it: the normal sprite under a tint).
	cropWiltDamage = 50
	// cropFadeFrom is the damage from which units fade out, one by one.
	cropFadeFrom = 70
	// cropMinShrink is the size units reach at 100% damage.
	cropMinShrink = 0.6
	// cropWiltedTint is the share of the full tint a wilted sprite wears.
	cropWiltedTint = 0.6
	// cropJitterAlong and cropJitterAcross are how far a unit may sit from
	// its pattern spot along its row and across it, in tiles: the rows stay.
	cropJitterAlong  = 0.08
	cropJitterAcross = 0.04
	// cropLODScale is the size of the one unit a tile draws below
	// lodLiveMin, so the tile keeps its mass.
	cropLODScale = 1.3
	// cropLeanMax limits a unit's lean, radians; cropWobble is the gentle
	// per-unit sway on top of the tile's wind lean.
	cropLeanMax = 0.3
	cropWobble  = 0.03
	// cropAtlasPad is the transparent margin around each packed sprite (the
	// shader's bilinear taps reach half a pixel out); cropAtlasRow its
	// shelf width.
	cropAtlasPad = 2
	cropAtlasRow = 2048
	// cropBatchQuads caps a batch (index range of one draw).
	cropBatchQuads = 12000
	// cropScaleMin and cropScaleMax bound the growth scale of a stage's
	// sprite (art whose stages differ wildly in size stays sane).
	cropScaleMin = 0.4
	cropScaleMax = 1.4
)

// cropUnitsPerTile is how many unit sprites a crop's tile shows.
var cropUnitsPerTile = map[string]int{
	"corn": 2, "wheat": 4, "brewgrain": 4, "cotton": 3, "potato": 3, "beans": 3,
	"healroot": 3, "ashleaf": 3, "lettuce": 4, "radish": 4, "pumpkin": 1,
}

// cropUnitCount is the units a tile of this crop draws (1 for others).
func cropUnitCount(species string) int {
	if n := cropUnitsPerTile[species]; n > 0 {
		return n
	}
	return 1
}

// cropBase is each unit count's pattern: (along the row, across it) in
// tiles. Two rows of staggered units for 3 and 4; a single row for 2, so
// neighbouring tiles line up into rows.
var cropBase = [5][][2]float64{
	1: {{0.5, 0.55}},
	2: {{0.25, 0.5}, {0.75, 0.5}},
	3: {{0.25, 0.3}, {0.75, 0.3}, {0.5, 0.78}},
	4: {{0.25, 0.3}, {0.75, 0.3}, {0.0, 0.76}, {0.5, 0.76}},
}

// cropUnit is one unit plant of a tile: its foot at (dx, dy) tiles from
// the tile's top-left, its size trim, share of the tile's sway and sway
// phase, the variant it draws (mod the stage's variants), mirrored or
// not, and its drop-out rank (0 goes first under damage).
type cropUnit struct {
	dx, dy, trim, sway, phase float64
	variant                   uint32
	mirror                    bool
	rank                      int
}

// cropUnits appends tile (x, y)'s n units to out: a pure function of the
// tile, the unit count and the field's axis (vertical: rows run north to
// south, else east to west).
func cropUnits(x, y, n int, vertical bool, out []cropUnit) []cropUnit {
	n = min(max(n, 1), 4)
	th := tileHash(x, y)
	var keys [4]uint64
	for k := 0; k < n; k++ {
		keys[k] = mix64(th ^ uint64(k+1)*0xD6E8FEB86659FD93 ^ 0x72616e6b)
	}
	for k, b := range cropBase[n] {
		h := mix64(th ^ uint64(k+1)*0x9E3779B97F4A7C15 ^ 0x63726f70)
		h2 := mix64(h ^ 0x756e6974)
		f := func(shift uint) float64 { return float64(h>>shift&0xffff) / 65535 }
		along := b[0] + (f(0)*2-1)*cropJitterAlong
		across := b[1] + (f(16)*2-1)*cropJitterAcross
		rank := 0
		for j := 0; j < n; j++ {
			if keys[j] < keys[k] || keys[j] == keys[k] && j < k {
				rank++
			}
		}
		u := cropUnit{
			trim:    0.9 + 0.2*f(32),
			sway:    0.8 + 0.4*f(48),
			phase:   2 * math.Pi * float64(h2&0xffffff) / float64(1<<24),
			variant: uint32(h2 >> 24 & 0xff),
			mirror:  h2>>40&1 != 0,
			rank:    rank,
		}
		u.dx, u.dy = along, across
		if vertical {
			u.dx, u.dy = across, along
		}
		out = append(out, u)
	}
	return out
}

// sortCropUnits orders a tile's units north to south (insertion sort: at
// most four), so a nearer unit overlaps the one behind it.
func sortCropUnits(us []cropUnit) {
	for i := 1; i < len(us); i++ {
		for j := i; j > 0 && us[j].dy < us[j-1].dy; j-- {
			us[j], us[j-1] = us[j-1], us[j]
		}
	}
}

// cropSwayAmount is the wind amount a crop species asks plantPose for
// (tall grains lean more).
func cropSwayAmount(species string) float64 {
	switch species {
	case "wheat", "corn", "brewgrain":
		return 0.07
	}
	return 0.04
}

// cropSwayFactor scales a species' lean and wobble: tall crops sway more.
func cropSwayFactor(species string) float64 {
	switch species {
	case "corn":
		return 1.35
	case "wheat", "brewgrain":
		return 1.2
	case "cotton":
		return 1
	case "beans":
		return 0.9
	case "potato", "healroot":
		return 0.8
	case "ashleaf":
		return 0.75
	case "lettuce", "radish":
		return 0.7
	case "pumpkin":
		return 0.5
	}
	return 1
}

// isCropPlant reports a sown or grown crop: a crop species, except a wild
// healroot (one outside a grow zone).
func isCropPlant(p *game.PlantView) bool {
	if _, ok := plantKeys.crops[p.Species]; !ok {
		return false
	}
	return p.Species != "healroot" || p.Zone != ""
}

// --- damage ---------------------------------------------------------------------

// Tints at full strength: rime is a pale blue-white, scorch a brown-yellow.
var (
	cropRimeCS    = colorScale{1.8, 1.7, 3.0, 1}
	cropScorchCS  = colorScale{1.3, 0.8, 0.35, 1}
	cropMarkRime  = color.RGBA{188, 206, 228, 255}
	cropMarkBurnt = color.RGBA{150, 104, 52, 255}
)

// cropDamageTint is the colour scale of a crop at this damage: none
// undamaged, a rime (cold) or scorch (heat) growing with the damage up to
// cropWiltDamage, then a lighter one over the wilted sprite.
func cropDamageTint(damage int, kind string) colorScale {
	if damage <= 0 {
		return noCS
	}
	f := float32(damage) / cropWiltDamage
	if damage >= cropWiltDamage {
		f = cropWiltedTint
	}
	max := cropRimeCS
	if kind == "heat" {
		max = cropScorchCS
	}
	return colorScale{1 + (max[0]-1)*f, 1 + (max[1]-1)*f, 1 + (max[2]-1)*f, 1}
}

// cropUnitLook is the size factor and opacity of the unit of drop-out
// rank `rank` among n at this damage: whole until cropWiltDamage, then
// shrinking towards cropMinShrink; from cropFadeFrom each unit fades out
// in its own band of the last 30%, rank 0 first, so a tile thins out one
// unit at a time and the last is gone at 100%.
func cropUnitLook(damage, rank, n int) (scale, alpha float64) {
	d := float64(min(max(damage, 0), 100))
	scale, alpha = 1, 1
	if d > cropWiltDamage {
		scale = 1 - (1-cropMinShrink)*(d-cropWiltDamage)/(100-cropWiltDamage)
	}
	if d > cropFadeFrom {
		span := (100 - cropFadeFrom) / float64(max(n, 1))
		lo := cropFadeFrom + span*float64(rank)
		alpha = clamp01(1 - (d-lo)/span)
		if alpha < 1e-9 {
			alpha = 0
		}
	}
	return scale, alpha
}

// damageShadowFactor is how much of a damaged plant's sun shadow is left:
// the mean size times opacity of its units (a crop) or of the whole plant,
// so a field's shadows thin out with it and are gone when it is.
func damageShadowFactor(p game.PlantView) float64 {
	if p.Damage <= 0 {
		return 1
	}
	n := 1
	if isCropPlant(&p) {
		n = cropUnitCount(p.Species)
	}
	sum := 0.0
	for k := 0; k < n; k++ {
		s, a := cropUnitLook(p.Damage, k, n)
		sum += s * a
	}
	return sum / float64(n)
}

// plantDamageLook is the whole-plant treatment of a damaged plant that is
// not a crop (a tree, a bush): its tint with the fade folded into the
// alpha, and its size factor.
func plantDamageLook(p game.PlantView) (colorScale, float64) {
	if p.Damage <= 0 {
		return noCS, 1
	}
	cs := cropDamageTint(p.Damage, p.DamageKind)
	scale, alpha := cropUnitLook(p.Damage, 0, 1)
	cs[3] *= float32(alpha)
	return cs, scale
}

// plantDamageLine is the inspector and hover line of a damaged plant.
func plantDamageLine(p game.PlantView) string {
	if p.Damage <= 0 {
		return ""
	}
	d := min(p.Damage, 100)
	switch p.DamageKind {
	case "cold":
		return fmt.Sprintf("Freezing: %d%% damaged", d)
	case "heat":
		return fmt.Sprintf("Scorching: %d%% damaged", d)
	}
	return fmt.Sprintf("Damaged: %d%%", d)
}

// plantDamageLabelAt is the hover line of the damaged plant on a tile.
func plantDamageLabelAt(w game.WorldView, x, y int) string {
	for i := range w.Plants {
		if p := &w.Plants[i]; p.X == x && p.Y == y {
			return plantDamageLine(*p)
		}
	}
	return ""
}

// cropStageMarks are the map bake's marks by drawn stage: soil brown for
// the sown, greens as it grows, gold ripe, dull when wilted.
var cropStageMarks = [numCropStages]color.RGBA{
	stSown: {104, 88, 62, 255}, stSprout: {128, 150, 84, 255}, stYoung: {134, 158, 76, 255},
	stMature: {150, 160, 70, 255}, stHarvestable: {198, 172, 74, 255}, stWilted: {120, 100, 72, 255},
}

// cropMarkColor is a crop's mark in the map bake: its stage's colour,
// pulled towards frost white or scorch brown by its damage.
func cropMarkColor(p *game.PlantView) color.RGBA {
	c := cropStageMarks[cropStage(*p)]
	if p.Damage <= 0 {
		return c
	}
	to := cropMarkRime
	if p.DamageKind == "heat" {
		to = cropMarkBurnt
	}
	f := 0.7 * float64(min(p.Damage, cropWiltDamage)) / cropWiltDamage
	mix := func(a, b uint8) uint8 { return uint8(float64(a) + (float64(b)-float64(a))*f + 0.5) }
	return color.RGBA{mix(c.R, to.R), mix(c.G, to.G), mix(c.B, to.B), 255}
}

// --- the atlas ----------------------------------------------------------------

// cropSprite is one unit sprite in the atlas: its source rectangle (px),
// its foot within it, and tiles per atlas pixel at scale 1.
type cropSprite struct {
	sx, sy, w, h float32
	fx, fy       float32
	k            float32
}

// furrowSprite is one tilled decal in the atlas: its whole canvas, the
// centre of its footprint within it, and tiles per atlas pixel.
type furrowSprite struct {
	sx, sy, w, h float32
	cx, cy       float32
	k            float32
}

// cropSet is a species' sprites: per stage the indexes into the atlas's
// sprites (one per distinct variant that exists), and each stage's
// nominal height in tiles.
type cropSet struct {
	stages  [numCropStages][]int32
	nominal [numCropStages]float32
}

// cropAtlas packs the unit sprites of the species drawn so far, and the
// furrow decals, into one image, so a whole field is one draw.
type cropAtlas struct {
	art     *artCatalog
	img     *ebiten.Image
	sprites []cropSprite
	sets    map[string]*cropSet
	order   []string // species in the atlas, in the order they joined
	furrows []furrowSprite
}

// cropArtKey resolves a crop sprite key to art that exists: the key, then
// its fallbacks (variant 0, the pre-variant sprite); "" when none does.
func cropArtKey(c *artCatalog, key string) string {
	if c.has(key) {
		return key
	}
	for _, k := range plantKeys.fallback[key] {
		if c.has(k) {
			return k
		}
	}
	return ""
}

// buildCropAtlas packs c's unit sprites for species (alpha-trimmed) and
// the furrow decals into one image. A species or stage without art has an
// empty set (nothing is drawn for it).
func buildCropAtlas(c *artCatalog, species []string) *cropAtlas {
	a := &cropAtlas{art: c, sets: map[string]*cropSet{}, order: slices.Clone(species)}
	type item struct {
		pix      *image.NRGBA
		box      image.Rectangle
		x, y     int
		spr      cropSprite
		fur      furrowSprite
		isFurrow bool
	}
	var items []item
	byKey := map[string]int32{} // resolved key -> index into a.sprites
	var itemOf []int            // a.sprites index -> items index
	measure := func(sp *artSprite) (kpx float64, ok bool) {
		if sp == nil || sp.pix == nil {
			return 0, false
		}
		kpx = 1
		if b := sp.pix.Bounds(); b.Dx() > 0 && sp.meta.Canvas[0] > 0 {
			kpx = float64(sp.meta.Canvas[0]) / float64(b.Dx())
		}
		return kpx, true
	}
	footprint := func(m artMeta) (float64, float64) {
		fw, fh := float64(m.Footprint[0]), float64(m.Footprint[1])
		if fw <= 0 {
			fw = 1
		}
		if fh <= 0 {
			fh = 1
		}
		return fw, fh
	}
	for _, name := range species {
		set := &cropSet{}
		a.sets[name] = set
		ks := plantKeys.crops[name]
		if ks == nil {
			continue
		}
		for st := 0; st < numCropStages; st++ {
			var heights []float32
			for v := 0; v < cropVariants; v++ {
				key := cropArtKey(c, ks[st][v])
				if key == "" {
					continue
				}
				idx, seen := byKey[key]
				if !seen {
					sp := c.sprite(key)
					kpx, ok := measure(sp)
					if !ok {
						continue
					}
					box, ok := alphaBox(sp.pix)
					if !ok {
						continue
					}
					fw, fh := footprint(sp.meta)
					fx := (sp.meta.Origin[0]+fw*artTilePx/2)/kpx - float64(box.Min.X)
					fy := (sp.meta.Origin[1]+fh*artTilePx)/kpx - float64(box.Min.Y)
					idx = int32(len(a.sprites))
					byKey[key] = idx
					itemOf = append(itemOf, len(items))
					a.sprites = append(a.sprites, cropSprite{})
					items = append(items, item{pix: sp.pix, box: box,
						spr: cropSprite{w: float32(box.Dx()), h: float32(box.Dy()), fx: float32(fx), fy: float32(fy), k: float32(kpx / artTilePx)}})
				}
				if slices.Contains(set.stages[st], idx) {
					continue
				}
				set.stages[st] = append(set.stages[st], idx)
				heights = append(heights, items[itemOf[idx]].spr.fy*items[itemOf[idx]].spr.k)
			}
			if len(heights) > 0 {
				slices.Sort(heights)
				set.nominal[st] = max(heights[len(heights)/2], 0.02)
			}
		}
	}
	for v := 0; v < 4; v++ {
		key := "overlay/tilled_" + strconv.Itoa(v)
		sp := c.sprite(key)
		kpx, ok := measure(sp)
		if !ok {
			continue
		}
		b := sp.pix.Bounds()
		fw, fh := footprint(sp.meta)
		items = append(items, item{pix: sp.pix, box: b, isFurrow: true,
			fur: furrowSprite{w: float32(b.Dx()), h: float32(b.Dy()),
				cx: float32((sp.meta.Origin[0] + fw*artTilePx/2) / kpx), cy: float32((sp.meta.Origin[1] + fh*artTilePx/2) / kpx),
				k: float32(kpx / artTilePx)}})
	}
	if len(items) == 0 {
		return a
	}
	// shelf packing
	x, y, rowH, width := cropAtlasPad, cropAtlasPad, 0, 0
	for i := range items {
		it := &items[i]
		if x+it.box.Dx()+cropAtlasPad > cropAtlasRow && x > cropAtlasPad {
			x, y, rowH = cropAtlasPad, y+rowH+cropAtlasPad, 0
		}
		it.x, it.y = x, y
		x += it.box.Dx() + cropAtlasPad
		rowH = max(rowH, it.box.Dy())
		width = max(width, x)
	}
	pack := image.NewNRGBA(image.Rect(0, 0, width, y+rowH+cropAtlasPad))
	for i := range items {
		it := &items[i]
		draw.Draw(pack, image.Rect(it.x, it.y, it.x+it.box.Dx(), it.y+it.box.Dy()), it.pix, it.box.Min, draw.Src)
	}
	a.img = ebiten.NewImageFromImage(pack)
	for i := range items {
		it := &items[i]
		if it.isFurrow {
			f := it.fur
			f.sx, f.sy = float32(it.x), float32(it.y)
			a.furrows = append(a.furrows, f)
		}
	}
	for idx, ii := range itemOf {
		s := items[ii].spr
		s.sx, s.sy = float32(items[ii].x), float32(items[ii].y)
		a.sprites[idx] = s
	}
	return a
}

// cropAnchors are the growth percents at which a stage's sprite is at its
// nominal height: the middle of its growth range (sprout 8-20, young
// 20-55, mature 55-100) and ripe.
var cropAnchors = [...]struct {
	stage int
	g     float64
}{{stSprout, 14}, {stYoung, 37.5}, {stMature, 77.5}, {stHarvestable, 100}}

// height is the drawn height in tiles of a plant at growth percent g: the
// stages' nominal heights joined by straight lines, so growth is
// continuous whichever stage's sprite shows.
func (s *cropSet) height(g float64) float64 {
	var pg, ph float64
	have := false
	for _, an := range cropAnchors {
		n := float64(s.nominal[an.stage])
		if n <= 0 {
			continue
		}
		if !have {
			if g <= an.g { // a sprout grows out of 70% of its size
				return n * (0.7 + 0.3*clamp01((g-8)/(an.g-8)))
			}
			have, pg, ph = true, an.g, n
			continue
		}
		if g <= an.g {
			return ph + (n-ph)*(g-pg)/(an.g-pg)
		}
		pg, ph = an.g, n
	}
	return ph
}

// scale is the size factor of a growth-stage sprite at growth percent g
// (the sprite's nominal height brought to the growth curve); sown seeds
// stay as drawn.
func (s *cropSet) scale(stage, growth int) float64 {
	n := float64(s.nominal[stage])
	if stage == stSown || n <= 0 {
		return 1
	}
	return min(max(s.height(float64(growth))/n, cropScaleMin), cropScaleMax)
}

// --- the batch ------------------------------------------------------------------

// cropBatch is the renderer's crop state: the atlas, the pending unit
// batch and the furrow batch, the field-axis memo and counters the tests
// read.
type cropBatch struct {
	atlas *cropAtlas
	// pending units: flushed before any other object draws
	verts []ebiten.Vertex
	idx   []uint32
	units []cropUnit
	// the last furrow pass (kept after it is drawn)
	fverts []ebiten.Vertex
	fidx   []uint32
	axes   cropAxes
	// counters (tests): unit quads queued, unit batches drawn and furrow
	// quads drawn since begin.
	quads, flushes, furrows int
}

// begin starts a frame: counters at zero, nothing pending.
func (cb *cropBatch) begin() {
	cb.quads, cb.flushes, cb.furrows = 0, 0, 0
	cb.verts, cb.idx = cb.verts[:0], cb.idx[:0]
}

// cropAxes memoises which cells lie in a north-south grow zone (taller
// than wide), keyed on the zones list of the world it was built from.
type cropAxes struct {
	first    *game.ZoneView
	n, tw    int
	vertical map[int32]struct{}
}

// sync rebuilds the memo when w's zones are not the ones it was built from.
func (ax *cropAxes) sync(w *game.WorldView) {
	first := litFirst(w.Zones)
	if ax.vertical != nil && ax.first == first && ax.n == len(w.Zones) && ax.tw == w.TerrainW {
		return
	}
	ax.first, ax.n, ax.tw = first, len(w.Zones), w.TerrainW
	if ax.vertical == nil {
		ax.vertical = map[int32]struct{}{}
	}
	clear(ax.vertical)
	for i := range w.Zones {
		z := &w.Zones[i]
		if z.Kind != game.ZoneGrow || z.MaxY-z.MinY <= z.MaxX-z.MinX {
			continue
		}
		for _, c := range z.Cells {
			ax.vertical[int32(c[1]*w.TerrainW+c[0])] = struct{}{}
		}
	}
}

// cropVertical reports a crop cell in a grow zone whose long axis runs
// north to south (else the rows run east to west).
func (r *Renderer) cropVertical(w *game.WorldView, x, y int) bool {
	if len(w.Zones) == 0 || w.TerrainW <= 0 {
		return false
	}
	ax := &r.lod.crops.axes
	ax.sync(w)
	_, ok := ax.vertical[int32(y*w.TerrainW+x)]
	return ok
}

// cropAtlas is the crop atlas of the current art, created on first use.
func (r *Renderer) cropAtlas(dst *ebiten.Image) *cropAtlas {
	if r.art == nil {
		return nil
	}
	cb := &r.lod.crops
	if cb.atlas == nil || cb.atlas.art != r.art {
		var order []string
		if cb.atlas != nil {
			order = cb.atlas.order
		}
		r.flushCrops(dst) // pending units index the old atlas
		cb.atlas = buildCropAtlas(r.art, order)
	}
	return cb.atlas
}

// cropAtlasFor is the atlas with species in it: a species that has not
// drawn yet joins, flushing the pending units first (their coordinates
// are the old atlas's) and repacking.
func (r *Renderer) cropAtlasFor(dst *ebiten.Image, species string) *cropAtlas {
	a := r.cropAtlas(dst)
	if a == nil {
		return nil
	}
	if _, ok := a.sets[species]; !ok {
		r.flushCrops(dst)
		a = buildCropAtlas(r.art, append(slices.Clone(a.order), species))
		r.lod.crops.atlas = a
	}
	return a
}

// drawCropObj queues a crop tile's units: its stage's sprites (the
// wilted ones from cropWiltDamage), placed by cropUnits, sized by growth
// and damage, swayed with the wind and lit like a sprite. They draw when
// the batch is flushed (flushCrops).
func (r *Renderer) drawCropObj(dst *ebiten.Image, w *game.WorldView, p *game.PlantView, tile float64) {
	a := r.cropAtlasFor(dst, p.Species)
	if a == nil || a.img == nil {
		return
	}
	set := a.sets[p.Species]
	sprites := set.stages[cropStage(*p)]
	if len(sprites) == 0 {
		return
	}
	cb := &r.lod.crops
	n := cropUnitCount(p.Species)
	cb.units = cropUnits(p.X, p.Y, n, r.cropVertical(w, p.X, p.Y), cb.units[:0])
	units := cb.units
	sortCropUnits(units)
	gs := set.scale(cropGrowthStage(*p), p.Growth)
	tint := cropDamageTint(p.Damage, p.DamageKind)
	live := tile >= lodLiveMin
	repScale, repAlpha := 1.0, 1.0
	if !live { // one unit for the tile: the last to go, centred, larger
		last := units[0]
		var sum float64
		for _, u := range units {
			if u.rank > last.rank {
				last = u
			}
			_, al := cropUnitLook(p.Damage, u.rank, n)
			sum += al
		}
		last.dx, last.dy, last.trim = 0.5, 0.6, 1
		units = append(units[:0], last)
		repScale, _ = cropUnitLook(p.Damage, 0, n)
		repScale *= cropLODScale
		repAlpha = sum / float64(n)
	}

	ps := r.plantPose(p.X, p.Y, kindCrop, cropSwayAmount(p.Species), 1)
	lean, hs := -ps.skew, min(max(ps.sy, 0.5), 1.25)
	swayK := cropSwayFactor(p.Species)
	t := r.envTime()
	sky, lamp := r.lightTile(p.X, p.Y)
	tx, ty := r.worldToScreenF(float64(p.X), float64(p.Y))
	tl := float32(tile)
	fsw, fsh := float32(dst.Bounds().Dx()), float32(dst.Bounds().Dy())
	for i := range units {
		u := &units[i]
		sc, al := repScale, repAlpha
		if live {
			sc, al = cropUnitLook(p.Damage, u.rank, n)
		}
		if al < 1.0/255 {
			continue
		}
		sp := &a.sprites[sprites[int(u.variant)%len(sprites)]]
		ks := float32(gs*u.trim*sc) * tl * sp.k // screen px per atlas px
		fx, fy := tx+float32(u.dx)*tl, ty+float32(u.dy)*tl
		above, below := sp.fy*ks*float32(hs), (sp.h-sp.fy)*ks
		wq := sp.w * ks
		left := fx - sp.fx*ks
		if u.mirror {
			left = fx - (sp.w-sp.fx)*ks
		}
		l := min(max(lean*u.sway*swayK+cropWobble*swayK*math.Sin(t*2.1+u.phase), -cropLeanMax), cropLeanMax)
		tn := float32(l + l*l*l/3)
		shT, shB := tn*above, -tn*below
		top, bot := fy-above, fy+below
		if bot < 0 || top > fsh || max(left+shT, left+shB)+wq < 0 || min(left+shT, left+shB) > fsw {
			continue
		}
		if len(cb.verts) >= cropBatchQuads*4 {
			r.flushCrops(dst)
		}
		u0, u1 := sp.sx, sp.sx+sp.w
		if u.mirror {
			u0, u1 = u1, u0
		}
		a32 := float32(al)
		cr, cg, cbl := tint[0]*a32, tint[1]*a32, tint[2]*a32
		n0 := uint32(len(cb.verts))
		cb.verts = append(cb.verts,
			ebiten.Vertex{DstX: left + shT, DstY: top, SrcX: u0, SrcY: sp.sy, ColorR: cr, ColorG: cg, ColorB: cbl, ColorA: a32, Custom0: sky, Custom1: lamp},
			ebiten.Vertex{DstX: left + shT + wq, DstY: top, SrcX: u1, SrcY: sp.sy, ColorR: cr, ColorG: cg, ColorB: cbl, ColorA: a32, Custom0: sky, Custom1: lamp},
			ebiten.Vertex{DstX: left + shB, DstY: bot, SrcX: u0, SrcY: sp.sy + sp.h, ColorR: cr, ColorG: cg, ColorB: cbl, ColorA: a32, Custom0: sky, Custom1: lamp},
			ebiten.Vertex{DstX: left + shB + wq, DstY: bot, SrcX: u1, SrcY: sp.sy + sp.h, ColorR: cr, ColorG: cg, ColorB: cbl, ColorA: a32, Custom0: sky, Custom1: lamp})
		cb.idx = append(cb.idx, n0, n0+1, n0+2, n0+1, n0+3, n0+2)
		cb.quads++
	}
}

// flushCrops draws the pending units as one lit triangle batch (plain
// when the lit shader is missing). It is a no-op with nothing queued.
func (r *Renderer) flushCrops(dst *ebiten.Image) {
	cb := &r.lod.crops
	if len(cb.idx) == 0 {
		return
	}
	defer func() { cb.verts, cb.idx = cb.verts[:0], cb.idx[:0] }()
	if cb.atlas == nil || cb.atlas.img == nil {
		return
	}
	cb.flushes++
	r.drawLitBatch(dst, cb.verts, cb.idx, cb.atlas.img)
}

// drawLitBatch draws a triangle batch of vertices carrying per-vertex sky
// and lamp light through the lit shader.
func (r *Renderer) drawLitBatch(dst *ebiten.Image, verts []ebiten.Vertex, idx []uint32, src *ebiten.Image) {
	r.lit.calls++
	sh := r.litShader()
	if sh == nil { // no shader: unlit
		dst.DrawTriangles32(verts, idx, src, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear})
		return
	}
	r.lit.syncUniforms()
	r.lit.opts.Images[0] = src
	dst.DrawTrianglesShader32(verts, idx, sh, &r.lit.opts)
}

// --- furrows --------------------------------------------------------------------

// drawFurrows draws the tilled furrow decal under every crop cell in
// view (the ground pass, after the ground and before the grass): one lit
// batch, the decal chosen by the tile's hash, turned a quarter on a
// north-south field. Crops of the chunk lists in view, or of w when w is
// not the frame's world (tests).
func (r *Renderer) drawFurrows(dst *ebiten.Image, w game.WorldView, tile float64) {
	cb := &r.lod.crops
	cb.fverts, cb.fidx = cb.fverts[:0], cb.fidx[:0]
	cb.furrows = 0
	a := r.cropAtlas(dst)
	if a == nil || a.img == nil || len(a.furrows) == 0 {
		return
	}
	vx0, vy0, vx1, vy1 := r.visibleTiles()
	one := func(p *game.PlantView) {
		if p.X < vx0-1 || p.X > vx1+1 || p.Y < vy0-1 || p.Y > vy1+1 || !isCropPlant(p) {
			return
		}
		fs := &a.furrows[mix64(tileHash(p.X, p.Y)^0x66757272)%uint64(len(a.furrows))]
		cx, cy := r.worldToScreenF(float64(p.X)+0.5, float64(p.Y)+0.5)
		sc := float32(tile) * fs.k // screen px per atlas px
		vertical := r.cropVertical(&w, p.X, p.Y)
		sky, lamp := r.lightTile(p.X, p.Y)
		n0 := uint32(len(cb.fverts))
		for k := 0; k < 4; k++ {
			ox, oy := float32(k&1)*fs.w-fs.cx, float32(k>>1)*fs.h-fs.cy
			if vertical {
				ox, oy = -oy, ox
			}
			dx, dy := cx+ox*sc, cy+oy*sc
			cb.fverts = append(cb.fverts, ebiten.Vertex{DstX: dx, DstY: dy,
				SrcX: fs.sx + float32(k&1)*fs.w, SrcY: fs.sy + float32(k>>1)*fs.h,
				ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1, Custom0: sky, Custom1: lamp})
		}
		cb.fidx = append(cb.fidx, n0, n0+1, n0+2, n0+1, n0+3, n0+2)
		cb.furrows++
	}
	if r.frameLists(&w) {
		for _, cv := range r.lod.views {
			for _, i := range cv.Plants {
				if int(i) < len(w.Plants) {
					one(&w.Plants[i])
				}
			}
		}
	} else {
		for i := range w.Plants {
			one(&w.Plants[i])
		}
	}
	if len(cb.fidx) > 0 {
		r.drawLitBatch(dst, cb.fverts, cb.fidx, a.img)
	}
}
