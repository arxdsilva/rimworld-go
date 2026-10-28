package ebiten

import (
	"image"
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/look"
)

// A renderer-free pawn portrait (NG-1 §2.9): the layered colonist facing
// south, cropped to the figure, drawn from the art catalog alone so the
// character screens need no Renderer. The colony figure draws from the
// same layer list (figureLayers) for the six builds.

// Portrait crops, as fractions of the figure's height from its head
// centre to its feet (H): the crop's top sits headroomK·H above the head
// centre (room for hair and a hat); the portrait ends footK·H below the
// feet, the bust bustK·H below the head centre (the shoulders). Widths
// follow from the boxes' aspect (w/h).
const (
	headroomK      = 0.55
	footK          = 0.10
	bustK          = 0.45
	portraitAspect = 0.75
	bustAspect     = 0.88
)

// pawnCrops are the parts of the south body's canvas (px) the portrait
// (the whole figure) and the colonist bar's bust (head and shoulders)
// show, from its metadata: the head attachment, origin_px and the feet
// (pawnFeetY). Without a head attachment the whole canvas is shown.
func pawnCrops(body *artSprite) (portrait, bust image.Rectangle) {
	if body == nil {
		return image.Rectangle{}, image.Rectangle{}
	}
	m := body.Meta
	head, ok := m.Attach["head"]
	if !ok {
		full := image.Rect(0, 0, m.Canvas[0], m.Canvas[1])
		return full, full
	}
	feet := m.Origin[1] + pawnFeetY*artTilePx
	h := feet - head[1]
	box := func(top, bottom, aspect float64) image.Rectangle {
		w := (bottom - top) * aspect
		return image.Rect(int(math.Round(head[0]-w/2)), int(math.Round(top)), int(math.Round(head[0]+w/2)), int(math.Round(bottom)))
	}
	top := head[1] - headroomK*h
	return box(top, feet+footK*h, portraitAspect), box(top, head[1]+bustK*h, bustAspect)
}

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
	for _, n := range c.TintNames("cloth") {
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
	return figureLayers(art, l, "south", figureDress{scarred: scarred})
}

// figureDress is what a figure wears beyond its look: outer apparel
// (parka, duster, tuque), the face scar, and raider clothing.
type figureDress struct {
	apparel []string
	scarred bool
	raider  bool // hostile: maroon shirt and vest, slate trousers
}

