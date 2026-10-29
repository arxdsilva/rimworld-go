package creature

import (
	"image/color"
	"math"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
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
	CarriedAhead = 0.14 // carried, facing east: the centre forward (west mirrored)
	carriedTurn  = 0.72 // carried: radians the forward end points down from level
	AimReach     = 0.35 // aimed: the item's centre out from the body centre
	loadDrop     = 0.07 // a load's centre below the body centre: held at the chest, under the chin
)

// HeldWeaponKey maps a wielded weapon (PawnView.Weapon, the def name) to
// its held sprite (fists, bites and the sentry gun: none).
func HeldWeaponKey(weapon string) (key string, ok bool) {
	switch weapon {
	case "rifle":
		return "weapon/rifle_basic", true
	case "knife":
		return "weapon/knife", true
	case "spear":
		return "weapon/spear", true
	case "club":
		return "weapon/club", true
	// DEF-2 (wave C): every new weapon by its def name (weapon.Def.Name,
	// singular: "frag grenade"), each key a literal so the per-frame
	// lookup never builds a string.
	case "mace":
		return "weapon/mace", true
	case "gladius":
		return "weapon/gladius", true
	case "longsword":
		return "weapon/longsword", true
	case "short bow":
		return "weapon/short_bow", true
	case "recurve bow":
		return "weapon/recurve_bow", true
	case "greatbow":
		return "weapon/greatbow", true
	case "pila":
		return "weapon/pila", true
	case "revolver":
		return "weapon/revolver", true
	case "autopistol":
		return "weapon/autopistol", true
	case "machine pistol":
		return "weapon/machine_pistol", true
	case "assault rifle":
		return "weapon/assault_rifle", true
	case "sniper rifle":
		return "weapon/sniper_rifle", true
	case "pump shotgun":
		return "weapon/pump_shotgun", true
	case "chain shotgun":
		return "weapon/chain_shotgun", true
	case "heavy smg":
		return "weapon/heavy_smg", true
	case "lmg":
		return "weapon/lmg", true
	case "minigun":
		return "weapon/minigun", true
	case "frag grenade":
		return "weapon/frag_grenade", true
	case "molotov cocktail":
		return "weapon/molotov_cocktail", true
	case "EMP grenade":
		return "weapon/emp_grenade", true
	case "incendiary launcher":
		return "weapon/incendiary_launcher", true
	case "emp launcher":
		return "weapon/emp_launcher", true
	}
	return "", false
}

// heldSprite resolves a held key to a sprite and its scale.
func (c *Set) heldSprite(v *render.View, key string) (*art.Sprite, float64) {
	sp := v.Art.Sprite(key)
	if sp == nil {
		return nil, 0
	}
	if strings.HasPrefix(key, "weapon/") {
		return sp, gearScale
	}
	return sp, itemScale
}

// HeldStance is how a held item sits (ART §27.2).
type HeldStance uint8

const (
	StanceCarried HeldStance = iota // on duty at rest: low across the body
	StanceAimed                     // aiming or striking: out towards (aimX, aimY)
	StanceLoad                      // a hauled item on the body
)

// HeldItem is what a pawn holds and how. set marks an action layer's
// choice (even "nothing held").
type HeldItem struct {
	Set    bool
	Key    string // sprite key ("" none)
	Stance HeldStance
	// AimX, AimY is the aimed item's target in world tiles.
	AimX, AimY float64
	// Angle turns the item about its centre, radians, positive raising its
	// forward end (the muzzle kick, a club's arc).
	Angle float64
	// Push moves an aimed item along the aim (tiles): a thrust forward,
	// recoil back. With no item it is a punch's reach (heldPoint's hold
	// point, for the melee trail).
	Push float64
	// Cs tints the item (a carried stone chunk's stone); zero is untinted.
	Cs sprite.ColorScale
}

// onDuty: drafted, a raider on the attack, or fighting right now (a hunt
// aims too).
func onDuty(p game.PawnView) bool {
	return p.Drafted || p.Hostile && !p.Fleeing || p.Action == game.ActionAim || p.Action == game.ActionMelee
}

// ReadyHeld is the wielded weapon carried (fists: nothing held).
func ReadyHeld(p game.PawnView) HeldItem {
	key, ok := HeldWeaponKey(p.Weapon)
	if !ok {
		return HeldItem{Set: true}
	}
	return HeldItem{Set: true, Key: key, Stance: StanceCarried}
}

