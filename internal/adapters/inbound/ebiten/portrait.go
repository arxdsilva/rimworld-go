package ebiten

import (
	"image"
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/domain/pawn/look"
)

// A renderer-free pawn portrait (NG-1 §2.9): the layered colonist facing
// south, cropped to head and torso, drawn from the art catalog alone so
// the character screens need no Renderer. The layer rules follow the
// colony renderer's (draw_art.go pawnLayers) for the six builds.

// portraitCrop is the part of the 160x297 pawn canvas the portrait shows:
// head and torso.
var portraitCrop = image.Rect(20, 72, 140, 232)

// portraitLayer is one sprite of the portrait and its tint (zero: none).
type portraitLayer struct {
	key  string
	tint color.NRGBA
}

// pawnKey is the art key of a pawn layer for a build and facing. The male
// average build keeps the original unsuffixed keys.
func pawnKey(layer string, l look.Look, facing string, built bool) string {
	if !built || l.BuildKey() == "m_avg" {
		return "pawn/" + layer + "_" + facing
	}
	return "pawn/" + layer + "_" + l.BuildKey() + "_" + facing
}

// clothTints are the cloth presets a look indexes (the raider tint is
// reserved for hostile clothing).
func (c *artCatalog) clothTints() []string {
	var out []string
	for _, n := range c.tintNames("cloth") {
		if n != raiderCloth {
			out = append(out, n)
		}
	}
	return out
}

// pick resolves palette index i of names (i % len, as the look contract says).
func pick(names []string, i uint8) string {
	if len(names) == 0 {
		return ""
	}
	return names[int(i)%len(names)]
}

// portraitLayers lists the portrait's sprites in draw order for a look.
func portraitLayers(art *artCatalog, l look.Look) []portraitLayer {
	return scarredLayers(art, l, false)
}

// scarredLayers is portraitLayers; scarred adds the face scar over the
// hair (a disfigured pawn).
func scarredLayers(art *artCatalog, l look.Look, scarred bool) []portraitLayer {
	skin := art.tint("skin", pick(art.tintNames("skin"), l.Skin))
	cloths := art.clothTints()
	cloth := func(i uint8) color.NRGBA { return art.tint("cloth", pick(cloths, i)) }
	hairNames := art.tintNames("hair")
	hairName := pick(hairNames, l.Hair)
	if l.Grey {
		hairName = "grey"
	}
	hair := art.tint("hair", hairName)
	// A build whose art is missing falls back to the unsuffixed keys.
	built := art.has(pawnKey("body", l, "south", true))
	key := func(layer string) string { return pawnKey(layer, l, "south", built) }

	out := []portraitLayer{{key("body"), skin}}
	if !l.Bare {
		out = append(out,
			portraitLayer{key("pants_basic"), cloth(l.Pants)},
			portraitLayer{key("shirt_basic"), cloth(l.Shirt)})
		if l.Vest {
			out = append(out, portraitLayer{key("vest_utility"), cloth(l.VestCloth)})
		}
	}
	if l.Beard > 0 && int(l.Beard) <= len(look.Beards) {
		out = append(out, portraitLayer{"pawn/beard_" + look.Beards[l.Beard-1] + "_south", hair})
	}
	if n := len(look.HairStyles); n > 0 {
		out = append(out, portraitLayer{"pawn/hair_" + look.HairStyles[int(l.HairStyle)%n] + "_south", hair})
	}
	if scarred {
		out = append(out, portraitLayer{"pawn/face_scar_south", color.NRGBA{}})
	}
	return out
}

// drawLookPortrait draws the pawn with look l fitted into the box (x, y,
// w, h) of dst. It reports whether anything was drawn (false while the art
// is missing, so the caller can draw a stand-in).
func drawLookPortrait(dst *ebiten.Image, art *artCatalog, l look.Look, x, y, w, h float64) bool {
	return drawPortrait(dst, art, l, false, x, y, w, h)
}

// drawPortrait is drawLookPortrait with the face scar of a disfigured
// pawn (scarred).
func drawPortrait(dst *ebiten.Image, art *artCatalog, l look.Look, scarred bool, x, y, w, h float64) bool {
	if art == nil || !art.has(pawnKey("body", l, "south", false)) {
		return false
	}
	cw, ch := float64(portraitCrop.Dx()), float64(portraitCrop.Dy())
	s := min(w/cw, h/ch)
	ox, oy := x+(w-cw*s)/2, y+(h-ch*s)/2
	drew := false
	for _, ly := range scarredLayers(art, l, scarred) {
		img := art.image(ly.key, ly.tint)
		if img == nil {
			continue
		}
		b := img.Bounds()
		// The art is @2x of a 160x297 canvas; the crop is in canvas px.
		k := float64(b.Dx()) / 160
		crop := image.Rect(int(float64(portraitCrop.Min.X)*k), int(float64(portraitCrop.Min.Y)*k),
			int(float64(portraitCrop.Max.X)*k), int(float64(portraitCrop.Max.Y)*k)).Intersect(b)
		if crop.Empty() {
			continue
		}
		sub := img.SubImage(crop).(*ebiten.Image)
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.GeoM.Scale(s/k, s/k)
		op.GeoM.Translate(ox+float64(crop.Min.X-int(float64(portraitCrop.Min.X)*k))*s/k, oy+float64(crop.Min.Y-int(float64(portraitCrop.Min.Y)*k))*s/k)
		dst.DrawImage(sub, op)
		drew = true
	}
	return drew
}
