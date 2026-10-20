package ebiten

import (
	"image/color"
	"math"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// Held items (ART §27.2): weapons and tools in the hands, slung or
// carried, drawn from the body frame's hand attachments.

var toolAngles = map[string][3]float64{
	// east, south, north
	"_work1": {-1.9, -1.57, -1.57}, "_work2": {-2.2, -1.75, -1.85},
	"_work3": {-1.2, -0.9, -1.1}, "_work4": {-0.4, 0.2, -0.2},
	"_work5": {0.55, 1.1, 0.9}, "_work6": {0.1, 0.6, 0.4},
}

// toolAngle is the held tool's screen rotation for a facing and frame;
// west mirrors east.
func toolAngle(f facing, frame string) float64 {
	a, ok := toolAngles[frame]
	if !ok {
		return 0
	}
	switch f {
	case faceWest:
		return -a[0]
	case faceSouth:
		return a[1]
	case faceNorth:
		return a[2]
	}
	return a[0]
}

// heldWeaponKey maps a wielded weapon to a sprite and how it is held.
func heldWeaponKey(weapon string) (key string, scale float64, ok bool) {
	switch weapon {
	case "rifle":
		return "weapon/rifle_basic", 0.62, true
	case "club":
		return "item/club_wood", 0.5, true
	}
	return "", 0, false
}

// drawHeldWeapon draws the wielded weapon with its grip at the body's
// hand attachment for this walk frame, pointing along the facing, and
// moved with the figure's pose.
func (r *Renderer) drawHeldWeapon(screen *ebiten.Image, p game.PawnView, f facing, suffix string, wx, wy, tile float64, ps pose) {
	key, k, ok := heldWeaponKey(p.Weapon)
	if !ok {
		return
	}
	body := r.art.sprite(r.framed("pawn/body_"+f.String(), suffix))
	wsp := r.art.sprite(key)
	img := r.art.image(key, color.NRGBA{})
	if body == nil || wsp == nil || img == nil {
		return
	}
	hand, okH := body.meta.Attach["hand"]
	if !okH {
		return
	}
	sx, sy := r.worldToScreenF(wx, wy)
	bx, by, s := artOrigin(body.meta, float64(sx), float64(sy), tile)
	hx, hy := bx+hand[0]*s, by+hand[1]*s
	grip, okG := wsp.meta.Attach["grip"]
	if !okG {
		grip = [2]float64{float64(wsp.meta.Canvas[0]) / 2, float64(wsp.meta.Canvas[1]) / 2}
	}
	g := ebiten.GeoM{}
	g.Translate(-grip[0], -grip[1])
	switch f {
	case faceWest:
		g.Scale(-1, 1)
	case faceSouth:
		g.Rotate(math.Pi / 2.6)
	case faceNorth:
		g.Rotate(-math.Pi / 2.6)
	}
	g.Scale(s*k, s*k)
	g.Translate(hx, hy)
	g.Concat(ps.geo(float64(sx), float64(sy), tile))
	op := &ebiten.DrawImageOptions{GeoM: g, Filter: ebiten.FilterLinear}
	screen.DrawImage(img, op)
}

// drawTool draws a work tool with its grip in the tool hand of the
// current arm frame, angled for the swing.
func (r *Renderer) drawTool(screen *ebiten.Image, key string, f facing, suffix string, wx, wy, tile float64, ps pose) {
	if key == "" {
		return
	}
	body := r.art.sprite(r.framed("pawn/body_"+f.String(), suffix))
	tsp := r.art.sprite(key)
	img := r.art.image(key, color.NRGBA{})
	if body == nil || tsp == nil || img == nil {
		return
	}
	hand, ok := body.meta.Attach["hand"]
	if !ok {
		return
	}
	grip, ok := tsp.meta.Attach["grip"]
	if !ok {
		grip = [2]float64{float64(tsp.meta.Canvas[0]) / 2, float64(tsp.meta.Canvas[1]) / 2}
	}
	sx, sy := r.worldToScreenF(wx, wy)
	bx, by, s := artOrigin(body.meta, float64(sx), float64(sy), tile)
	g := ebiten.GeoM{}
	g.Translate(-grip[0], -grip[1])
	if f == faceWest {
		g.Scale(-1, 1)
	}
	g.Rotate(toolAngle(f, suffix))
	g.Scale(s*0.62, s*0.62)
	g.Translate(bx+hand[0]*s, by+hand[1]*s)
	g.Concat(ps.geo(float64(sx), float64(sy), tile))
	screen.DrawImage(img, &ebiten.DrawImageOptions{GeoM: g, Filter: ebiten.FilterLinear})
}

// heldStance is how a held item sits (ART §27.2).
type heldStance uint8

const (
	stanceHands   heldStance = iota // in the hands: grip on hand, support on hand_l
	stanceSlung                     // on the back (rifle while off duty)
	stanceUpright                   // upright in one hand (spear while walking)
	stanceLoad                      // a load carried in both arms
)

// heldItem is what a pawn holds and how. set marks an action layer's
// choice (even "nothing held"); angle is extra rotation, e.g. aiming.
type heldItem struct {
	set    bool
	key    string // sprite key ("" none)
	stance heldStance
	angle  float64
}

// idleHeld is what a pawn holds when no action layer says otherwise.
func (r *Renderer) idleHeld(p game.PawnView, m moveState) heldItem {
	key, _, ok := heldWeaponKey(p.Weapon)
	if !ok {
		return heldItem{}
	}
	return heldItem{key: key, stance: stanceHands}
}

// drawHeld draws the held item's parts on one side of the body: behind
// (before the body layers) or in front (after them).
func (r *Renderer) drawHeld(screen *ebiten.Image, p game.PawnView, f facing, suffix string, h heldItem, wx, wy, tile float64, ps pose, behind bool) {
	if h.key == "" || behind != (f == faceNorth) {
		return
	}
	if strings.HasPrefix(h.key, "tool/") {
		r.drawTool(screen, h.key, f, suffix, wx, wy, tile, ps)
		return
	}
	r.drawHeldWeapon(screen, p, f, suffix, wx, wy, tile, ps)
}
