package ebiten

import (
	"image/color"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// Stone, ore and metal presentation (ART-015, SIM-029). Stone art is
// tint-neutral; the renderer multiplies each stone's tint (palette group
// stone_tint) as a colour scale, or bakes it through the sprite's tint
// mask (ore). Materials arrive as item names: "wood", "granite blocks",
// "steel", and "stone" from read models made before the migration.

// stoneNames and oreNames map WorldView.StoneType / Ore codes to names.
var (
	stoneNames = game.StoneNames()
	oreNames   = game.OreNames()
)

// defaultStone is the stone of rock and legacy stuff with no stone data.
const defaultStone = "granite"

// isStoneStuff reports a stone building material: cut blocks, or the
// legacy "stone".
func isStoneStuff(stuff string) bool {
	return stuff == "stone" || strings.HasSuffix(stuff, " blocks")
}

// stuffStone is the stone of a stone material ("granite" for "granite
// blocks" and the legacy "stone"), "" for anything else.
func stuffStone(stuff string) string {
	switch {
	case stuff == "stone":
		return defaultStone
	case strings.HasSuffix(stuff, " blocks"):
		return strings.TrimSuffix(stuff, " blocks")
	}
	return ""
}

// stuffMaterial is the art material of a building stuff: wood, stone or
// steel.
func stuffMaterial(stuff string) string {
	switch {
	case isStoneStuff(stuff):
		return "stone"
	case stuff == "steel":
		return "steel"
	}
	return "wood"
}

// stuffAdjective names a non-wood material for titles: "Granite",
// "Steel", "Stone" (legacy); "" for wood.
func stuffAdjective(stuff string) string {
	if s := stuffStone(stuff); s != "" {
		if stuff == "stone" {
			return "Stone"
		}
		return capitalize(s)
	}
	if stuff == "steel" {
		return "Steel"
	}
	return ""
}

// materialTitle is "Granite wall" / "Steel door" / "Wall".
func materialTitle(kind, stuff string) string {
	if adj := stuffAdjective(stuff); adj != "" {
		return adj + " " + kind
	}
	return capitalize(kind)
}

// wallPrefix is the linked wall family for a material.
func wallPrefix(stuff string) string {
	switch stuffMaterial(stuff) {
	case "stone":
		return "structure/wall_stone"
	case "steel":
		return "structure/wall_steel"
	}
	return "structure/wall_wood"
}

// rockPrefix is the linked rock family for a rock terrain and its ore.
func rockPrefix(terrain, ore string) string {
	if terrain == "rock_smooth" {
		return "rock/rock_smooth"
	}
	switch ore {
	case "steel":
		return "rock/ore_steel"
	case "silver":
		return "rock/ore_silver"
	case "gold":
		return "rock/ore_gold"
	case "machinery":
		return "rock/ore_machinery"
	}
	return "rock/rock_wall"
}

// linkFallback is the family drawn when a family's art is missing.
var linkFallback = map[string]string{
	"rock/rock_smooth":     "rock/rock_wall",
	"rock/ore_steel":       "rock/rock_wall",
	"rock/ore_silver":      "rock/rock_wall",
	"rock/ore_gold":        "rock/rock_wall",
	"rock/ore_machinery":   "rock/rock_wall",
	"structure/wall_steel": "structure/wall_stone",
}

// stoneArtCache memoises stone tints and resolved linked keys for one
// art catalog.
type stoneArtCache struct {
	art    *artCatalog
	stones []color.NRGBA // by StoneType code
	stuff  map[string]color.NRGBA
	keys   map[string]*[256]linkKeyEntry
	tiles  [][soilVariants]tileKeyEntry // ground tile per terrain code (draw_art.go)
}

type tileKeyEntry struct {
	key  string
	cs   colorScale
	done bool
}

type linkKeyEntry struct {
	key  string
	done bool
}

func (r *Renderer) stoneCache() *stoneArtCache {
	c := &r.stoneArt
	if c.art != r.art || c.stones == nil {
		*c = stoneArtCache{art: r.art, stones: make([]color.NRGBA, len(stoneNames)),
			stuff: map[string]color.NRGBA{}, keys: map[string]*[256]linkKeyEntry{}}
		for i, n := range stoneNames {
			if n == "" {
				n = defaultStone
			}
			c.stones[i] = r.art.Tint("stone_tint", n)
		}
	}
	return c
}

// stoneTint is the tint of a cell's stone (StoneType code); no stone data
// draws as granite.
func (r *Renderer) stoneTint(code uint8) color.NRGBA {
	c := r.stoneCache()
	if int(code) >= len(c.stones) {
		code = 0
	}
	return c.stones[code]
}

// stoneTintAt is the stone tint of tile (x, y) of w.
func (r *Renderer) stoneTintAt(w game.WorldView, x, y int) color.NRGBA {
	var code uint8
	if w.TerrainW > 0 && x >= 0 && y >= 0 && x < w.TerrainW {
		if i := y*w.TerrainW + x; i < len(w.StoneType) {
			code = w.StoneType[i]
		}
	}
	return r.stoneTint(code)
}

// stuffTint is a material's stone tint (zero for wood, steel and the rest).
func (r *Renderer) stuffTint(stuff string) color.NRGBA {
	c := r.stoneCache()
	if t, ok := c.stuff[stuff]; ok {
		return t
	}
	var t color.NRGBA
	if s := stuffStone(stuff); s != "" {
		t = r.art.Tint("stone_tint", s)
	}
	c.stuff[stuff] = t
	return t
}

// tintCS converts a tint to a multiplying colour scale (identity for none).
func tintCS(t color.NRGBA) colorScale {
	if t.A == 0 {
		return noCS
	}
	return colorScale{float32(t.R) / 255, float32(t.G) / 255, float32(t.B) / 255, 1}
}

// mulCS multiplies two colour scales channel by channel.
func mulCS(a, b colorScale) colorScale {
	return colorScale{a[0] * b[0], a[1] * b[1], a[2] * b[2], a[3] * b[3]}
}

// linkKey resolves the sprite drawn for a linked family at (mask,
// corners): the baked corner variant, else the plain mask variant, else
// the family's fallback, else "" (nothing in the catalog). Cached.
func (r *Renderer) linkKey(prefix string, mask, corners int) string {
	c := r.stoneCache()
	tab := c.keys[prefix]
	if tab == nil {
		tab = &[256]linkKeyEntry{}
		c.keys[prefix] = tab
	}
	i := (mask&15)<<4 | corners&15
	if e := tab[i]; e.done {
		return e.key
	}
	key := ""
	for p := prefix; p != "" && key == ""; p = linkFallback[p] {
		if k := linkedKey(p, mask, corners); r.art.Has(k) {
			key = k
		} else if k := p + "_" + twoDigits(mask&15); r.art.Has(k) {
			key = k
		}
	}
	tab[i] = linkKeyEntry{key: key, done: true}
	return key
}

// linkedDraw is one queued linked sprite (no closure: mountains queue
// hundreds of them per frame).
type linkedDraw struct {
	key  string
	x, y int
	tint color.NRGBA
	cs   colorScale
}

// linkedSprite prepares a linked draw of prefix at (x, y): the stone tint
// goes through the sprite's tint mask when it has one (ore), else it
// multiplies cs. ok is false when the catalog has no art for it.
func (r *Renderer) linkedSprite(prefix string, x, y, mask, corners int, tint color.NRGBA, cs colorScale) (linkedDraw, bool) {
	key := r.linkKey(prefix, mask, corners)
	if key == "" {
		return linkedDraw{}, false
	}
	if tint.A != 0 {
		if sp := r.art.Sprite(key); sp == nil || sp.Mask == nil {
			cs, tint = mulCS(tintCS(tint), cs), color.NRGBA{}
		}
	}
	return linkedDraw{key: key, x: x, y: y, tint: tint, cs: cs}, true
}

func (r *Renderer) drawLinkedSprite(screen *ebiten.Image, d linkedDraw, tile float64, sw, sh int) {
	r.drawSpriteScaled(screen, d.key, d.tint, float64(d.x), float64(d.y), tile, sw, sh, d.cs, 1)
}

// drawLinked draws a linked family sprite now; false when there is no art.
func (r *Renderer) drawLinked(screen *ebiten.Image, prefix string, x, y, mask, corners int, tint color.NRGBA, cs colorScale, tile float64, sw, sh int) bool {
	d, ok := r.linkedSprite(prefix, x, y, mask, corners, tint, cs)
	if ok {
		r.drawLinkedSprite(screen, d, tile, sw, sh)
	}
	return ok
}

// defaultStoneStuff is the stone material the Architect offers first: the
// first stone blocks (enum order) with free stock, else granite blocks.
func defaultStoneStuff(w game.WorldView) string {
	for _, s := range stoneNames {
		if s == "" {
			continue
		}
		if k := s + " blocks"; stockOf(w, k) > 0 {
			return k
		}
	}
	return defaultStone + " blocks"
}
