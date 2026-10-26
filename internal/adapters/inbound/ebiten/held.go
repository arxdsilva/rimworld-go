package ebiten

import (
	"image/color"
	"math"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// Held items (ART §27.2): weapons and tools held, slung or carried,
// anchored on the body's attachments. The body has no arms: an item in
// use puts its grip on `hold` and turns about it (per-action transforms
// set the turn); a slung rifle hangs from `back`. No hands are drawn: the
// item floats at the body's side, as in the reference style. Gear sprites
// point east; everything is placed in the drawn
// facing (south, east, north) and the pose mirrors it facing west.

// Held sprite sizes and limits.
const (
	gearScale = 0.62 // weapon/* and tool/* relative to the figure
	itemScale = 0.5  // ground-item sprites used in the hands (old club, loads)
	maxAim    = 0.8  // radians an aimed item may turn off its pose
)

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
	stanceHands   heldStance = iota // in use: grip on hold, pointing along the facing
	stanceSlung                     // on the back (rifle while off duty)
	stanceUpright                   // in one hand at rest: spear upright, small tools hanging
	stanceLoad                      // a load carried in both arms
)

// heldItem is what a pawn holds and how. set marks an action layer's
// choice (even "nothing held").
type heldItem struct {
	set    bool
	key    string // sprite key on hold ("" none)
	stance heldStance
	// angle turns the item about its grip, radians, positive raising
	// its forward end (muzzle rise, a tool's wind-up; low ready is
	// negative).
	angle float64
	// off is a sprite held level on hold under the main item (the bowl
	// under the spoon), "" none.
	off string
	// aim turns the item towards the world point (aimX, aimY), at most
	// maxAim off its pose.
	aim        bool
	aimX, aimY float64
	// lift moves the main item from hold towards the head (0..1): the
	// spoon to the mouth.
	lift float64
	// push moves the main item along its own direction (tiles): a thrust
	// at full reach. With no item it is a punch's reach (heldPoint's
	// hold point, for the melee trail).
	push float64
	// cs tints the main item (a carried stone chunk's stone); zero is
	// untinted.
	cs colorScale
}

// onDuty: drafted, a raider on the attack, or fighting right now.
func onDuty(p game.PawnView) bool {
	return p.Drafted || p.Hostile && !p.Fleeing || p.Action == game.ActionAim || p.Action == game.ActionMelee
}

// readyHeld is the wielded weapon held ready (fists: nothing held).
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

// heldRig is the pawn body a held item anchors on.
type heldRig struct {
	body   *artSprite      // the pawn's body (its build) in the drawn facing
	face   facing          // drawn facing: south, east or north
	mirror bool            // facing west: the pose flips the east art
	layers []portraitLayer // the figure's layers in the drawn facing, body first
}

// rigOf resolves p's body facing f: its layers in the drawn facing and the
// body sprite they start with.
func (r *Renderer) rigOf(p game.PawnView, f facing) heldRig {
	fs, mirror := facingKey(f)
	rig := heldRig{face: drawnFacing(f), mirror: mirror, layers: r.pawnFigureLayers(p, fs)}
	if len(rig.layers) > 0 {
		rig.body = r.art.sprite(rig.layers[0].key)
	}
	return rig
}

// heldBehind reports whether an item is drawn before the body (hidden
// by it), in the drawn facing: everything held facing north, a slung
// rifle unless facing north.
func heldBehind(st heldStance, f facing) bool {
	if st == stanceSlung {
		return f != faceNorth
	}
	return f == faceNorth
}

// drawHeld draws the held item's parts on one side of the body, behind
// (before the body layers) or in front (after them).
func (r *Renderer) drawHeld(screen *ebiten.Image, rig heldRig, h heldItem, wx, wy, tile float64, ps pose, behind bool) {
	if h.off != "" && heldBehind(stanceUpright, rig.face) == behind {
		r.drawHeldPart(screen, rig, h, true, wx, wy, tile, ps)
	}
	if h.key != "" && heldBehind(h.stance, rig.face) == behind {
		r.drawHeldPart(screen, rig, h, false, wx, wy, tile, ps)
	}
}

func (r *Renderer) drawHeldPart(screen *ebiten.Image, rig heldRig, h heldItem, off bool, wx, wy, tile float64, ps pose) {
	g, key, ok := r.heldGeo(rig, h, off, wx, wy, tile, ps)
	if !ok {
		return
	}
	cs := noCS
	if !off && h.cs != (colorScale{}) {
		cs = h.cs
	}
	if img := r.art.image(key, color.NRGBA{}); img != nil {
		r.drawLit(screen, img, g, cs, int(math.Floor(wx)), int(math.Floor(wy)))
	}
}

// loadAnchor is where a load sits in both arms: on hold seen from the
// side, centred under the head seen from the front or back.
func loadAnchor(at map[string][2]float64, f facing, hold [2]float64) [2]float64 {
	if head, ok := at["head"]; ok && f != faceEast {
		return [2]float64{head[0], hold[1]}
	}
	return hold
}

