package creature

import (
	"image/color"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/look"
)

// Worn armor layers (DEF-2, ART §15–§16): the flak, plate and marine
// pieces and the helmets a pawn wears (PawnView.Worn) are drawn over the
// root's figure (Host.FigureLayers) in the order
//
//	shirt → flak pants → flak vest → [parka, duster] → flak jacket →
//	plate / marine armor → shield belt → smokepop pack → beard → hair → helmet
//
// Body pieces have one sprite per build and facing
// (pawn/<layer>_<build>_<facing>, the male average build unsuffixed);
// helmets one per facing. Stuff-made pieces (plate armor, the simple and
// advanced helmet) carry a tint mask the piece's material colours.
// Every key is resolved from a table built once, so dressing a figure
// each frame allocates nothing.

// Worn body pieces, in draw order.
const (
	wornFlakPants = iota
	wornFlakVest
	wornFlakJacket
	wornPlate
	wornMarine
	wornShieldBelt
	wornSmokepopPack
	numWornBody
)

// Helmets, in draw order.
const (
	wornSimpleHelmet = iota
	wornAdvancedHelmet
	wornMarineHelmet
	numWornHelmets
)

// numBuilds is the art builds (look.Sex × look.Build); numFacings the
// rendered facings (south, east, north).
const (
	numBuilds  = 6
	numFacings = 3
	avgBuild   = int(look.Average) // the male average build: unsuffixed keys
)

// wornBodyLayers and wornHelmetLayers are the sprite layer names; the
// builds are look.Look.BuildKey's names in buildIndex order.
var (
	wornBodyLayers   = [numWornBody]string{"flak_pants", "flak_vest", "flak_jacket", "plate_armor", "marine_armor", "shield_belt", "smokepop_pack"}
	wornHelmetLayers = [numWornHelmets]string{"simple_helmet", "advanced_helmet", "marine_helmet"}
	wornBuildKeys    = [numBuilds]string{"m_avg", "m_thin", "m_heavy", "f_avg", "f_thin", "f_heavy"}
	wornFacings      = [numFacings]string{"south", "east", "north"}
)

// wornBodyPiece is item kind's worn body layer.
func wornBodyPiece(kind string) (int, bool) {
	switch kind {
	case "flak pants":
		return wornFlakPants, true
	case "flak vest":
		return wornFlakVest, true
	case "flak jacket":
		return wornFlakJacket, true
	case "plate armor":
		return wornPlate, true
	case "marine armor":
		return wornMarine, true
	case "shield belt":
		return wornShieldBelt, true
	case "smokepop pack":
		return wornSmokepopPack, true
	}
	return 0, false
}

// wornHelmet is item kind's helmet layer.
func wornHelmet(kind string) (int, bool) {
	switch kind {
	case "simple helmet":
		return wornSimpleHelmet, true
	case "advanced helmet":
		return wornAdvancedHelmet, true
	case "marine helmet":
		return wornMarineHelmet, true
	}
	return 0, false
}

// buildIndex is l's art build (wornBuildKeys order).
func buildIndex(l look.Look) int {
	i := int(l.Build) % 3
	if l.Sex == look.Female {
		i += 3
	}
	return i
}

// facingIndex is art facing fs (wornFacings order); false for another.
func facingIndex(fs string) (int, bool) {
	switch fs {
	case "south":
		return 0, true
	case "east":
		return 1, true
	case "north":
		return 2, true
	}
	return 0, false
}

// wornStuffTint is the mask colour of a piece made of stuff: a stone's
// tint, or a metal's or wood's base colour (assets/palettes/materials.yaml);
// steel and any other material stay untinted (the art's own steel).
func wornStuffTint(p *sprite.Painter, stuff string) color.NRGBA {
	switch stuff {
	case "", "steel":
		return color.NRGBA{}
	case "silver":
		return color.NRGBA{R: 0xcb, G: 0xcf, B: 0xd4, A: 0xff}
	case "gold":
		return color.NRGBA{R: 0xe2, G: 0xbe, B: 0x5d, A: 0xff}
	case "wood":
		return color.NRGBA{R: 0x9a, G: 0x77, B: 0x54, A: 0xff}
	}
	if p == nil || p.Art == nil {
		return color.NRGBA{}
	}
	return p.StuffTint(stuff)
}

// WornKeys is the worn layers' key table, built on first use.
type WornKeys struct {
	body [numWornBody][numBuilds][numFacings]string
	helm [numWornHelmets][numFacings]string
	done bool
}

