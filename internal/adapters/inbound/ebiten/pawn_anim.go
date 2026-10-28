package ebiten

import (
	"image/color"
	"math"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
)

// Pawn figure animation (ART §27): one standing sprite per facing moved
// as a whole: gait bob, sway and lean, idle breathing, per-action
// transforms towards the worked tile, recoil/lunge/hit reactions,
// tipping over when downed, lying in bed under the blanket, and being
// carried. Facing west draws the east sprites mirrored.

const (
	// fallFrames is how long (present frames) a figure takes to tip over.
	fallFrames = 16
	// buckleFrames is the squash before a downed or dead figure tips over.
	buckleFrames = 16
	deadCSR      = 0.62
)

var deadCS = colorScale{deadCSR, 0.6, 0.64, 1}

// pawnFeetX, pawnFeetY is where a standing pawn's feet meet the ground,
// in tiles from its tile's top-left: the rig's feet (families/pawn.py CX,
// CY). The ground plane is drawn without shear or lean in every camera
// (contracts.py §5), so camera changes never move this point; everything
// above it is read from the body metadata.
const pawnFeetX, pawnFeetY = 0.5, 0.52

// lyingCentreK is where a lying figure's middle is, as a fraction of the
// way from its feet to its head centre (the head's top rises above it).
const lyingCentreK = 0.65

// lyingDrop lowers a lying figure (tiles): rotated about its feet it
// would lie on the feet's line, a little above the tile's middle.
const lyingDrop = 0.06

// drawnFacing is the facing whose sprites draw f: west draws east.
func drawnFacing(f facing) facing {
	if f == faceWest {
		return faceEast
	}
	return f
}

// facingKey is the art facing ("south", "east", "north") that draws f and
// whether it is mirrored: pawns are rendered south, east and north only;
// west is the east art flipped about the figure's pivot (ART §10).
func facingKey(f facing) (key string, mirror bool) {
	return drawnFacing(f).String(), f == faceWest
}

// mirrored flips a pose left-right about its pivot.
func mirrored(ps pose) pose {
	kx, ky := ps.scale()
	ps.sx, ps.sy = -kx, ky
	return ps
}

// standPose is the untransformed pose of an upright pawn.
func standPose() pose { return pose{px: pawnFeetX, py: pawnFeetY} }

// bodyAttach is attachment name of a body sprite in tiles from its tile's
// top-left (canvas px relative to origin_px, over artTilePx).
func bodyAttach(sp *artSprite, name string) (x, y float64, ok bool) {
	if sp == nil {
		return 0, 0, false
	}
	a, ok := sp.Meta.Attach[name]
	if !ok {
		return 0, 0, false
	}
	return (a[0] - sp.Meta.Origin[0]) / artTilePx, (a[1] - sp.Meta.Origin[1]) / artTilePx, true
}

// pawnHead is a standing figure's head centre in tiles from its tile's
// top-left, from pawn/body_south's head attachment (a stand-in without
// art).
func pawnHead(c *artCatalog) (x, y float64) {
	if x, y, ok := bodyAttach(c.Sprite("pawn/body_south"), "head"); ok {
		return x, y
	}
	return pawnFeetX, -0.24
}

// lyingPose is a figure lying on the ground on its side (side +1: head to
// the east), its middle over the feet's tile.
func lyingPose(c *artCatalog, side float64) pose {
	_, hy := pawnHead(c)
	shift := (pawnFeetY - hy) * lyingCentreK
	return pose{px: pawnFeetX, py: pawnFeetY, rot: side * math.Pi / 2, dx: -side * shift, dy: lyingDrop}
}

// poseFacing is the facing to draw: the motion's smoothed facing, turned
// to the worked tile while working in place and to the blocker while
// stuck behind one.
func (r *Renderer) poseFacing(p game.PawnView, m moveState) facing {
	f := m.Facing
	if m.Blend >= minBlend {
		return f
	}
	if tf, ok := targetFacing(p, r.view.Pawns, r.view.W.Animals, f); ok {
		return tf
	}
	if p.Acting != "" && (p.ActX != p.X || p.ActY != p.Y) {
		return facingFor(p.ActX-p.X, p.ActY-p.Y, f)
	}
	return blockedFacing(p, r.view.Pawns, r.view.W.Animals, f)
}

// actionPose is one action layer's contribution to an upright pawn (ART
// §27.2): what it holds and how, and a transform added to the standing
// pose. Layers are tried in priority order by standingPose: hit reaction,
// combat, carrying, work.
type actionPose struct {
	ok     bool
	held   heldItem // replaces the default held item when held.set
	dx, dy float64  // added offset, tiles
	rot    float64  // added rotation, radians
	squash float64  // vertical squash 0..1 (bending, crouching)
	flash  float32  // hit flash 0..1
}

