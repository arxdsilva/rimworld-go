package ebiten

import (
	"image/color"
	"math"
	"sort"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
)

// Draw path for the original generated art. Order (assets/AGENTS.md):
// terrain -> zones/blueprints -> objects north-to-south -> overlays.

const (
	soilVariants = 4
	pawnBobPx    = 0.035 // of a tile
	grassEvery   = 11
)

type artDraw struct {
	row, col float64
	fn       func()
}

// colorScale is a per-draw RGBA multiplier (ghosts, blueprints).
type colorScale [4]float32

var (
	noCS        = colorScale{1, 1, 1, 1}
	blueprintCS = colorScale{0.55, 0.85, 1.25, 0.5}
	ghostOKCS   = colorScale{0.7, 1.25, 0.7, 0.65}
	ghostBadCS  = colorScale{1.3, 0.45, 0.4, 0.6}
)

func (r *Renderer) drawSprite(screen *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, sw, sh int) {
	r.drawSpriteCS(screen, key, tint, wx, wy, tile, sw, sh, noCS)
}

func (r *Renderer) drawSpriteCS(screen *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, sw, sh int, cs colorScale) {
	sp := r.art.sprite(key)
	if sp == nil {
		return
	}
	sx, sy := r.worldToScreenF(wx, wy)
	x, y, s := artOrigin(sp.meta, float64(sx), float64(sy), tile)
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
	if cs != noCS {
		op.ColorScale.Scale(cs[0]*cs[3], cs[1]*cs[3], cs[2]*cs[3], cs[3])
	}
	screen.DrawImage(img, op)
}

// drawPlacementGhost previews a build tool: the real sprite per cell,
// green when placeable, red when not. Walls autotile among the ghosts.
func (r *Renderer) drawPlacementGhost(screen *ebiten.Image, kind, facing string, cells []ghostCell) {
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	tile := r.tilePx()
	set := map[[2]int]bool{}
	for _, c := range cells {
		set[[2]int{c.x, c.y}] = true
	}
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
		mask := wallMask(set, c.x, c.y)
		key := buildSprite(kind, facing, mask, mask&(wallE|wallW) != 0, false)
		r.drawSpriteCS(screen, key, color.NRGBA{}, float64(c.x), float64(c.y), tile, sw, sh, cs)
	}
}

