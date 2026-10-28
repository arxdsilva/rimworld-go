package sprite

import (
	"image/color"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
)

// Stone, ore and metal presentation (ART-015, SIM-029). Stone art is
// tint-neutral; the painter multiplies each stone's tint (palette group
// stone_tint) as a colour scale, or bakes it through the sprite's tint
// mask (ore). Materials arrive as item names: "wood", "granite blocks",
// "steel", and "stone" from read models made before the migration.

// StoneNames and OreNames map WorldView.StoneType / Ore codes to names.
var (
	StoneNames = game.StoneNames()
	OreNames   = game.OreNames()
)

// DefaultStone is the stone of rock and legacy stuff with no stone data.
const DefaultStone = "granite"

// IsStoneStuff reports a stone building material: cut blocks, or the
// legacy "stone".
func IsStoneStuff(stuff string) bool {
	return stuff == "stone" || strings.HasSuffix(stuff, " blocks")
}

// StuffStone is the stone of a stone material ("granite" for "granite
// blocks" and the legacy "stone"), "" for anything else.
func StuffStone(stuff string) string {
	switch {
	case stuff == "stone":
		return DefaultStone
	case strings.HasSuffix(stuff, " blocks"):
		return strings.TrimSuffix(stuff, " blocks")
	}
	return ""
}

// StuffMaterial is the art material of a building stuff: wood, stone or
// steel.
func StuffMaterial(stuff string) string {
	switch {
	case IsStoneStuff(stuff):
		return "stone"
	case stuff == "steel":
		return "steel"
	}
	return "wood"
}

// WallPrefix is the linked wall family for a material.
func WallPrefix(stuff string) string {
	switch StuffMaterial(stuff) {
	case "stone":
		return "structure/wall_stone"
	case "steel":
		return "structure/wall_steel"
	}
	return "structure/wall_wood"
}

// RockPrefix is the linked rock family for a rock terrain and its ore.
func RockPrefix(terrain, ore string) string {
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

// LinkFallback is the family drawn when a family's art is missing.
var LinkFallback = map[string]string{
	"rock/rock_smooth":     "rock/rock_wall",
	"rock/ore_steel":       "rock/rock_wall",
	"rock/ore_silver":      "rock/rock_wall",
	"rock/ore_gold":        "rock/rock_wall",
	"rock/ore_machinery":   "rock/rock_wall",
	"structure/wall_steel": "structure/wall_stone",
}

// StoneCache memoises stone tints and resolved linked keys for one art
// catalog; a Painter on another catalog resets it.
type StoneCache struct {
	art    *art.Catalog
	stones []color.NRGBA // by StoneType code
	stuff  map[string]color.NRGBA
	keys   map[string]*[256]linkKeyEntry
}

type linkKeyEntry struct {
	key  string
	done bool
}

// sync points the cache at catalog c, starting over on a new catalog.
func (s *StoneCache) sync(c *art.Catalog) *StoneCache {
	if s.art != c || s.stones == nil {
		*s = StoneCache{art: c, stones: make([]color.NRGBA, len(StoneNames)),
			stuff: map[string]color.NRGBA{}, keys: map[string]*[256]linkKeyEntry{}}
		for i, n := range StoneNames {
			if n == "" {
				n = DefaultStone
			}
			s.stones[i] = c.Tint("stone_tint", n)
		}
	}
	return s
}

// stones is the painter's stone cache, synced to its catalog.
func (p *Painter) stones() *StoneCache { return p.Stone.sync(p.Art) }

// StoneTint is the tint of a cell's stone (StoneType code); no stone data
// draws as granite.
func (p *Painter) StoneTint(code uint8) color.NRGBA {
	c := p.stones()
	if int(code) >= len(c.stones) {
		code = 0
	}
	return c.stones[code]
}

// StoneTintAt is the stone tint of tile (x, y) of w.
func (p *Painter) StoneTintAt(w *game.WorldView, x, y int) color.NRGBA {
	var code uint8
	if w.TerrainW > 0 && x >= 0 && y >= 0 && x < w.TerrainW {
		if i := y*w.TerrainW + x; i < len(w.StoneType) {
			code = w.StoneType[i]
		}
	}
	return p.StoneTint(code)
}

// StuffTint is a material's stone tint (zero for wood, steel and the rest).
func (p *Painter) StuffTint(stuff string) color.NRGBA {
	c := p.stones()
	if t, ok := c.stuff[stuff]; ok {
		return t
	}
	var t color.NRGBA
	if s := StuffStone(stuff); s != "" {
		t = p.Art.Tint("stone_tint", s)
	}
	c.stuff[stuff] = t
	return t
}

// LinkKey resolves the sprite drawn for a linked family at (mask,
// corners): the baked corner variant, else the plain mask variant, else
// the family's fallback, else "" (nothing in the catalog). Cached.
func (p *Painter) LinkKey(prefix string, mask, corners int) string {
	c := p.stones()
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
	for f := prefix; f != "" && key == ""; f = LinkFallback[f] {
		if k := LinkedKey(f, mask, corners); p.Art.Has(k) {
			key = k
		} else if k := f + "_" + TwoDigits(mask&15); p.Art.Has(k) {
			key = k
		}
	}
	tab[i] = linkKeyEntry{key: key, done: true}
	return key
}

// LinkedDraw is one queued linked sprite (no closure: mountains queue
// hundreds of them per frame).
type LinkedDraw struct {
	key  string
	x, y int
	tint color.NRGBA
	cs   ColorScale
}

// LinkedSprite prepares a linked draw of prefix at (x, y): the stone tint
// goes through the sprite's tint mask when it has one (ore), else it
// multiplies cs. ok is false when the catalog has no art for it.
func (p *Painter) LinkedSprite(prefix string, x, y, mask, corners int, tint color.NRGBA, cs ColorScale) (LinkedDraw, bool) {
	key := p.LinkKey(prefix, mask, corners)
	if key == "" {
		return LinkedDraw{}, false
	}
	if tint.A != 0 {
		if sp := p.Art.Sprite(key); sp == nil || sp.Mask == nil {
			cs, tint = MulCS(TintCS(tint), cs), color.NRGBA{}
		}
	}
	return LinkedDraw{key: key, x: x, y: y, tint: tint, cs: cs}, true
}

// DrawLinkedSprite draws a prepared linked sprite in pass ps.
func (p *Painter) DrawLinkedSprite(v *render.View, ps Pass, dst *ebiten.Image, d LinkedDraw, tile float64, sw, sh int) {
	p.DrawScaled(v, ps, dst, d.key, d.tint, float64(d.x), float64(d.y), tile, sw, sh, d.cs, 1)
}

// DrawLinked draws a linked family sprite now; false when there is no art.
func (p *Painter) DrawLinked(v *render.View, ps Pass, dst *ebiten.Image, prefix string, x, y, mask, corners int, tint color.NRGBA, cs ColorScale, tile float64, sw, sh int) bool {
	d, ok := p.LinkedSprite(prefix, x, y, mask, corners, tint, cs)
	if ok {
		p.DrawLinkedSprite(v, ps, dst, d, tile, sw, sh)
	}
	return ok
}