// IdleHeld is what a pawn holds when no action layer says otherwise: on
// duty its weapon is carried; off duty nothing is drawn.
func (c *Set) IdleHeld(p game.PawnView, m motion.MoveState) HeldItem {
	if !onDuty(p) {
		return HeldItem{}
	}
	h := ReadyHeld(p)
	h.Set = false
	return h
}

// HeldRig is the pawn body a held item anchors on.
type HeldRig struct {
	Body   *art.Sprite // the pawn's body (its build) in the drawn facing
	Face   art.Facing  // drawn facing: south, east or north
	Mirror bool        // facing west: the pose flips the east art
	Layers []Layer     // the figure's layers in the drawn facing, body first
}

// RigOf resolves p's body facing f: its layers in the drawn facing and the
// body sprite they start with. The layers are its own copy.
func (c *Set) RigOf(v *render.View, p game.PawnView, f art.Facing) HeldRig {
	mark := c.mark()
	rig := c.rigOf(v, p, f)
	rig.Layers = append([]Layer(nil), rig.Layers...)
	c.release(mark)
	return rig
}

// rigOf is RigOf with the layers on the layer stack: the caller releases
// them (mark, release).
func (c *Set) rigOf(v *render.View, p game.PawnView, f art.Facing) HeldRig {
	fs, mirror := FacingKey(f)
	rig := HeldRig{Face: drawnFacing(f), Mirror: mirror, Layers: c.figureLayers(p, fs)}
	if len(rig.Layers) > 0 {
		rig.Body = v.Art.Sprite(rig.Layers[0].Key)
	}
	return rig
}

// BodyCentre is a body sprite's centre in canvas px: the head's x, halfway
// from the head down to the feet.
func BodyCentre(m art.Meta) ([2]float64, bool) {
	head, ok := m.Attach["head"]
	if !ok {
		return [2]float64{}, false
	}
	feet := m.Origin[1] + PawnFeetY*m.PxPerTile()
	return [2]float64{head[0], (head[1] + feet) / 2}, true
}

// HeldPivot is the sprite point a held item turns about and is placed by:
// a load's tile centre (at its own density: a big carcass bakes below
// artTilePx, PLAY-022); a gear sprite's centre on its centre line (the
// grip's height).
func HeldPivot(sp *art.Sprite, st HeldStance) [2]float64 {
	if st == StanceLoad {
		p := sp.Meta.PxPerTile()
		return [2]float64{sp.Meta.Origin[0] + p/2, sp.Meta.Origin[1] + p/2}
	}
	c := [2]float64{float64(sp.Meta.Canvas[0]) / 2, float64(sp.Meta.Canvas[1]) / 2}
	if g, ok := sp.Meta.Attach["grip"]; ok {
		c[1] = g[1]
	}
	return c
}

// HeldPlacement is a held sprite's screen transform, its key, and whether
// it is drawn before the body (hidden by it).
type HeldPlacement struct {
	G      ebiten.GeoM
	Key    string
	Behind bool
}

// HeldPlace places what h holds on the figure at (wx, wy) in pose ps.
// Draw order: behind the body facing north; an aimed item only when it
// aims into the north half.
func (c *Set) HeldPlace(v *render.View, rig HeldRig, h HeldItem, wx, wy, tile float64, ps motion.Pose) (HeldPlacement, bool) {
	if rig.Body == nil || h.Key == "" {
		return HeldPlacement{}, false
	}
	sp, k := c.heldSprite(v, h.Key)
	if sp == nil {
		return HeldPlacement{}, false
	}
	bc, ok := BodyCentre(rig.Body.Meta)
	if !ok {
		return HeldPlacement{}, false
	}
	sx, sy := v.WorldToScreenF(wx, wy)
	bx, by, s := art.Origin(rig.Body.Meta, float64(sx), float64(sy), tile)
	bp := rig.Body.Meta.PxPerTile()
	k *= bp / sp.Meta.PxPerTile() // the held sprite at its own density (s is the body's)
	poseG := ps.Geo(float64(sx), float64(sy), tile)
	pv := HeldPivot(sp, h.Stance)
	g := ebiten.GeoM{}
	g.Translate(-pv[0], -pv[1])
	switch h.Stance {
	case StanceAimed:
		px, py := poseG.Apply(bx+bc[0]*s, by+bc[1]*s)
		tx, ty := v.WorldToScreenF(h.AimX, h.AimY)
		dx, dy := float64(tx)-px, float64(ty)-py
		if dx == 0 && dy == 0 { // on the body: along the facing
			dx, dy = facingUnit(rig.Face)
			if rig.Mirror {
				dx = -dx
			}
		}
		theta := math.Atan2(dy, dx)
		side := 1.0
		if math.Cos(theta) < 0 { // aiming left: flipped, so it stays upright
			g.Scale(1, -1)
			side = -1
		}
		g.Rotate(theta - side*h.Angle)
		g.Scale(s*k, s*k)
		reach := (AimReach + h.Push) * tile
		g.Translate(px+math.Cos(theta)*reach, py+math.Sin(theta)*reach)
		return HeldPlacement{G: g, Key: h.Key, Behind: math.Sin(theta) < 0}, true
	case StanceLoad:
		g.Scale(s*k, s*k)
		g.Translate(bx+bc[0]*s, by+(bc[1]+loadDrop*bp)*s)
	default:
		ahead := 0.0
		if rig.Face == art.FaceEast {
			ahead = CarriedAhead
		}
		g.Rotate(carriedTurn - h.Angle)
		g.Scale(s*k, s*k)
		g.Translate(bx+(bc[0]+ahead*bp)*s, by+(bc[1]+carriedDrop*bp)*s)
	}
	g.Concat(poseG)
	return HeldPlacement{G: g, Key: h.Key, Behind: rig.Face == art.FaceNorth}, true
}

