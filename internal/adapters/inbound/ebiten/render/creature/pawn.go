package creature

import (
	"image/color"
	"math"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
)

// Pawn figure animation (ART §27): one standing sprite per facing moved
// as a whole: gait bob, sway and lean, idle breathing, per-action
// transforms towards the worked tile, recoil/lunge/hit reactions,
// tipping over when downed, lying in bed under the blanket, and being
// carried. Facing west draws the east sprites mirrored.

const (
	// FallFrames is how long (present frames) a figure takes to tip over.
	FallFrames = 16
	// BuckleFrames is the squash before a downed or dead figure tips over.
	BuckleFrames = 16
	deadCSR      = 0.62
)

// DeadCS tints a dead figure.
var DeadCS = sprite.ColorScale{deadCSR, 0.6, 0.64, 1}

// PawnFeetX, PawnFeetY is where a standing pawn's feet meet the ground,
// in tiles from its tile's top-left: the rig's feet (families/pawn.py CX,
// CY). The ground plane is drawn without shear or lean in every camera
// (contracts.py §5), so camera changes never move this point; everything
// above it is read from the body metadata.
const PawnFeetX, PawnFeetY = 0.5, 0.52

// lyingCentreK is where a lying figure's middle is, as a fraction of the
// way from its feet to its head centre (the head's top rises above it).
const lyingCentreK = 0.65

// lyingDrop lowers a lying figure (tiles): rotated about its feet it
// would lie on the feet's line, a little above the tile's middle.
const lyingDrop = 0.06

// drawnFacing is the facing whose sprites draw f: west draws east.
func drawnFacing(f art.Facing) art.Facing {
	if f == art.FaceWest {
		return art.FaceEast
	}
	return f
}

// FacingKey is the art facing ("south", "east", "north") that draws f and
// whether it is mirrored: pawns are rendered south, east and north only;
// west is the east art flipped about the figure's pivot (ART §10).
func FacingKey(f art.Facing) (key string, mirror bool) {
	return drawnFacing(f).String(), f == art.FaceWest
}

// Mirrored flips a pose left-right about its pivot.
func Mirrored(ps motion.Pose) motion.Pose {
	kx, ky := ps.Scale()
	ps.Sx, ps.Sy = -kx, ky
	return ps
}

// StandPose is the untransformed pose of an upright pawn.
func StandPose() motion.Pose { return motion.Pose{Px: PawnFeetX, Py: PawnFeetY} }

// BodyAttach is attachment name of a body sprite in tiles from its tile's
// top-left (canvas px relative to origin_px, over artTilePx).
func BodyAttach(sp *art.Sprite, name string) (x, y float64, ok bool) {
	if sp == nil {
		return 0, 0, false
	}
	a, ok := sp.Meta.Attach[name]
	if !ok {
		return 0, 0, false
	}
	return (a[0] - sp.Meta.Origin[0]) / art.TilePx, (a[1] - sp.Meta.Origin[1]) / art.TilePx, true
}

// PawnHead is a standing figure's head centre in tiles from its tile's
// top-left, from pawn/body_south's head attachment (a stand-in without
// art).
func PawnHead(c *art.Catalog) (x, y float64) {
	if x, y, ok := BodyAttach(c.Sprite("pawn/body_south"), "head"); ok {
		return x, y
	}
	return PawnFeetX, -0.24
}

// LyingPose is a figure lying on the ground on its side (side +1: head to
// the east), its middle over the feet's tile.
func LyingPose(c *art.Catalog, side float64) motion.Pose {
	_, hy := PawnHead(c)
	shift := (PawnFeetY - hy) * lyingCentreK
	return motion.Pose{Px: PawnFeetX, Py: PawnFeetY, Rot: side * math.Pi / 2, Dx: -side * shift, Dy: lyingDrop}
}

// PoseFacing is the facing to draw: the motion's smoothed facing, turned
// to the worked tile while working in place and to the blocker while
// stuck behind one.
func (c *Set) PoseFacing(v *render.View, p game.PawnView, m motion.MoveState) art.Facing {
	f := m.Facing
	if m.Blend >= motion.MinBlend {
		return f
	}
	if tf, ok := TargetFacing(p, v.Pawns, v.W.Animals, f); ok {
		return tf
	}
	if p.Acting != "" && (p.ActX != p.X || p.ActY != p.Y) {
		return art.FacingFor(p.ActX-p.X, p.ActY-p.Y, f)
	}
	return motion.BlockedFacing(p, v.Pawns, v.W.Animals, f)
}