func (w *WornKeys) build() {
	if w.done {
		return
	}
	w.done = true
	for pc, layer := range wornBodyLayers {
		for b, bk := range wornBuildKeys {
			for f, fs := range wornFacings {
				if b == avgBuild {
					w.body[pc][b][f] = "pawn/" + layer + "_" + fs
				} else {
					w.body[pc][b][f] = "pawn/" + layer + "_" + bk + "_" + fs
				}
			}
		}
	}
	for h, layer := range wornHelmetLayers {
		for f, fs := range wornFacings {
			w.helm[h][f] = "pawn/" + layer + "_" + fs
		}
	}
}

// BodyKey is body piece pc's key for build b in facing f: the build's own
// art, else the unsuffixed (male average) art.
func (w *WornKeys) bodyKey(c *art.Catalog, pc, b, f int) string {
	w.build()
	if k := w.body[pc][b][f]; c == nil || c.Has(k) {
		return k
	}
	return w.body[pc][avgBuild][f]
}

// isBaseClothes reports the root's body and base clothes, which the flak
// pants and vest go over.
func isBaseClothes(key string) bool {
	return strings.HasPrefix(key, "pawn/body_") || strings.HasPrefix(key, "pawn/pants_basic") ||
		strings.HasPrefix(key, "pawn/shirt_basic") || strings.HasPrefix(key, "pawn/vest_utility")
}

// isHeadLayer reports the root's head layers (beard, hair, face scar,
// hat), which every body piece goes under.
func isHeadLayer(key string) bool {
	return strings.HasPrefix(key, "pawn/beard_") || strings.HasPrefix(key, "pawn/hair_") ||
		strings.HasPrefix(key, "pawn/face_scar_") || strings.HasPrefix(key, "pawn/tuque_")
}

// insertLayers inserts add into ls at index at.
func insertLayers(ls []Layer, at int, add []Layer) []Layer {
	if len(add) == 0 {
		return ls
	}
	n := len(ls)
	ls = append(ls, add...)
	copy(ls[at+len(add):], ls[at:n])
	copy(ls[at:], add)
	return ls
}

// Dress inserts p's worn armor into its figure ls[from:] (the root's
// layers in art facing fs, bottom first): flak pants and vest over the
// base clothes, the jacket and plate or marine armor under the head,
// helmets on top. It reuses ls's capacity and allocates nothing once the
// table is built.
func (w *WornKeys) Dress(ls []Layer, from int, c *art.Catalog, p *sprite.Painter, pv *game.PawnView, fs string) []Layer {
	if len(pv.Worn) == 0 {
		return ls
	}
	f, ok := facingIndex(fs)
	if !ok {
		return ls
	}
	w.build()
	var under, over [numWornBody]Layer // under: pants, vest; over: jacket, plate, marine
	var head [numWornHelmets]Layer
	var has [numWornBody]bool
	var hasHelm [numWornHelmets]bool
	b := buildIndex(pv.Look)
	for i := range pv.Worn {
		wv := &pv.Worn[i]
		if pc, ok := wornBodyPiece(wv.Kind); ok {
			has[pc] = true
			l := Layer{Key: w.bodyKey(c, pc, b, f), Tint: wornStuffTint(p, wv.Stuff)}
			if pc <= wornFlakVest {
				under[pc] = l
			} else {
				over[pc] = l
			}
		} else if h, ok := wornHelmet(wv.Kind); ok {
			hasHelm[h] = true
			head[h] = Layer{Key: w.helm[h][f], Tint: wornStuffTint(p, wv.Stuff)}
		}
	}
	var buf [numWornBody]Layer
	pick := func(src *[numWornBody]Layer, lo, hi int) []Layer {
		n := 0
		for pc := lo; pc < hi; pc++ {
			if has[pc] {
				buf[n] = src[pc]
				n++
			}
		}
		return buf[:n]
	}
	at := from // after the base clothes
	for at < len(ls) && isBaseClothes(ls[at].Key) {
		at++
	}
	headAt := at // under the head layers
	for headAt < len(ls) && !isHeadLayer(ls[headAt].Key) {
		headAt++
	}
	ls = insertLayers(ls, headAt, pick(&over, wornFlakJacket, numWornBody))
	ls = insertLayers(ls, at, pick(&under, wornFlakPants, wornFlakJacket))
	for h := range head {
		if hasHelm[h] {
			ls = append(ls, head[h])
		}
	}
	return ls
}
