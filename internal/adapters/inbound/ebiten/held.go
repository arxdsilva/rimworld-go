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
//
// A two-handed item puts its grip on `hand` and turns so its `support`
// lies on the line to `hand_l`. A one-handed item hangs from its hand
// along the forearm (shoulder to hand). Gear sprites point east; facing
// west mirrors them so they never draw upside down.

// Held sprite sizes and body landmarks, in tiles.
const (
	gearScale    = 0.62 // weapon/* and tool/* relative to the figure
	itemScale    = 0.5  // ground-item sprites used in the hands (old club, loads)
	shoulderDrop = 0.22 // shoulder line below the head attachment
	shoulderSide = 0.11 // shoulder offset from the head seen from front/back
	slungDrop    = 0.36 // where a slung rifle sits, below the head
	maxAim       = 0.8  // radians an aimed item may turn off its pose
	minHandSpan  = 4.0  // hand to hand_l, art px, below which the line is unusable
)

var toolAngles = map[string][3]float64{
	// east, south, north
	"_work1": {-1.9, -1.57, -1.57}, "_work2": {-2.2, -1.75, -1.85},
	"_work3": {-1.2, -0.9, -1.1}, "_work4": {-0.4, 0.2, -0.2},
	"_work5": {0.55, 1.1, 0.9}, "_work6": {0.1, 0.6, 0.4},
}

// toolAngle is the held tool's screen rotation for a facing and an old
// `work` frame (art without hand_l); west mirrors east.
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

// heldWeaponKey maps a wielded weapon to its held sprite (fists: none).
func heldWeaponKey(weapon string) (key string, ok bool) {
	switch weapon {
	case "rifle":
		return "weapon/rifle_basic", true
	case "knife":
		return "weapon/knife", true
	case "spear":
		return "weapon/spear", true
	case "club":
		return "weapon/club", true
	}
	return "", false
}

// twoHanded items sit in both hands (grip and support).
func twoHanded(key string) bool {
	switch key {
	case "weapon/rifle_basic", "weapon/spear", "tool/axe", "tool/pickaxe":
		return true
	}
	return false
}

// heldFallback is drawn while a held sprite has not been generated yet.
var heldFallback = map[string]string{"weapon/club": "item/club_wood"}

// heldSprite resolves a held key to a sprite and its scale.
func (r *Renderer) heldSprite(key string) (string, *artSprite, float64) {
	if sp := r.art.sprite(key); sp != nil {
		return key, sp, heldScale(key)
	}
	if fb, ok := heldFallback[key]; ok {
		if sp := r.art.sprite(fb); sp != nil {
			return fb, sp, heldScale(fb)
		}
	}
	return "", nil, 0
}

func heldScale(key string) float64 {
	if strings.HasPrefix(key, "weapon/") || strings.HasPrefix(key, "tool/") {
		return gearScale
	}
	return itemScale
}

// heldStance is how a held item sits (ART §27.2).
type heldStance uint8

const (
	stanceHands   heldStance = iota // in the hands: grip on hand, support on hand_l
	stanceSlung                     // on the back (rifle while off duty)
	stanceUpright                   // in one hand: spear upright, small tools along the forearm
	stanceLoad                      // a load carried in both arms
)

// heldItem is what a pawn holds and how. set marks an action layer's
// choice (even "nothing held").
type heldItem struct {
	set    bool
	key    string // sprite key in the right hand ("" none)
	stance heldStance
	// angle turns the item about its grip, radians, positive raising
	// its forward end (muzzle rise, low ready is negative).
	angle float64
	// off is a sprite held in the left hand (hand_l), "" none.
	off string
	// aim turns the item towards the world point (aimX, aimY), at most
	// maxAim off its pose.
	aim        bool
	aimX, aimY float64
	// cs tints the right-hand item (a carried stone chunk's stone); zero
	// is untinted.
	cs colorScale
}

// onDuty: drafted, a raider on the attack, or fighting right now.
func onDuty(p game.PawnView) bool {
	return p.Drafted || p.Hostile && !p.Fleeing || p.Action == game.ActionAim || p.Action == game.ActionMelee
}

// readyHeld is the wielded weapon held ready in both hands (nothing for
// fists).
func readyHeld(p game.PawnView) heldItem {
	key, ok := heldWeaponKey(p.Weapon)
	if !ok {
		return heldItem{set: true}
	}
	return heldItem{set: true, key: key, stance: stanceHands}
}

