package ebiten

import (
	"image/color"
	"math"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// Held items (ART §15, §27.2; ART-027): a weapon is drawn only on duty
// (drafted, a raider on the attack, aiming or fighting); off duty nothing
// is held, and no tool is ever drawn. Placement is centre-based, from the
// body's centre (the head's x, halfway from the head down to the feet):
//
//   - carried (on duty, at rest): low across the body at a fixed diagonal;
//   - aimed (aim, melee): pushed out from the body centre towards the
//     target and turned to it, flipped when aiming left so it stays
//     upright;
//   - load (hauling): the item on the body, at the chest.
//
// Gear sprites point east. Carried items and loads are placed in the
// drawn facing (south, east, north) and inherit the pose (bob, squash,
// mirror facing west); an aimed item is placed in screen space from the
// posed body centre, so it points exactly at the target in every facing.

// Held sprite sizes and placement, tiles unless noted.
const (
	gearScale = 0.62 // weapon/* relative to the figure
	itemScale = 0.5  // ground-item sprites carried as a load (about 0.5 tile)

	carriedDrop  = 0.18 // carried: the item's centre below the body centre
	carriedAhead = 0.14 // carried, facing east: the centre forward (west mirrored)
	carriedTurn  = 0.72 // carried: radians the forward end points down from level
	aimReach     = 0.35 // aimed: the item's centre out from the body centre
	loadDrop     = 0.07 // a load's centre below the body centre: held at the chest, under the chin
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

// heldSprite resolves a held key to a sprite and its scale.
func (r *Renderer) heldSprite(key string) (*artSprite, float64) {
	sp := r.art.Sprite(key)
	if sp == nil {
		return nil, 0
	}
	if strings.HasPrefix(key, "weapon/") {
		return sp, gearScale
	}
	return sp, itemScale
}

// heldStance is how a held item sits (ART §27.2).
type heldStance uint8

const (
	stanceCarried heldStance = iota // on duty at rest: low across the body
	stanceAimed                     // aiming or striking: out towards (aimX, aimY)
	stanceLoad                      // a hauled item on the body
)

// heldItem is what a pawn holds and how. set marks an action layer's
// choice (even "nothing held").
type heldItem struct {
	set    bool
	key    string // sprite key ("" none)
	stance heldStance
	// aimX, aimY is the aimed item's target in world tiles.
	aimX, aimY float64
	// angle turns the item about its centre, radians, positive raising its
	// forward end (the muzzle kick, a club's arc).
	angle float64
	// push moves an aimed item along the aim (tiles): a thrust forward,
	// recoil back. With no item it is a punch's reach (heldPoint's hold
	// point, for the melee trail).
	push float64
	// cs tints the item (a carried stone chunk's stone); zero is untinted.
	cs colorScale
}

// onDuty: drafted, a raider on the attack, or fighting right now (a hunt
// aims too).
func onDuty(p game.PawnView) bool {
	return p.Drafted || p.Hostile && !p.Fleeing || p.Action == game.ActionAim || p.Action == game.ActionMelee
}

// readyHeld is the wielded weapon carried (fists: nothing held).
func readyHeld(p game.PawnView) heldItem {
	key, ok := heldWeaponKey(p.Weapon)
	if !ok {
		return heldItem{set: true}
	}
	return heldItem{set: true, key: key, stance: stanceCarried}
}

// idleHeld is what a pawn holds when no action layer says otherwise: on
// duty its weapon is carried; off duty nothing is drawn.
func (r *Renderer) idleHeld(p game.PawnView, m moveState) heldItem {
	if !onDuty(p) {
		return heldItem{}
	}
	h := readyHeld(p)
	h.set = false
	return h
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
		rig.body = r.art.Sprite(rig.layers[0].Key)
	}
	return rig
}

// bodyCentre is a body sprite's centre in canvas px: the head's x, halfway
// from the head down to the feet.
func bodyCentre(m artMeta) ([2]float64, bool) {
	head, ok := m.Attach["head"]
	if !ok {
		return [2]float64{}, false
	}
	feet := m.Origin[1] + pawnFeetY*m.PxPerTile()
	return [2]float64{head[0], (head[1] + feet) / 2}, true
}

// heldPivot is the sprite point a held item turns about and is placed by:
// a load's tile centre (at its own density: a big carcass bakes below
// artTilePx, PLAY-022); a gear sprite's centre on its centre line (the
// grip's height).
func heldPivot(sp *artSprite, st heldStance) [2]float64 {
	if st == stanceLoad {
		p := sp.Meta.PxPerTile()
		return [2]float64{sp.Meta.Origin[0] + p/2, sp.Meta.Origin[1] + p/2}
	}
	c := [2]float64{float64(sp.Meta.Canvas[0]) / 2, float64(sp.Meta.Canvas[1]) / 2}
	if g, ok := sp.Meta.Attach["grip"]; ok {
		c[1] = g[1]
	}
	return c
}

// heldPlacement is a held sprite's screen transform, its key, and whether
// it is drawn before the body (hidden by it).
type heldPlacement struct {
	g      ebiten.GeoM
	key    string
	behind bool
}

// heldPlace places what h holds on the figure at (wx, wy) in pose ps.
// Draw order: behind the body facing north; an aimed item only when it
// aims into the north half.
func (r *Renderer) heldPlace(rig heldRig, h heldItem, wx, wy, tile float64, ps pose) (heldPlacement, bool) {
	if rig.body == nil || h.key == "" {
		return heldPlacement{}, false
	}
	sp, k := r.heldSprite(h.key)
	if sp == nil {
		return heldPlacement{}, false
	}
	c, ok := bodyCentre(rig.body.Meta)
	if !ok {
		return heldPlacement{}, false
	}
	sx, sy := r.worldToScreenF(wx, wy)
	bx, by, s := artOrigin(rig.body.Meta, float64(sx), float64(sy), tile)
	bp := rig.body.Meta.PxPerTile()
	k *= bp / sp.Meta.PxPerTile() // the held sprite at its own density (s is the body's)
	poseG := ps.Geo(float64(sx), float64(sy), tile)
	pv := heldPivot(sp, h.stance)
	g := ebiten.GeoM{}
	g.Translate(-pv[0], -pv[1])
	switch h.stance {
	case stanceAimed:
		px, py := poseG.Apply(bx+c[0]*s, by+c[1]*s)
		tx, ty := r.worldToScreenF(h.aimX, h.aimY)
		dx, dy := float64(tx)-px, float64(ty)-py
		if dx == 0 && dy == 0 { // on the body: along the facing
			dx, dy = facingUnit(rig.face)
			if rig.mirror {
				dx = -dx
			}
		}
		theta := math.Atan2(dy, dx)
		side := 1.0
		if math.Cos(theta) < 0 { // aiming left: flipped, so it stays upright
			g.Scale(1, -1)
			side = -1
		}
		g.Rotate(theta - side*h.angle)
		g.Scale(s*k, s*k)
		reach := (aimReach + h.push) * tile
		g.Translate(px+math.Cos(theta)*reach, py+math.Sin(theta)*reach)
		return heldPlacement{g: g, key: h.key, behind: math.Sin(theta) < 0}, true
	case stanceLoad:
		g.Scale(s*k, s*k)
		g.Translate(bx+c[0]*s, by+(c[1]+loadDrop*bp)*s)
	default:
		ahead := 0.0
		if rig.face == faceEast {
			ahead = carriedAhead
		}
		g.Rotate(carriedTurn - h.angle)
		g.Scale(s*k, s*k)
		g.Translate(bx+(c[0]+ahead*bp)*s, by+(c[1]+carriedDrop*bp)*s)
	}
	g.Concat(poseG)
	return heldPlacement{g: g, key: h.key, behind: rig.face == faceNorth}, true
}

// drawHeld draws the held item if it sits on this side of the body:
// behind (before the body layers) or in front (after them).
func (r *Renderer) drawHeld(screen *ebiten.Image, rig heldRig, h heldItem, wx, wy, tile float64, ps pose, behind bool) {
	pl, ok := r.heldPlace(rig, h, wx, wy, tile, ps)
	if !ok || pl.behind != behind {
		return
	}
	cs := noCS
	if h.cs != (colorScale{}) {
		cs = h.cs
	}
	if img := r.art.Image(pl.key, color.NRGBA{}); img != nil {
		r.drawLit(screen, img, pl.g, cs, wx, wy)
	}
}

// det is a GeoM's determinant: negative when it mirrors.
func det(g ebiten.GeoM) float64 {
	return g.Element(0, 0)*g.Element(1, 1) - g.Element(0, 1)*g.Element(1, 0)
}

func wrapAngle(a float64) float64 {
	return math.Remainder(a, 2*math.Pi)
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
		a, ok := rig.body.Meta.Attach[attach]
		if !ok {
			return vec{}, false
		}
		if attach == "hold" && h.push != 0 {
			fx, fy := facingUnit(rig.face)
			p := rig.body.Meta.PxPerTile()
			a = [2]float64{a[0] + fx*h.push*p, a[1] + fy*h.push*p}
		}
		sx, sy := r.worldToScreenF(wx, wy)
		bx, by, s := artOrigin(rig.body.Meta, float64(sx), float64(sy), tile)
		g := ps.Geo(float64(sx), float64(sy), tile)
		x, y = g.Apply(bx+a[0]*s, by+a[1]*s)
	} else {
		pl, ok := r.heldPlace(rig, h, wx, wy, tile, ps)
		if !ok {
			return vec{}, false
		}
		pt, ok := r.art.Sprite(pl.key).Meta.Attach[attach]
		if !ok {
			return vec{}, false
		}
		x, y = pl.g.Apply(pt[0], pt[1])
	}
	return vec{(x + r.cam.OffsetX) / tile, (y + r.cam.OffsetY) / tile}, true
}