// heldGeo is the screen transform of a held sprite (the main item, or the
// off item under it when off) and the sprite key it draws.
func (r *Renderer) heldGeo(rig heldRig, h heldItem, off bool, wx, wy, tile float64, ps pose) (ebiten.GeoM, string, bool) {
	key, stance := h.key, h.stance
	if off {
		key, stance = h.off, stanceUpright
	}
	if rig.body == nil || key == "" {
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
	at := rig.body.meta.Attach
	hold, okH := at["hold"]
	if !okH {
		return ebiten.GeoM{}, "", false
	}
	sa := sp.meta.Attach
	grip, okG := sa["grip"]
	if !okG {
		grip = [2]float64{float64(sp.meta.Canvas[0]) / 2, float64(sp.meta.Canvas[1]) / 2}
	}
	anchor := hold
	var theta float64 // the item's direction in the drawn facing, before the pose
	angle := h.angle
	switch {
	case stance == stanceSlung:
		back, ok := at["back"]
		if !ok {
			return ebiten.GeoM{}, "", false
		}
		anchor, angle = back, 0
		if b, ok := sa["back"]; ok {
			grip = b // the slung sprite is drawn as is
		} else {
			if mz, ok := sa["muzzle"]; ok {
				grip = [2]float64{(grip[0] + mz[0]) / 2, (grip[1] + mz[1]) / 2}
			}
			theta = slungAngle(rig.face)
		}
	case off:
		angle = 0
		if key != "tool/bowl" { // the bowl is held level
			theta = hangAngle(rig.face)
		}
	case stance == stanceLoad:
		anchor, angle = loadAnchor(at, rig.face, hold), 0
		grip = [2]float64{sp.meta.Origin[0] + artTilePx/2, sp.meta.Origin[1] + artTilePx/2}
	case stance == stanceUpright && key == "weapon/spear":
		theta = -math.Pi / 2 // shaft upright, tip up
	case stance == stanceUpright:
		theta = hangAngle(rig.face)
	default:
		theta = baseAngle(rig.face)
	}
	if head, ok := at["head"]; ok && !off && h.lift != 0 {
		anchor = [2]float64{anchor[0] + (head[0]-anchor[0])*h.lift, anchor[1] + (head[1]-anchor[1])*h.lift}
	}
	sx, sy := r.worldToScreenF(wx, wy)
	bx, by, s := artOrigin(rig.body.meta, float64(sx), float64(sy), tile)
	ax, ay := bx+anchor[0]*s, by+anchor[1]*s
	poseG := ps.geo(float64(sx), float64(sy), tile)
	if h.aim && !off && stance == stanceHands {
		px, py := poseG.Apply(ax, ay)
		fx, fy := poseG.Apply(ax+math.Cos(theta), ay+math.Sin(theta))
		tx, ty := r.worldToScreenF(h.aimX, h.aimY)
		delta := clampAbs(wrapAngle(math.Atan2(float64(ty)-py, float64(tx)-px)-math.Atan2(fy-py, fx-px)), maxAim)
		if det(poseG) < 0 { // a mirrored pose turns the other way
			delta = -delta
		}
		theta += delta
	}
	if !off && h.push != 0 && stance != stanceLoad {
		dir := theta - angle
		ax, ay = ax+math.Cos(dir)*h.push*tile, ay+math.Sin(dir)*h.push*tile
	}
	g := ebiten.GeoM{}
	g.Translate(-grip[0], -grip[1])
	if angle != 0 {
		g.Rotate(-angle) // raise the forward end (sprite space)
	}
	g.Rotate(theta)
	g.Scale(s*k, s*k)
	g.Translate(ax, ay)
	g.Concat(poseG)
	return g, key, true
}

// det is a GeoM's determinant: negative when it mirrors.
func det(g ebiten.GeoM) float64 {
	return g.Element(0, 0)*g.Element(1, 1) - g.Element(0, 1)*g.Element(1, 0)
}

// baseAngle points an item in use along the drawn facing: east, down and
// out in front seen from the front, up behind seen from the back.
func baseAngle(f facing) float64 {
	switch f {
	case faceSouth:
		return math.Pi / 2.6
	case faceNorth:
		return -math.Pi / 2.6
	}
	return 0
}

// hangAngle is a one-handed item at rest: hanging from hold, its head
// down and a little forward.
func hangAngle(f facing) float64 {
	switch f {
	case faceSouth:
		return math.Pi/2 - 0.25
	case faceNorth:
		return math.Pi/2 + 0.25
	}
	return math.Pi/2 - 0.45
}

// slungAngle lays the rifle diagonally across the back, muzzle up and
// behind, when there is no slung sprite.
func slungAngle(f facing) float64 {
	switch f {
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
// world tiles; with no item, the body's attachment (hold, pushed forward
// by a punch's reach).
func (r *Renderer) heldPoint(rig heldRig, h heldItem, wx, wy float64, ps pose, attach string) (vec, bool) {
	tile := r.tilePx()
	var x, y float64
	if h.key == "" {
		if rig.body == nil {
			return vec{}, false
		}
		a, ok := rig.body.meta.Attach[attach]
		if !ok {
			return vec{}, false
		}
		if attach == "hold" && h.push != 0 {
			fx, fy := facingUnit(rig.face)
			a = [2]float64{a[0] + fx*h.push*artTilePx, a[1] + fy*h.push*artTilePx}
		}
		sx, sy := r.worldToScreenF(wx, wy)
		bx, by, s := artOrigin(rig.body.meta, float64(sx), float64(sy), tile)
		g := ps.geo(float64(sx), float64(sy), tile)
		x, y = g.Apply(bx+a[0]*s, by+a[1]*s)
	} else {
		g, key, ok := r.heldGeo(rig, h, false, wx, wy, tile, ps)
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