func (r *Renderer) drawArt(screen *ebiten.Image, sw, sh int, tile float64) {
	none := color.NRGBA{}
	w := r.game.World()

	// terrain: the domain has no terrain types yet, so soil variants only
	for y := 0; y < r.game.Height(); y++ {
		for x := 0; x < r.game.Width(); x++ {
			h := tileHash(x, y)
			r.drawSprite(screen, artKey("terrain", "soil", int(h%soilVariants)), none, float64(x), float64(y), tile, sw, sh)
		}
	}
	for y := 0; y < r.game.Height(); y++ {
		for x := 0; x < r.game.Width(); x++ {
			if h := tileHash(x, y) >> 8; h%grassEvery == 0 {
				r.drawSprite(screen, artKey("plant", "grass", int(h/grassEvery%2)), none, float64(x), float64(y), tile, sw, sh)
			}
		}
	}
	for _, z := range w.Stockpiles {
		for y := z.MinY; y <= z.MaxY; y++ {
			for x := z.MinX; x <= z.MaxX; x++ {
				r.drawSprite(screen, "overlay/stockpile", none, float64(x), float64(y), tile, sw, sh)
			}
		}
	}
	for _, f := range w.Frames {
		r.drawSprite(screen, "overlay/blueprint", none, float64(f.X), float64(f.Y), tile, sw, sh)
	}

	// floors (and planned floors) sit under everything else
	for _, b := range w.Buildings {
		if b.Floor {
			r.drawSprite(screen, "structure/floor_wood", none, float64(b.X), float64(b.Y), tile, sw, sh)
		}
	}
	for _, f := range w.Frames {
		if f.Floor {
			r.drawSpriteCS(screen, "structure/floor_wood", none, float64(f.X), float64(f.Y), tile, sw, sh, blueprintCS)
		}
	}

	walls := map[[2]int]bool{}
	for _, b := range w.Buildings {
		if b.Kind == "wall" || b.Kind == "door" {
			walls[[2]int{b.X, b.Y}] = true
		}
	}
	occupied := map[[2]int]bool{}
	for _, p := range r.game.Snapshot() {
		occupied[[2]int{p.X, p.Y}] = true
	}
	var draws []artDraw
	add := func(row, col float64, fn func()) { draws = append(draws, artDraw{row, col, fn}) }
	cloth := r.art.tintNames("cloth")
	for _, b := range w.Buildings {
		b := b
		if b.Floor {
			continue
		}
		mask := wallMask(walls, b.X, b.Y)
		key := buildSprite(b.Kind, b.Facing, mask, mask&(wallE|wallW) != 0, occupied[[2]int{b.X, b.Y}])
		t := none
		if b.Kind == "bed" && len(cloth) > 0 {
			t = r.art.tint("cloth", cloth[mix64(uint64(b.ID))%uint64(len(cloth))])
		}
		add(float64(b.Y), float64(b.X), func() {
			r.drawSprite(screen, key, t, float64(b.X), float64(b.Y), tile, sw, sh)
			if b.Deconstruct {
				r.drawSprite(screen, "overlay/designation_deconstruct", none, float64(b.X), float64(b.Y), tile, sw, sh)
			}
		})
	}
	// planned walls autotile with each other and with built walls
	planned := map[[2]int]bool{}
	for k := range walls {
		planned[k] = true
	}
	for _, f := range w.Frames {
		if f.Kind == "wall" || f.Kind == "door" {
			planned[[2]int{f.X, f.Y}] = true
		}
	}
	for _, f := range w.Frames {
		f := f
		if f.Floor {
			continue
		}
		pm := wallMask(planned, f.X, f.Y)
		key := buildSprite(f.Kind, f.Facing, pm, pm&(wallE|wallW) != 0, false)
		add(float64(f.Y), float64(f.X), func() {
			if f.Wood > 0 {
				r.drawSprite(screen, "structure/frame_1x1", none, float64(f.X), float64(f.Y), tile, sw, sh)
			}
			r.drawSpriteCS(screen, key, none, float64(f.X), float64(f.Y), tile, sw, sh, blueprintCS)
		})
	}
	for _, p := range w.Plants {
		p := p
		name := artKey("plant", "bush", int(tileHash(p.X, p.Y)%2))
		if p.Harvestable {
			name += "_harvestable"
		}
		add(float64(p.Y), float64(p.X), func() {
			r.drawSprite(screen, name, none, float64(p.X), float64(p.Y), tile, sw, sh)
		})
	}
	for _, it := range w.Items {
		it := it
		if it.Carried {
			continue
		}
		key := itemSprite(it.Kind)
		add(float64(it.Y), float64(it.X), func() {
			if !r.art.has(key) {
				pad := float32(tile) * 0.3
				sx, sy := r.worldToScreen(it.X, it.Y)
				vector.FillRect(screen, sx+pad, sy+pad, float32(tile)-2*pad, float32(tile)-2*pad, color.RGBA{200, 90, 40, 255}, false)
				return
			}
			r.drawSprite(screen, key, none, float64(it.X), float64(it.Y), tile, sw, sh)
		})
	}
	for _, p := range r.game.Snapshot() {
		p := p
		wx, wy := r.interpWorld(p)
		add(wy+0.01, wx, func() { r.drawPawnArt(screen, p, wx, wy, tile, sw, sh) })
	}
	sort.SliceStable(draws, func(i, j int) bool {
		if draws[i].row != draws[j].row {
			return draws[i].row < draws[j].row
		}
		return draws[i].col < draws[j].col
	})
	for _, d := range draws {
		d.fn()
	}

	for _, p := range w.Plants {
		if p.Designated {
			r.drawSprite(screen, "overlay/designation_harvest", none, float64(p.X), float64(p.Y), tile, sw, sh)
		}
	}
	if r.hasSelect {
		for _, p := range r.game.Snapshot() {
			if p.ID == r.selected {
				wx, wy := r.interpWorld(p)
				r.drawSprite(screen, "overlay/selection", none, wx, wy, tile, sw, sh)
			}
		}
	}
}

func itemSprite(kind string) string {
	switch kind {
	case "wood":
		return "item/wood_pile"
	case "food":
		return "item/food_crate"
	case "steel":
		return "item/steel_bars"
	case "stone", "stone_blocks":
		return "item/stone_blocks"
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
	return lookFor(uint64(p.ID), r.art.tintNames("skin"), r.art.tintNames("hair"), r.art.tintNames("cloth"))
}

// pawnLayers lists (sprite key, tint) in draw order for one facing.
func (r *Renderer) pawnLayers(l pawnLook, f facing) []struct {
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
	out = append(out, struct {
		key  string
		tint color.NRGBA
	}{"pawn/" + l.HairStyle + "_" + fs, r.art.tint("hair", l.Hair)})
	return out
}

func (r *Renderer) drawPawnArt(screen *ebiten.Image, p game.PawnView, wx, wy, tile float64, sw, sh int) {
	vis := r.last[p.ID]
	moving := vis.fromX != p.X || vis.fromY != p.Y || walkHold(r.present, vis.lastMove, walkSustainFrames)
	bob := 0.0
	if moving {
		bob = -math.Abs(math.Sin(float64(r.present)*math.Pi/float64(walkAnimPeriod))) * pawnBobPx
	}
	for _, ly := range r.pawnLayers(r.look(p), vis.facing) {
		r.drawSprite(screen, ly.key, ly.tint, wx, wy+bob, tile, sw, sh)
	}
	if p.Drafted {
		if img := r.art.image("ui_icon/draft", color.NRGBA{}); img != nil {
			sx, sy := r.worldToScreenF(wx+0.62, wy-0.95)
			drawIcon(screen, img, float64(sx), float64(sy), tile*0.3)
		}
	}
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
	for _, ly := range r.pawnLayers(r.look(p), faceSouth) {
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