// ActionPose is one action layer's contribution to an upright pawn (ART
// §27.2): what it holds and how, and a transform added to the standing
// pose. Layers are tried in priority order by StandingPose: hit reaction,
// combat, carrying, work.
type ActionPose struct {
	Ok     bool
	Held   HeldItem // replaces the default held item when held.set
	Dx, Dy float64  // added offset, tiles
	Rot    float64  // added rotation, radians
	Squash float64  // vertical squash 0..1 (bending, crouching)
	Flash  float32  // hit flash 0..1
}

// TargetFacing turns a pawn that is aiming or fighting towards its
// target from the first moment, not only once a shot has been fired.
func TargetFacing(p game.PawnView, pawns []game.PawnView, animals []game.AnimalView, prev art.Facing) (art.Facing, bool) {
	if p.ActTarget == 0 || (p.Action != game.ActionAim && p.Action != game.ActionMelee) {
		return prev, false
	}
	for _, q := range pawns {
		if q.ID == p.ActTarget {
			return art.FacingFor(q.X-p.X, q.Y-p.Y, prev), true
		}
	}
	for _, an := range animals {
		if an.ID == p.ActTarget {
			return art.FacingFor(an.X-p.X, an.Y-p.Y, prev), true
		}
	}
	return prev, false
}

// Action layers in StandingPose's priority order.
const (
	layerHit = iota
	layerCombat
	layerCarry
	layerWork
	layerCount
)

// actionLayer is action layer i's contribution to p.
func (c *Set) actionLayer(v *render.View, i int, p game.PawnView, m motion.MoveState) ActionPose {
	switch i {
	case layerHit:
		return c.HitReaction(v, p, m)
	case layerCombat:
		return c.CombatPose(v, p, m)
	case layerCarry:
		return c.CarryPose(v, p, m)
	}
	return c.WorkPose(v, p, m)
}

// StandingPose is the live transform for an upright pawn and what it
// holds: walking or breathing, then the first action layer that applies.
func (c *Set) StandingPose(v *render.View, p game.PawnView, m motion.MoveState) (motion.Pose, HeldItem) {
	ps := motion.WalkPose(StandPose(), m.Facing, m)
	if m.Blend < motion.MinBlend {
		b := math.Sin(2 * math.Pi * float64(v.Present+uint64(p.ID)*37) / 150)
		ps.Sy = 1 + 0.012*b
	}
	held := c.IdleHeld(p, m)
	for i := range layerCount {
		a := c.actionLayer(v, i, p, m)
		if !a.Ok {
			continue
		}
		if a.Held.Set {
			held = a.Held
		}
		ps.Dx += a.Dx
		ps.Dy += a.Dy
		ps.Rot += a.Rot
		if a.Squash != 0 {
			_, sy := ps.Scale()
			ps.Sy = sy * (1 - a.Squash)
		}
		if a.Flash > ps.Flash {
			ps.Flash = a.Flash
		}
		break
	}
	dx, dy, fl := c.CombatOffset(v, p)
	ps.Dx, ps.Dy = ps.Dx+dx, ps.Dy+dy
	if fl > ps.Flash {
		ps.Flash = fl
	}
	return ps, held
}

// FigurePose is everything a pawn's figure draw needs this frame,
// resolved once (from read-only state) so the object pass and every
// light of the shadow pass draw the same figure.
type FigurePose struct {
	Vis  Visual      // the presentation state (lying pawns read it)
	Face art.Facing  // the facing (west draws the east art mirrored)
	Held HeldItem    // what it holds
	Ps   motion.Pose // unmirrored: DrawPawnFigure flips it facing west
}

// FigurePoseOf resolves p's figure pose for this frame. It reads layer
// state and writes none.
func (c *Set) FigurePoseOf(v *render.View, p game.PawnView) FigurePose {
	fp := FigurePose{Vis: c.Host.Visual(p.ID)}
	if p.Lying {
		return fp // DrawLyingPawn poses it
	}
	m := c.motion(v, p.ID)
	m.Facing = c.PoseFacing(v, p, m)
	fp.Ps, fp.Held = c.StandingPose(v, p, m)
	fp.Face = m.Facing
	return fp
}