// DrawHeld draws the held item if it sits on this side of the body:
// behind (before the body layers) or in front (after them).
func (c *Set) DrawHeld(v *render.View, pass sprite.Pass, screen *ebiten.Image, rig HeldRig, h HeldItem, wx, wy, tile float64, ps motion.Pose, behind bool) {
	pl, ok := c.HeldPlace(v, rig, h, wx, wy, tile, ps)
	if !ok || pl.Behind != behind {
		return
	}
	cs := sprite.NoCS
	if h.Cs != (sprite.ColorScale{}) {
		cs = h.Cs
	}
	if img := v.Art.Image(pl.Key, color.NRGBA{}); img != nil {
		c.lit.Draw(v, screen, img, pl.G, cs, wx, wy, pass.Cast, pass.Baking, pass.Blood)
	}
}

// Det is a GeoM's determinant: negative when it mirrors.
func Det(g ebiten.GeoM) float64 {
	return g.Element(0, 0)*g.Element(1, 1) - g.Element(0, 1)*g.Element(1, 0)
}

func WrapAngle(a float64) float64 {
	return math.Remainder(a, 2*math.Pi)
}

// HeldPoint is where a held item's attachment (muzzle, tip, grip) is in
// world tiles; with no item, the body's attachment (hold, pushed forward
// by a punch's reach).
func (c *Set) HeldPoint(v *render.View, rig HeldRig, h HeldItem, wx, wy float64, ps motion.Pose, attach string) (Vec, bool) {
	tile := v.Cam.Tile()
	var x, y float64
	if h.Key == "" {
		if rig.Body == nil {
			return Vec{}, false
		}
		a, ok := rig.Body.Meta.Attach[attach]
		if !ok {
			return Vec{}, false
		}
		if attach == "hold" && h.Push != 0 {
			fx, fy := facingUnit(rig.Face)
			p := rig.Body.Meta.PxPerTile()
			a = [2]float64{a[0] + fx*h.Push*p, a[1] + fy*h.Push*p}
		}
		sx, sy := v.WorldToScreenF(wx, wy)
		bx, by, s := art.Origin(rig.Body.Meta, float64(sx), float64(sy), tile)
		g := ps.Geo(float64(sx), float64(sy), tile)
		x, y = g.Apply(bx+a[0]*s, by+a[1]*s)
	} else {
		pl, ok := c.HeldPlace(v, rig, h, wx, wy, tile, ps)
		if !ok {
			return Vec{}, false
		}
		pt, ok := v.Art.Sprite(pl.Key).Meta.Attach[attach]
		if !ok {
			return Vec{}, false
		}
		x, y = pl.G.Apply(pt[0], pt[1])
	}
	return Vec{X: (x + v.Cam.OffsetX) / tile, Y: (y + v.Cam.OffsetY) / tile}, true
}

// bodyRig is the rig without its layers (the held-point lookups need the
// body and the facing only).
func (c *Set) bodyRig(v *render.View, p game.PawnView, f art.Facing) HeldRig {
	mark := c.mark()
	rig := c.rigOf(v, p, f)
	c.release(mark)
	rig.Layers = nil
	return rig
}