// targetFacing turns a pawn that is aiming or fighting towards its
// target from the first moment, not only once a shot has been fired.
func targetFacing(p game.PawnView, pawns []game.PawnView, animals []game.AnimalView, prev facing) (facing, bool) {
	if p.ActTarget == 0 || (p.Action != game.ActionAim && p.Action != game.ActionMelee) {
		return prev, false
	}
	for _, q := range pawns {
		if q.ID == p.ActTarget {
			return facingFor(q.X-p.X, q.Y-p.Y, prev), true
		}
	}
	for _, an := range animals {
		if an.ID == p.ActTarget {
			return facingFor(an.X-p.X, an.Y-p.Y, prev), true
		}
	}
	return prev, false
}

// standingPose is the live transform for an upright pawn and what it
// holds: walking or breathing, then the first action layer that applies.
func (r *Renderer) standingPose(p game.PawnView, m moveState) (pose, heldItem) {
	ps := walkPose(m.Facing, m)
	if m.Blend < minBlend {
		b := math.Sin(2 * math.Pi * float64(r.present+uint64(p.ID)*37) / 150)
		ps.sy = 1 + 0.012*b
	}
	held := r.idleHeld(p, m)
	for _, layer := range []func(game.PawnView, moveState) actionPose{r.hitReaction, r.combatPose, r.carryPose, r.workPose} {
		a := layer(p, m)
		if !a.ok {
			continue
		}
		if a.held.set {
			held = a.held
		}
		ps.dx += a.dx
		ps.dy += a.dy
		ps.rot += a.rot
		if a.squash != 0 {
			_, sy := ps.scale()
			ps.sy = sy * (1 - a.squash)
		}
		if a.flash > ps.flash {
			ps.flash = a.flash
		}
		break
	}
	dx, dy, fl := r.combatOffset(p)
	ps.dx, ps.dy = ps.dx+dx, ps.dy+dy
	if fl > ps.flash {
		ps.flash = fl
	}
	return ps, held
}

// drawPawnArt draws a pawn in the object pass: its figure, then the
// extras that are not part of its shape (hostile marker, draft icon).
// It also does the pass's bookkeeping, which the figure draw never does.
func (r *Renderer) drawPawnArt(screen *ebiten.Image, p game.PawnView, wx, wy, tile float64, sw, sh int) {
	if p.CarriedBy != 0 {
		return // drawn over its rescuer
	}
	r.animState().dropCarry(p.ID)
	fp := r.pawnFigurePoseOf(p)
	r.drawPawnFigure(screen, p, wx, wy, tile, fp)
	if p.Lying {
		return
	}
	ps := fp.ps
	if p.Hostile {
		r.drawHostileMarker(screen, p, wx+ps.dx, wy+ps.dy, pawnMarkerLift(r.art), tile)
	}
	if p.Drafted {
		if img := r.art.Image("ui_icon/draft", color.NRGBA{}); img != nil {
			sx, sy := r.worldToScreenF(wx+0.62+ps.dx, wy-pawnMarkerLift(r.art)+ps.dy)
			worldIcon(screen, img, float64(sx), float64(sy), tile*0.3/uiScale()) // tile is physical
		}
	}
}

// pawnFigurePose is everything a pawn's figure draw needs this frame,
// resolved once (from read-only state) so the object pass and every
// light of the shadow pass draw the same figure.
type pawnFigurePose struct {
	vis  pawnVisual // the presentation state (lying pawns read it)
	face facing     // the facing (west draws the east art mirrored)
	held heldItem   // what it holds
	ps   pose       // unmirrored: drawPawnFigure flips it facing west
}

// pawnFigurePoseOf resolves p's figure pose for this frame. It reads
// renderer state and writes none.
func (r *Renderer) pawnFigurePoseOf(p game.PawnView) pawnFigurePose {
	fp := pawnFigurePose{vis: r.last[p.ID]}
	if p.Lying {
		return fp // drawLyingPawn poses it
	}
	m := r.pawnMotion(p.ID)
	m.Facing = r.poseFacing(p, m)
	fp.ps, fp.held = r.standingPose(p, m)
	fp.face = m.Facing
	return fp
}

// figureLying reports a pawn lying on the bare ground (downed, dead or
// asleep there), whose silhouette lies flat; one in bed casts none.
func figureLying(p game.PawnView) bool { return p.Lying && !p.InBed }