// FigureLying reports a pawn lying on the bare ground (downed, dead or
// asleep there), whose silhouette lies flat; one in bed casts none.
func FigureLying(p game.PawnView) bool { return p.Lying && !p.InBed }

// DrawPawnFigure draws only the pawn's figure: body, apparel, hair, the
// held item, and what it carries (a load or a
// rescued patient), in pose and facing. It writes no layer state and
// draws no labels, bars, rings or flashes of its own, so the shadow pass
// (pass.Cast set) calls it to draw the pawn's silhouette.
func (c *Set) DrawPawnFigure(v *render.View, pass sprite.Pass, dst *ebiten.Image, p game.PawnView, wx, wy, tile float64, fp FigurePose) {
	if p.Lying {
		c.DrawLyingPawn(v, pass, dst, p, fp.Vis, wx, wy, tile)
		return
	}
	ps, item := fp.Ps, fp.Held
	if pass.Cast != nil {
		ps.Flash = 0 // a light effect, not part of the shape
	}
	mark := c.mark()
	rig := c.rigOf(v, p, fp.Face)
	if rig.Mirror {
		ps = Mirrored(ps)
	}
	// a rescued patient lies across the shoulders, behind the rescuer's
	// head; facing north the back, and so the patient, faces the viewer
	carriedBehind := rig.Face != art.FaceNorth
	if carriedBehind {
		c.drawCarriedBy(v, pass, dst, p, fp, wx, wy, tile)
	}
	c.DrawHeld(v, pass, dst, rig, item, wx, wy, tile, ps, true) // first: what the body hides (facing north, or aiming north)
	c.drawLayers(v, pass, dst, p, rig.Layers, wx, wy, tile, ps)
	c.DrawHeld(v, pass, dst, rig, item, wx, wy, tile, ps, false) // the parts in front
	if !carriedBehind {
		c.drawCarriedBy(v, pass, dst, p, fp, wx, wy, tile)
	}
	c.release(mark)
}

// drawCarriedBy draws the patients rescuer p carries.
func (c *Set) drawCarriedBy(v *render.View, pass sprite.Pass, dst *ebiten.Image, p game.PawnView, fp FigurePose, wx, wy, tile float64) {
	for _, q := range v.Pawns {
		if q.CarriedBy != 0 && q.CarriedBy == p.ID {
			c.drawCarried(v, pass, dst, q, fp.Face, wx+fp.Ps.Dx, wy+fp.Ps.Dy, tile)
		}
	}
}

// drawLayers draws a figure's layers in one pose: body and apparel take
// the pawn's blood, hair and beard stay clean.
func (c *Set) drawLayers(v *render.View, pass sprite.Pass, dst *ebiten.Image, p game.PawnView, layers []Layer, wx, wy, tile float64, ps motion.Pose) {
	blood := c.Host.PawnBlood(p) // body and apparel only (Host.PawnBlood)
	for _, ly := range layers {
		pass.Blood = c.Host.LayerBlood(ly.Key, blood)
		if strings.HasPrefix(ly.Key, "pawn/beard_") {
			pass.Blood = 0
		}
		c.DrawSpritePose(v, pass, dst, ly.Key, ly.Tint, wx, wy, tile, ps)
	}
}

// DrawFigureIn draws a pawn's layers facing f (west mirrored) in pose ps.
func (c *Set) DrawFigureIn(v *render.View, pass sprite.Pass, screen *ebiten.Image, p game.PawnView, f art.Facing, wx, wy, tile float64, ps motion.Pose) {
	fs, mirror := FacingKey(f)
	if mirror {
		ps = Mirrored(ps)
	}
	mark := c.mark()
	c.drawLayers(v, pass, screen, p, c.figureLayers(p, fs), wx, wy, tile, ps)
	c.release(mark)
}

// DrawFigure draws the figure facing south with a pose (lying, in bed,
// carried).
func (c *Set) DrawFigure(v *render.View, pass sprite.Pass, screen *ebiten.Image, p game.PawnView, wx, wy, tile float64, ps motion.Pose) {
	c.DrawFigureIn(v, pass, screen, p, art.FaceSouth, wx, wy, tile, ps)
}