// idleHeld is what a pawn holds when no action layer says otherwise
// (§27.2 holster rules): on duty the weapon is held ready; off duty the
// rifle is slung, the knife is on the belt (not drawn), the spear is
// carried upright and the club stays in hand.
func (r *Renderer) idleHeld(p game.PawnView, m moveState) heldItem {
	if _, ok := heldWeaponKey(p.Weapon); !ok {
		return heldItem{}
	}
	if onDuty(p) {
		h := readyHeld(p)
		h.set = false
		if p.Weapon == "rifle" {
			h.angle = -lowReady
		}
		return h
	}
	switch p.Weapon {
	case "rifle":
		return heldItem{key: "weapon/rifle_slung", stance: stanceSlung}
	case "knife":
		return heldItem{}
	case "spear":
		return heldItem{key: "weapon/spear", stance: stanceUpright}
	}
	return heldItem{key: "weapon/club", stance: stanceUpright}
}

// heldBehind reports whether an item is drawn before the body (hidden
// by it): anything in front of a north-facing pawn, a slung rifle unless
// facing north, and a one-handed item on the far arm.
func heldBehind(st heldStance, f facing, left bool) bool {
	switch {
	case st == stanceSlung:
		return f != faceNorth
	case f == faceNorth:
		return true
	case st == stanceUpright && !left:
		return f == faceWest
	case left:
		return f == faceEast
	}
	return false
}

// drawHeld draws the held item's parts on one side of the body: behind
// (before the body layers) or in front (after them).
func (r *Renderer) drawHeld(screen *ebiten.Image, p game.PawnView, f facing, suffix string, h heldItem, wx, wy, tile float64, ps pose, behind bool) {
	if h.key != "" && heldBehind(h.stance, f, false) == behind {
		r.drawHeldPart(screen, f, suffix, h, false, wx, wy, tile, ps)
	}
	if h.off != "" && heldBehind(stanceUpright, f, true) == behind {
		r.drawHeldPart(screen, f, suffix, h, true, wx, wy, tile, ps)
	}
}

func (r *Renderer) drawHeldPart(screen *ebiten.Image, f facing, suffix string, h heldItem, left bool, wx, wy, tile float64, ps pose) {
	g, key, ok := r.heldGeo(f, suffix, h, left, wx, wy, tile, ps)
	if !ok {
		return
	}
	cs := noCS
	if !left && h.cs != (colorScale{}) {
		cs = h.cs
	}
	if img := r.art.image(key, color.NRGBA{}); img != nil {
		r.drawLit(screen, img, g, cs, int(math.Floor(wx)), int(math.Floor(wy)))
	}
}

// heldGeo is the screen transform of a held sprite (the right-hand item,
// or the left-hand one when left) and the sprite key it draws.
func (r *Renderer) heldGeo(f facing, suffix string, h heldItem, left bool, wx, wy, tile float64, ps pose) (ebiten.GeoM, string, bool) {
	body := r.art.sprite(r.framed("pawn/body_"+f.String(), suffix))
	key, stance := h.key, h.stance
	if left {
		key, stance = h.off, stanceUpright
	}
	if body == nil || key == "" {
		return ebiten.GeoM{}, "", false
	}
	key, sp, k := r.heldSprite(key)
	if sp == nil && stance == stanceSlung {
		// no slung sprite yet: the rifle diagonally across the back
		key, sp, k = r.heldSprite("weapon/rifle_basic")
	}
	if sp == nil {
		return ebiten.GeoM{}, "", false
	}
	at := body.meta.Attach
	hand, okH := at["hand"]
	handL, okL := at["hand_l"]
	if left && !okL {
		return ebiten.GeoM{}, "", false
	}
	sa := sp.meta.Attach
	grip, okG := sa["grip"]
	if !okG {
		grip = [2]float64{float64(sp.meta.Canvas[0]) / 2, float64(sp.meta.Canvas[1]) / 2}
	}
	mirror := f == faceWest
	anchor := hand
	var theta float64 // screen rotation after the mirror
	turn := func(phi float64, v [2]float64) {
		if mirror {
			v[0] = -v[0]
		}
		theta = phi - math.Atan2(v[1], v[0])
	}
	east := [2]float64{1, 0}
	switch {
	case stance == stanceSlung:
		head, okHead := at["head"]
		if !okHead {
			return ebiten.GeoM{}, "", false
		}
		anchor = [2]float64{head[0], head[1] + slungDrop*artTilePx}
		if back, ok := sa["back"]; ok {
			grip = back // the slung sprite is drawn as is
		} else {
			if mz, ok := sa["muzzle"]; ok {
				grip = [2]float64{(grip[0] + mz[0]) / 2, (grip[1] + mz[1]) / 2}
			}
			turn(slungAngle(f), east)
		}
	case left:
		anchor = handL
		if key == "tool/bowl" {
			theta = 0 // held level
		} else {
			turn(forearm(at, f, handL), east)
		}
	case !okH:
		return ebiten.GeoM{}, "", false
	case stance == stanceLoad:
		if okL {
			anchor = [2]float64{(hand[0] + handL[0]) / 2, (hand[1] + handL[1]) / 2}
		}
		grip = [2]float64{sp.meta.Origin[0] + artTilePx/2, sp.meta.Origin[1] + artTilePx/2}
		mirror = false
	case stance == stanceUpright && key == "weapon/spear":
		turn(-math.Pi/2, east) // shaft upright, tip up
	case stance == stanceHands && twoHanded(key):
		sup, okS := sa["support"]
		d := [2]float64{handL[0] - hand[0], handL[1] - hand[1]}
		switch {
		case okL && okS && math.Hypot(d[0], d[1]) >= minHandSpan:
			turn(math.Atan2(d[1], d[0]), [2]float64{sup[0] - grip[0], sup[1] - grip[1]})
		case strings.HasPrefix(suffix, "_work"):
			theta = toolAngle(f, suffix) // older swing frames
		default:
			turn(baseAngle(f), east)
		}
	default:
		turn(forearm(at, f, hand), east) // one hand, along the forearm
	}
	sx, sy := r.worldToScreenF(wx, wy)
	bx, by, s := artOrigin(body.meta, float64(sx), float64(sy), tile)
	ax, ay := bx+anchor[0]*s, by+anchor[1]*s
	poseG := ps.geo(float64(sx), float64(sy), tile)
	if h.aim && !left && stance == stanceHands {
		px, py := poseG.Apply(ax, ay)
		tx, ty := r.worldToScreenF(h.aimX, h.aimY)
		fwd := theta + ps.rot
		if mirror {
			fwd += math.Pi
		}
		theta += clampAbs(wrapAngle(math.Atan2(float64(ty)-py, float64(tx)-px)-fwd), maxAim)
	}
	g := ebiten.GeoM{}
	g.Translate(-grip[0], -grip[1])
	if !left && h.angle != 0 && stance != stanceLoad {
		g.Rotate(-h.angle) // raise the forward end (sprite space)
	}
	if mirror {
		g.Scale(-1, 1)
	}
	g.Rotate(theta)
	g.Scale(s*k, s*k)
	g.Translate(ax, ay)
	g.Concat(poseG)
	return g, key, true
}