// drawPawnFigure draws only the pawn's figure: body, apparel, hair, the
// held item, and what it carries (a load or a
// rescued patient), in pose and facing. It writes no renderer state and
// draws no labels, bars, rings or flashes of its own, so the shadow pass
// (r.cast set) calls it to draw the pawn's silhouette.
func (r *Renderer) drawPawnFigure(dst *ebiten.Image, p game.PawnView, wx, wy, tile float64, fp pawnFigurePose) {
	if p.Lying {
		r.drawLyingPawn(dst, p, fp.vis, wx, wy, tile)
		return
	}
	ps, item := fp.ps, fp.held
	if r.cast != nil {
		ps.flash = 0 // a light effect, not part of the shape
	}
	rig := r.rigOf(p, fp.face)
	if rig.mirror {
		ps = mirrored(ps)
	}
	// a rescued patient lies across the shoulders, behind the rescuer's
	// head; facing north the back, and so the patient, faces the viewer
	carriedBehind := rig.face != faceNorth
	if carriedBehind {
		r.drawCarriedBy(dst, p, fp, wx, wy, tile)
	}
	r.drawHeld(dst, rig, item, wx, wy, tile, ps, true) // first: what the body hides (facing north, or aiming north)
	r.drawLayers(dst, p, rig.layers, wx, wy, tile, ps)
	r.drawHeld(dst, rig, item, wx, wy, tile, ps, false) // the parts in front
	if !carriedBehind {
		r.drawCarriedBy(dst, p, fp, wx, wy, tile)
	}
}

// drawCarriedBy draws the patients rescuer p carries.
func (r *Renderer) drawCarriedBy(dst *ebiten.Image, p game.PawnView, fp pawnFigurePose, wx, wy, tile float64) {
	for _, q := range r.view.Pawns {
		if q.CarriedBy != 0 && q.CarriedBy == p.ID {
			r.drawCarried(dst, q, fp.face, wx+fp.ps.dx, wy+fp.ps.dy, tile)
		}
	}
}

// drawLayers draws a figure's layers in one pose: body and apparel take
// the pawn's blood, hair and beard stay clean.
func (r *Renderer) drawLayers(dst *ebiten.Image, p game.PawnView, layers []portraitLayer, wx, wy, tile float64, ps pose) {
	blood := r.pawnBlood(p) // body and apparel only (blood_draw.go)
	for _, ly := range layers {
		r.figureBlood = layerBlood(ly.key, blood)
		if strings.HasPrefix(ly.key, "pawn/beard_") {
			r.figureBlood = 0
		}
		r.drawSpritePose(dst, ly.key, ly.tint, wx, wy, tile, ps)
	}
	r.figureBlood = 0
}

// drawFigureIn draws a pawn's layers facing f (west mirrored) in pose ps.
func (r *Renderer) drawFigureIn(screen *ebiten.Image, p game.PawnView, f facing, wx, wy, tile float64, ps pose) {
	fs, mirror := facingKey(f)
	if mirror {
		ps = mirrored(ps)
	}
	r.drawLayers(screen, p, r.pawnFigureLayers(p, fs), wx, wy, tile, ps)
}

// drawFigure draws the figure facing south with a pose (lying, in bed,
// carried).
func (r *Renderer) drawFigure(screen *ebiten.Image, p game.PawnView, wx, wy, tile float64, ps pose) {
	r.drawFigureIn(screen, p, faceSouth, wx, wy, tile, ps)
}

// drawLyingPawn: in a bed under the blanket, or on the ground (fallPose:
// tipping over away from the last hit, then lying there).
func (r *Renderer) drawLyingPawn(screen *ebiten.Image, p game.PawnView, vis pawnVisual, wx, wy, tile float64) {
	cs := noCS
	if p.Status == "dead" {
		cs = deadCS
	}
	if p.InBed {
		if r.cast != nil {
			return // the bed casts the shadow
		}
		if b, ok := bedUnder(r.view.W, p.X, p.Y); ok {
			r.drawInBed(screen, p, b, vis.lyingSince, tile, cs)
			return
		}
	}
	ps := r.fallPose(p, vis)
	ps.cs = cs
	if r.cast != nil {
		ps.flash = 0
	}
	r.drawFigure(screen, p, wx, wy, tile, ps)
}