// figureLayers lists a pawn's sprites bottom first for look l in art
// facing fs ("south", "east" or "north"): the build's body and clothes,
// outer apparel, beard, hair, the face scar, then a hat. The colony
// figure, the portraits and the colonist bar all draw from it, so the
// look built on the pawns screen is the one walking in the colony.
func figureLayers(art *artCatalog, l look.Look, fs string, d figureDress) []portraitLayer {
	skin := art.Tint("skin", pick(art.TintNames("skin"), l.Skin))
	cloths := art.clothTints()
	cloth := func(i uint8) color.NRGBA { return art.Tint("cloth", pick(cloths, i)) }
	shirt, pants, vest := cloth(l.Shirt), cloth(l.Pants), cloth(l.VestCloth)
	if d.raider {
		l.Bare, l.Vest = false, true
		shirt, vest, pants = art.Tint("cloth", raiderCloth), art.Tint("cloth", raiderCloth), art.Tint("cloth", "slate")
	}
	hairName := pick(art.TintNames("hair"), l.Hair)
	if l.Grey {
		hairName = "grey"
	}
	hair := art.Tint("hair", hairName)
	// A build whose art is missing falls back to the unsuffixed keys.
	built := art.Has(pawnKey("body", l, fs, true))
	key := func(layer string) string { return pawnKey(layer, l, fs, built) }

	out := make([]portraitLayer, 0, 9)
	out = append(out, portraitLayer{key("body"), skin})
	if !l.Bare {
		out = append(out, portraitLayer{key("pants_basic"), pants}, portraitLayer{key("shirt_basic"), shirt})
		if l.Vest {
			out = append(out, portraitLayer{key("vest_utility"), vest})
		}
	}
	hat := ""
	for _, a := range d.apparel { // outer garments go over shirt and vest
		switch a {
		case "parka":
			out = append(out, portraitLayer{key("parka_leather"), color.NRGBA{}})
		case "wool parka": // the leather parka layer, tinted cream
			out = append(out, portraitLayer{key("parka_leather"), color.NRGBA{R: 238, G: 228, B: 200, A: 255}})
		case "duster":
			out = append(out, portraitLayer{key("duster_leather"), color.NRGBA{}})
		case "tuque":
			hat = "pawn/tuque_knit_" + fs
		}
	}
	if l.Beard > 0 && int(l.Beard) <= len(look.Beards) {
		out = append(out, portraitLayer{"pawn/beard_" + look.Beards[l.Beard-1] + "_" + fs, hair})
	}
	if n := len(look.HairStyles); n > 0 {
		out = append(out, portraitLayer{"pawn/hair_" + look.HairStyles[int(l.HairStyle)%n] + "_" + fs, hair})
	}
	if d.scarred {
		if k := "pawn/face_scar_" + fs; art.Has(k) {
			out = append(out, portraitLayer{k, color.NRGBA{}})
		}
	}
	if hat != "" {
		out = append(out, portraitLayer{hat, pants})
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
	if art == nil || !art.Has(pawnKey("body", l, "south", false)) {
		return false
	}
	body := art.Sprite(pawnKey("body", l, "south", false))
	portraitCrop, _ := pawnCrops(body)
	cw, ch := float64(portraitCrop.Dx()), float64(portraitCrop.Dy())
	s := min(w/cw, h/ch)
	ox, oy := x+(w-cw*s)/2, y+(h-ch*s)/2
	drew := false
	for _, ly := range scarredLayers(art, l, scarred) {
		img := art.Image(ly.key, ly.tint)
		if img == nil {
			continue
		}
		b := img.Bounds()
		// The crop is in canvas px; the image may be a multiple of it.
		k := float64(b.Dx()) / float64(body.Meta.Canvas[0])
		crop := image.Rect(int(float64(portraitCrop.Min.X)*k), int(float64(portraitCrop.Min.Y)*k),
			int(float64(portraitCrop.Max.X)*k), int(float64(portraitCrop.Max.Y)*k)).Intersect(b)
		if crop.Empty() {
			continue
		}
		sub := img.SubImage(crop).(*ebiten.Image)
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.GeoM.Scale(s/k, s/k)
		op.GeoM.Translate(ox+float64(crop.Min.X-int(float64(portraitCrop.Min.X)*k))*s/k, oy+float64(crop.Min.Y-int(float64(portraitCrop.Min.Y)*k))*s/k)
		uiDrawImage(dst, sub, op)
		drew = true
	}
	return drew
}

// drawPawnPortraitArt draws the layered pawn (facing south, the portrait
// crop of pawnCrops) fitted into the logical w x h box at (x, y), for HUD
// chrome (the inspector); it draws through the UI canvas, so the art
// scales with the UI like the names beside it (PLAY-017).
func (r *Renderer) drawPawnPortraitArt(screen *ebiten.Image, p game.PawnView, x, y, w, h float64) bool {
	body := r.art.Sprite("pawn/body_south")
	if body == nil {
		return false
	}
	crop, _ := pawnCrops(body)
	r.drawPawnCropped(screen, p, body, crop, x, y, w, h)
	return true
}

// drawPawnBustArt draws the layered pawn's head and shoulders (the bust
// crop of the south-facing figure, pawnCrops) fitted into the logical
// w x h box at (x, y), for the colonist bar; it draws through the UI
// canvas like drawPawnPortraitArt.
func (r *Renderer) drawPawnBustArt(screen *ebiten.Image, p game.PawnView, x, y, w, h float64) bool {
	body := r.art.Sprite("pawn/body_south")
	if body == nil {
		return false
	}
	_, crop := pawnCrops(body)
	r.drawPawnCropped(screen, p, body, crop, x, y, w, h)
	return true
}

// drawPawnCropped draws p's portrait layers cut to crop (canvas px of
// body's canvas) fitted into the logical w x h box at (x, y).
func (r *Renderer) drawPawnCropped(screen *ebiten.Image, p game.PawnView, body *artSprite, crop image.Rectangle, x, y, w, h float64) {
	canvasW := float64(body.Meta.Canvas[0])
	cw, ch := float64(crop.Dx()), float64(crop.Dy())
	if cw <= 0 || ch <= 0 {
		return
	}
	s := math.Min(w/cw, h/ch)
	ox, oy := x+(w-cw*s)/2, y+(h-ch*s)/2
	for _, ly := range r.pawnPortraitLayers(p) {
		img := r.art.Image(ly.key, ly.tint)
		if img == nil {
			continue
		}
		b := img.Bounds()
		k := float64(b.Dx()) / canvasW // image pixels per canvas pixel (1 unless the art is @2x)
		x0, y0 := int(float64(crop.Min.X)*k), int(float64(crop.Min.Y)*k)
		cut := image.Rect(x0, y0, int(float64(crop.Max.X)*k), int(float64(crop.Max.Y)*k)).Intersect(b)
		if cut.Empty() {
			continue
		}
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.GeoM.Scale(s/k, s/k)
		op.GeoM.Translate(ox+float64(cut.Min.X-x0)*s/k, oy+float64(cut.Min.Y-y0)*s/k)
		uiDrawImage(screen, img.SubImage(cut).(*ebiten.Image), op)
	}
}