// forearm is the screen direction from the shoulder to a hand: the line
// a one-handed item hangs along.
func forearm(at map[string][2]float64, f facing, hand [2]float64) float64 {
	head, ok := at["head"]
	if !ok {
		return math.Pi / 2
	}
	sh := [2]float64{head[0], head[1] + shoulderDrop*artTilePx}
	if f == faceSouth || f == faceNorth {
		side := 1.0
		if hand[0] < head[0] {
			side = -1
		}
		sh[0] += side * shoulderSide * artTilePx
	}
	return math.Atan2(hand[1]-sh[1], hand[0]-sh[0])
}

// baseAngle points a two-handed item along the facing when the frame has
// no hand_l (art from before the two-hand rig).
func baseAngle(f facing) float64 {
	switch f {
	case faceWest:
		return math.Pi
	case faceSouth:
		return math.Pi / 2.6
	case faceNorth:
		return -math.Pi / 2.6
	}
	return 0
}

// slungAngle lays the rifle diagonally across the back, muzzle up and
// behind, when there is no slung sprite.
func slungAngle(f facing) float64 {
	switch f {
	case faceWest:
		return -0.84
	case faceSouth:
		return -0.9
	case faceNorth:
		return -2.2
	}
	return -2.3
}

func wrapAngle(a float64) float64 {
	return math.Remainder(a, 2*math.Pi)
}

func clampAbs(a, m float64) float64 {
	return math.Max(-m, math.Min(m, a))
}

// heldPoint is where a held item's attachment (muzzle, tip, grip) is in
// world tiles; "hand" with no item is the body's hand.
func (r *Renderer) heldPoint(f facing, suffix string, h heldItem, wx, wy float64, ps pose, attach string) (vec, bool) {
	tile := r.tilePx()
	var x, y float64
	if h.key == "" {
		body := r.art.sprite(r.framed("pawn/body_"+f.String(), suffix))
		if body == nil {
			return vec{}, false
		}
		hand, ok := body.meta.Attach[attach]
		if !ok {
			return vec{}, false
		}
		sx, sy := r.worldToScreenF(wx, wy)
		bx, by, s := artOrigin(body.meta, float64(sx), float64(sy), tile)
		g := ps.geo(float64(sx), float64(sy), tile)
		x, y = g.Apply(bx+hand[0]*s, by+hand[1]*s)
	} else {
		g, key, ok := r.heldGeo(f, suffix, h, false, wx, wy, tile, ps)
		if !ok {
			return vec{}, false
		}
		pt, ok := r.art.sprite(key).meta.Attach[attach]
		if !ok {
			return vec{}, false
		}
		x, y = g.Apply(pt[0], pt[1])
	}
	return vec{(x + r.cam.offsetX) / tile, (y + r.cam.offsetY) / tile}, true
}