// DrawLyingPawn: in a bed under the blanket, or on the ground (FallPose:
// tipping over away from the last hit, then lying there).
func (c *Set) DrawLyingPawn(v *render.View, pass sprite.Pass, screen *ebiten.Image, p game.PawnView, vis Visual, wx, wy, tile float64) {
	cs := sprite.NoCS
	if p.Status == "dead" {
		cs = DeadCS
	}
	if p.InBed {
		if pass.Cast != nil {
			return // the bed casts the shadow
		}
		if b, ok := BedUnder(v.W, p.X, p.Y); ok {
			c.drawInBed(v, pass, screen, p, b, vis.LyingSince, tile, cs)
			return
		}
	}
	ps := c.FallPose(v, p, vis)
	ps.Cs = cs
	if pass.Cast != nil {
		ps.Flash = 0
	}
	c.DrawFigure(v, pass, screen, p, wx, wy, tile, ps)
}

// BedUnder is the sleeping furniture covering tile (x, y).
func BedUnder(w game.WorldView, x, y int) (game.BuildingView, bool) {
	for _, b := range w.Buildings {
		if b.Kind != "bed" && b.Kind != "double bed" && b.Kind != "sleeping spot" && b.Kind != hospitalBed {
			continue
		}
		bw, bh := max(b.W, 1), max(b.H, 1)
		if x >= b.X && y >= b.Y && x < b.X+bw && y < b.Y+bh {
			return b, true
		}
	}
	return game.BuildingView{}, false
}

// drawInBed lays the figure along the bed (or sleeping spot) with its
// head on the pillow, breathing slowly; a bed then draws its blanket over
// it, while on a floor marking the figure lies flatter in the open. The
// figure turns about its head centre (pawnHead, from the body metadata).
func (c *Set) drawInBed(v *render.View, pass sprite.Pass, screen *ebiten.Image, p game.PawnView, b game.BuildingView, since uint64, tile float64, cs sprite.ColorScale) {
	bw, bh := float64(max(b.W, 1)), float64(max(b.H, 1))
	x0, y0 := float64(b.X), float64(b.Y)
	spot := b.Kind == "sleeping spot"
	// pillow centre on screen: along the bed it sits at the head end
	// (bed: 0.36 in and raised by the shear; spot: the flat painted
	// pillow, 0.27 in); across the bed it is the sleeper's own column or
	// row, so two can share a double bed.
	cx, cy := float64(p.X)+0.5, float64(p.Y)
	hx, hy, rot := cx, y0+0.2, 0.0
	switch {
	case spot && b.Facing == "east":
		hx, hy, rot = x0+0.27, cy+0.5, -math.Pi/2
	case spot && b.Facing == "west":
		hx, hy, rot = x0+bw-0.27, cy+0.5, math.Pi/2
	case spot && b.Facing == "north":
		hx, hy, rot = cx, y0+bh-0.27, math.Pi
	case spot:
		hy = y0 + 0.27
	case b.Facing == "east":
		hx, hy, rot = x0+0.36, cy+0.3, -math.Pi/2
	case b.Facing == "west":
		hx, hy, rot = x0+bw-0.36, cy+0.3, math.Pi/2
	case b.Facing == "north":
		hx, hy, rot = cx, y0+bh-0.56, math.Pi
	}
	breath := 1 + 0.025*math.Sin(2*math.Pi*float64(v.Present+uint64(p.ID)*37)/170)
	sy := 0.85
	if spot && (b.Facing == "" || b.Facing == "south" || b.Facing == "north") {
		sy = 0.72 // lying flat along the floor, seen from above
	}
	headX, headY := PawnHead(v.Art)
	ps := motion.Pose{Px: headX, Py: headY, Rot: rot, Sx: 0.85, Sy: sy * breath, Cs: cs}
	// lie down over easeFrames: from standing on the tile to the bed pose
	k := motion.SmoothEase(v.Present, since, motion.EaseFrames)
	ox, oy := hx-headX, hy-headY
	if k < 1 {
		stand := motion.Pose{Px: headX, Py: headY, Sx: 1, Sy: 1, Cs: cs}
		ps = motion.LerpPose(stand, ps, k)
		ox, oy = float64(p.X)*(1-k)+ox*k, float64(p.Y)*(1-k)+oy*k
	}
	c.DrawFigure(v, pass, screen, p, ox, oy, tile, ps)
	if p.Status == "alive" {
		defer c.DrawSnoreFor(v, screen, uint64(p.ID), hx, hy, tile)
	}
	if spot || b.Kind == hospitalBed { // a hospital bed's sheet is part of its sprite
		return
	}
	key := "furniture/bed_basic_blanket_" + facingOr(b.Facing)
	if b.Kind == "double bed" {
		key = "furniture/bed_double_blanket_" + facingOr(b.Facing)
	}
	bcs := cs
	if k < 1 {
		bcs[3] *= float32(k) // the blanket settles over the sleeper
	}
	pass.Blood = 0
	c.DrawSpritePose(v, pass, screen, key, c.bedTint(v, b), x0, y0, tile, motion.Pose{Cs: bcs})
}