// bedUnder is the sleeping furniture covering tile (x, y).
func bedUnder(w game.WorldView, x, y int) (game.BuildingView, bool) {
	for _, b := range w.Buildings {
		if b.Kind != "bed" && b.Kind != "double bed" && b.Kind != "sleeping spot" && b.Kind != hospitalBedKind {
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
func (r *Renderer) drawInBed(screen *ebiten.Image, p game.PawnView, b game.BuildingView, since uint64, tile float64, cs colorScale) {
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
	breath := 1 + 0.025*math.Sin(2*math.Pi*float64(r.present+uint64(p.ID)*37)/170)
	sy := 0.85
	if spot && (b.Facing == "" || b.Facing == "south" || b.Facing == "north") {
		sy = 0.72 // lying flat along the floor, seen from above
	}
	headX, headY := pawnHead(r.art)
	ps := pose{px: headX, py: headY, rot: rot, sx: 0.85, sy: sy * breath, cs: cs}
	// lie down over easeFrames: from standing on the tile to the bed pose
	k := smoothEase(r.present, since, easeFrames)
	ox, oy := hx-headX, hy-headY
	if k < 1 {
		stand := pose{px: headX, py: headY, sx: 1, sy: 1, cs: cs}
		ps = lerpPose(stand, ps, k)
		ox, oy = float64(p.X)*(1-k)+ox*k, float64(p.Y)*(1-k)+oy*k
	}
	r.drawFigure(screen, p, ox, oy, tile, ps)
	if p.Status == "alive" {
		defer r.drawSnore(screen, p, hx, hy, tile)
	}
	if spot || b.Kind == hospitalBedKind { // a hospital bed's sheet is part of its sprite
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
	r.drawSpritePose(screen, key, r.bedTint(b), x0, y0, tile, pose{cs: bcs})
}

// bedTint is the per-bed blanket colour (stable by building id).
func (r *Renderer) bedTint(b game.BuildingView) color.NRGBA {
	cloth := r.colonyCloths()
	if len(cloth) == 0 {
		return color.NRGBA{}
	}
	return r.art.Tint("cloth", cloth[mix64(uint64(b.ID))%uint64(len(cloth))])
}

// slungScale is the size of a patient slung over a rescuer's shoulders;
// slungLift raises its middle above the rescuer's back attachment (tiles),
// onto the shoulders. It is drawn behind the rescuer (drawPawnFigure), so
// the rescuer's head stays in front of it.
const (
	slungScale = 0.8
	slungLift  = 0.14
)

// carryPoses are a patient's pose lying where it was picked up and
// slung across the rescuer's shoulders: its middle over the rescuer's
// back attachment (body metadata), a little above it.
func carryPoses(c *artCatalog, f facing) (ground, slung pose) {
	side := 1.0
	if f == faceWest {
		side = -1
	}
	ground = lyingPose(c, side)
	_, hy := pawnHead(c)
	backY := hy + 0.5*(pawnFeetY-hy) // between the shoulders when the art has no back point
	if _, y, ok := bodyAttach(c.Sprite("pawn/body_south"), "back"); ok {
		backY = y
	}
	shift := (pawnFeetY - hy) * lyingCentreK * slungScale
	slung = pose{px: pawnFeetX, py: pawnFeetY, rot: math.Pi / 2, sx: slungScale, sy: slungScale,
		dx: -shift, dy: backY - pawnFeetY - slungLift}
	return ground, slung
}

// drawCarried draws a patient slung across its rescuer's shoulders,
// lifted from the ground over easeFrames.
func (r *Renderer) drawCarried(screen *ebiten.Image, q game.PawnView, f facing, wx, wy, tile float64) {
	ground, slung := carryPoses(r.art, f)
	st := r.animState()
	var k float64
	if r.cast == nil {
		k = st.carryEase(q.ID, r.present) // the first sighting starts the lift
	} else if born, ok := st.carryBorn[q.ID]; ok {
		k = smoothEase(r.present, born, easeFrames) // shadow pass: read only
	}
	r.drawFigure(screen, q, wx, wy, tile, lerpPose(ground, slung, k))
}

// drawSnore floats small "z"s up and away from a sleeper's head.
func (r *Renderer) drawSnore(screen *ebiten.Image, p game.PawnView, hx, hy, tile float64) {
	r.drawSnoreFor(screen, uint64(p.ID), hx, hy, tile)
}

// drawSnoreFor is drawSnore for any sleeper (a colonist or an animal).
func (r *Renderer) drawSnoreFor(screen *ebiten.Image, id uint64, hx, hy, tile float64) {
	const period = 75
	for i := 0; i < 3; i++ {
		t := float64((r.present+id*29+uint64(i*period/3))%period) / period
		size := tile * (0.07 + 0.06*t)
		x := hx + 0.18 + 0.28*t + 0.04*math.Sin(t*6)
		y := hy - 0.25 - 0.55*t
		sx, sy := r.worldToScreenF(x, y)
		c := fade(color.RGBA{238, 240, 250, 255}, 0.9*math.Sin(math.Pi*t))
		w := float32(size)
		lw := float32(math.Max(1.5, tile*0.022))
		vector.StrokeLine(screen, sx, sy, sx+w, sy, lw, c, true)
		vector.StrokeLine(screen, sx+w, sy, sx, sy+w, lw, c, true)
		vector.StrokeLine(screen, sx, sy+w, sx+w, sy+w, lw, c, true)
	}
}