// bedTint is the per-bed blanket colour (stable by building id).
func (c *Set) bedTint(v *render.View, b game.BuildingView) color.NRGBA {
	cloth := c.Host.ColonyCloths()
	if len(cloth) == 0 {
		return color.NRGBA{}
	}
	return v.Art.Tint("cloth", cloth[art.Mix64(uint64(b.ID))%uint64(len(cloth))])
}

// slungScale is the size of a patient slung over a rescuer's shoulders;
// slungLift raises its middle above the rescuer's back attachment (tiles),
// onto the shoulders. It is drawn behind the rescuer (DrawPawnFigure), so
// the rescuer's head stays in front of it.
const (
	slungScale = 0.8
	slungLift  = 0.14
)

// CarryPoses are a patient's pose lying where it was picked up and
// slung across the rescuer's shoulders: its middle over the rescuer's
// back attachment (body metadata), a little above it.
func CarryPoses(c *art.Catalog, f art.Facing) (ground, slung motion.Pose) {
	side := 1.0
	if f == art.FaceWest {
		side = -1
	}
	ground = LyingPose(c, side)
	_, hy := PawnHead(c)
	backY := hy + 0.5*(PawnFeetY-hy) // between the shoulders when the art has no back point
	if _, y, ok := BodyAttach(c.Sprite("pawn/body_south"), "back"); ok {
		backY = y
	}
	shift := (PawnFeetY - hy) * lyingCentreK * slungScale
	slung = motion.Pose{Px: PawnFeetX, Py: PawnFeetY, Rot: math.Pi / 2, Sx: slungScale, Sy: slungScale,
		Dx: -shift, Dy: backY - PawnFeetY - slungLift}
	return ground, slung
}

// drawCarried draws a patient slung across its rescuer's shoulders,
// lifted from the ground over easeFrames.
func (c *Set) drawCarried(v *render.View, pass sprite.Pass, screen *ebiten.Image, q game.PawnView, f art.Facing, wx, wy, tile float64) {
	ground, slung := CarryPoses(v.Art, f)
	var k float64
	if pass.Cast == nil {
		k = c.CarryEase(q.ID, v.Present) // the first sighting starts the lift
	} else if born, ok := c.CarryBorn[q.ID]; ok {
		k = motion.SmoothEase(v.Present, born, motion.EaseFrames) // shadow pass: read only
	}
	c.DrawFigure(v, pass, screen, q, wx, wy, tile, motion.LerpPose(ground, slung, k))
}

// DrawSnoreFor floats small "z"s up and away from a sleeper's head (a
// colonist or an animal).
func (c *Set) DrawSnoreFor(v *render.View, screen *ebiten.Image, id uint64, hx, hy, tile float64) {
	const period = 75
	for i := 0; i < 3; i++ {
		t := float64((v.Present+id*29+uint64(i*period/3))%period) / period
		size := tile * (0.07 + 0.06*t)
		x := hx + 0.18 + 0.28*t + 0.04*math.Sin(t*6)
		y := hy - 0.25 - 0.55*t
		sx, sy := v.WorldToScreenF(x, y)
		clr := Fade(color.RGBA{238, 240, 250, 255}, 0.9*math.Sin(math.Pi*t))
		w := float32(size)
		lw := float32(math.Max(1.5, tile*0.022))
		vector.StrokeLine(screen, sx, sy, sx+w, sy, lw, clr, true)
		vector.StrokeLine(screen, sx+w, sy, sx, sy+w, lw, clr, true)
		vector.StrokeLine(screen, sx, sy+w, sx+w, sy+w, lw, clr, true)
	}
}

// hospitalBed is the hospital bed's building kind (its sheet is part of
// its sprite, so no blanket is drawn over a patient).
const hospitalBed = "hospital bed"

// facingOr is a building's facing, south when unset.
func facingOr(f string) string {
	if f == "" {
		return "south"
	}
	return f
}
